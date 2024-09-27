package error

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/server/tools/tracer"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/workflow"
	"gitlab.com/shar-workflow/shar/model"
)

func TestUnhandledError(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)
	sub := tracer.Trace(tst.NatsURL)
	defer sub.Close()
	d := &testErrorUnhandledHandlerDef{finished: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "error_handled_test_couldThrowError.yaml", d.mayFail)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "error_handled_test_fixSituation.yaml", d.fixSituation)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/errors.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestUnhandledError", WorkflowBPMN: b})
	require.NoError(t, err)

	// A hook to watch for completion
	err = cl.RegisterProcessComplete("Process_07lm3kx", d.processEnd)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_07lm3kx"})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// wait for the workflow to complete
	support.WaitForChan(t, d.finished, 30*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.False(t, d.fixed)
}

type testErrorUnhandledHandlerDef struct {
	finished chan struct{}
	fixed    bool
}

// A "Hello World" service task
func (d *testErrorUnhandledHandlerDef) mayFail(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Throw unhandled error")
	retVars := model.NewVars()
	retVars.SetBool("success", false)
	return retVars, &workflow.Error{Code: "102"}
}

// A "Hello World" service task
func (d *testErrorUnhandledHandlerDef) fixSituation(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Fixing")
	carried, err := vars.GetInt64("carried")
	if err != nil {
		return nil, fmt.Errorf("get carried: %w", err)
	}
	fmt.Println("carried", carried)
	d.fixed = true
	return model.NewVars(), nil
}

func (d *testErrorUnhandledHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
