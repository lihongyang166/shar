package api

import (
	"fmt"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
)

// Endpoints provides API Endpoints for SHAR
type Endpoints struct {
	operations workflow.Ops
	auth       Auth
	Listener   *Listener
}

// NewEndpoints creates a new instance of the Endpoints of the SHAR API server
func NewEndpoints(operations workflow.Ops, auth Auth, listener *Listener) *Endpoints {
	ss := &Endpoints{
		auth:       auth,
		operations: operations,
		Listener:   listener,
	}
	return ss
}

// StartListening registers shar endspoints and starts listening for requests to them
func (s *Endpoints) StartListening() error {

	RegisterEndpointFn(s.Listener, "APIStoreWorkflow", messages.APIStoreWorkflow, func() *model.StoreWorkflowRequest { return &model.StoreWorkflowRequest{} }, s.storeWorkflow)
	RegisterEndpointFn(s.Listener, "APICancelProcessInstance", messages.APICancelProcessInstance, func() *model.CancelProcessInstanceRequest { return &model.CancelProcessInstanceRequest{} }, s.cancelProcessInstance)
	RegisterEndpointFn(s.Listener, "APILaunchProcess", messages.APILaunchProcess, func() *model.LaunchWorkflowRequest { return &model.LaunchWorkflowRequest{} }, s.launchProcess)
	RegisterEndpointFn(s.Listener, "APISendMessage", messages.APISendMessage, func() *model.SendMessageRequest { return &model.SendMessageRequest{} }, s.sendMessage)
	RegisterEndpointFn(s.Listener, "APICompleteManualTask", messages.APICompleteManualTask, func() *model.CompleteManualTaskRequest { return &model.CompleteManualTaskRequest{} }, s.completeManualTask)
	RegisterEndpointFn(s.Listener, "APICompleteServiceTask", messages.APICompleteServiceTask, func() *model.CompleteServiceTaskRequest { return &model.CompleteServiceTaskRequest{} }, s.completeServiceTask)
	RegisterEndpointFn(s.Listener, "APICompleteUserTask", messages.APICompleteUserTask, func() *model.CompleteUserTaskRequest { return &model.CompleteUserTaskRequest{} }, s.completeUserTask)
	RegisterEndpointFn(s.Listener, "APIGetUserTask", messages.APIGetUserTask, func() *model.GetUserTaskRequest { return &model.GetUserTaskRequest{} }, s.getUserTask)
	RegisterEndpointFn(s.Listener, "APIGetJob", messages.APIGetJob, func() *model.GetJobRequest { return &model.GetJobRequest{} }, s.getJob)
	RegisterEndpointFn(s.Listener, "APIHandleWorkflowError", messages.APIHandleWorkflowError, func() *model.HandleWorkflowErrorRequest { return &model.HandleWorkflowErrorRequest{} }, s.handleWorkflowError)
	RegisterEndpointFn(s.Listener, "APIHandleWorkflowFatalError", messages.APIHandleWorkflowFatalError, func() *model.HandleWorkflowFatalErrorRequest { return &model.HandleWorkflowFatalErrorRequest{} }, s.handleWorkflowFatalError)
	RegisterEndpointFn(s.Listener, "APICompleteSendMessageTask", messages.APICompleteSendMessageTask, func() *model.CompleteSendMessageRequest { return &model.CompleteSendMessageRequest{} }, s.completeSendMessageTask)
	RegisterEndpointFn(s.Listener, "APIGetWorkflow", messages.APIGetWorkflow, func() *model.GetWorkflowRequest { return &model.GetWorkflowRequest{} }, s.getWorkflow)
	RegisterEndpointFn(s.Listener, "APIGetProcessHistory", messages.APIGetVersionInfo, func() *model.GetVersionInfoRequest { return &model.GetVersionInfoRequest{} }, s.versionInfo)
	RegisterEndpointFn(s.Listener, "APIRegisterTask", messages.APIRegisterTask, func() *model.RegisterTaskRequest { return &model.RegisterTaskRequest{} }, s.registerTask)
	RegisterEndpointFn(s.Listener, "APIGetTaskSpec", messages.APIGetTaskSpec, func() *model.GetTaskSpecRequest { return &model.GetTaskSpecRequest{} }, s.getTaskSpec)
	RegisterEndpointFn(s.Listener, "APIDeprecateServiceTask", messages.APIDeprecateServiceTask, func() *model.DeprecateServiceTaskRequest { return &model.DeprecateServiceTaskRequest{} }, s.deprecateServiceTask)
	RegisterEndpointFn(s.Listener, "APIGetTaskSpecUsage", messages.APIGetTaskSpecUsage, func() *model.GetTaskSpecUsageRequest { return &model.GetTaskSpecUsageRequest{} }, s.getTaskSpecUsage)
	RegisterEndpointFn(s.Listener, "APIHeartbeat", messages.APIHeartbeat, func() *model.HeartbeatRequest { return &model.HeartbeatRequest{} }, s.heartbeat)
	RegisterEndpointFn(s.Listener, "APILog", messages.APILog, func() *model.LogRequest { return &model.LogRequest{} }, s.log)
	RegisterEndpointFn(s.Listener, "APIResolveWorkflow", messages.APIResolveWorkflow, func() *model.ResolveWorkflowRequest { return &model.ResolveWorkflowRequest{} }, s.resolveWorkflow)
	RegisterEndpointFn(s.Listener, "APIListExecutionProcesses", messages.APIListExecutionProcesses, func() *model.ListExecutionProcessesRequest { return &model.ListExecutionProcessesRequest{} }, s.listExecutionProcesses)
	RegisterEndpointFn(s.Listener, "APIListTaskSpecUIDs", messages.APIListTaskSpecUIDs, func() *model.ListTaskSpecUIDsRequest { return &model.ListTaskSpecUIDsRequest{} }, s.listTaskSpecUIDs)
	RegisterEndpointFn(s.Listener, "APIGetTaskSpecVersions", messages.APIGetTaskSpecVersions, func() *model.GetTaskSpecVersionsRequest { return &model.GetTaskSpecVersionsRequest{} }, s.getTaskSpecVersions)
	RegisterEndpointFn(s.Listener, "APIGetCompensationInputVariables", messages.APIGetCompensationInputVariables, func() *model.GetCompensationInputVariablesRequest {
		return &model.GetCompensationInputVariablesRequest{}
	}, s.getCompensationInputVariables)
	RegisterEndpointFn(s.Listener, "APIGetCompensationOutputVariables", messages.APIGetCompensationOutputVariables, func() *model.GetCompensationOutputVariablesRequest {
		return &model.GetCompensationOutputVariablesRequest{}
	}, s.getCompensationOutputVariables)
	RegisterEndpointFn(s.Listener, "APIListUserTaskIDs", messages.APIListUserTaskIDs, func() *model.ListUserTasksRequest { return &model.ListUserTasksRequest{} }, s.listUserTaskIDs)
	RegisterEndpointFn(s.Listener, "APIRetry", messages.APIRetry, func() *model.RetryActivityRequest { return &model.RetryActivityRequest{} }, s.retryActivity)
	RegisterEndpointFn(s.Listener, "APIGetProcessInstanceHeaders", messages.APIGetProcessInstanceHeaders, func() *model.GetProcessHeadersRequest { return &model.GetProcessHeadersRequest{} }, s.getProcessHeaders)
	RegisterEndpointFn(s.Listener, "APIDisableWorkflow", messages.APIDisableWorkflow, func() *model.DisableWorkflowRequest { return &model.DisableWorkflowRequest{} }, s.disableWorkflow)
	RegisterEndpointFn(s.Listener, "APIEnableWorkflow", messages.APIEnableWorkflow, func() *model.EnableWorkflowRequest { return &model.EnableWorkflowRequest{} }, s.enableWorkflow)

	RegisterEndpointStreamingFn(s.Listener, "APIGetWorkflowVersions", messages.APIGetWorkflowVersions, func() *model.GetWorkflowVersionsRequest { return &model.GetWorkflowVersionsRequest{} }, s.getWorkflowVersions)
	RegisterEndpointStreamingFn(s.Listener, "APIGetProcessInstanceStatus", messages.APIGetProcessInstanceStatus, func() *model.GetProcessInstanceStatusRequest { return &model.GetProcessInstanceStatusRequest{} }, s.getProcessInstanceStatus)
	RegisterEndpointStreamingFn(s.Listener, "APIListWorkflows", messages.APIListWorkflows, func() *model.ListWorkflowsRequest { return &model.ListWorkflowsRequest{} }, s.listWorkflows)
	RegisterEndpointStreamingFn(s.Listener, "APIListExecution", messages.APIListExecution, func() *model.ListExecutionRequest { return &model.ListExecutionRequest{} }, s.listExecution)
	RegisterEndpointStreamingFn(s.Listener, "APIGetProcessHistory", messages.APIGetProcessHistory, func() *model.GetProcessHistoryRequest { return &model.GetProcessHistoryRequest{} }, s.getProcessHistory)
	RegisterEndpointStreamingFn(s.Listener, "APIListExecutableProcess", messages.APIListExecutableProcess, func() *model.ListExecutableProcessesRequest { return &model.ListExecutableProcessesRequest{} }, s.listExecutableProcesses)
	RegisterEndpointStreamingFn(s.Listener, "APIGetFatalErrors", messages.APIGetFatalErrors, func() *model.GetFatalErrorRequest { return &model.GetFatalErrorRequest{} }, s.getFatalErrors)

	err := s.Listener.StartListening()
	if err != nil {
		return fmt.Errorf("failed to start endpoint listener: %w", err)
	}
	return nil
}
