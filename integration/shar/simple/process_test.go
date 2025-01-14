package simple

import (
	"context"
	"gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
)

func TestProcessPersistenceNonUniqueName(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{})}
	_, err = integration_support.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// load another process with different workflow name but same process name
	pb, err := os.ReadFile("../../../testdata/simple-workflow-same-processid.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowSameProcessIdTest", WorkflowBPMN: pb})
	require.Error(t, err)
	require.ErrorContains(t, err, "[process: SimpleProcess, workflow: SimpleWorkflowTest]")
}
