package telemetry

import (
	"context"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"gitlab.com/shar-workflow/shar/common/namespace"

	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSimpleTelemetryServerTrace(t *testing.T) {
	tst := &support.Integration{}
	tst.WithTrace = false

	tst.Setup()
	defer tst.Teardown()

	// Create a starting context

	ctx := context.Background()
	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithOpenTelemetry()) // client.Experimental_WithNamespace("fooNS"),

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleTelemetrySvrTraceHandlerDef{t: t, finished: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(namespace.Default, t, 120)
}

type testSimpleTelemetrySvrTraceHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testSimpleTelemetrySvrTraceHandlerDef) integrationSimple(ctx context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	// sc := trace.SpanContextFromContext(ctx)
	// assert.True(d.t, sc.IsValid(), "Invalid span context")
	// assert.Equal(d.t, int64(32768), vars["carried"].(int))
	// assert.Equal(d.t, int64(42), vars["localVar"].(int))
	vars.SetBool("Success", true)
	return vars, nil
}

func (d *testSimpleTelemetrySvrTraceHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	// sc := trace.SpanContextFromContext(ctx)
	// assert.True(d.t, sc.IsValid(), "Invalid span context")
	close(d.finished)
}
