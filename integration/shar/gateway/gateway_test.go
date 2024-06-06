package gateway

import (
	"bytes"
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
	"gitlab.com/shar-workflow/shar/client/parser"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/model"
)

func TestExclusiveParse(t *testing.T) {
	t.Parallel()

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-exclusive-out-and-in-test.bpmn")
	require.NoError(t, err)

	wf, err := parser.Parse("SimpleWorkflowTest", bytes.NewBuffer(b))
	require.NoError(t, err)

	els := make(map[string]*model.Element)
	common.IndexProcessElements(wf.Process["Process_0ljss15"].Elements, els)
	assert.Equal(t, model.GatewayType_exclusive, els["Gateway_01xjq2a"].Gateway.Type)
	assert.Equal(t, model.GatewayDirection_divergent, els["Gateway_01xjq2a"].Gateway.Direction)
	assert.Equal(t, model.GatewayType_exclusive, els["Gateway_1ps8xyt"].Gateway.Type)
	assert.Equal(t, model.GatewayDirection_convergent, els["Gateway_1ps8xyt"].Gateway.Direction)
	assert.Equal(t, "Gateway_01xjq2a", els["Gateway_1ps8xyt"].Gateway.ReciprocalId)
	assert.Equal(t, "Gateway_1ps8xyt", els["Gateway_01xjq2a"].Gateway.ReciprocalId)
}

func TestNestedExclusiveParse(t *testing.T) {
	t.Parallel()

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-multi-exclusive-out-and-in-test.bpmn")
	require.NoError(t, err)

	wf, err := parser.Parse("SimpleWorkflowTest", bytes.NewBuffer(b))
	require.NoError(t, err)

	els := make(map[string]*model.Element)
	common.IndexProcessElements(wf.Process["Process_0ljss15"].Elements, els)
	assert.Equal(t, "Gateway_0bcqcrc", els["Gateway_1ucd1b5"].Gateway.ReciprocalId)
	assert.Equal(t, "Gateway_1ps8xyt", els["Gateway_01xjq2a"].Gateway.ReciprocalId)
}

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
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage1.yaml", g.stage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage2.yaml", g.stage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage3.yaml", g.stage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-exclusive-out-and-in-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "ExclusiveGatewayTest", b)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0ljss15", g.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "Process_0ljss15", model.Vars{"carried": 32768})
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
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage1.yaml", g.stage1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage2.yaml", g.stage2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "gateway_test_stage3.yaml", g.stage3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/gateway-inclusive-out-and-in-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "InclusiveGatewayTest", b)
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

func (g *gatewayTest) stage2(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Stage 2")
	g.stg2 = true
	return model.Vars{"value2": 2}, nil
}

func (g *gatewayTest) stage1(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Stage 1")
	g.stg1 = true
	return model.Vars{"value1": 1}, nil
}

func (g *gatewayTest) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	switch g.typ {
	case model.GatewayType_inclusive:
		assert.Equal(g.t, 1, vars["value1"])
		assert.Equal(g.t, 2, vars["value2"])
		assert.Equal(g.t, true, g.stg3)
	case model.GatewayType_exclusive:
		assert.True(g.t, vars["value1"] != vars["value2"], "values are equal")
		assert.True(g.t, vars["value1"] == 1 || vars["value2"] == 2, "both values are present")
		assert.True(g.t, vars["value1"] == nil || vars["value2"] == nil, "both values are nil")
		assert.Equal(g.t, true, g.stg3)
	}
	close(g.finished)
}
