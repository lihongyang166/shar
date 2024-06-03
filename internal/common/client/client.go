package client

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/client/api"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/workflow"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/vars"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ServiceFnExecutor is the prototype function for executing a service task function.
type ServiceFnExecutor func(ctx context.Context, trackingID string, job *model.WorkflowState, svcFn task.ServiceFn, inVars model.Vars) (model.Vars, error)

// ServiceFnLocator is the prototype function for locating a service task function.
type ServiceFnLocator func(job *model.WorkflowState) (task.ServiceFn, error)

// MessageFnLocator is the prototype function for locating a message task function.
type MessageFnLocator func(job *model.WorkflowState) (task.SenderFn, error)

// ServiceTaskCompleter is the prototype function for completing a service task.
type ServiceTaskCompleter func(ctx context.Context, trackingID string, newVars model.Vars, compensating bool) error

// MessageSendCompleter the prototype function for completing a message task.
type MessageSendCompleter func(ctx context.Context, trackingID string, newVars model.Vars) error

// MessageFnExecutor is the prototype function for executing a message task function.
type MessageFnExecutor func(ctx context.Context, trackingID string, job *model.WorkflowState, fn task.SenderFn, inVars model.Vars) error

// WorkflowErrorHandler is the prototype function for handling a workflow error.
type WorkflowErrorHandler func(ctx context.Context, ns string, trackingID string, errorCode string, binVars []byte) (*model.HandleWorkflowErrorResponse, error)

// ProcessInstanceErrorCanceller is the prototype function for cancelling a process instance in the event of an error.
type ProcessInstanceErrorCanceller func(ctx context.Context, processInstanceID string, wfe *model.Error) error

// ServiceTaskProcessParams provides a behaviour to the service task processor.
type ServiceTaskProcessParams struct {
	SvcFnExecutor    ServiceFnExecutor
	MsgFnExecutor    MessageFnExecutor
	SvcFnLocator     ServiceFnLocator
	MsgFnLocator     MessageFnLocator
	SvcTaskCompleter ServiceTaskCompleter
	MsgSendCompleter MessageSendCompleter
	WfErrorHandler   WorkflowErrorHandler
	PiErrorCanceller ProcessInstanceErrorCanceller
}

// JobGetter represents the ability to retrieve SHAR Jobs.
type JobGetter interface {
	GetJob(ctx context.Context, id string) (*model.WorkflowState, error)
}

// ReParentSpan re-parents a span in the given context with the span ID obtained from the WorkflowState ID.
// If the span context in the context is valid, it replaces the span ID with the 64-bit representation
// obtained from the WorkflowState ID. Otherwise, it returns the original context.
//
// Parameters:
// - ctx: The context to re-parent the span in.
// - state: The WorkflowState containing the ID to extract the new span ID from.
//
// Returns:
// - The context with the re-parented span ID or the original context if the span context is invalid.
func ReParentSpan(ctx context.Context, state *model.WorkflowState) context.Context {
	sCtx := trace.SpanContextFromContext(ctx)
	if sCtx.IsValid() {
		c := common.KSuidTo64bit(common.TrackingID(state.Id).ID())
		return trace.ContextWithSpanContext(ctx, sCtx.WithSpanID(c))
	}
	return ctx
}

// InternalProcessInstanceId is a constant of type ContextKey used for storing the internal process instance ID in a context.
const InternalProcessInstanceId keys.ContextKey = "__INTERNAL_PIID"

