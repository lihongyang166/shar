package workflow

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestWorkflowChanged(t *testing.T) {
	tst := support.NewIntegrationT(t, nil, nil, false, func() (bool, string) {
		return !support.IsNatsPersist(), "only valid when NOT persisting to nats"
	}, nil)
	tst.Setup()
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &workflowChangedHandlerDef{t: t, finished: make(chan struct{})}

	// Register a service task
	_, err = support.RegisterTaskYamlFile(ctx, cl, "workflow_changed_SimpleProcess.yaml", d.integrationSimple)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	changed, err := cl.HasWorkflowDefinitionChanged(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)
	assert.True(t, changed)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	changed, err = cl.HasWorkflowDefinitionChanged(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)
	assert.False(t, changed)

	// Load second BPMN workflow
	b, err = os.ReadFile("../../../testdata/simple-workflow-changed.bpmn")
	require.NoError(t, err)
	changed, err = cl.HasWorkflowDefinitionChanged(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)
	assert.True(t, changed)
}

type workflowChangedHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *workflowChangedHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	vars.SetBool("Success", true)
	return vars, nil
}
