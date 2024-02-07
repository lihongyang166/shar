package intTest

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"testing"
	time2 "time"
)

func TestRegisterNil(t *testing.T) {
	tst := support.NewIntegrationT(t, nil, nil, false, nil, 60*time2.Second)
	//tst.WithTrace = true

	tst.Setup()
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task

	err = taskutil.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", nil)
	assert.NoError(t, err)
}
