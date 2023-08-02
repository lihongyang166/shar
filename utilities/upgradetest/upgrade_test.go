package upgradetest

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
)

func Test_SHAR_upgrade(t *testing.T) {
	ctx := context.Background()
	sharC, natsC, nw, err := getContainers(ctx, "2.9.20", "1.0.451")
	if err != nil {
		t.Error(err)
	}

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	endpoint, err := natsC.Endpoint(ctx, "")
	require.NoError(t, err)
	err = cl.Dial(ctx, endpoint)
	require.NoError(t, err)

	defer func() {
		if err := killContainers(ctx, sharC, natsC, nw); err != nil {
			t.Error(err)
		}
	}()
}

type testSimpleHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testSimpleHandlerDef) integrationSimple(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["localVar"].(int))
	vars["Success"] = true
	return vars, nil
}

func (d *testSimpleHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
