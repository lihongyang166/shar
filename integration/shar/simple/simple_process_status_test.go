package simple

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSimpleProcessStatus(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testSimpleProcessStatsHandlerDef{t: t, finished: make(chan struct{})}

	// Register a service task
	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_process_status_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest"}, b)
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// Launch the workflow
	wi, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	pis, err := cl.ListExecutionProcesses(ctx, wi)
	require.NoError(t, err)
	for _, pi := range pis.ProcessInstanceId {
		ps, err := cl.GetProcessInstanceStatus(ctx, pi)
		require.NoError(t, err)
		assert.Equal(t, "SimpleProcess", *ps[0].Execute)
	}
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 120*time.Second)
}

type testSimpleProcessStatsHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testSimpleProcessStatsHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["localVar"].(int))
	vars["Success"] = true
	time.Sleep(3 * time.Second)
	return vars, nil
}

func (d *testSimpleProcessStatsHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
