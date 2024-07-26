package api

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/validation"
	version2 "gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"strings"
)

func (s *Endpoints) getProcessInstanceStatus(ctx context.Context, req *model.GetProcessInstanceStatusRequest, wch chan<- *model.ProcessHistoryEntry, errs chan<- error) {
	// TODO: Auth for process
	ctx, _, err2 := s.authFromProcessInstanceID(ctx, req.Id)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.operations.GetActiveEntries(ctx, req.Id, wch, errs)

}

func (s *Endpoints) listExecutionProcesses(ctx context.Context, req *model.ListExecutionProcessesRequest) (*model.ListExecutionProcessesResponse, error) {
	ctx, instance, err2 := s.authFromExecutionID(ctx, req.Id)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	res, err := s.operations.ListExecutionProcesses(ctx, instance.ExecutionId)
	if err != nil {
		return nil, fmt.Errorf("get execution status: %w", err)
	}
	return &model.ListExecutionProcessesResponse{ProcessInstanceId: res}, nil
}

func (s *Endpoints) listWorkflows(ctx context.Context, _ *model.ListWorkflowsRequest, res chan<- *model.ListWorkflowResponse, errs chan<- error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.operations.ListWorkflows(ctx, res, errs)
}

func (s *Endpoints) listExecutableProcesses(ctx context.Context, req *model.ListExecutableProcessesRequest, res chan<- *model.ListExecutableProcessesItem, errs chan<- error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.operations.ListExecutableProcesses(ctx, res, errs)
}

func (s *Endpoints) sendMessage(ctx context.Context, req *model.SendMessageRequest) (*model.SendMessageResponse, error) {
	//TODO: how do we auth this?

	messageName := req.Name
	if req.CorrelationKey == "" {
		if processId, err := s.operations.GetProcessIdFor(ctx, messageName); err != nil {
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
		subject := fmt.Sprintf(messages.WorkflowMessage, subj.GetNS(ctx))
		sharMsg := &model.MessageInstance{
			Name:           messageName,
			CorrelationKey: req.CorrelationKey,
			Vars:           req.Vars,
		}
		if err := s.operations.PublishMsg(ctx, subject, sharMsg); err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}
	}
	return &model.SendMessageResponse{}, nil
}

func (s *Endpoints) completeManualTask(ctx context.Context, req *model.CompleteManualTaskRequest) (*model.CompleteManualTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	if err := s.operations.CompleteManualTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete manual task: %w", err)
	}
	return nil, nil
}

func (s *Endpoints) completeServiceTask(ctx context.Context, req *model.CompleteServiceTaskRequest) (*model.CompleteServiceTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	if err := s.operations.CompleteServiceTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete service task: %w", err)
	}
	return nil, nil
}

func (s *Endpoints) completeSendMessageTask(ctx context.Context, req *model.CompleteSendMessageRequest) (*model.CompleteSendMessageResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	if err := s.operations.CompleteSendMessageTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete send message task: %w", err)
	}
	return nil, nil
}

func (s *Endpoints) completeUserTask(ctx context.Context, req *model.CompleteUserTaskRequest) (*model.CompleteUserTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize complete user task: %w", err2)
	}
	if err := s.operations.CompleteUserTask(ctx, job, req.Vars); err != nil {
		return nil, fmt.Errorf("complete user task: %w", err)
	}
	return nil, nil
}

func (s *Endpoints) getCompensationInputVariables(ctx context.Context, req *model.GetCompensationInputVariablesRequest) (*model.GetCompensationInputVariablesResponse, error) {
	ctx, _, err2 := s.authFromProcessInstanceID(ctx, req.ProcessInstanceId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize get compensation input variables: %w", err2)
	}
	v, err := s.operations.GetCompensationInputVariables(ctx, req.ProcessInstanceId, req.TrackingId)
	if err != nil {
		return nil, fmt.Errorf("get compensation input variables: %w", err)
	}

	return &model.GetCompensationInputVariablesResponse{
		Vars: v,
	}, nil
}

