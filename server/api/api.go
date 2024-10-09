package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	version2 "github.com/hashicorp/go-version"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	"gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/internal"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/server/server/option"
	"gitlab.com/shar-workflow/shar/server/services/natz"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"reflect"
	"runtime"
	"sync"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

// Listener provides the mechanism via which api requests are received and responded to
type Listener struct {
	nc                   *natz.NatsConnConfiguration
	panicRecovery        bool
	subs                 *sync.Map
	tr                   trace.Tracer
	receiveApiMiddleware []middleware.Receive
	sendMiddleware       []middleware.Send
	endpoints            *endpoints
}

// NewListener creates a new Listener
func NewListener(nc *natz.NatsConnConfiguration, options *option.ServerOptions, operations workflow.Ops) *Listener {
	auth := newSharAuth(options.ApiAuthorizer, options.ApiAuthenticator, operations)
	endpoints := newEndpoints(operations, auth)

	l := &Listener{
		nc:                   nc,
		panicRecovery:        options.PanicRecovery,
		subs:                 &sync.Map{},
		tr:                   otel.GetTracerProvider().Tracer("shar", trace.WithInstrumentationVersion(version.Version)),
		receiveApiMiddleware: []middleware.Receive{telemetry.CtxWithTraceParentFromNatsMsgMiddleware(), telemetry.NatsMsgToCtxWithSpanMiddleware()},
		sendMiddleware:       []middleware.Send{telemetry.CtxSpanToNatsMsgMiddleware()},
		endpoints:            endpoints,
	}
	return l
}

var shutdownOnce sync.Once

// Shutdown gracefully shuts down the SHAR API server
func (s *Listener) Shutdown() {
	slog.Info("stopping shar api listener")
	shutdownOnce.Do(func() {
		s.subs.Range(func(key, _ any) bool {
			sub := key.(*nats.Subscription)
			if err := sub.Drain(); err != nil {
				slog.Error("drain subscription for "+sub.Subject, "error", err)
				return false
			}
			return true
		})
		slog.Info("shar api listener stopped")
	})
}

