package simple

import (
	"context"
	"fmt"
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

func TestMockServiceTaskWithSwimlane(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "swimlane_test_task1.yaml", nil)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "swimlane_test_task2.yaml", nil)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("testSwimlaneProcess-0-0-5-process-2", d.processEnd)
	require.NoError(t, err)

	err = cl.RegisterMessageSender(ctx, "swimlane_test", "continueMessage", d.sendMessage)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("swimlane_test.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "swimlane_test"}, b)
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "testSwimlaneProcess-0-0-5-process-1", Vars: model.Vars{"orderId": "dummyOrder"}})
	require.NoError(t, err)

	go func() {
		tst.TrackingUpdatesFor(ns, executionId, d.trackingReceived, 20*time.Second, t)
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

func (d *testSimpleHandlerDef) processEnd(_ context.Context, _ model.Vars, _ *model.Error, _ model.CancellationState) {
	close(d.finished)
}

func (x *testSimpleHandlerDef) sendMessage(ctx context.Context, cmd task.MessageClient, vars model.Vars) error {
	if err := cmd.SendMessage(ctx, "continueMessage", vars["orderId"], model.Vars{"carried": vars["carried"], "orderId": vars["orderId"]}); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}
