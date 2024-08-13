package intTest

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSubWorkflow(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testSubWorkflowHandlerDef{finished: make(chan struct{}), t: t}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "sub_workflow_test_BeforeCallingSubProcess.yaml", d.beforeCallingSubProcess)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "sub_workflow_test_DuringSubProcess.yaml", d.duringSubProcess)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "sub_workflow_test_AfterCallingSubProcess.yaml", d.afterCallingSubProcess)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("WorkflowDemo", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflows
	w1, err := os.ReadFile("../../testdata/sub-workflow-parent.bpmn")
	require.NoError(t, err)
	w2, err := os.ReadFile("../../testdata/sub-workflow-child.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "MasterWorkflowDemo", w1)
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SubWorkflowDemo", w2)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "WorkflowDemo", model.Vars{})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSubWorkflowHandlerDef struct {
	finished chan struct{}
	t        *testing.T
}

func (d *testSubWorkflowHandlerDef) afterCallingSubProcess(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println(vars["x"])
	fmt.Println("carried", vars["carried"])
	return model.Vars{}, nil
}

func (d *testSubWorkflowHandlerDef) duringSubProcess(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	x := vars["z"].(int)
	return model.Vars{"z": x + 41}, nil
}

func (d *testSubWorkflowHandlerDef) beforeCallingSubProcess(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	return model.Vars{"x": 1}, nil
}

func (d *testSubWorkflowHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	assert.Equal(d.t, 42, vars["x"])
	close(d.finished)
}
