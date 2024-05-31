package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/validation"
	version2 "gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
)

func (s *SharServer) getProcessInstanceStatus(ctx context.Context, req *model.GetProcessInstanceStatusRequest, wch chan<- *model.WorkflowState, errs chan<- error) {
	// TODO: Auth for process
	ctx, _, err2 := s.authFromProcessInstanceID(ctx, req.Id)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.engine.GetProcessInstanceStatus(ctx, req.Id, wch, errs)
}

func (s *SharServer) listExecutionProcesses(ctx context.Context, req *model.ListExecutionProcessesRequest) (*model.ListExecutionProcessesResponse, error) {
	ctx, instance, err2 := s.authFromExecutionID(ctx, req.Id)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	res, err := s.engine.ListExecutionProcesses(ctx, instance.ExecutionId)
	if err != nil {
		return nil, fmt.Errorf("get execution status: %w", err)
	}
	return &model.ListExecutionProcessesResponse{ProcessInstanceId: res}, nil
}

func (s *SharServer) listWorkflows(ctx context.Context, _ *model.ListWorkflowsRequest, res chan<- *model.ListWorkflowResponse, errs chan<- error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.engine.ListWorkflows(ctx, res, errs)
}

func (s *SharServer) listExecutableProcesses(ctx context.Context, req *model.ListExecutableProcessesRequest, res chan<- *model.ListExecutableProcessesItem, errs chan<- error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.engine.ListExecutableProcesses(ctx, res, errs)
}

func (s *SharServer) sendMessage(ctx context.Context, req *model.SendMessageRequest) (*model.SendMessageResponse, error) {
	//TODO: how do we auth this?

	messageName := req.Name
	if req.CorrelationKey == "" {
		if processId, err := s.engine.GetProcessIdFor(ctx, messageName); err != nil {
			return nil, fmt.Errorf("error retrieving process id for message name: %w", err)
		} else {
			launchWorkflowRequest := &model.LaunchWorkflowRequest{
				ProcessId: processId,
				Vars:      req.Vars,
			}
			launchWorkflowResponse, err := s.launchProcess(ctx, launchWorkflowRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to launch process with message: %w", err)
			}
			executionId := launchWorkflowResponse.ExecutionId
			workflowId := launchWorkflowResponse.WorkflowId
			return &model.SendMessageResponse{ExecutionId: executionId, WorkflowId: workflowId}, nil
		}
	} else {
		if err := s.engine.PublishMessage(ctx, messageName, req.CorrelationKey, req.Vars); err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}
	}
	return &model.SendMessageResponse{}, nil
}

func (s *SharServer) completeManualTask(ctx context.Context, req *model.CompleteManualTaskRequest) (*model.CompleteManualTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	if err := s.engine.CompleteManualTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete manual task: %w", err)
	}
	return nil, nil
}

func (s *SharServer) completeServiceTask(ctx context.Context, req *model.CompleteServiceTaskRequest) (*model.CompleteServiceTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	if err := s.engine.CompleteServiceTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete service task: %w", err)
	}
	return nil, nil
}

func (s *SharServer) completeSendMessageTask(ctx context.Context, req *model.CompleteSendMessageRequest) (*model.CompleteSendMessageResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	if err := s.engine.CompleteSendMessageTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete send message task: %w", err)
	}
	return nil, nil
}

func (s *SharServer) completeUserTask(ctx context.Context, req *model.CompleteUserTaskRequest) (*model.CompleteUserTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize complete user task: %w", err2)
	}
	if err := s.engine.CompleteUserTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete user task: %w", err)
	}
	return nil, nil
}

func (s *SharServer) getCompensationInputVariables(ctx context.Context, req *model.GetCompensationInputVariablesRequest) (*model.GetCompensationInputVariablesResponse, error) {
	ctx, _, err2 := s.authFromProcessInstanceID(ctx, req.ProcessInstanceId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize get compensation input variables: %w", err2)
	}
	v, err := s.engine.GetCompensationInputVariables(ctx, req.ProcessInstanceId, req.TrackingId)
	if err != nil {
		return nil, fmt.Errorf("get compensation input variables: %w", err)
	}

	return &model.GetCompensationInputVariablesResponse{
		Vars: v,
	}, nil
}

func (s *SharServer) getCompensationOutputVariables(ctx context.Context, req *model.GetCompensationOutputVariablesRequest) (*model.GetCompensationOutputVariablesResponse, error) {
	ctx, _, err2 := s.authFromProcessInstanceID(ctx, req.ProcessInstanceId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize get compensation output variables: %w", err2)
	}
	v, err := s.engine.GetCompensationOutputVariables(ctx, req.ProcessInstanceId, req.TrackingId)
	if err != nil {
		return nil, fmt.Errorf("get compensation output variables: %w", err)
	}
	return &model.GetCompensationOutputVariablesResponse{
		Vars: v,
	}, nil
}

