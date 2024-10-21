package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/workflow"
	api2 "gitlab.com/shar-workflow/shar/internal/client/api"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
)

func (c *Client) backoff(ctx context.Context, msg jetstream.Msg) error {
	state := &model.WorkflowState{}
	if err := proto.Unmarshal(msg.Data(), state); err != nil {
		slog.Error("unmarshalling state", "error", err)
		return fmt.Errorf("service task listener: %w", err)
	}

	// get the metadata including delivery attempts
	meta, err := msg.Metadata()
	if err != nil {
		return fmt.Errorf("fetching message metadata")
	}
	// get the workflow this task belongs to
	wf, err := c.GetWorkflow(ctx, state.WorkflowId)
	if err != nil {
		if errors.Is(err, errors2.ErrWorkflowNotFound) {
			slog.ErrorContext(ctx, "terminated a task without a workflow", "error", err)
			if err2 := msg.Term(); err2 != nil {
				slog.ErrorContext(ctx, "failed to terminate task without a workflow", "error", err)
			}
		}
		return fmt.Errorf("getting workflow: %w", err)
	}
	// And the service task element
	elem := common.ElementTable(wf)[state.ElementId]

	// and extract the retry behaviour
	retryBehaviour := elem.RetryBehaviour

	// Is this the last time to fail?
	if meta.NumDelivered >= uint64(retryBehaviour.Number) {
		// Retries exceeded, defer to ensure first termination and then notification of retries
		//  being exceeded ALWAYS happens, no matter the case.
		defer func() {
			// Notify retries exceeded
			if err = notifyRetryExceeded(c, msg); err != nil {
				logx.Err(ctx, "notify retry exceedded err", err)
			}
		}()
		defer func() {
			// Kill the message
			if err := msg.Term(); err != nil {
				logx.Err(ctx, "message termination error", err)
			}
		}()
		switch retryBehaviour.DefaultExceeded.Action {
		case model.RetryErrorAction_FailWorkflow:
			if err := c.CancelProcessInstance(ctx, state.ProcessInstanceId); err != nil {
				return fmt.Errorf("cancelling process instance: %w", err)
			}
		case model.RetryErrorAction_ThrowWorkflowError:
			trackingID := common.TrackingID(state.Id).ID()
			res := &model.HandleWorkflowErrorResponse{}
			req := &model.HandleWorkflowErrorRequest{TrackingId: trackingID, ErrorCode: retryBehaviour.DefaultExceeded.ErrorCode, Vars: []byte{}}
			if err2 := api2.Call(ctx, c.txCon, messages.APIHandleWorkflowError, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err2 != nil {
				// TODO: This isn't right.  If this call fails it assumes it is handled!
				reterr := fmt.Errorf("retry workflow error handle: %w", err2)
				return logx.Err(ctx, "call workflow error handler", reterr, slog.Any("workflowError", &workflow.Error{Code: retryBehaviour.DefaultExceeded.ErrorCode, WrappedError: err2}))
			}
			if !res.Handled {
				return fmt.Errorf("handle workflow error with code %s", retryBehaviour.DefaultExceeded.ErrorCode)
			}
		case model.RetryErrorAction_PauseWorkflow:
			c.signalFatalErr(ctx, state, slog.Default())
		case model.RetryErrorAction_SetVariableValue:
			trackingID := common.TrackingID(state.Id).ID()
			retVars := model.NewVars()
			switch retryBehaviour.DefaultExceeded.VariableType {
			case "int":
				retVar, pErr := strconv.ParseInt(retryBehaviour.DefaultExceeded.VariableValue, 10, 64)
				if pErr != nil {
					return fmt.Errorf("parse retry variable as integer: %w", pErr)
				}
				retVars.SetInt64(retryBehaviour.DefaultExceeded.Variable, int64(retVar))
			case "float":
				retVar, pErr := strconv.ParseFloat(retryBehaviour.DefaultExceeded.VariableValue, 64)
				if pErr != nil {
					return fmt.Errorf("parse retry variable as float: %w", pErr)
				}
				retVars.SetFloat64(retryBehaviour.DefaultExceeded.Variable, retVar)

			case "bool":
				retVar, pErr := strconv.ParseBool(retryBehaviour.DefaultExceeded.VariableValue)
				if pErr != nil {
					return fmt.Errorf("parse retry variable as boolean: %w", pErr)
				}
				retVars.SetBool(retryBehaviour.DefaultExceeded.Variable, retVar)
			default: // string
				retVars.SetString(retryBehaviour.DefaultExceeded.Variable, retryBehaviour.DefaultExceeded.VariableValue)
			}

			if err := c.completeServiceTask(ctx, trackingID, retVars, state.State == model.CancellationState_compensating); err != nil {
				return fmt.Errorf("complete service task with error variable: %w", err)
			}
		}
		return nil
	}

	var offset time.Duration
	messageTime := time.Unix(0, state.UnixTimeNano)
	initial := retryBehaviour.InitMilli
	strategy := retryBehaviour.Strategy
	interval := retryBehaviour.IntervalMilli
	ceiling := retryBehaviour.MaxMilli
	deliveryCount := int64(meta.NumDelivered) //nolint:gosec

	offset = getOffset(strategy, initial, interval, deliveryCount, ceiling, messageTime)

	if err := msg.NakWithDelay(offset); err != nil {
		return fmt.Errorf("linear backoff: %w", err)
	}
	return nil
}

func notifyRetryExceeded(c *Client, msg jetstream.Msg) error {
	newMsg := nats.NewMsg(strings.Replace(msg.Subject(), messages.StateJobExecute, ".State.Job.RetryExceeded.", 1))
	newMsg.Data = msg.Data()
	if err := c.con.PublishMsg(newMsg); err != nil {
		return fmt.Errorf("publish retry exceeded notification: %w", err)
	}
	return nil
}

func getOffset(strategy model.RetryStrategy, initial int64, interval int64, deliveryCount int64, ceiling int64, messageTime time.Time) time.Duration {
	initial = initial * int64(time.Millisecond)
	interval = interval * int64(time.Millisecond)
	ceiling = ceiling * int64(time.Millisecond)
	if interval == 0 {
		interval = int64(1 * time.Second)
	}
	if ceiling < interval {
		ceiling = interval
	}
	var offset int64
	if deliveryCount > 1 {
		if strategy == model.RetryStrategy_Linear {
			offset = initial + interval*(deliveryCount-1)
		} else {
			incrementMultiplier := int64(math.Pow(2, float64(deliveryCount-1)))
			offset = initial + (interval * incrementMultiplier)
		}
		if offset > ceiling {
			offset = ceiling
		}
	}
	if time.Since(messageTime) > time.Duration(offset)*time.Millisecond { //nolint:gosec
		offset = 0
	}
	return time.Duration(offset) //nolint:gosec
}
