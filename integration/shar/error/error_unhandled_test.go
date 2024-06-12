package error

import (
	"context"
	"fmt"
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
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestUnhandledError", b)
	require.NoError(t, err)

	// A hook to watch for completion
	err = cl.RegisterProcessComplete("Process_07lm3kx", d.processEnd)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "Process_07lm3kx", model.Vars{})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// wait for the workflow to complete
	support.WaitForChan(t, d.finished, 30*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testErrorUnhandledHandlerDef struct {
	finished chan struct{}
}

// A "Hello World" service task
func (d *testErrorUnhandledHandlerDef) mayFail(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Throw unhandled error")
	return model.Vars{"success": false}, &workflow.Error{Code: "102"}
}

// A "Hello World" service task
func (d *testErrorUnhandledHandlerDef) fixSituation(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Fixing")
	fmt.Println("carried", vars["carried"])
	return model.Vars{}, nil
}

func (d *testErrorUnhandledHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
