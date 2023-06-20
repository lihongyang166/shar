package intTest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/cli/commands"
	"gitlab.com/shar-workflow/shar/cli/flag"
	"gitlab.com/shar-workflow/shar/cli/output"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/element"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"strings"
	"testing"
	"time"
)

func TestCLI(t *testing.T) {
	tst := &support.Integration{}
	tst.Setup(t, nil, nil)
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testLaunchWorkflow{t: t, allowContinue: make(chan interface{}), finished: make(chan struct{})}

	// Register a service task
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)
	err = cl.RegisterServiceTask(ctx, "SimpleProcess", d.integrationSimple)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	flag.Value.Server = tst.NatsURL
	flag.Value.Json = true

	// Load Workflow
	wf := &struct {
		WorkflowID string
	}{}
	sharExecf(t, wf, "bpmn load SimpleWorkflow ../../testdata/simple-workflow.bpmn --server %s --json", tst.NatsURL)
	assert.NotEmpty(t, wf.WorkflowID)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	js, err := tst.GetJetstream()
	require.NoError(t, err)
	l := js.Consumers("WORKFLOW")
	for i := range l {
		fmt.Println(i.Name)
	}
	// List Workflows
	wfs := &struct {
		Workflow []*model.ListWorkflowResult
	}{}
	sharExecf(t, wfs, "workflow list --server %s --json", tst.NatsURL)
	assert.Equal(t, 1, len(wfs.Workflow))
	assert.Equal(t, "SimpleWorkflow", wfs.Workflow[0].Name)
	assert.Equal(t, int32(1), wfs.Workflow[0].Version)

	// Start Workflow
	wfi := &struct {
		WorkflowInstanceID string
	}{}
	sharExecf(t, wfi, "workflow start SimpleWorkflow --server %s --json", tst.NatsURL)
	assert.NotEmpty(t, wfi.WorkflowInstanceID)

	time.Sleep(3 * time.Second)
	// Get Workflow Instances
	instances := &struct {
		WorkflowInstance []model.ListWorkflowInstanceResult
	}{}
	sharExecf(t, &instances, "instance list SimpleWorkflow --server %s --json", tst.NatsURL)
	assert.Equal(t, 1, len(instances.WorkflowInstance))
	assert.Equal(t, wfi.WorkflowInstanceID, instances.WorkflowInstance[0].Id)

	fmt.Println()

	//Get Workflow Instance Status
	type retState struct {
		TrackingId string
		ID         string
		Type       string
		State      string
		Executing  string
		Since      int64
	}
	// Get Workflow Instances
	status := struct {
		InstanceId string
		Processes  map[string][]retState
	}{}

	var firstProcess retState

	sharExecf(t, &status, "instance status %s --server %s --json", wfi.WorkflowInstanceID, tst.NatsURL)

	for _, i := range status.Processes {
		firstProcess = i[len(i)-1]
		break
	}

	assert.NotEmpty(t, firstProcess.ID)
	assert.Equal(t, "Step1", firstProcess.ID)
	assert.Equal(t, element.ServiceTask, firstProcess.Type)
	assert.Equal(t, "executing", firstProcess.State)
	assert.Equal(t, "SimpleProcess", firstProcess.ID)
	assert.NotZero(t, firstProcess.Since)

	// Allow workflow to continue
	close(d.allowContinue)

	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV()
}

func sharExecf[T any](t *testing.T, ret *T, command string, args ...any) {
	res := bytes.NewBuffer(make([]byte, 0))
	output.Stream = res
	commands.RootCmd.SetArgs(strings.Split(fmt.Sprintf(command, args...), " "))
	err := commands.RootCmd.Execute()
	require.NoError(t, err)
	err = json.Unmarshal(res.Bytes(), ret)
	require.NoError(t, err)
}

type testLaunchWorkflow struct {
	t             *testing.T
	allowContinue chan interface{}
	finished      chan struct{}
}

func (d *testLaunchWorkflow) integrationSimple(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	<-d.allowContinue
	assert.Equal(d.t, 32768, vars["carried"].(int))
	vars["Success"] = true
	return vars, nil
}

func (d *testLaunchWorkflow) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
