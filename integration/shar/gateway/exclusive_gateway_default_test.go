package gateway

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

func TestExclusiveGatewayDefault(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testExclusiveGatewayDefaultDef{t: t, gameResult: "Win", finished: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "exclusive_gateway_default_Default.yaml", d.defaultOption)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "exclusive_gateway_default_Option1.yaml", d.option1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "exclusive_gateway_default_Option2.yaml", d.option2)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/exclusive-gateway-default.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "ExclusiveGatewayDefaultTest"}, b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("GatewayTest", d.processEnd)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "GatewayTest", Vars: model.Vars{"val1": 2}})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.True(t, d.opt2)
	assert.False(t, d.opt1)
}

type testExclusiveGatewayDefaultDef struct {
	t          *testing.T
	gameResult string
	finished   chan struct{}
	opt2       bool
	opt1       bool
}

func (d *testExclusiveGatewayDefaultDef) defaultOption(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Default Triggered")
	return vars, nil
}

func (d *testExclusiveGatewayDefaultDef) option1(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Option 1")
	d.opt1 = true
	return vars, nil
}

func (d *testExclusiveGatewayDefaultDef) option2(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Option 2")
	d.opt2 = true
	return vars, nil
}

func (d *testExclusiveGatewayDefaultDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
