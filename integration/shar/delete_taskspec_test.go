package intTest

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
	"time"
)

func TestDeleteTaskSpecSimple(t *testing.T) {
	tst := &support.Integration{}
	//tst.WithTrace = true

	tst.Setup(t, nil, nil)
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testDeleteTaskHandlerDef{t: t}

	err = taskutil.RegisterTaskYamlFile(ctx, cl, "delete_taskspec_test.yaml", d.integrationSimple)
	require.NoError(t, err)

	res, err := cl.UnregisterTask(ctx, "SimpleProcess")
	require.NoError(t, err)

	fmt.Println(res)

	tst.AssertCleanKV()
}

type testDeleteTaskHandlerDef struct {
	t *testing.T
}

func (d *testDeleteTaskHandlerDef) integrationSimple(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	time.Sleep(5 * time.Second)
	return vars, nil
}
