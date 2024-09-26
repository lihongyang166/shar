package full

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/cli/output"
	"golang.org/x/exp/slices"
	"testing"
	"time"
)

func TestExample(t *testing.T) {

	r1 := cmd[output.WorkflowListOutput](t, "shar workflow list")
	assert.Len(t, r1.Workflow, 0)
	r2 := cmd[output.AddTaskOutput](t, "shar servicetask add ../../../integration/shar/simple/simple_test.yaml")
	assert.NotEmpty(t, r2)
	r3 := cmd[output.ListServiceTaskOutput](t, "shar servicetask list")
	assert.Len(t, r3.Tasks, 1)
	assert.Equal(t, r3.Tasks[0].Metadata.Uid, r2.ServiceTaskID)
	r4 := cmd[output.LoadWorkflowOutput](t, `shar bpmn load SimpleWorkflowTest ../../../testdata/simple-workflow.bpmn`)
	assert.NotEmpty(t, r4.WorkflowID)
	r5 := cmd[output.WorkflowListOutput](t, "shar workflow list")
	assert.Equal(t, "SimpleWorkflowTest", r5.Workflow[0].Name)
	r6 := cmd[output.ListExecutionOutput](t, "shar execution list SimpleWorkflowTest")
	assert.Len(t, r6.Execution, 0)
	r7 := cmd[output.StartWorkflowResult](t, "shar workflow start SimpleProcess")
	assert.NotEmpty(t, r7.WorkflowID)
	r8 := cmd[output.ListExecutionOutput](t, "shar execution list SimpleWorkflowTest")
	assert.Len(t, r8.Execution, 1)
	var r9 output.ExecutionOutput
	var pIDs []output.ProcessInstanceOutput
	var pid0 output.ProcessInstanceOutput

	ts := time.Now()
	for {
		r9 = cmd[output.ExecutionOutput](t, "shar execution status "+r7.ExecutionID)
		pIDs = r9.Process
		pid0 = pIDs[0]
		if len(pid0.State) < 2 {
			if time.Since(ts) > 20*time.Second {
				assert.Fail(t, "Timed out")
				break
			}
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}

	assert.Len(t, pIDs, 1)
	assert.Len(t, pid0.State, 2)
	ix := slices.IndexFunc(pid0.State, func(listOutput output.ProcessStatusListOutput) bool {
		fmt.Println(listOutput.ItemType)
		return listOutput.ItemType == "jobExecute"
	})
	assert.NotEqual(t, -1, ix)
	assert.Equal(t, "executing", pid0.State[ix].CancellationState)
}
