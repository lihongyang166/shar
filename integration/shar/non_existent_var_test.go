package intTest

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/namespace"
	support "gitlab.com/shar-workflow/shar/integration-support"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"os"
	"testing"
	"time"
)

func TestNonExistentVar(t *testing.T) {
	tst := support.NewIntegrationT(t, nil, nil, false, nil)
	tst.Setup()
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/bad/non-existent-process-variable.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	assert.ErrorIs(t, err, errors2.ErrUndefinedVariable)
	tst.AssertCleanKV(namespace.Default, t, 60*time.Second)
}
