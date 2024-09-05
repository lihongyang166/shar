package output

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/common"
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
func (c *Json) OutputExecutionStatus(executionID string, states map[string][]*model.WorkflowState) {

	rs := make(map[string][]StateOutput, len(states))
	for pi, sts := range states {
		rsa := make([]StateOutput, 0, len(sts))
		for _, st := range sts {
			rsa = append(rsa, StateOutput{
				TrackingId: common.TrackingID(st.Id).ID(),
				ID:         st.ElementId,
				Type:       st.ElementType,
				State:      st.State.String(),
				Executing:  readStringPtr(st.Execute),
				Since:      st.UnixTimeNano,
			})
		}
		rs[pi] = rsa
	}
	c.outJson(ExecutionOutput{ExecutionId: executionID, Processes: rs})
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
	c.Cmd.Println(string(op))
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
