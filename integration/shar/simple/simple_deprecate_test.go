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
	"gitlab.com/shar-workflow/shar/common/namespace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestLaunchProcessWithDeprecated(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleDeprecateHandlerDef{t: t, finished: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_deprecate_test.yaml", d.integrationSimple)
	require.NoError(t, err)

	v, err := cl.GetTaskSpecVersions(ctx, "SimpleProcess")
	require.NoError(t, err)

	err = cl.DeprecateTaskSpec(ctx, v[0])
	require.NoError(t, err)
	fmt.Println(v)

	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.ErrorContains(t, err, "contains deprecated")

	tst.AssertCleanKV(namespace.Default, t, 60*time.Second)
}

func TestDeprecateExecuting(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleDeprecateHandlerDef{t: t, finished: make(chan struct{}), wait: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_deprecate_test.yaml", d.integrationSimple)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	v, err := cl.GetTaskSpecVersions(ctx, "SimpleProcess")
	require.NoError(t, err)

	err = cl.DeprecateTaskSpec(ctx, v[0])
	inUseError := &client.ErrTaskInUse{}
	require.ErrorAs(t, err, &inUseError)
	fmt.Println(inUseError.Usage.ExecutingProcessInstance)
	close(d.wait)
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

func TestGetUsage(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleDeprecateHandlerDef{t: t, finished: make(chan struct{}), wait: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_deprecate_test.yaml", d.integrationSimple)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	v, err := cl.GetTaskSpecVersions(ctx, "SimpleProcess")
	require.NoError(t, err)

	use1, err := cl.GetTaskSpecUsage(ctx, v[0])
	require.NoError(t, err)
	fmt.Println(use1)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	err = cl.DeprecateTaskSpec(ctx, v[0])
	inUseError := &client.ErrTaskInUse{}
	require.ErrorAs(t, err, &inUseError)
	fmt.Println(inUseError.Usage.ExecutingProcessInstance)

	use2, err := cl.GetTaskSpecUsage(ctx, v[0])
	require.NoError(t, err)
	fmt.Println(use2)
	assert.Equal(t, use1.Workflow, []string{"SimpleWorkflowTest"})
	assert.Equal(t, use1.Process, []string{"SimpleProcess"})
	assert.Equal(t, len(use1.ExecutingProcessInstance), 0)
	assert.Equal(t, len(use1.ExecutingWorkflow), 0)
	assert.Equal(t, use2.Workflow, []string{"SimpleWorkflowTest"})
	assert.Equal(t, use2.Process, []string{"SimpleProcess"})
	assert.Equal(t, len(use2.ExecutingProcessInstance), 1)
	assert.Equal(t, len(use2.ExecutingWorkflow), 1)
	close(d.wait)
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSimpleDeprecateHandlerDef struct {
	t        *testing.T
	finished chan struct{}
	wait     chan struct{}
}

func (d *testSimpleDeprecateHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	<-d.wait
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	localVar, err := vars.GetInt64("localVar")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(42), localVar)
	vars.SetBool("Success", true)
	return vars, nil
}

func (d *testSimpleDeprecateHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