// ClientProcessFn is a function that processes client messages in a Jetstream subscription.
// It increments a counter, checks version compatibility, re-parents a span, and completes service tasks or message tasks.
//
// Parameters:
// - ackTimeout: The duration for waiting to acknowledge the message.
// - counter: A pointer to an atomic.Int64 variable representing a counter.
// - jobGetter: An implementation of the JobGetter interface that retrieves SHAR Jobs.
// - params: An instance of ServiceTaskProcessParams that provides various behaviours to the service task processor.
//
// Returns:
// - A function that processes client messages and returns a boolean and an error.
// The boolean indicates whether the message processing is complete, and the error provides any encountered error.
func ClientProcessFn(ackTimeout time.Duration, counter *atomic.Int64, noRecovery bool, jobGetter JobGetter, params ServiceTaskProcessParams) func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	return func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		counter.Add(1)
		defer func() {
			counter.Add(-1)
		}()
		// Check version compatibility of incoming call.
		sharCompat := msg.Headers().Get(header.NatsCompatHeader)
		if sharCompat != "" {
			sVer, err := version.NewVersion(sharCompat)
			if err != nil {
				return false, fmt.Errorf("compatibility issue: shar server version corrupt %s: %w", sVer, err)
			}

			if compat, ver := upgrader.IsCompatible(sVer); !compat {
				return false, fmt.Errorf("compatibility issue: shar server level %s, client version level: %s: %w", sVer, ver, err)
			}
		}

		// Start a loop keeping this connection alive.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		var fnMx sync.Mutex
		waitCancelSig := make(chan struct{})

		messageNamespace := strings.SplitN(msg.Subject(), ".", 3)[1]

		// Acknowledge until waitCancel is closed
		go func(ctx context.Context) {
			select {
			case <-time.After(ackTimeout / 2):

			case <-waitCancelSig:
				cancel()
				return
			}
			fnMx.Lock()
			select {
			case <-waitCancelSig:
				cancel()
				return
			default:
			}
			if err := msg.InProgress(); err != nil {
				cancel()
				fnMx.Unlock()
				return
			}
			fnMx.Unlock()
		}(ctx)

		ut := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), ut); err != nil {
			log.Error("unmarshalling", "error", err)
			return false, fmt.Errorf("service task listener: %w", err)
		}

		subj.SetNS(ctx, msg.Headers().Get(header.SharNamespace))
		ctx = context.WithValue(ctx, ctxkey.ExecutionID, ut.ExecutionId)
		ctx = context.WithValue(ctx, ctxkey.ProcessInstanceID, ut.ProcessInstanceId)
		ctx = ReParentSpan(ctx, ut)
		ctx, err := header.FromMsgHeaderToCtx(ctx, msg.Headers())
		if err != nil {
			return true, &errors.ErrWorkflowFatal{Err: fmt.Errorf("obtain headers from message: %w", err)}
		}

		switch ut.ElementType {
		case element.ServiceTask:
			trackingID := common.TrackingID(ut.Id).ID()
			job, err := jobGetter.GetJob(ctx, trackingID)
			if err != nil {
				log.Error("get job", "error", err, slog.String("JobId", trackingID))
				return false, fmt.Errorf("get service task job kv: %w", err)
			}

			svcFn, err := params.SvcFnLocator(job)
			if err != nil {
				return false, fmt.Errorf("service task locator: %w", errors.ErrWorkflowFatal{Err: err})
			}
			dv, err := vars.Decode(ctx, job.Vars)
			if err != nil {
				log.Error("decode vars", "error", err, slog.String("fn", *job.Execute))
				return false, fmt.Errorf("decode service task job variables: %w", err)
			}
			newVars, err := func(noRecovery bool) (v model.Vars, e error) {

				if !noRecovery {
					defer func() {
						if r := recover(); r != nil {
							v = model.Vars{}
							e = &errors.ErrWorkflowFatal{Err: fmt.Errorf("call to service task \"%s\" terminated in panic: %w", *ut.Execute, r.(error))}
						}
					}()
				}
				vrs, err := params.SvcFnExecutor(ctx, trackingID, job, svcFn, dv)
				return vrs, err
			}(noRecovery)
			if err != nil {
				var handled bool
				wfe := &workflow.Error{}
				if errors2.As(err, &wfe) {
					v, err := vars.Encode(ctx, newVars)
					if err != nil {
						return true, &errors.ErrWorkflowFatal{Err: fmt.Errorf("encode service task variables: %w", err)}
					}

					res, err := params.WfErrorHandler(ctx, messageNamespace, trackingID, wfe.Code, v)
					if err != nil {
						return true, &errors.ErrWorkflowFatal{Err: fmt.Errorf("handle workflow error: %w", err)}
					}
					fmt.Printf("##### %+v\n", wfe)
					handled = res.Handled
				}
				if !handled {
					log.Warn("execution of service task function", "error", err)
				}
				return wfe.Code != "", err
			}
			err = params.SvcTaskCompleter(ctx, trackingID, newVars, job.State == model.CancellationState_compensating)
			ae := &api.Error{}
			if errors2.As(err, &ae) {
				if codes.Code(ae.Code) == codes.Internal {
					log.Error("complete service task", "error", err)
					e := &model.Error{
						Id:   "",
						Name: ae.Message,
						Code: "client-" + strconv.Itoa(ae.Code),
					}
					if err := params.PiErrorCanceller(ctx, ut.ProcessInstanceId, e); err != nil {
						log.Error("cancel execution in response to fatal error", "error", err)
					}
					return true, nil
				}
			} else if errors.IsWorkflowFatal(err) {
				return true, err
			}
			if err != nil {
				log.Warn("complete service task", "error", err)
				return false, fmt.Errorf("complete service task: %w", err)
			}
			return true, nil

		case element.MessageIntermediateThrowEvent:
			trackingID := common.TrackingID(ut.Id).ID()
			job, err := jobGetter.GetJob(ctx, trackingID)
			if err != nil {
				log.Error("get send message task", "error", err, slog.String("JobId", common.TrackingID(ut.Id).ID()))
				return false, fmt.Errorf("complete send message task: %w", err)
			}

			msgFn, err := params.MsgFnLocator(job)
			if err != nil {
				return false, fmt.Errorf("msgFn locator: %w", errors.ErrWorkflowFatal{Err: err})
			}

			dv, err := vars.Decode(ctx, job.Vars)
			if err != nil {
				log.Error("decode vars", "error", err, slog.String("fn", *job.Execute))
				return false, &errors.ErrWorkflowFatal{Err: fmt.Errorf("decode send message variables: %w", err)}
			}
			ctx = context.WithValue(ctx, ctxkey.TrackingID, trackingID)
			pidCtx := context.WithValue(ctx, InternalProcessInstanceId, job.ProcessInstanceId)
			pidCtx = ReParentSpan(pidCtx, job)
			if err := params.MsgFnExecutor(pidCtx, trackingID, job, msgFn, dv); err != nil {
				log.Warn("nats listener", "error", err)
				return false, err
			}
			if err := params.MsgSendCompleter(ctx, trackingID, make(map[string]any)); errors.IsWorkflowFatal(err) {
				log.Error("a fatal error occurred in message sender "+*job.Execute, "error", err)
			} else if err != nil {
				log.Error("API error", "error", err)
				return false, err
			}
			return true, nil
		}
		return true, nil
	}
}
