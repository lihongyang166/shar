package gateway

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestExclusiveGatewayDecision(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testExclusiveGatewayDecisionDef{t: t, gameResult: "Win", finished: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "exclusive_gateway_test_PlayGame.yaml", d.playGame)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "exclusive_gateway_test_ReceiveTrophy.yaml", d.win)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "exclusive_gateway_test_ReceiveCommiserations.yaml", d.lose)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/exclusive-gateway.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "ExclusiveGatewayTest", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_1k2x28n", d.processEnd)
	require.NoError(t, err)

	// Launch the workflow
	launchVars := model.NewVars()
	launchVars.SetInt64("carried", 32768)
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_1k2x28n", Vars: launchVars})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testExclusiveGatewayDecisionDef struct {
	t          *testing.T
	gameResult string
	finished   chan struct{}
}

func (d *testExclusiveGatewayDecisionDef) playGame(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	vars.SetString("GameResult", d.gameResult)
	return vars, nil
}

func (d *testExclusiveGatewayDecisionDef) win(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	GameResult, err := vars.GetString("GameResult")
	require.NoError(d.t, err)
	assert.Equal(d.t, "Win", GameResult)
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	return vars, nil
}

func (d *testExclusiveGatewayDecisionDef) lose(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	GameResult, err := vars.GetString("GameResult")
	require.NoError(d.t, err)
	assert.Equal(d.t, "Lose", GameResult)
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	vars.SetBool("Success", true)
	return vars, nil
}

func (d *testExclusiveGatewayDecisionDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
