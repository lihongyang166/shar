// Code generated by nats-proto-gen-go. DO NOT EDIT.
package natsrpc

import (
    "gitlab.com/shar-workflow/shar/model"
    "github.com/nats-io/nats.go"
    "gitlab.com/shar-workflow/nats-proto-gen-go/core"
    "sync"
    "fmt"
)

// SharServer - The Shar server.
type SharServer struct {
    server Shar
    panicRecovery bool
    subs          *sync.Map
}

// NewSharServer creates a new Shar server.
func NewSharServer(server Shar, panicRecovery bool) *SharServer {
    s := &SharServer{
		server: server,
        panicRecovery: panicRecovery,
		subs: &sync.Map{},
    }
	return s
}

// Listen starts listening for incoming API requests.
func (s *SharServer) Listen(con *nats.Conn, middleware []core.Handler, errorHandler core.ErrorHandler) error {
    if errorHandler == nil {
        errorHandler = core.DefaultErrorHandler
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.StoreWorkflow", &model.StoreWorkflowRequest{}, s.server.StoreWorkflow); err != nil {
        return fmt.Errorf("WORKFLOW.Api.StoreWorkflow: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.CancelProcessInstance", &model.CancelProcessInstanceRequest{}, s.server.CancelProcessInstance); err != nil {
        return fmt.Errorf("WORKFLOW.Api.CancelProcessInstance: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.LaunchProcess", &model.LaunchWorkflowRequest{}, s.server.LaunchProcess); err != nil {
        return fmt.Errorf("WORKFLOW.Api.LaunchProcess: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.ListWorkflows", &model.ListWorkflowsRequest{}, s.server.ListWorkflows); err != nil {
        return fmt.Errorf("WORKFLOW.Api.ListWorkflows: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.ListExecutionProcesses", &model.ListExecutionProcessesRequest{}, s.server.ListExecutionProcesses); err != nil {
        return fmt.Errorf("WORKFLOW.Api.ListExecutionProcesses: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.ListExecution", &model.ListExecutionRequest{}, s.server.ListExecution); err != nil {
        return fmt.Errorf("WORKFLOW.Api.ListExecution: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.SendMessage", &model.SendMessageRequest{}, s.server.SendMessage); err != nil {
        return fmt.Errorf("WORKFLOW.Api.SendMessage: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.CompleteManualTask", &model.CompleteManualTaskRequest{}, s.server.CompleteManualTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.CompleteManualTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.CompleteServiceTask", &model.CompleteServiceTaskRequest{}, s.server.CompleteServiceTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.CompleteServiceTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.CompleteUserTask", &model.CompleteUserTaskRequest{}, s.server.CompleteUserTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.CompleteUserTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.ListUserTaskIDs", &model.ListUserTasksRequest{}, s.server.ListUserTaskIDs); err != nil {
        return fmt.Errorf("WORKFLOW.Api.ListUserTaskIDs: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetUserTask", &model.GetUserTaskRequest{}, s.server.GetUserTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetUserTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.HandleWorkflowError", &model.HandleWorkflowErrorRequest{}, s.server.HandleWorkflowError); err != nil {
        return fmt.Errorf("WORKFLOW.Api.HandleWorkflowError: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.CompleteSendMessageTask", &model.CompleteSendMessageRequest{}, s.server.CompleteSendMessageTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.CompleteSendMessageTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetWorkflowVersions", &model.GetWorkflowVersionsRequest{}, s.server.GetWorkflowVersions); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetWorkflowVersions: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetWorkflow", &model.GetWorkflowRequest{}, s.server.GetWorkflow); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetWorkflow: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetProcessInstanceStatus", &model.GetProcessInstanceStatusRequest{}, s.server.GetProcessInstanceStatus); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetProcessInstanceStatus: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetProcessHistory", &model.GetProcessHistoryRequest{}, s.server.GetProcessHistory); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetProcessHistory: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetVersionInfo", &model.GetVersionInfoRequest{}, s.server.GetVersionInfo); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetVersionInfo: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.RegisterTask", &model.RegisterTaskRequest{}, s.server.RegisterTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.RegisterTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetTaskSpec", &model.GetTaskSpecRequest{}, s.server.GetTaskSpec); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetTaskSpec: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.DeprecateServiceTask", &model.DeprecateServiceTaskRequest{}, s.server.DeprecateServiceTask); err != nil {
        return fmt.Errorf("WORKFLOW.Api.DeprecateServiceTask: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetTaskSpecVersions", &model.GetTaskSpecVersionsRequest{}, s.server.GetTaskSpecVersions); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetTaskSpecVersions: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.GetTaskSpecUsage", &model.GetTaskSpecUsageRequest{}, s.server.GetTaskSpecUsage); err != nil {
        return fmt.Errorf("WORKFLOW.Api.GetTaskSpecUsage: %w", err)
    }
    if err := core.Listen(con, s.panicRecovery, s.subs, middleware, errorHandler, "WORKFLOW.Api.ListTaskSpecUIDs", &model.ListTaskSpecUIDsRequest{}, s.server.ListTaskSpecUIDs); err != nil {
        return fmt.Errorf("WORKFLOW.Api.ListTaskSpecUIDs: %w", err)
    }
    return nil
}
