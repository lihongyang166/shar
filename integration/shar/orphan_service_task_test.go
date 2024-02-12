package intTest

import (
	"context"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/namespace"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"os"
	"testing"
	"time"
)

func TestRegisterOrphanServiceTask(t *testing.T) {
	tst := support.NewIntegrationT(t, nil, nil, false, nil, nil)
	tst.Setup()
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	require.ErrorContains(t, err, "task SimpleProcess is not registered")

	tst.AssertCleanKV(namespace.Default, t, 60*time.Second)
}
