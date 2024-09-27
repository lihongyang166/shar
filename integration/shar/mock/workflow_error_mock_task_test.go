package simple

import (
	"context"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestWorkflowErrorMockTask(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testWorkflowErrorMockTaskHandlerDef{t: t, finished: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_mock_task_test.yaml", nil)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow-mock-task.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	newVars := model.NewVars()
	newVars.SetInt64("carried", 32768)
	newVars.SetString("errorCode", "101")
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess", Vars: newVars})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testWorkflowErrorMockTaskHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testWorkflowErrorMockTaskHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	processVar, err := vars.GetInt64("processVar")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	assert.Equal(d.t, int64(42), processVar)
	assert.Equal(d.t, "101", wfError.Code)
}
