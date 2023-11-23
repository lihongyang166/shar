package intTest

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
)

func TestTelemetryStream(t *testing.T) {
	tst := &support.Integration{}
	//tst.WithTrace = true

	tst.Setup(t, nil, nil)
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testTelemetryStreamDef{t: t, finished: make(chan struct{})}

	err = taskutil.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	js, err := tst.GetJetstream()
	require.NoError(t, err)
	sub, err := js.Subscribe("WORKFLOW-TELEMETRY.>", func(msg *nats.Msg) {
		fmt.Println(msg.Subject)
		msg.Ack()
	})
	require.NoError(t, err)
	defer sub.Drain()

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV()
}

type testTelemetryStreamDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testTelemetryStreamDef) integrationSimple(ctx context.Context, client client.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	client.Log(ctx, slog.LevelInfo, "Info message logged from client: integration simple", map[string]string{"value1": "good"})
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["localVar"].(int))
	vars["Success"] = true
	return vars, nil
}

func (d *testTelemetryStreamDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
