package simple

import (
	"context"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

const noOfProcessesToLaunch = 10

func TestPauseAndResumeServiceTask(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{})}

	// Register a service task
	uid, err := support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", d.svcTaskFn)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// Launch the workflows
	launchWorkflows(5, t, cl, ctx)

	//assert that exactly 5 svc tasks have been invoked after 10 secs
	assert.Eventually(t, func() bool {
		return d.svcTaskInvokations == 5
	}, time.Second*10, time.Second)
	assert.Never(t, func() bool {
		return d.svcTaskInvokations > 5
	}, time.Second*5, time.Second)

	//pause svc task here
	err = cl.PauseServiceTask(ctx, uid)
	require.NoError(t, err)

	launchWorkflows(5, t, cl, ctx)

	assert.Eventually(t, func() bool {
		return d.svcTaskInvokations == 5
	}, time.Second*10, time.Second)
	assert.Never(t, func() bool {
		return d.svcTaskInvokations > 5
	}, time.Second*5, time.Second)

	err = cl.ResumeServiceTask(ctx, uid)
	require.NoError(t, err)

	//assert we are now at 10
	assert.Eventually(t, func() bool {
		return d.svcTaskInvokations == 10
	}, time.Second*30, time.Second)
	assert.Never(t, func() bool {
		return d.svcTaskInvokations > 10
	}, time.Second*5, time.Second)

	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

func launchWorkflows(noToLaunch int, t *testing.T, cl *client.Client, ctx context.Context) {
	for i := 0; i < noToLaunch; i++ {
		go func() {
			_, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
			require.NoError(t, err)
		}()
	}
}

type testSimpleHandlerDef struct {
	t                         *testing.T
	finished                  chan struct{}
	svcTaskInvokations        int
	svcTaskInvokationsLock    sync.Mutex
	endProcessInvokations     int
	endProcessInvokationsLock sync.Mutex
}

func (d *testSimpleHandlerDef) svcTaskFn(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	d.svcTaskInvokationsLock.Lock()
	d.svcTaskInvokations++
	d.svcTaskInvokationsLock.Unlock()
	return vars, nil
}

func (d *testSimpleHandlerDef) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	d.endProcessInvokationsLock.Lock()
	d.endProcessInvokations++
	if d.endProcessInvokations == noOfProcessesToLaunch {
		close(d.finished)
	}
	d.endProcessInvokationsLock.Unlock()
}
