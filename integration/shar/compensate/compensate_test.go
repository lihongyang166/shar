package simple

import (
	"context"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/server/tools/tracer"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestCompensate(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	s := tracer.Trace(tst.NatsURL)
	defer s.Close()

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "task1.yaml", d.task1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "task2.yaml", d.task2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "compensate_task1.yaml", d.compensate1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "compensate_task2.yaml", d.compensate2)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("Compensator", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/compensate.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "Compensator", b)
	require.NoError(t, err)

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, "Compensator", model.Vars{"compensate": 1})
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

type testSimpleHandlerDef struct {
	t                *testing.T
	finished         chan struct{}
	trackingReceived chan struct{}
}

func (d *testSimpleHandlerDef) task1(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("Task1")
	return vars, nil
}

func (d *testSimpleHandlerDef) task2(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("Task2")
	return vars, nil
}
func (d *testSimpleHandlerDef) compensate1(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("Compensate Task1")
	return vars, nil
}
func (d *testSimpleHandlerDef) compensate2(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("Compensate Task2")
	return vars, nil
}
func (d *testSimpleHandlerDef) processEnd(_ context.Context, _ model.Vars, _ *model.Error, _ model.CancellationState) {
	close(d.finished)
}
