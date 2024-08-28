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

func TestSimpleHeaders(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleLaunchHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	processHeaderName := "testheader"
	processHeaderVal := "testval"
	executionId, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess",
		LaunchHeaders: map[string]string{processHeaderName: processHeaderVal}})
	require.NoError(t, err)

	listExecutionProcessesResponse, err := cl.ListExecutionProcesses(ctx, executionId)
	require.NoError(t, err)

	require.Equal(t, 1, len(listExecutionProcessesResponse.ProcessInstanceId))
	headers, err := cl.GetProcessInstanceHeaders(ctx, listExecutionProcessesResponse.ProcessInstanceId[0])
	require.NoError(t, err)

	actualProcessHeaderVal, hasProcessHeader := headers[processHeaderName]

	assert.True(t, hasProcessHeader)
	assert.Equal(t, processHeaderVal, actualProcessHeaderVal)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSimpleLaunchHandlerDef struct {
	t                *testing.T
	finished         chan struct{}
	trackingReceived chan struct{}
}

func (d *testSimpleLaunchHandlerDef) integrationSimple(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["localVar"].(int))
	vars["Success"] = true
	hdr, err := client.Headers(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get headers: %w", err)
	}
	assert.Equal(d.t, "testval", hdr["testheader"])
	return vars, nil
}

func (d *testSimpleLaunchHandlerDef) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["processVar"].(int))
	close(d.finished)
}
