package client

import (
	"context"
	"errors"
	"fmt"
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
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
)

func (c *Client) backoff(ctx context.Context, msg jetstream.Msg) error {
	state := &model.WorkflowState{}
	if err := proto.Unmarshal(msg.Data(), state); err != nil {
		slog.Error("unmarshalling state", "error", err)
		return fmt.Errorf("service task listener: %w", err)
	}

	// Get the metadata including delivery attempts
	meta, err := msg.Metadata()
	if err != nil {
		return fmt.Errorf("fetching message metadata")
	}
	// Get the workflow this task belongs to
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
		//TODO: Retries exceeded
		switch retryBehaviour.DefaultExceeded.Action {
		case model.RetryErrorAction_FailWorkflow:
			if err := c.CancelProcessInstance(ctx, state.ProcessInstanceId); err != nil {
				return fmt.Errorf("cancelling process instance: %w", err)
			}
			goto notifyRetryExceeded
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
			goto notifyRetryExceeded
		case model.RetryErrorAction_PauseWorkflow:
			c.signalFatalErr(ctx, state, slog.Default())
			goto notifyRetryExceeded
		case model.RetryErrorAction_SetVariableValue:
			trackingID := common.TrackingID(state.Id).ID()
			var retVar any
			switch retryBehaviour.DefaultExceeded.VariableType {
			case "int":
				retVar, err = strconv.ParseInt(retryBehaviour.DefaultExceeded.VariableValue, 10, 64)
				if err != nil {
					retVar = err.Error()
				}
			case "float":
				retVar, err = strconv.ParseFloat(retryBehaviour.DefaultExceeded.VariableValue, 64)
				if err != nil {
					retVar = err.Error()
				}
			case "bool":
				retVar, err = strconv.ParseBool(retryBehaviour.DefaultExceeded.VariableValue)
				if err != nil {
					retVar = err.Error()
				}
			default: // string
				retVar = retryBehaviour.DefaultExceeded.VariableValue
			}
			if err := c.completeServiceTask(ctx, trackingID, model.Vars{retryBehaviour.DefaultExceeded.Variable: retVar}, state.State == model.CancellationState_compensating); err != nil {
				return fmt.Errorf("complete service task with error variable: %w", err)
			}
		}
		// Kill the message
		if err := msg.Term(); err != nil {
			return fmt.Errorf("terminate message delivery: %w", err)
		}
	notifyRetryExceeded:
		newMsg := nats.NewMsg(strings.Replace(msg.Subject(), messages.StateJobExecute, ".State.Job.RetryExceeded.", 1))
		newMsg.Data = msg.Data()
		if err := c.con.PublishMsg(newMsg); err != nil {
			return fmt.Errorf("publish retry exceeded notification: %w", err)
		}
		return nil
	}

	var offset time.Duration
	messageTime := time.Unix(0, state.UnixTimeNano)
	initial := uint64(retryBehaviour.InitMilli)
	strategy := retryBehaviour.Strategy
	interval := uint64(retryBehaviour.IntervalMilli)
	ceiling := uint64(retryBehaviour.MaxMilli)
	deliveryCount := meta.NumDelivered

	offset = getOffset(strategy, initial, interval, deliveryCount, ceiling, messageTime)

	if err := msg.NakWithDelay(offset); err != nil {
		return fmt.Errorf("linear backoff: %w", err)
	}
	return nil
}

func getOffset(strategy model.RetryStrategy, initial uint64, interval uint64, deliveryCount uint64, ceiling uint64, messageTime time.Time) time.Duration {
	initial = initial * uint64(time.Millisecond)
	interval = interval * uint64(time.Millisecond)
	ceiling = ceiling * uint64(time.Millisecond)
	if interval == 0 {
		interval = uint64(1 * time.Second)
	}
	if ceiling < interval {
		ceiling = interval
	}
	var offset uint64
	if deliveryCount > 1 {
		if strategy == model.RetryStrategy_Linear {
			offset = initial + interval*(deliveryCount-1)
		} else {
			incrementMultiplier := uint64(math.Pow(2, float64(deliveryCount-1)))
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
