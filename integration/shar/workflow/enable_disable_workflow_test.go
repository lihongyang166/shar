package workflow

import (
	"context"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/task"
	integration_support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"os"
	"testing"
	"time"
)

func TestDisableEnableLaunchWorkflow(t *testing.T) {
	// create client
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &enableDisableWorkflowHandlerDef{finished: make(chan struct{})}
	_, err = integration_support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", simpleTestSvcTaskFn)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	wfName := "SimpleWorkflowTest"
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: wfName, WorkflowBPMN: b})
	require.NoError(t, err)

	// register completion listener
	processId := "SimpleProcess"
	err = cl.RegisterProcessComplete(processId, d.processCompleteFn)
	require.NoError(t, err)

	// start client listener
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// disable workflow
	err = cl.DisableWorkflowExecution(ctx, wfName)
	require.NoError(t, err)

	// launch workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: processId})
	require.ErrorContains(t, err, "the workflow is not executable")

	//enable wf
	err = cl.EnableWorkflowExecution(ctx, wfName)
	require.NoError(t, err)
	//launch wf
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: processId})
	require.NoError(t, err)

	integration_support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 20*time.Second)
}

type enableDisableWorkflowHandlerDef struct {
	finished chan struct{}
}

func (d *enableDisableWorkflowHandlerDef) processCompleteFn(_ context.Context, _ model.Vars, _ *model.Error, _ model.CancellationState) {
	d.finished <- struct{}{}
}

func simpleTestSvcTaskFn(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	return vars, nil
}
