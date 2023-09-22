package api

import (
	"context"
	"fmt"
	version2 "github.com/hashicorp/go-version"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/internal"
	"gitlab.com/shar-workflow/shar/server/services/storage"
	"log/slog"
	"sync"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/workflow"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// SharServer provides API endpoints for SHAR
type SharServer struct {
	ns            *storage.Nats
	engine        *workflow.Engine
	subs          *sync.Map
	panicRecovery bool
	apiAuthZFn    authz.APIFunc
	apiAuthNFn    authn.Check
}

// New creates a new instance of the SHAR API server
func New(ns *storage.Nats, panicRecovery bool, apiAuthZFn authz.APIFunc, apiAuthNFn authn.Check) (*SharServer, error) {
	engine, err := workflow.New(ns)
	if err != nil {
		return nil, fmt.Errorf("create SHAR engine instance: %w", err)
	}
	if err := engine.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("start SHAR engine: %w", err)
	}
	return &SharServer{
		apiAuthZFn:    apiAuthZFn,
		apiAuthNFn:    apiAuthNFn,
		ns:            ns,
		engine:        engine,
		panicRecovery: panicRecovery,
		subs:          &sync.Map{},
	}, nil
}

var shutdownOnce sync.Once

// Shutdown gracefully shuts down the SHAR API server and Engine
func (s *SharServer) Shutdown() {
	slog.Info("stopping shar api listener")
	shutdownOnce.Do(func() {
		s.subs.Range(func(key, _ any) bool {
			sub := key.(*nats.Subscription)
			if err := sub.Drain(); err != nil {
				slog.Error("drain subscription for "+sub.Subject, err)
				return false
			}
			return true
		})
		s.engine.Shutdown()
		slog.Info("shar api listener stopped")
	})
}

// Listen starts the SHAR API server listening to incoming requests
func (s *SharServer) Listen() error {
	con := s.ns.Conn()

	if err := listen(con, s.panicRecovery, s.subs, messages.APIStoreWorkflow, &model.Workflow{}, s.storeWorkflow); err != nil {
		return fmt.Errorf("APIStoreWorkflow failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APICancelWorkflowInstance, &model.CancelWorkflowInstanceRequest{}, s.cancelWorkflowInstance); err != nil {
		return fmt.Errorf("APICancelWorkflowInstance failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APILaunchProcess, &model.LaunchWorkflowRequest{}, s.launchWorkflow); err != nil {
		return fmt.Errorf("APILaunchProcess failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIListWorkflows, &emptypb.Empty{}, s.listWorkflows); err != nil {
		return fmt.Errorf("APIListWorkflows failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIListWorkflowInstanceProcesses, &model.ListWorkflowInstanceProcessesRequest{}, s.listWorkflowInstanceProcesses); err != nil {
		return fmt.Errorf("APIListWorkflowInstanceProcesses failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIListWorkflowInstance, &model.ListWorkflowInstanceRequest{}, s.listWorkflowInstance); err != nil {
		return fmt.Errorf("APIListWorkflowInstance failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APISendMessage, &model.SendMessageRequest{}, s.sendMessage); err != nil {
		return fmt.Errorf("APISendMessage failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APICompleteManualTask, &model.CompleteManualTaskRequest{}, s.completeManualTask); err != nil {
		return fmt.Errorf("APICompleteManualTask failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APICompleteServiceTask, &model.CompleteServiceTaskRequest{}, s.completeServiceTask); err != nil {
		return fmt.Errorf("APICompleteServiceTask failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APICompleteUserTask, &model.CompleteUserTaskRequest{}, s.completeUserTask); err != nil {
		return fmt.Errorf("APICompleteUserTask failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIListUserTaskIDs, &model.ListUserTasksRequest{}, s.listUserTaskIDs); err != nil {
		return fmt.Errorf("APIListUserTaskIDs failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetUserTask, &model.GetUserTaskRequest{}, s.getUserTask); err != nil {
		return fmt.Errorf("APIGetUserTask failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIHandleWorkflowError, &model.HandleWorkflowErrorRequest{}, s.handleWorkflowError); err != nil {
		return fmt.Errorf("APIHandleWorkflowError failed: %w", err)
	}
	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetServerInstanceStats, &emptypb.Empty{}, s.getServerInstanceStats); err != nil {
		return fmt.Errorf("APIGetServerInstanceStats failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetServiceTaskRoutingID, &model.GetServiceTaskRoutingIDRequest{}, s.getServiceTaskRoutingID); err != nil {
		return fmt.Errorf("APIGetServiceTaskRoutingID failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APICompleteSendMessageTask, &model.CompleteSendMessageRequest{}, s.completeSendMessageTask); err != nil {
		return fmt.Errorf("APICompleteSendMessageTask failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetWorkflowVersions, &model.GetWorkflowVersionsRequest{}, s.getWorkflowVersions); err != nil {
		return fmt.Errorf("APIGetWorkflowVersions failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetWorkflow, &model.GetWorkflowRequest{}, s.getWorkflow); err != nil {
		return fmt.Errorf("APIGetWorkflow failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetProcessInstanceStatus, &model.GetProcessInstanceStatusRequest{}, s.getProcessInstanceStatus); err != nil {
		return fmt.Errorf("APIGetProcessInstanceStatus failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetProcessHistory, &model.GetProcessHistoryRequest{}, s.getProcessHistory); err != nil {
		return fmt.Errorf("APIGetProcessHistory failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APISpoolWorkflowEvents, &model.SpoolWorkflowEventsRequest{}, s.spoolWorkflowEvents); err != nil {
		return fmt.Errorf("APIGetProcessHistory failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIGetVersionInfo, &model.GetVersionInfoRequest{}, s.versionInfo); err != nil {
		return fmt.Errorf("APIGetProcessHistory failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.APIRegisterTask, &model.RegisterTaskRequest{}, s.registerTask); err != nil {
		return fmt.Errorf("APIRegisterTask failed: %w", err)
	}

	if err := listen(con, s.panicRecovery, s.subs, messages.ApiGetTaskSpec, &model.GetTaskSpecRequest{}, s.getTaskSpec); err != nil {
		return fmt.Errorf("ApiGetTaskSpec failed: %w", err)
	}

	slog.Info("shar api listener started")
	return nil
}

func listen[T proto.Message, U proto.Message](con common.NatsConn, panicRecovery bool, subList *sync.Map, subject string, req T, fn func(ctx context.Context, req T) (U, error)) error {
	sub, err := con.QueueSubscribe(subject, subject, func(msg *nats.Msg) {
		if msg.Subject != messages.APIGetVersionInfo {
			callerVersion, err := version2.NewVersion(msg.Header.Get(header.NatsVersionHeader))
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
		if err := callAPI(ctx, panicRecovery, req, msg, fn); err != nil {
			log.Error("API call for "+subject+" failed", err)
		}
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
		errorResponse(msg, codes.Internal, r)
		slog.Info("recovered from ", r)
	}
}

func errorResponse(m *nats.Msg, code codes.Code, msg any) {
	if err := m.Respond(apiError(code, msg)); err != nil {
		slog.Error("send error response: "+string(apiError(codes.Internal, msg)), err)
	}
}

func apiError(code codes.Code, msg any) []byte {
	err := fmt.Sprintf("%s%d%s%+v", internal.ErrorPrefix, code, internal.ErrorSeparator, msg)
	return []byte(err)
}