// StartListening starts the SHAR API server listening to incoming requests
func (s *Listener) StartListening() error {

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIStoreWorkflow, s.receiveApiMiddleware, &model.StoreWorkflowRequest{}, s.endpoints.storeWorkflow); err != nil {
		return fmt.Errorf("APIStoreWorkflow: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APICancelProcessInstance, s.receiveApiMiddleware, &model.CancelProcessInstanceRequest{}, s.endpoints.cancelProcessInstance); err != nil {
		return fmt.Errorf("APICancelProcessInstance: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APILaunchProcess, s.receiveApiMiddleware, &model.LaunchWorkflowRequest{}, s.endpoints.launchProcess); err != nil {
		return fmt.Errorf("APILaunchProcess: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APISendMessage, s.receiveApiMiddleware, &model.SendMessageRequest{}, s.endpoints.sendMessage); err != nil {
		return fmt.Errorf("APISendMessage: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APICompleteManualTask, s.receiveApiMiddleware, &model.CompleteManualTaskRequest{}, s.endpoints.completeManualTask); err != nil {
		return fmt.Errorf("APICompleteManualTask: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APICompleteServiceTask, s.receiveApiMiddleware, &model.CompleteServiceTaskRequest{}, s.endpoints.completeServiceTask); err != nil {
		return fmt.Errorf("APICompleteServiceTask: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APICompleteUserTask, s.receiveApiMiddleware, &model.CompleteUserTaskRequest{}, s.endpoints.completeUserTask); err != nil {
		return fmt.Errorf("APICompleteUserTask: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetUserTask, s.receiveApiMiddleware, &model.GetUserTaskRequest{}, s.endpoints.getUserTask); err != nil {
		return fmt.Errorf("APIGetUserTask: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetJob, s.receiveApiMiddleware, &model.GetJobRequest{}, s.endpoints.getJob); err != nil {
		return fmt.Errorf("APIGetJob: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIHandleWorkflowError, s.receiveApiMiddleware, &model.HandleWorkflowErrorRequest{}, s.endpoints.handleWorkflowError); err != nil {
		return fmt.Errorf("APIHandleWorkflowError: %w", err)
	}
	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIHandleWorkflowFatalError, s.receiveApiMiddleware, &model.HandleWorkflowFatalErrorRequest{}, s.endpoints.handleWorkflowFatalError); err != nil {
		return fmt.Errorf("APIHandleWorkflowFatalError: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APICompleteSendMessageTask, s.receiveApiMiddleware, &model.CompleteSendMessageRequest{}, s.endpoints.completeSendMessageTask); err != nil {
		return fmt.Errorf("APICompleteSendMessageTask: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetWorkflow, s.receiveApiMiddleware, &model.GetWorkflowRequest{}, s.endpoints.getWorkflow); err != nil {
		return fmt.Errorf("APIGetWorkflow: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetVersionInfo, s.receiveApiMiddleware, &model.GetVersionInfoRequest{}, s.endpoints.versionInfo); err != nil {
		return fmt.Errorf("APIGetProcessHistory: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIRegisterTask, s.receiveApiMiddleware, &model.RegisterTaskRequest{}, s.endpoints.registerTask); err != nil {
		return fmt.Errorf("APIRegisterTask: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetTaskSpec, s.receiveApiMiddleware, &model.GetTaskSpecRequest{}, s.endpoints.getTaskSpec); err != nil {
		return fmt.Errorf("APIGetTaskSpec: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIDeprecateServiceTask, s.receiveApiMiddleware, &model.DeprecateServiceTaskRequest{}, s.endpoints.deprecateServiceTask); err != nil {
		return fmt.Errorf("APIGetTaskSpec: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetTaskSpecUsage, s.receiveApiMiddleware, &model.GetTaskSpecUsageRequest{}, s.endpoints.getTaskSpecUsage); err != nil {
		return fmt.Errorf("APIGetTaskSpec: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIHeartbeat, s.receiveApiMiddleware, &model.HeartbeatRequest{}, s.endpoints.heartbeat); err != nil {
		return fmt.Errorf("APIGetTaskSpec: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APILog, s.receiveApiMiddleware, &model.LogRequest{}, s.endpoints.log); err != nil {
		return fmt.Errorf("APIGetTaskSpec: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIResolveWorkflow, s.receiveApiMiddleware, &model.ResolveWorkflowRequest{}, s.endpoints.resolveWorkflow); err != nil {
		return fmt.Errorf("APIResolveWorkflow: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIListExecutionProcesses, s.receiveApiMiddleware, &model.ListExecutionProcessesRequest{}, s.endpoints.listExecutionProcesses); err != nil {
		return fmt.Errorf("APIListExecutionProcesses: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIListTaskSpecUIDs, s.receiveApiMiddleware, &model.ListTaskSpecUIDsRequest{}, s.endpoints.listTaskSpecUIDs); err != nil {
		return fmt.Errorf("APIListTaskSpecUIDs: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetTaskSpecVersions, s.receiveApiMiddleware, &model.GetTaskSpecVersionsRequest{}, s.endpoints.getTaskSpecVersions); err != nil {
		return fmt.Errorf("APIGetTaskSpecVersions: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetCompensationInputVariables, s.receiveApiMiddleware, &model.GetCompensationInputVariablesRequest{}, s.endpoints.getCompensationInputVariables); err != nil {
		return fmt.Errorf("APIListExecutableProcess: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetCompensationOutputVariables, s.receiveApiMiddleware, &model.GetCompensationOutputVariablesRequest{}, s.endpoints.getCompensationOutputVariables); err != nil {
		return fmt.Errorf("APIListExecutableProcess: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIListUserTaskIDs, s.receiveApiMiddleware, &model.ListUserTasksRequest{}, s.endpoints.listUserTaskIDs); err != nil {
		return fmt.Errorf("APIListUserTaskIDs: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIRetry, s.receiveApiMiddleware, &model.RetryActivityRequest{}, s.endpoints.retryActivity); err != nil {
		return fmt.Errorf("APIRetry: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetProcessInstanceHeaders, s.receiveApiMiddleware, &model.GetProcessHeadersRequest{}, s.endpoints.getProcessHeaders); err != nil {
		return fmt.Errorf("APIGetProcessInstanceHeaders: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIDisableWorkflow, s.receiveApiMiddleware, &model.DisableWorkflowRequest{}, s.endpoints.disableWorkflow); err != nil {
		return fmt.Errorf("APIDisableWorkflow: %w", err)
	}

	if err := listen(s.nc.Conn, s.panicRecovery, s.subs, messages.APIEnableWorkflow, s.receiveApiMiddleware, &model.EnableWorkflowRequest{}, s.endpoints.enableWorkflow); err != nil {
		return fmt.Errorf("APIEnableWorkflow: %w", err)
	}

	/* COMPLETED */
	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetWorkflowVersions, s.receiveApiMiddleware, &model.GetWorkflowVersionsRequest{}, s.endpoints.getWorkflowVersions); err != nil {
		return fmt.Errorf("APIGetWorkflowVersions: %w", err)
	}

	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetProcessInstanceStatus, s.receiveApiMiddleware, &model.GetProcessInstanceStatusRequest{}, s.endpoints.getProcessInstanceStatus); err != nil {
		return fmt.Errorf("APIGetProcessInstanceStatus: %w", err)
	}

	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIListWorkflows, s.receiveApiMiddleware, &model.ListWorkflowsRequest{}, s.endpoints.listWorkflows); err != nil {
		return fmt.Errorf("APIListWorkflows: %w", err)
	}

	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIListExecution, s.receiveApiMiddleware, &model.ListExecutionRequest{}, s.endpoints.listExecution); err != nil {
		return fmt.Errorf("APIListExecution: %w", err)
	}

	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetProcessHistory, s.receiveApiMiddleware, &model.GetProcessHistoryRequest{}, s.endpoints.getProcessHistory); err != nil {
		return fmt.Errorf("APIGetProcessHistory: %w", err)
	}

	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIListExecutableProcess, s.receiveApiMiddleware, &model.ListExecutableProcessesRequest{}, s.endpoints.listExecutableProcesses); err != nil {
		return fmt.Errorf("APIListExecutableProcess: %w", err)
	}

	if err := ListenReturnStream(s.nc.Conn, s.panicRecovery, s.subs, messages.APIGetFatalErrors, s.receiveApiMiddleware, &model.GetFatalErrorRequest{}, s.endpoints.getFatalErrors); err != nil {
		return fmt.Errorf("APIGetFatalErrors: %w", err)
	}

	slog.Info("shar api listener started")
	return nil
}

// ListenReturnStream is a function that sets up a NATS subscription to handle streaming reply messages.
// It executes the provided function to process the request and send the response messages.
// The function runs in a separate goroutine that continuously listens for return messages and error messages, and publishes them to the reply inbox.
// the function exits when an error or cancellation occurs.
func ListenReturnStream[T proto.Message, U proto.Message](con common.NatsConn, panicRecovery bool, subList *sync.Map, subject string, receiveAPIMiddleware []middleware.Receive, req T, fn func(ctx context.Context, req T, res chan<- U, errs chan<- error)) error {
	sub, err := common.StreamingReplyServer(con, subject, func(msg *nats.Msg, retMsgs chan *nats.Msg, retErrs chan error) {
		if msg.Subject != messages.APIGetVersionInfo {
			callerVersion, err := version2.NewVersion(msg.Header.Get(header.NatsCompatHeader))
			if err != nil {
				retErrs <- errors.New(string(apiError(codes.PermissionDenied, "version: client version invalid")))
				return
			} else {
				if ok, ver := upgrader.IsCompatible(callerVersion); !ok {
					retErrs <- errors.New(string(apiError(codes.PermissionDenied, "version: client version >= "+ver.String()+" required")))
					return
				}
			}
		}
		ctx, log := logx.NatsMessageLoggingEntrypoint(context.Background(), "server", msg.Header)
		ctx = subj.SetNS(ctx, msg.Header.Get(header.SharNamespace))
		for _, i := range receiveAPIMiddleware {
			var err error
			ctx, err = i(ctx, common.NewNatsMsgWrapper(msg))
			if err != nil {
				retErrs <- errors.New(string(apiError(codes.Internal, fmt.Sprintf("receive middleware %s: %s", reflect.TypeOf(i), err.Error()))))
				return
			}
		}
		ctx, span := telemetry.StartApiSpan(ctx, "shar", msg.Subject)
		if err := callAPIReturnStream(ctx, panicRecovery, req, msg, retMsgs, retErrs, fn); err != nil {
			log.Error("API call for "+subject+" failed", "error", err)
		}
		span.End()
	})
	if err != nil {
		return fmt.Errorf("streaming subscribe to %s: %w", subject, err)
	}
	subList.Store(sub, struct{}{})
	return nil
}

func callAPIReturnStream[T proto.Message, U proto.Message](ctx context.Context, panicRecovery bool, container T, msg *nats.Msg, res chan<- *nats.Msg, errs chan<- error, fn func(ctx context.Context, req T, res chan<- U, errs chan<- error)) error {
	if panicRecovery {
		defer recoverAPIpanic(msg)
	}
	if err := proto.Unmarshal(msg.Data, container); err != nil {
		errorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal message data during callAPI: %w", err)
	}
	ctx, err := header.FromMsgHeaderToCtx(ctx, msg.Header)
	if err != nil {
		return errors2.ErrWorkflowFatal{Err: fmt.Errorf("decode context value from NATS message for API call: %w", err)}
	}
	ctx = context.WithValue(ctx, ctxkey.APIFunc, msg.Subject)
	iRes := make(chan U)
	iErrs := make(chan error, 1)
	go func() {
		fn(ctx, container, iRes, iErrs)
		close(iErrs)
	}()
	for {
		select {
		case e := <-iErrs:
			if e != nil {
				svrErr := errors.New(string(apiError(codes.Internal, e.Error())))
				errs <- svrErr
			}
			return e
		case r := <-iRes:
			b, err := proto.Marshal(r)
			if err != nil {
				//TODO: DEAL WITH WORKFLOW FATAL
				return fmt.Errorf("marshal streaming result: %w", err)
			}
			retMsg := nats.NewMsg("return")
			retMsg.Data = b
			res <- retMsg
		}
	}
}

func listen[T proto.Message, U proto.Message](con common.NatsConn, panicRecovery bool, subList *sync.Map, subject string, receiveApiMiddleware []middleware.Receive, req T, fn func(ctx context.Context, req T) (U, error)) error {
	sub, err := con.QueueSubscribe(subject, subject, func(msg *nats.Msg) {
		if msg.Subject != messages.APIGetVersionInfo {
			callerVersion, err := version2.NewVersion(msg.Header.Get(header.NatsCompatHeader))
			if err != nil {
				errorResponse(msg, codes.PermissionDenied, "version: client version invalid")
				return
			} else {
				if ok, ver := upgrader.IsCompatible(callerVersion); !ok {
					errorResponse(msg, codes.PermissionDenied, "version: client version >= "+ver.String()+" required")
					return
				}
			}
		}
		ctx, log := logx.NatsMessageLoggingEntrypoint(context.Background(), "server", msg.Header)
		ctx = subj.SetNS(ctx, msg.Header.Get(header.SharNamespace))
		for _, i := range receiveApiMiddleware {
			var err error
			ctx, err = i(ctx, common.NewNatsMsgWrapper(msg))
			if err != nil {
				errorResponse(msg, codes.Internal, fmt.Sprintf("receive middleware %s: %s", reflect.TypeOf(i), err.Error()))
				return
			}
		}
		ctx, span := telemetry.StartApiSpan(ctx, "shar", msg.Subject)
		if err := callAPI(ctx, panicRecovery, req, msg, fn); err != nil {
			log.Error("API call for "+subject+" failed", "error", err)
		}
		span.End()
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", subject, err)
	}
	subList.Store(sub, struct{}{})
	return nil
}

func callAPI[T proto.Message, U proto.Message](ctx context.Context, panicRecovery bool, container T, msg *nats.Msg, fn func(ctx context.Context, req T) (U, error)) error {
	if panicRecovery {
		defer recoverAPIpanic(msg)
	}
	if err := proto.Unmarshal(msg.Data, container); err != nil {
		errorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal message data during callAPI: %w", err)
	}
	ctx, err := header.FromMsgHeaderToCtx(ctx, msg.Header)
	if err != nil {
		return errors2.ErrWorkflowFatal{Err: fmt.Errorf("decode context value from NATS message for API call: %w", err)}
	}
	ctx = context.WithValue(ctx, ctxkey.APIFunc, msg.Subject)
	resMsg, err := fn(ctx, container)
	if err != nil {
		c := codes.Unknown
		if errors2.IsWorkflowFatal(err) {
			c = codes.Internal
		}
		errorResponse(msg, c, err.Error())
		return fmt.Errorf("API call: %w", err)
	}
	res, err := proto.Marshal(resMsg)
	if err != nil {
		errorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal API response: %w", err)
	}
	if err := msg.Respond(res); err != nil {
		errorResponse(msg, codes.FailedPrecondition, err.Error())
		return fmt.Errorf("API response: %w", err)
	}
	return nil
}

func recoverAPIpanic(msg *nats.Msg) {
	if r := recover(); r != nil {
		buf := make([]byte, 16384)
		runtime.Stack(buf, false)
		stack := buf[:bytes.IndexByte(buf, 0)]
		fmt.Println(stack)
		errorResponse(msg, codes.Internal, r)
		slog.Info("recovered from ", r)
	}
}

func errorResponse(m *nats.Msg, code codes.Code, msg any) {
	if err := m.Respond(apiError(code, msg)); err != nil {
		slog.Error("send error response: "+string(apiError(codes.Internal, msg)), "error", err)
	}
}

func apiError(code codes.Code, msg any) []byte {
	err := fmt.Sprintf("%s%d%s%+v", internal.ErrorPrefix, code, internal.ErrorSeparator, msg)
	return []byte(err)
}
