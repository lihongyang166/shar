package intTest

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestStartingVariable(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testStartingVariableHandlerDef{finished: make(chan struct{})}

	// Register a service task
	_, err = integration_support.RegisterTaskYamlFile(ctx, cl, "starting_variable_test_DummyTask.yaml", d.integrationSimple)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/bad/expects-starting-variable.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleWorkflowTest"})

	assert.Error(t, err)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testStartingVariableHandlerDef struct {
	finished chan struct{}
}

func (d *testStartingVariableHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")

	carried, err := vars.GetInt64("carried")
	if err != nil {
		return nil, fmt.Errorf("get carried: %w", err)
	}
	fmt.Println("carried", carried)
	return vars, nil
}