func (s *Endpoints) getCompensationOutputVariables(ctx context.Context, req *model.GetCompensationOutputVariablesRequest) (*model.GetCompensationOutputVariablesResponse, error) {
	ctx, _, err2 := s.authFromProcessInstanceID(ctx, req.ProcessInstanceId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize get compensation output variables: %w", err2)
	}
	v, err := s.operations.GetCompensationOutputVariables(ctx, req.ProcessInstanceId, req.TrackingId)
	if err != nil {
		return nil, fmt.Errorf("get compensation output variables: %w", err)
	}
	return &model.GetCompensationOutputVariablesResponse{
		Vars: v,
	}, nil
}

func (s *Endpoints) storeWorkflow(ctx context.Context, wf *model.StoreWorkflowRequest) (*model.StoreWorkflowResponse, error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, wf.Workflow.Name)
	if err2 != nil {
		return nil, fmt.Errorf("authorize complete user task: %w", err2)
	}
	res, err := s.operations.LoadWorkflow(ctx, wf.Workflow)
	if err != nil {
		return nil, fmt.Errorf("store workflow: %w", err)
	}
	return &model.StoreWorkflowResponse{WorkflowId: res}, nil
}

func (s *Endpoints) launchProcess(ctx context.Context, req *model.LaunchWorkflowRequest) (*model.LaunchWorkflowResponse, error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, req.ProcessId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize complete user task: %w", err2)
	}
	executionID, wfID, err := s.operations.Launch(ctx, req.ProcessId, req.Vars, req.Headers)
	if err != nil {
		return nil, fmt.Errorf("launch execution kv: %w", err)
	}
	return &model.LaunchWorkflowResponse{WorkflowId: wfID, ExecutionId: executionID}, nil
}

func (s *Endpoints) cancelProcessInstance(ctx context.Context, req *model.CancelProcessInstanceRequest) (*model.CancelProcessInstanceResponse, error) {
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
	err := s.operations.CancelProcessInstance(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("cancel execution kv: %w", err)
	}
	return &model.CancelProcessInstanceResponse{}, nil
}

func (s *Endpoints) listExecution(ctx context.Context, req *model.ListExecutionRequest, ret chan<- *model.ListExecutionItem, errs chan<- error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, req.WorkflowName)
	if err2 != nil {
		errs <- fmt.Errorf("authorize complete user task: %w", err2)
	}
	s.operations.ListExecutions(ctx, req.WorkflowName, ret, errs)
}

func (s *Endpoints) handleWorkflowFatalError(ctx context.Context, req *model.HandleWorkflowFatalErrorRequest) (*model.HandleWorkflowFatalErrorResponse, error) {
	//auth against the wf name
	ctx, err := s.authorize(ctx, req.WorkflowState.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err)
	}
	s.operations.SignalFatalErrorPause(ctx, req.WorkflowState, logx.FromContext(ctx))
	return &model.HandleWorkflowFatalErrorResponse{}, nil
}

