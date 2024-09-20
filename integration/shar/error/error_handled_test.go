package error

import (
	"context"
	"errors"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/workflow"
	"gitlab.com/shar-workflow/shar/model"
)

func TestHandledError(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := errorHandledHandlerDef{test: t, finished: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "error_handled_test_couldThrowError.yaml", d.mayFail)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "error_handled_test_fixSituation.yaml", d.fixSituation)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/errors.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestHandleError", WorkflowBPMN: b})
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
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.True(t, d.fixed)
}

type errorHandledHandlerDef struct {
	fixed    bool
	test     *testing.T
	finished chan struct{}
}

// A "Hello World" service task
func (d *errorHandledHandlerDef) mayFail(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	// Throw handled error
	return model.Vars{"success": false, "myVar": 69}, &workflow.Error{Code: "101", WrappedError: errors.New("things went badly")}
}

// A "Hello World" service task
func (d *errorHandledHandlerDef) fixSituation(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	testVal1, err := vars.GetInt64("testVal")
	require.NoError(d.test, err)
	carried, err := vars.GetInt64("carried")
	require.NoError(d.test, err)
	assert.Equal(d.test, int64(69), testVal1)
	assert.Equal(d.test, int64(32768), carried)
	d.fixed = true
	return model.Vars{}, nil
}

func (d *errorHandledHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
