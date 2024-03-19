// Code generated by nats-proto-gen-go. DO NOT EDIT.
package natsrpc

import (
    "context"
    "gitlab.com/shar-workflow/shar/model"
)

// Shar represents all of the API methods on the service.
type Shar interface {
	StoreWorkflow(ctx context.Context, req *model.StoreWorkflowRequest) (*model.StoreWorkflowResponse, error)
	CancelProcessInstance(ctx context.Context, req *model.CancelProcessInstanceRequest) (*model.CancelProcessInstanceResponse, error)
	LaunchProcess(ctx context.Context, req *model.LaunchWorkflowRequest) (*model.LaunchWorkflowResponse, error)
	ListWorkflows(ctx context.Context, req *model.ListWorkflowsRequest) (*model.ListWorkflowsResponse, error)
	ListExecutionProcesses(ctx context.Context, req *model.ListExecutionProcessesRequest) (*model.ListExecutionProcessesResponse, error)
	ListExecution(ctx context.Context, req *model.ListExecutionRequest) (*model.ListExecutionResponse, error)
	SendMessage(ctx context.Context, req *model.SendMessageRequest) (*model.SendMessageResponse, error)
	CompleteManualTask(ctx context.Context, req *model.CompleteManualTaskRequest) (*model.CompleteManualTaskResponse, error)
	CompleteServiceTask(ctx context.Context, req *model.CompleteServiceTaskRequest) (*model.CompleteServiceTaskResponse, error)
	CompleteUserTask(ctx context.Context, req *model.CompleteUserTaskRequest) (*model.CompleteUserTaskResponse, error)
	ListUserTaskIDs(ctx context.Context, req *model.ListUserTasksRequest) (*model.ListUserTasksResponse, error)
	GetUserTask(ctx context.Context, req *model.GetUserTaskRequest) (*model.GetUserTaskResponse, error)
	HandleWorkflowError(ctx context.Context, req *model.HandleWorkflowErrorRequest) (*model.HandleWorkflowErrorResponse, error)
	CompleteSendMessageTask(ctx context.Context, req *model.CompleteSendMessageRequest) (*model.CompleteSendMessageResponse, error)
	GetWorkflowVersions(ctx context.Context, req *model.GetWorkflowVersionsRequest) (*model.GetWorkflowVersionsResponse, error)
	GetWorkflow(ctx context.Context, req *model.GetWorkflowRequest) (*model.GetWorkflowResponse, error)
	GetProcessInstanceStatus(ctx context.Context, req *model.GetProcessInstanceStatusRequest) (*model.GetProcessInstanceStatusResponse, error)
	GetProcessHistory(ctx context.Context, req *model.GetProcessHistoryRequest) (*model.GetProcessHistoryResponse, error)
	GetVersionInfo(ctx context.Context, req *model.GetVersionInfoRequest) (*model.GetVersionInfoResponse, error)
	RegisterTask(ctx context.Context, req *model.RegisterTaskRequest) (*model.RegisterTaskResponse, error)
	GetTaskSpec(ctx context.Context, req *model.GetTaskSpecRequest) (*model.GetTaskSpecResponse, error)
	DeprecateServiceTask(ctx context.Context, req *model.DeprecateServiceTaskRequest) (*model.DeprecateServiceTaskResponse, error)
	GetTaskSpecVersions(ctx context.Context, req *model.GetTaskSpecVersionsRequest) (*model.GetTaskSpecVersionsResponse, error)
	GetTaskSpecUsage(ctx context.Context, req *model.GetTaskSpecUsageRequest) (*model.GetTaskSpecUsageResponse, error)
	ListTaskSpecUIDs(ctx context.Context, req *model.ListTaskSpecUIDsRequest) (*model.ListTaskSpecUIDsResponse, error)
	Heartbeat(ctx context.Context, req *model.HeartbeatRequest) (*model.HeartbeatResponse, error)
	Log(ctx context.Context, req *model.LogRequest) (*model.LogResponse, error)
	GetJob(ctx context.Context, req *model.GetJobRequest) (*model.GetJobResponse, error)
	ResolveWorkflow(ctx context.Context, req *model.ResolveWorkflowRequest) (*model.ResolveWorkflowResponse, error)
}
