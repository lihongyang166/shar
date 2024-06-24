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

func TestParallelGateway(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)

	require.NoError(t, err)

	g := &parallelGatewayTest{finished: make(chan struct{}), t: t}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage1.yaml", g.parStage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage2.yaml", g.parStage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage3.yaml", g.parStage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-parallel-out-and-in-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "ParallelGatewayTest", b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0ljss15", g.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "Process_0ljss15", model.Vars{"testValue": 32768})
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

const testValueStartVal = 32768

func TestParallelJoiningGateway(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)

	require.NoError(t, err)

	g := &parallelGatewayTest{finished: make(chan struct{}), t: t}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_joining_test_stage1.yaml", g.parStage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_joining_test_stage2.yaml", g.parStage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_joining_test_stage3.yaml", g.parStage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/test-parallel-joining-gateway-diagram.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "ParallelJoiningGatewayTest", b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("testParallelJoiningGateway-0-0-2-process-1", func(ctx context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
		assert.True(g.t, 3 == vars["value3"])
		processEndAssertions(g, vars)
	})
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "testParallelJoiningGateway-0-0-2-process-1", model.Vars{"testValue": testValueStartVal})
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

func TestParallelJoiningGatewayWithDelay(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)

	require.NoError(t, err)

	g := &parallelGatewayTest{finished: make(chan struct{}), t: t}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_joining_test_mock_stage1.yaml", g.parDelayStage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_joining_test_mock_stage2.yaml", g.parDelayStage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_joining_test_mock_stage3.yaml", g.parDelayStage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/test-parallel-joining-gateway-2-0-0-diagram.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "ParallelJoiningGatewayMockTest", b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("testParallelJoiningGateway-2-0-0-process-1", func(ctx context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
		//assert.Equal(g.t, "branch_three_output", vars["sample"])
		assert.Equal(g.t, "three_output", vars["branch_three"])
		assert.Equal(g.t, "two_output", vars["branch_two"])
		assert.Equal(g.t, "one_output", vars["branch_one"])
		assert.Equal(g.t, 500, vars["delay"])
		assert.Equal(g.t, true, g.stg1)
		assert.Equal(g.t, true, g.stg2)
		assert.Equal(g.t, true, g.stg3)
		close(g.finished)
	})

	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "testParallelJoiningGateway-2-0-0-process-1", model.Vars{"name": "name_string", "delay": 100})
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

type parallelGatewayTest struct {
	finished chan struct{}
	t        *testing.T
	stg3     bool
	stg2     bool
	stg1     bool
}

func (g *parallelGatewayTest) parStage3(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 3")
	g.stg3 = true
	return model.Vars{"value3": 3}, nil
}

func (g *parallelGatewayTest) parStage2(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	return model.Vars{"value2": 2}, nil
}

func (g *parallelGatewayTest) parStage1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	return model.Vars{"value1": 1}, nil
}

func (g *parallelGatewayTest) parDelayStage3(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 3")
	g.stg3 = true
	return model.Vars{"sample": "branch_three_output", "branch_three": "three_output"}, nil
}

func (g *parallelGatewayTest) parDelayStage2(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	return model.Vars{"sample": "branch_two_output", "branch_two": "two_output"}, nil
}

func (g *parallelGatewayTest) parDelayStage1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	return model.Vars{"sample": "branch_one_output", "branch_one": "one_output", "delay": 500}, nil
}

func (g *parallelGatewayTest) processEnd(ctx context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	processEndAssertions(g, vars)
}

func processEndAssertions(g *parallelGatewayTest, vars model.Vars) {
	assert.True(g.t, 1 == vars["value1"])
	assert.True(g.t, 2 == vars["value2"])
	assert.Equal(g.t, true, g.stg3)
	close(g.finished)
}
