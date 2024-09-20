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
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "ParallelGatewayTest", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0ljss15", g.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	newVars := model.NewVars()
	newVars.SetInt64("testValue", 32768)
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_0ljss15", Vars: newVars})
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
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "ParallelJoiningGatewayTest", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("testParallelJoiningGateway-0-0-2-process-1", func(ctx context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
		value3, err := vars.GetInt64("value3")
		require.NoError(g.t, err)
		assert.Equal(g.t, int64(3), value3)
		processEndAssertions(g, vars)
	})
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "testParallelJoiningGateway-0-0-2-process-1", Vars: model.Vars{"testValue": testValueStartVal}})
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
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "ParallelJoiningGatewayMockTest", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("testParallelJoiningGateway-2-0-0-process-1", func(ctx context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
		assert.Equal(g.t, "branch_three_output", vars["sample"])
		branchOne, err := vars.GetString("branch_one")
		require.NoError(g.t, err)
		branchTwo, err := vars.GetString("branch_two")
		require.NoError(g.t, err)
		branchThree, err := vars.GetString("branch_three")
		require.NoError(g.t, err)
		delay, err := vars.GetInt64("delay")
		require.NoError(g.t, err)
		assert.Equal(g.t, "three_output", branchThree)
		assert.Equal(g.t, "two_output", branchTwo)
		assert.Equal(g.t, "one_output", branchOne)
		assert.Equal(g.t, int64(500), delay)
		assert.Equal(g.t, true, g.stg1)
		assert.Equal(g.t, true, g.stg2)
		assert.Equal(g.t, true, g.stg3)
		close(g.finished)
	})

	require.NoError(t, err)
	// Launch the workflow
	newVars := model.NewVars()
	newVars.SetString("name", "name_string")
	newVars.SetInt64("delay", 100)
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "testParallelJoiningGateway-2-0-0-process-1", Vars: newVars})
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
	newVars := model.NewVars()
	newVars.SetInt64("value3", 3)
	return newVars, nil
}

func (g *parallelGatewayTest) parStage2(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	newVars := model.NewVars()
	newVars.SetInt64("value2", 2)
	return newVars, nil
}

func (g *parallelGatewayTest) parStage1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	newVars := model.NewVars()
	newVars.SetInt64("value1", 1)
	return newVars, nil
}

func (g *parallelGatewayTest) parDelayStage3(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 3")
	g.stg3 = true
	newVars := model.NewVars()
	newVars.SetString("sample", "branch_three_output")
	newVars.SetString("branch_three", "three_output")
	return newVars, nil
}

func (g *parallelGatewayTest) parDelayStage2(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	newVars := model.NewVars()
	newVars.SetString("sample", "branch_two_output")
	newVars.SetString("branch_two", "two_output")
	return newVars, nil
}

func (g *parallelGatewayTest) parDelayStage1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	newVars := model.NewVars()
	newVars.SetString("sample", "branch_one_output")
	newVars.SetString("branch_one", "one_output")
	newVars.SetInt64("delay", 500)
	return newVars, nil
}

func (g *parallelGatewayTest) processEnd(ctx context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	processEndAssertions(g, vars)
}

func processEndAssertions(g *parallelGatewayTest, vars model.Vars) {
	value1, err := vars.GetInt64("value1")
	require.NoError(g.t, err)
	value2, err := vars.GetInt64("value2")
	require.NoError(g.t, err)
	assert.Equal(g.t, int64(1), value1)
	assert.Equal(g.t, int64(2), value2)
	assert.Equal(g.t, true, g.stg3)
	close(g.finished)
}
