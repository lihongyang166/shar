package simple

import (
	"context"
	"fmt"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestListExecutableProcess(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testListExecutableProcessHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	res, err := cl.ListExecutableProcesses(ctx)
	require.NoError(t, err)
	assert.Len(t, res, 1)
}

type testListExecutableProcessHandlerDef struct {
	t                *testing.T
	finished         chan struct{}
	trackingReceived chan struct{}
}

func (d *testListExecutableProcessHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	localVar, err := vars.GetInt64("localVar")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(42), localVar)
	vars.SetBool("Success", true)
	return vars, nil
}

func (d *testListExecutableProcessHandlerDef) processEnd(_ context.Context, _ model.Vars, _ *model.Error, _ model.CancellationState) {
	close(d.finished)
}
