package intTest

import (
	"context"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"os"
	"testing"
	time2 "time"
)

func TestProcessPersistenceNonUniqueName(t *testing.T) {
	tst := support.NewIntegrationT(t, nil, nil, false, nil, 60*time2.Second)
	//tst.WithTrace = true

	tst.Setup(t)
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{})}
	err = taskutil.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)

	//load another process with different workflow name but same process name
	pb, err := os.ReadFile("../../testdata/simple-workflow-same-processname.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowSameProcessNameTest", pb)
	require.Error(t, err)
	require.ErrorContains(t, err, "[process: SimpleProcess, workflow: SimpleWorkflowTest]")

}
