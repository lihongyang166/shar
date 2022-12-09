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
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"strings"
	"testing"
	"time"
)

func TestCLI(t *testing.T) {
	tst := &support.Integration{}
	tst.Setup(t)
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(support.NatsURL)
	require.NoError(t, err)

	complete := make(chan *model.WorkflowInstanceComplete, 100)

	d := &testLaunchWorkflo{t: t, allowContinue: make(chan interface{})}

	// Register a service task
	cl.RegisterWorkflowInstanceComplete(complete)
	err = cl.RegisterServiceTask(ctx, "SimpleProcess", d.integrationSimple)
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	flag.Value.Server = support.NatsURL
	flag.Value.Json = true

	// Load Workflow
	wf := &struct {
		WorkflowID string
	}{}
	sharExecf(t, wf, "bpmn load SimpleWorkflow ../../testdata/simple-workflow.bpmn --server %s --json", support.NatsURL)
	assert.NotEmpty(t, wf.WorkflowID)

	// List Workflows
	wfs := &struct {
		Workflow []*model.ListWorkflowResult
	}{}
	sharExecf(t, wfs, "workflow list --server %s --json", support.NatsURL)
	assert.Equal(t, 1, len(wfs.Workflow))
	assert.Equal(t, "SimpleWorkflow", wfs.Workflow[0].Name)
	assert.Equal(t, int32(1), wfs.Workflow[0].Version)

	// Start Workflow
	wfi := &struct {
		WorkflowInstanceID string
	}{}
	sharExecf(t, wfi, "workflow start SimpleWorkflow --server %s --json", support.NatsURL)
	assert.NotEmpty(t, wfi.WorkflowInstanceID)

	// Get Workflow Instances
	instances := &struct {
		WorkflowInstance []model.ListWorkflowInstanceResult
	}{}
	sharExecf(t, &instances, "instance list SimpleWorkflow --server %s --json", support.NatsURL)
	assert.Equal(t, 1, len(instances.WorkflowInstance))
	assert.Equal(t, wfi.WorkflowInstanceID, instances.WorkflowInstance[0].Id)

	//Get Workflow Instance Status
	status := &struct {
		TrackingId string
		ID         string
		Type       string
		State      string
		Executing  string
		Since      int64
	}{}
	sharExecf(t, &status, "instance status %s --server %s --json", wfi.WorkflowInstanceID, support.NatsURL)
	assert.NotEmpty(t, status.TrackingId)
	assert.Equal(t, "Step1", status.ID)
	assert.Equal(t, "serviceTask", status.Type)
	assert.Equal(t, "executing", status.State)
	assert.Equal(t, "SimpleProcess", status.Executing)
	assert.NotZero(t, status.Since)
	// Allow workflow to continue
	close(d.allowContinue)

	select {
	case c := <-complete:
		fmt.Println("completed " + c.WorkflowInstanceId)
	case <-time.After(20 * time.Second):
		assert.Fail(t, "Timed out")
	}
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

type testLaunchWorkflo struct {
	t             *testing.T
	allowContinue chan interface{}
}

func (d *testLaunchWorkflo) integrationSimple(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	<-d.allowContinue
	assert.Equal(d.t, 32768, vars["carried"].(int))
	vars["Success"] = true
	return vars, nil
}