func (s *Endpoints) handleWorkflowError(ctx context.Context, req *model.HandleWorkflowErrorRequest) (*model.HandleWorkflowErrorResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	// Sanity check
	if req.ErrorCode == "" {
		return nil, fmt.Errorf("ErrorCode may not be empty: %w", errors2.ErrMissingErrorCode)
	}

	err := s.operations.HandleWorkflowError(ctx, req.ErrorCode, req.Vars, job)
	if errors.Is(err, errors2.ErrUnhandledWorkflowError) {
		return &model.HandleWorkflowErrorResponse{Handled: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("handle workflow error: %w", err)
	}
	return &model.HandleWorkflowErrorResponse{Handled: true}, nil
}

func (s *Endpoints) listUserTaskIDs(ctx context.Context, req *model.ListUserTasksRequest) (*model.UserTasks, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	oid, err := s.operations.OwnerID(ctx, req.Owner)
	if err != nil {
		return nil, fmt.Errorf("get owner ID: %w", err)
	}
	ut, err := s.operations.GetUserTaskIDs(ctx, oid)
	if errors.Is(err, jetstream.ErrKeyNotFound) {
		return &model.UserTasks{Id: []string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user task IDs: %w", err)
	}
	return ut, nil
}

func (s *Endpoints) getUserTask(ctx context.Context, req *model.GetUserTaskRequest) (*model.GetUserTaskResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.TrackingId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	wf, err := s.operations.GetWorkflow(ctx, job.WorkflowId)
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

func (s *Endpoints) getJob(ctx context.Context, req *model.GetJobRequest) (*model.GetJobResponse, error) {
	ctx, job, err2 := s.authFromJobID(ctx, req.JobId)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	return &model.GetJobResponse{
		Job: job,
	}, nil
}

func (s *Endpoints) getWorkflowVersions(ctx context.Context, req *model.GetWorkflowVersionsRequest, wch chan<- *model.WorkflowVersion, errs chan<- error) {
	ctx, err2 := s.authForNamedWorkflow(ctx, req.Name)
	if err2 != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
		return
	}
	s.operations.GetWorkflowVersions(ctx, req.Name, wch, errs)
}

func (s *Endpoints) getWorkflow(ctx context.Context, req *model.GetWorkflowRequest) (*model.GetWorkflowResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	ret, err := s.operations.GetWorkflow(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	return &model.GetWorkflowResponse{Definition: ret}, nil
}

func (s *Endpoints) getProcessHistory(ctx context.Context, req *model.GetProcessHistoryRequest, wch chan<- *model.ProcessHistoryEntry, errs chan<- error) {
	ctx, _, err := s.authFromProcessInstanceID(ctx, req.Id)
	if err != nil {
		errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err)
		return
	}
	s.operations.GetProcessHistory(ctx, req.Id, wch, errs)
}

func fatalErrorKeyPrefixBuilder(req *model.GetFatalErrorRequest) string {
	keyPrefixSegments := []string{req.WfName, req.ExecutionId, req.ProcessInstanceId}
	mappedKeyPrefixSegments := make([]string, 0, len(keyPrefixSegments))

	for _, segment := range keyPrefixSegments {
		if segment == "" {
			mappedKeyPrefixSegments = append(mappedKeyPrefixSegments, "*")
		} else {
			mappedKeyPrefixSegments = append(mappedKeyPrefixSegments, segment)
		}
	}

	return strings.Join(mappedKeyPrefixSegments, ".")
}

func (s *Endpoints) getFatalErrors(ctx context.Context, req *model.GetFatalErrorRequest, wch chan<- *model.FatalError, errs chan<- error) {
	//TODO sort out auth - it depends on whether we can resolve wf name...
	//what if the client doesn't specify any field?? is this even allowable? should we validate against it???
	//ctx, _, err := s.authFromProcessInstanceID(ctx, req.Id)
	//if err != nil {
	//	errs <- fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err)
	//	return
	//}

	s.operations.GetFatalErrors(ctx, fatalErrorKeyPrefixBuilder(req), wch, errs)
}

func (s *Endpoints) retryActivity(ctx context.Context, req *model.RetryActivityRequest) (*model.RetryActivityResponse, error) {
	ctx, err := s.authForNamedWorkflow(ctx, req.WorkflowState.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err)
	}

	if err := s.operations.RetryActivity(ctx, req.WorkflowState); err != nil {
		return nil, fmt.Errorf("failed retrying activity: %w", err)
	}
	return &model.RetryActivityResponse{}, nil
}

func (s *Endpoints) versionInfo(ctx context.Context, req *model.GetVersionInfoRequest) (*model.GetVersionInfoResponse, error) {
	ctx, _, err2 := s.authenticate(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	// For clients that can't supply the compatible version
	if req.CompatibleVersion == "" {
		return nil, fmt.Errorf("client version too old, please upgrade to %s", version2.Version)
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
	return ret, nil
}

func (s *Endpoints) registerTask(ctx context.Context, req *model.RegisterTaskRequest) (*model.RegisterTaskResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	if err := validation.ValidateTaskSpec(req.Spec); err != nil {
		return nil, fmt.Errorf("validaet service task: %w", err)
	}

	uid, err := s.operations.PutTaskSpec(ctx, req.Spec)

	if err != nil {
		return nil, fmt.Errorf("register task spec: %w", err)
	}

	return &model.RegisterTaskResponse{Uid: uid}, nil
}

func (s *Endpoints) deprecateServiceTask(ctx context.Context, req *model.DeprecateServiceTaskRequest) (*model.DeprecateServiceTaskResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	usage, err := s.operations.GetTaskSpecUsage(ctx, []string{req.Name})
	if err != nil {
		return nil, fmt.Errorf("deprecate service task get initial task usage: %w", err)
	}

	if len(usage.ExecutingWorkflow)+len(usage.ExecutingProcessInstance) > 0 {
		return &model.DeprecateServiceTaskResponse{Usage: usage, Success: false}, nil
	}

	// Deprecate the task to ensure it can't get launched.
	err = s.operations.DeprecateTaskSpec(ctx, []string{req.Name})
	if err != nil {
		return nil, fmt.Errorf("delete service task get spec UID: %w", err)
	}
	return &model.DeprecateServiceTaskResponse{Usage: usage, Success: true}, nil
}
func (s *Endpoints) getTaskSpec(ctx context.Context, req *model.GetTaskSpecRequest) (*model.GetTaskSpecResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	spec, err := s.operations.GetTaskSpecByUID(ctx, req.Uid)
	if err != nil {
		return nil, fmt.Errorf("get task spec: %w", err)
	}
	return &model.GetTaskSpecResponse{Spec: spec}, nil
}

func (s *Endpoints) getTaskSpecVersions(ctx context.Context, req *model.GetTaskSpecVersionsRequest) (*model.GetTaskSpecVersionsResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	vers, err := s.operations.GetTaskSpecVersions(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("get task spec versions: %w", err)
	}
	return &model.GetTaskSpecVersionsResponse{Versions: vers}, nil
}

func (s *Endpoints) getTaskSpecUsage(ctx context.Context, req *model.GetTaskSpecUsageRequest) (*model.TaskSpecUsageReport, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	usage, err := s.operations.GetTaskSpecUsage(ctx, []string{req.Id})
	if err != nil {
		return nil, fmt.Errorf("get task spec versions: %w", err)
	}
	return usage, nil
}

func (s *Endpoints) listTaskSpecUIDs(ctx context.Context, req *model.ListTaskSpecUIDsRequest) (*model.ListTaskSpecUIDsResponse, error) {
	ctx, err2 := s.authForNonWorkflow(ctx)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}

	uids, err := s.operations.ListTaskSpecUIDs(ctx, req.IncludeDeprecated)
	if err != nil {
		return nil, fmt.Errorf("list task spec uids: %w", err)
	}
	return &model.ListTaskSpecUIDsResponse{Uid: uids}, nil
}

func (s *Endpoints) heartbeat(ctx context.Context, req *model.HeartbeatRequest) (*model.HeartbeatResponse, error) {
	if err := s.operations.Heartbeat(ctx, req); err != nil {
		return nil, fmt.Errorf("heartbeat: %w", err)
	}
	return &model.HeartbeatResponse{}, nil
}

func (s *Endpoints) log(ctx context.Context, req *model.LogRequest) (*model.LogResponse, error) {
	if err := s.operations.Log(ctx, req); err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	return &model.LogResponse{}, nil
}

func (s *Endpoints) resolveWorkflow(ctx context.Context, req *model.ResolveWorkflowRequest) (*model.ResolveWorkflowResponse, error) {
	wf := req.Workflow
	if err := s.operations.ProcessServiceTasks(ctx, wf, workflow.NoOpServiceTaskConsumerFn, workflow.NoOpWorkFlowProcessMappingFn); err != nil {
		return nil, fmt.Errorf("resolveWorkflow: %w", err)
	}

	return &model.ResolveWorkflowResponse{Workflow: wf}, nil
}

func (s *Endpoints) getProcessHeaders(ctx context.Context, req *model.GetProcessHeadersRequest) (*model.GetProcessHeadersResponse, error) {
	ctx, pi, err2 := s.authFromProcessInstanceID(ctx, req.ProcessInstanceID)
	if err2 != nil {
		return nil, fmt.Errorf("authorize %v: %w", ctx.Value(ctxkey.APIFunc), err2)
	}
	hdr := make(map[string]string)
	if len(pi.Headers) == 0 {
		return &model.GetProcessHeadersResponse{Headers: hdr}, nil
	}
	d := gob.NewDecoder(bytes.NewBuffer(pi.Headers))
	if err := d.Decode(&hdr); err != nil {
		return nil, fmt.Errorf("decode headers: %w", err)
	}
	return &model.GetProcessHeadersResponse{Headers: hdr}, nil
}
