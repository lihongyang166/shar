package output

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/model"
)

// Json contains the output methods for returning json CLI responses
type Json struct {
	Cmd *cobra.Command
}

// SetCmd sets the active command on the output processor
func (c *Json) SetCmd(command *cobra.Command) {
	c.Cmd = command
}

// OutputStartWorkflowResult returns a CLI response
func (c *Json) OutputStartWorkflowResult(executionID string, wfID string) {
	c.outJson(StartWorkflowResult{
		ExecutionID: executionID,
		WorkflowID:  wfID,
	})
}

// OutputWorkflow returns a CLI response
func (c *Json) OutputWorkflow(res []*model.ListWorkflowResponse) {
	c.outJson(WorkflowListOutput{
		Workflow: res,
	})
}

// OutputListExecution returns a CLI response
func (c *Json) OutputListExecution(res []*model.ListExecutionItem) {
	c.outJson(ListExecutionOutput{
		Execution: res,
	})
}

// OutputUserTaskIDs returns a CLI response
func (c *Json) OutputUserTaskIDs(ut []*model.GetUserTaskResponse) {
	c.outJson(UserTaskIDsOutput{
		UserTasks: ut,
	})
}

// OutputExecutionStatus outputs an execution status to console
func (c *Json) OutputExecutionStatus(executionID string, processes map[string][]*model.ProcessHistoryEntry) {
	rs := make([]ProcessInstanceOutput, 0, len(processes))

	for pi, sts := range processes {
		pio := ProcessInstanceOutput{
			ProcessId:         "",
			ProcessInstanceId: pi,
			WorkflowName:      "",
			WorkflowId:        "",
			ExecutionId:       "",
			State:             make([]ProcessStatusListOutput, len(sts)),
		}
		for i, st := range sts {
			if i == 0 {
				pio.ProcessId = st.ProcessId
				pio.WorkflowName = st.WorkflowName
				pio.ProcessInstanceId = *st.ProcessInstanceId
				pio.WorkflowId = *st.WorkflowId
				pio.ExecutionId = *st.ExecutionId
			}
			pio.State[i] = ProcessStatusListOutput{
				ItemType:          model.ProcessHistoryType.String(st.ItemType),
				ElementId:         *st.ElementId,
				ElementName:       *st.ElementName,
				CancellationState: model.CancellationState.String(*st.CancellationState),
				UnixTimeNano:      st.UnixTimeNano,
				Execute:           *st.Execute,
				Id:                st.Id,
				Compensating:      st.Compensating,
				PreviousActivity:  st.PreviousActivity,
				PreviousElement:   st.PreviousElement,
			}
		}
		rs = append(rs, pio)
	}
	c.outJson(ExecutionOutput{ExecutionId: executionID, Process: rs})
}

// OutputLoadResult returns a CLI response
func (c *Json) OutputLoadResult(workflowID string) {
	c.outJson(LoadWorkflowOutput{
		WorkflowID: workflowID,
	})
}

// OutputCancelledProcessInstance returns a CLI response
func (c *Json) OutputCancelledProcessInstance(id string) {
	c.outJson(CancelProcessInstanceOutput{
		Cancelled: id,
	})
}

func (c *Json) outJson(js interface{}) {
	op, err := json.Marshal(&js)
	if err != nil {
		panic(err)
	}
	if _, err := Stream.Write(op); err != nil {
		panic(err)
	}
	c.Cmd.Println(string(op) + "\n")
}

// OutputAddTaskResult returns a CLI response
func (c *Json) OutputAddTaskResult(taskID string) {
	c.outJson(AddTaskOutput{
		ServiceTaskID: taskID,
	})
}

// OutputServiceTasks returns a CLI response
func (c *Json) OutputServiceTasks(res []*model.TaskSpec) {
	ret := ListServiceTaskOutput{
		Tasks: res,
	}
	c.outJson(ret)
}
