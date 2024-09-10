package output

import (
	"gitlab.com/shar-workflow/shar/model"
)

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
	Process     []ProcessInstanceOutput
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

// ProcessStatusListOutput is the output format for displaying activities and tasks in a process.
type ProcessStatusListOutput struct {
	ItemType          string   `json:"itemType,omitempty"`
	ElementId         string   `json:"elementId,omitempty"`
	ElementName       string   `json:"elementName,omitempty"`
	CancellationState string   `json:"cancellationState,omitempty"`
	UnixTimeNano      int64    `json:"unixTimeNano,omitempty"`
	Execute           string   `json:"execute,omitempty"`
	Id                []string `json:"id,omitempty"`
	Compensating      bool     `json:"compensating,omitempty"`
	PreviousActivity  string   `json:"previousActivity,omitempty"` // PreviousActivity - the ID of the last activity
	PreviousElement   string   `json:"previousElement,omitempty"`  // PreviousElement - the ID of the last element.
}

// ProcessInstanceOutput is the output format for displaying processes
type ProcessInstanceOutput struct {
	ProcessId         string                    `json:"processId,omitempty"`
	ProcessInstanceId string                    `json:"processInstanceId,omitempty"`
	WorkflowName      string                    `json:"workflowName,omitempty"`
	WorkflowId        string                    `json:"workflowId,omitempty"`
	ExecutionId       string                    `json:"executionId,omitempty"`
	State             []ProcessStatusListOutput `json:"state,omitempty"`
}
