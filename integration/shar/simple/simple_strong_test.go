package simple

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSimpleStrong(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleStrongHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = taskutil.LoadTaskFromYamlFile(ctx, cl, "simple_test.yaml")
	require.NoError(t, err)
	err = client.RegisterTaskWithSpecFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = client.RegisterProcessComplete(ctx, cl, "SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
	require.NoError(t, err)

	go func() {
		tst.TrackingUpdatesFor(ns, executionId, d.trackingReceived, 20*time.Second, t)
	}()

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, d.trackingReceived, 20*time.Second)
	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSimpleStrongHandlerDef struct {
	t                *testing.T
	finished         chan struct{}
	trackingReceived chan struct{}
}

type inParams struct {
	Carried  int `shar:"carried"`
	LocalVar int `shar:"localVar"`
}

type outParams struct {
	Success bool
}

func (d *testSimpleStrongHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, in inParams) (outParams, error) {
	fmt.Println("Hi")
	assert.Equal(d.t, 32768, in.Carried)
	assert.Equal(d.t, 42, in.LocalVar)
	//vars["Success"] = true
	return outParams{
		Success: true,
	}, nil
}

type finalParams struct {
	Carried    int `shar:"carried"`
	ProcessVar int `shar:"processVar"`
}

func (d *testSimpleStrongHandlerDef) processEnd(_ context.Context, params finalParams, _ *model.Error, _ model.CancellationState) {
	assert.Equal(d.t, 32768, params.Carried)
	assert.Equal(d.t, 42, params.ProcessVar)
	close(d.finished)
}
