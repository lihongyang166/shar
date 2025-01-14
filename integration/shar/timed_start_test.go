package intTest

import (
	"context"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

const wfHeaderName = "auto-launch"
const wfHeaderVal = "test-value"

func TestTimedStart(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &timedStartHandlerDef{tst: tst, t: t, finished: make(chan struct{})}

	// Register a service task
	_, err = support.RegisterTaskYamlFile(ctx, cl, "timed_start_test_SimpleProcess.yaml", d.integrationSimple)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/timed-start-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TimedStartTest", LaunchHeaders: map[string]string{wfHeaderName: wfHeaderVal}, WorkflowBPMN: b})
	require.NoError(t, err)

	// A hook to watch for completion
	err = cl.RegisterProcessComplete("Process_1hikszy", d.processEnd)
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	for i := 1; i <= 3; i++ {
		support.WaitForChan(t, d.finished, 30*time.Second)
	}
	d.mx.Lock()
	defer d.mx.Unlock()
	carried, err := tst.FinalVars.GetInt64("carried")
	require.NoError(t, err)
	assert.Equal(t, int64(32768), carried)
	assert.Equal(t, 3, d.count)
	tst.AssertCleanKV(ns, t, tst.Cooldown)
}

type timedStartHandlerDef struct {
	mx       sync.Mutex
	count    int
	tst      *support.Integration
	t        *testing.T
	finished chan struct{}
}

func (d *timedStartHandlerDef) integrationSimple(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	// TODO: Include for diagnosing timed start bug
	// assert.Equal(d.t, int64(32768), vars["carried"])
	d.mx.Lock()
	defer d.mx.Unlock()
	d.tst.FinalVars = vars
	d.count++
	hdr, err := client.Headers(ctx)
	assert.NoError(d.t, err)
	assert.Equal(d.t, wfHeaderVal, hdr[wfHeaderName])
	return vars, nil
}

func (d *timedStartHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	d.finished <- struct{}{}
}