func (s *SharServer) storeWorkflow(ctx context.Context, wf *model.StoreWorkflowRequest) (*model.StoreWorkflowResponse, error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, wf.Workflow.Name)
	if err2 != nil {
		return nil, fmt.Errorf("authorize complete user task: %w", err2)
	}
	res, err := s.engine.LoadWorkflow(ctx, wf.Workflow)
	if err != nil {
		return nil, fmt.Errorf("store workflow: %w", err)
	}
	return &model.StoreWorkflowResponse{WorkflowId: res}, nil
}

func (s *SharServer) launchProcess(ctx context.Context, req *model.LaunchWorkflowRequest) (*model.LaunchWorkflowResponse, error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, req.ProcessId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize complete user task: %w", err2)
	}
	executionID, wfID, err := s.engine.Launch(ctx, req.ProcessId, req.Vars)
	if err != nil {
		return nil, fmt.Errorf("launch execution kv: %w", err)
	}
	return &model.LaunchWorkflowResponse{WorkflowId: wfID, ExecutionId: executionID}, nil
}

func (s *SharServer) cancelProcessInstance(ctx context.Context, req *model.CancelProcessInstanceRequest) (*model.CancelProcessInstanceResponse, error) {
	ctx, instance, err2 := s.authFromProcessInstanceID(ctx, req.Id)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	// TODO: get working state here
	state := &model.WorkflowState{
		ExecutionId:       instance.ExecutionId,
		ProcessInstanceId: instance.ProcessInstanceId,
		State:             req.State,
		Error:             req.Error,
	}
	err := s.engine.CancelProcessInstance(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("cancel execution kv: %w", err)
	}
	return &model.CancelProcessInstanceResponse{}, nil
}

func (s *SharServer) listExecution(ctx context.Context, req *model.ListExecutionRequest, ret chan<- *model.ListExecutionItem, errs chan<- error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, req.WorkflowName)
	if err2 != nil {
		errs <- fmt.Errorf("authorize complete user task: %w", err2)
	}
	s.engine.ListExecutions(ctx, req.WorkflowName, ret, errs)
}

