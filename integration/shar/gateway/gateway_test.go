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

func TestExclusiveRun(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)

	require.NoError(t, err)

	g := &gatewayTest{finished: make(chan struct{}), t: t, typ: model.GatewayType_exclusive}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage1.yaml", g.excStage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage2.yaml", g.excStage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage3.yaml", g.stage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-exclusive-out-and-in-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "ExclusiveGatewayTest"}, b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0ljss15", g.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_0ljss15", Vars: model.Vars{"carried": 32768}})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, g.finished, time.Second*20)
	tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.NotEqual(t, g.stg1, g.stg2)
	assert.True(t, g.stg3)
}

func TestInclusiveRun(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)

	require.NoError(t, err)

	g := &gatewayTest{finished: make(chan struct{}), t: t, typ: model.GatewayType_inclusive}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage1.yaml", g.incStage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage2.yaml", g.incStage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage3.yaml", g.stage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-inclusive-out-and-in-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "InclusiveGatewayTest"}, b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0ljss15", g.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_0ljss15", Vars: model.Vars{"testValue": 32768}})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, g.finished, 20*time.Second)
	assert.True(t, g.stg1)
	assert.True(t, g.stg2)
	assert.True(t, g.stg3)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type gatewayTest struct {
	finished chan struct{}
	t        *testing.T
	stg3     bool
	stg2     bool
	stg1     bool
	typ      model.GatewayType
}

func (g *gatewayTest) stage3(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Stage 3")
	g.stg3 = true
	return vars, nil
}

func (g *gatewayTest) excStage2(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	return model.Vars{"value2": 2, "value1": nil}, nil
}

func (g *gatewayTest) excStage1(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	return model.Vars{"value1": 1, "value2": nil}, nil
}

func (g *gatewayTest) incStage2(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	return model.Vars{"value2": 22, "value1": 11}, nil
}

func (g *gatewayTest) incStage1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	return model.Vars{"value1": 1, "value2": 2}, nil
}

func (g *gatewayTest) processEnd(ctx context.Context, vars model.Vars, _ *model.Error, state model.CancellationState) {
	switch g.typ {
	case model.GatewayType_inclusive:
		assert.True(g.t, 1 == vars["value1"] || 11 == vars["value1"])
		assert.True(g.t, 2 == vars["value2"] || 22 == vars["value2"])
		assert.Equal(g.t, true, g.stg3)
	case model.GatewayType_exclusive:
		assert.True(g.t, vars["value1"] != vars["value2"], "values are equal")
		assert.True(g.t, vars["value1"] == 1 || vars["value2"] == 2, "both values are present")
		assert.True(g.t, vars["value1"] == nil || vars["value2"] == nil, "both values are nil")
		assert.Equal(g.t, true, g.stg3)
	}
	close(g.finished)
}
