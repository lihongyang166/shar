// Code generated by nats-proto-gen-go. DO NOT EDIT.
package natsrpc

import (
	"fmt"
	"context"
    "github.com/nats-io/nats.go"
    "gitlab.com/shar-workflow/nats-proto-gen-go/core"
    "gitlab.com/shar-workflow/shar/model"
)
// Shar - The Shar service.
type SharClient struct {
    middleware []core.Handler
    customErrorHandler core.ErrorHandler
	txConn *nats.Conn
}

// NewSharClient - Creates a new client for Shar
func NewSharClient(serverNatsConnection *nats.Conn, middleware []core.Handler, customErrorHandler core.ErrorHandler) SharClient {
	return SharClient{
		middleware: middleware,
        customErrorHandler: customErrorHandler,
		txConn: serverNatsConnection,
    }
}
// StoreWorkflow - The StoreWorkflow method.
func (c *SharClient) StoreWorkflow(ctx context.Context, req *model.StoreWorkflowRequest) (*model.StoreWorkflowResponse, error) {
	res := &model.StoreWorkflowResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.StoreWorkflow", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.StoreWorkflow: %w", err)
    }
	return res, nil
}
// CancelProcessInstance - The CancelProcessInstance method.
func (c *SharClient) CancelProcessInstance(ctx context.Context, req *model.CancelProcessInstanceRequest) (*model.CancelProcessInstanceResponse, error) {
	res := &model.CancelProcessInstanceResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.CancelProcessInstance", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.CancelProcessInstance: %w", err)
    }
	return res, nil
}
// LaunchProcess - The LaunchProcess method.
func (c *SharClient) LaunchProcess(ctx context.Context, req *model.LaunchWorkflowRequest) (*model.LaunchWorkflowResponse, error) {
	res := &model.LaunchWorkflowResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.LaunchProcess", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.LaunchProcess: %w", err)
    }
	return res, nil
}
// ListWorkflows - The ListWorkflows method.
func (c *SharClient) ListWorkflows(ctx context.Context, req *model.ListWorkflowsRequest) (*model.ListWorkflowsResponse, error) {
	res := &model.ListWorkflowsResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.ListWorkflows", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.ListWorkflows: %w", err)
    }
	return res, nil
}
// ListExecutionProcesses - The ListExecutionProcesses method.
func (c *SharClient) ListExecutionProcesses(ctx context.Context, req *model.ListExecutionProcessesRequest) (*model.ListExecutionProcessesResponse, error) {
	res := &model.ListExecutionProcessesResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.ListExecutionProcesses", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.ListExecutionProcesses: %w", err)
    }
	return res, nil
}
// ListExecution - The ListExecution method.
func (c *SharClient) ListExecution(ctx context.Context, req *model.ListExecutionRequest) (*model.ListExecutionResponse, error) {
	res := &model.ListExecutionResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.ListExecution", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.ListExecution: %w", err)
    }
	return res, nil
}
// SendMessage - The SendMessage method.
func (c *SharClient) SendMessage(ctx context.Context, req *model.SendMessageRequest) (*model.SendMessageResponse, error) {
	res := &model.SendMessageResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.SendMessage", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.SendMessage: %w", err)
    }
	return res, nil
}
// CompleteManualTask - The CompleteManualTask method.
func (c *SharClient) CompleteManualTask(ctx context.Context, req *model.CompleteManualTaskRequest) (*model.CompleteManualTaskResponse, error) {
	res := &model.CompleteManualTaskResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.CompleteManualTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.CompleteManualTask: %w", err)
    }
	return res, nil
}
// CompleteServiceTask - The CompleteServiceTask method.
func (c *SharClient) CompleteServiceTask(ctx context.Context, req *model.CompleteServiceTaskRequest) (*model.CompleteServiceTaskResponse, error) {
	res := &model.CompleteServiceTaskResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.CompleteServiceTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.CompleteServiceTask: %w", err)
    }
	return res, nil
}
// CompleteUserTask - The CompleteUserTask method.
func (c *SharClient) CompleteUserTask(ctx context.Context, req *model.CompleteUserTaskRequest) (*model.CompleteUserTaskResponse, error) {
	res := &model.CompleteUserTaskResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.CompleteUserTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.CompleteUserTask: %w", err)
    }
	return res, nil
}
// ListUserTaskIDs - The ListUserTaskIDs method.
func (c *SharClient) ListUserTaskIDs(ctx context.Context, req *model.ListUserTasksRequest) (*model.ListUserTasksResponse, error) {
	res := &model.ListUserTasksResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.ListUserTaskIDs", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.ListUserTaskIDs: %w", err)
    }
	return res, nil
}
// GetUserTask - The GetUserTask method.
func (c *SharClient) GetUserTask(ctx context.Context, req *model.GetUserTaskRequest) (*model.GetUserTaskResponse, error) {
	res := &model.GetUserTaskResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetUserTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetUserTask: %w", err)
    }
	return res, nil
}
// HandleWorkflowError - The HandleWorkflowError method.
func (c *SharClient) HandleWorkflowError(ctx context.Context, req *model.HandleWorkflowErrorRequest) (*model.HandleWorkflowErrorResponse, error) {
	res := &model.HandleWorkflowErrorResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.HandleWorkflowError", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.HandleWorkflowError: %w", err)
    }
	return res, nil
}
// CompleteSendMessageTask - The CompleteSendMessageTask method.
func (c *SharClient) CompleteSendMessageTask(ctx context.Context, req *model.CompleteSendMessageRequest) (*model.CompleteSendMessageResponse, error) {
	res := &model.CompleteSendMessageResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.CompleteSendMessageTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.CompleteSendMessageTask: %w", err)
    }
	return res, nil
}
// GetWorkflowVersions - The GetWorkflowVersions method.
func (c *SharClient) GetWorkflowVersions(ctx context.Context, req *model.GetWorkflowVersionsRequest) (*model.GetWorkflowVersionsResponse, error) {
	res := &model.GetWorkflowVersionsResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetWorkflowVersions", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetWorkflowVersions: %w", err)
    }
	return res, nil
}
// GetWorkflow - The GetWorkflow method.
func (c *SharClient) GetWorkflow(ctx context.Context, req *model.GetWorkflowRequest) (*model.GetWorkflowResponse, error) {
	res := &model.GetWorkflowResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetWorkflow", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetWorkflow: %w", err)
    }
	return res, nil
}
// GetProcessInstanceStatus - The GetProcessInstanceStatus method.
func (c *SharClient) GetProcessInstanceStatus(ctx context.Context, req *model.GetProcessInstanceStatusRequest) (*model.GetProcessInstanceStatusResponse, error) {
	res := &model.GetProcessInstanceStatusResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetProcessInstanceStatus", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetProcessInstanceStatus: %w", err)
    }
	return res, nil
}
// GetProcessHistory - The GetProcessHistory method.
func (c *SharClient) GetProcessHistory(ctx context.Context, req *model.GetProcessHistoryRequest) (*model.GetProcessHistoryResponse, error) {
	res := &model.GetProcessHistoryResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetProcessHistory", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetProcessHistory: %w", err)
    }
	return res, nil
}
// GetVersionInfo - The GetVersionInfo method.
func (c *SharClient) GetVersionInfo(ctx context.Context, req *model.GetVersionInfoRequest) (*model.GetVersionInfoResponse, error) {
	res := &model.GetVersionInfoResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetVersionInfo", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetVersionInfo: %w", err)
    }
	return res, nil
}
// RegisterTask - The RegisterTask method.
func (c *SharClient) RegisterTask(ctx context.Context, req *model.RegisterTaskRequest) (*model.RegisterTaskResponse, error) {
	res := &model.RegisterTaskResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.RegisterTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.RegisterTask: %w", err)
    }
	return res, nil
}
// GetTaskSpec - The GetTaskSpec method.
func (c *SharClient) GetTaskSpec(ctx context.Context, req *model.GetTaskSpecRequest) (*model.GetTaskSpecResponse, error) {
	res := &model.GetTaskSpecResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetTaskSpec", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetTaskSpec: %w", err)
    }
	return res, nil
}
// DeprecateServiceTask - The DeprecateServiceTask method.
func (c *SharClient) DeprecateServiceTask(ctx context.Context, req *model.DeprecateServiceTaskRequest) (*model.DeprecateServiceTaskResponse, error) {
	res := &model.DeprecateServiceTaskResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.DeprecateServiceTask", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.DeprecateServiceTask: %w", err)
    }
	return res, nil
}
// GetTaskSpecVersions - The GetTaskSpecVersions method.
func (c *SharClient) GetTaskSpecVersions(ctx context.Context, req *model.GetTaskSpecVersionsRequest) (*model.GetTaskSpecVersionsResponse, error) {
	res := &model.GetTaskSpecVersionsResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetTaskSpecVersions", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetTaskSpecVersions: %w", err)
    }
	return res, nil
}
// GetTaskSpecUsage - The GetTaskSpecUsage method.
func (c *SharClient) GetTaskSpecUsage(ctx context.Context, req *model.GetTaskSpecUsageRequest) (*model.GetTaskSpecUsageResponse, error) {
	res := &model.GetTaskSpecUsageResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.GetTaskSpecUsage", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.GetTaskSpecUsage: %w", err)
    }
	return res, nil
}
// ListTaskSpecUIDs - The ListTaskSpecUIDs method.
func (c *SharClient) ListTaskSpecUIDs(ctx context.Context, req *model.ListTaskSpecUIDsRequest) (*model.ListTaskSpecUIDsResponse, error) {
	res := &model.ListTaskSpecUIDsResponse{}
    if err := core.Call(ctx, c.txConn, c.middleware, c.customErrorHandler, "WORKFLOW.Api.ListTaskSpecUIDs", req, res); err != nil {
        return nil, fmt.Errorf("client call to WORKFLOW.Api.ListTaskSpecUIDs: %w", err)
    }
	return res, nil
}