func (s *SharServer) handleWorkflowError(ctx context.Context, req *model.HandleWorkflowErrorRequest) (*model.HandleWorkflowErrorResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	// Sanity check
	if req.ErrorCode == "" {
		return nil, fmt.Errorf("ErrorCode may not be empty: %w", errors2.ErrMissingErrorCode)
	}

	err := s.engine.HandleWorkflowError(ctx, req.ErrorCode, req.Message, req.Vars, job)
	if errors.Is(err, ErrErrorUnhandled) {
		return &model.HandleWorkflowErrorResponse{Handled: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.HandleWorkflowErrorResponse{Handled: true}, nil
}

var ErrErrorUnhandled = errors.New("error not handled")

func (s *SharServer) listUserTaskIDs(ctx context.Context, req *model.ListUserTasksRequest) (*model.UserTasks, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	oid, err := s.engine.OwnerID(ctx, req.Owner)
	if err != nil {
		return nil, fmt.Errorf("get owner ID: %w", err)
	}
	ut, err := s.engine.GetUserTaskIDs(ctx, oid)
	if errors.Is(err, jetstream.ErrKeyNotFound) {
		return &model.UserTasks{Id: []string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user task IDs: %w", err)
	}
	return ut, nil
}

func (s *SharServer) getUserTask(ctx context.Context, req *model.GetUserTaskRequest) (*model.GetUserTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	wf, err := s.engine.GetWorkflow(ctx, job.WorkflowId)
	if err != nil {
		return nil, fmt.Errorf("get user task failed to get workflow: %w", err)
	}
	els := make(map[string]*model.Element)
	for _, v := range wf.Process {
		common.IndexProcessElements(v.Elements, els)
	}
	return &model.GetUserTaskResponse{
		TrackingId:  common.TrackingID(job.Id).ID(),
		Owner:       req.Owner,
		Name:        els[job.ElementId].Name,
		Description: els[job.ElementId].Documentation,
		Vars:        job.Vars,
	}, nil
}

func (s *SharServer) getJob(ctx context.Context, req *model.GetJobRequest) (*model.GetJobResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.JobId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	return &model.GetJobResponse{
		Job: job,
	}, nil
}

func (s *SharServer) getWorkflowVersions(ctx context.Context, req *model.GetWorkflowVersionsRequest, wch chan<- *model.WorkflowVersion, errs chan<- error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, req.Name)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.engine.GetWorkflowVersions(ctx, req.Name, wch, errs)
}

func (s *SharServer) getWorkflow(ctx context.Context, req *model.GetWorkflowRequest) (*model.GetWorkflowResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	ret, err := s.engine.GetWorkflow(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	return &model.GetWorkflowResponse{Definition: ret}, nil
}

func (s *SharServer) getProcessHistory(ctx context.Context, req *model.GetProcessHistoryRequest, wch chan<- *model.ProcessHistoryEntry, errs chan<- error) {
	ctx, _, err := s.authFromProcessInstanceID(ctx, req.Id)
	if err != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err)
		return
	}
	s.engine.GetProcessHistory(ctx, req.Id, wch, errs)
}

func (s *SharServer) versionInfo(ctx context.Context, req *model.GetVersionInfoRequest) (*model.GetVersionInfoResponse, error) {
	ctx, _, err2 := s.authenticate(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	// For clients that can't supply the compatible version
	if req.CompatibleVersion == "" {
		return nil, fmt.Errorf("client version too old, please upgrade to " + version2.Version)
	}

	v, err := version.NewVersion(req.CompatibleVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing client version '%s': %w", req.ClientVersion, err)
	}
	ok, ver := upgrader.IsCompatible(v)
	ret := &model.GetVersionInfoResponse{
		ServerVersion:        version2.Version,
		MinCompatibleVersion: ver.String(),
		Connect:              ok,
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return ret, nil
}

func (s *SharServer) registerTask(ctx context.Context, req *model.RegisterTaskRequest) (*model.RegisterTaskResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	if err := validation.ValidateTaskSpec(req.Spec); err != nil {
		return nil, fmt.Errorf("validaet service task: %w", err)
	}

	uid, err := s.engine.PutTaskSpec(ctx, req.Spec)

	if err != nil {
		return nil, fmt.Errorf("register task spec: %w", err)
	}

	return &model.RegisterTaskResponse{Uid: uid}, nil
}

func (s *SharServer) deprecateServiceTask(ctx context.Context, req *model.DeprecateServiceTaskRequest) (*model.DeprecateServiceTaskResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	usage, err := s.engine.GetTaskSpecUsage(ctx, []string{req.Name})
	if err != nil {
		return nil, fmt.Errorf("deprecate service task get initial task usage: %w", err)
	}

	if len(usage.ExecutingWorkflow)+len(usage.ExecutingProcessInstance) > 0 {
		return &model.DeprecateServiceTaskResponse{Usage: usage, Success: false}, nil
	}

	// Deprecate the task to ensure it can't get launched.
	err = s.engine.DeprecateTaskSpec(ctx, []string{req.Name})
	if err != nil {
		return nil, fmt.Errorf("delete service task get spec UID: %w", err)
	}
	return &model.DeprecateServiceTaskResponse{Usage: usage, Success: true}, nil
}
func (s *SharServer) getTaskSpec(ctx context.Context, req *model.GetTaskSpecRequest) (*model.GetTaskSpecResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	spec, err := s.engine.GetTaskSpecByUID(ctx, req.Uid)
	if err != nil {
		return nil, fmt.Errorf("get task spec: %w", err)
	}
	return &model.GetTaskSpecResponse{Spec: spec}, nil
}

func (s *SharServer) getTaskSpecVersions(ctx context.Context, req *model.GetTaskSpecVersionsRequest) (*model.GetTaskSpecVersionsResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	vers, err := s.engine.GetTaskSpecVersions(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("get task spec versions: %w", err)
	}
	return &model.GetTaskSpecVersionsResponse{Versions: vers}, nil
}

func (s *SharServer) getTaskSpecUsage(ctx context.Context, req *model.GetTaskSpecUsageRequest) (*model.TaskSpecUsageReport, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	usage, err := s.engine.GetTaskSpecUsage(ctx, []string{req.Id})
	if err != nil {
		return nil, fmt.Errorf("get task spec versions: %w", err)
	}
	return usage, nil
}

func (s *SharServer) listTaskSpecUIDs(ctx context.Context, req *model.ListTaskSpecUIDsRequest) (*model.ListTaskSpecUIDsResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	uids, err := s.engine.ListTaskSpecUIDs(ctx, req.IncludeDeprecated)
	if err != nil {
		return nil, fmt.Errorf("list task spec uids: %w", err)
	}
	return &model.ListTaskSpecUIDsResponse{Uid: uids}, nil
}

func (s *SharServer) heartbeat(ctx context.Context, req *model.HeartbeatRequest) (*model.HeartbeatResponse, error) {
	if err := s.engine.Heartbeat(ctx, req); err != nil {
		return nil, fmt.Errorf("heartbeat: %w", err)
	}
	return &model.HeartbeatResponse{}, nil
}

func (s *SharServer) log(ctx context.Context, req *model.LogRequest) (*model.LogResponse, error) {
	if err := s.engine.Log(ctx, req); err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	return &model.LogResponse{}, nil
}

func (s *SharServer) resolveWorkflow(ctx context.Context, req *model.ResolveWorkflowRequest) (*model.ResolveWorkflowResponse, error) {
	wf := req.Workflow
	if err := s.engine.ProcessServiceTasks(ctx, wf, workflow.NoOpServiceTaskConsumerFn, workflow.NoOpWorkFlowProcessMappingFn); err != nil {
		return nil, fmt.Errorf("resolveWorkflow: %w", err)
	}

	return &model.ResolveWorkflowResponse{Workflow: wf}, nil
}
