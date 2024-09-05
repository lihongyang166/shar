package output

import "gitlab.com/shar-workflow/shar/model"

// WorkflowListOutput is the output format for workflow list.
type WorkflowListOutput struct {
	Workflow []*model.ListWorkflowResponse
}

// StartWorkflowResult is the output format for starting a workflow.
type StartWorkflowResult struct {
	ExecutionID string
	WorkflowID  string
}

// ListExecutionOutput is the output format for listing executions
type ListExecutionOutput struct {
	Execution []*model.ListExecutionItem
}

// UserTaskIDsOutput is the output format for listing user tasks.
type UserTaskIDsOutput struct {
	UserTasks []*model.GetUserTaskResponse
}

// StateOutput is the output format for a workflow state.
type StateOutput struct {
	TrackingId string
	ID         string
	Type       string
	State      string
	Executing  string
	Since      int64
}

// ExecutionOutput is the output format for an execution.
type ExecutionOutput struct {
	ExecutionId string
	Processes   map[string][]StateOutput
}

// LoadWorkflowOutput is the output format for adding a workflow to SHAR.
type LoadWorkflowOutput struct {
	WorkflowID string
}

// CancelProcessInstanceOutput is the output format for cancelling a process.
type CancelProcessInstanceOutput struct {
	Cancelled string
}

// AddTaskOutput is the output format for adding a service task to SHAR.
type AddTaskOutput struct {
	ServiceTaskID string
}

// ListServiceTaskOutput is the output format for listing service tasks.
type ListServiceTaskOutput struct {
	Tasks []*model.TaskSpec
}
