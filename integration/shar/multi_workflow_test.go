package intTest

import (
	"context"
	"fmt"
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

//goland:noinspection GoNilness
func TestMultiWorkflow(t *testing.T) {
	t.Parallel()

	handlers := &testMultiworkflowMessagingHandlerDef{t: t, finished: make(chan struct{})}

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "multi_workflow_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "multi_workflow_test_step2.yaml", handlers.step2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "multi_workflow_test_SimpleProcess.yaml", handlers.simpleProcess)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/message-workflow.bpmn")
	require.NoError(t, err)

	// Load BPMN workflow 2
	b2, err := os.ReadFile("../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestMultiWorkflow1", b)
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestMultiWorkflow2", b2)
	require.NoError(t, err)

	err = cl.RegisterMessageSender(ctx, "TestMultiWorkflow1", "continueMessage", handlers.sendMessage)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_03llwnm", handlers.processEnd)
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	n := 100
	mx := sync.Mutex{}
	instances := make(map[string]struct{})
	// wg := sync.WaitGroup{}
	for inst := 0; inst < n; inst++ {
		// wg.Add(1)
		go func(inst int) {
			// Launch the processes
			wfiID, _, err := cl.LaunchProcess(ctx, "Process_03llwnm", model.Vars{"orderId": inst})
			require.NoError(t, err)
			mx.Lock()
			instances[wfiID] = struct{}{}
			mx.Unlock()

			wfiID2, _, err := cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
			require.NoError(t, err)
			mx.Lock()
			instances[wfiID2] = struct{}{}
			mx.Unlock()
		}(inst)
	}

	support.WaitForExpectedCompletions(t, n, handlers.finished, time.Second*60)

	tst.AssertCleanKV(ns, t, 140*time.Second)
}

type testMultiworkflowMessagingHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (x *testMultiworkflowMessagingHandlerDef) step1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	return model.Vars{}, nil
}

func (x *testMultiworkflowMessagingHandlerDef) step2(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	assert.Equal(x.t, "carried1value", vars["carried"].(string))
	assert.Equal(x.t, "carried2value", vars["carried2"].(string))
	return model.Vars{}, nil
}

func (x *testMultiworkflowMessagingHandlerDef) sendMessage(ctx context.Context, cmd task.MessageClient, vars model.Vars) error {
	if err := cmd.SendMessage(ctx, "continueMessage", vars["orderId"].(int), model.Vars{"carried": vars["carried"]}); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

// A "Hello World" service task
func (x *testMultiworkflowMessagingHandlerDef) simpleProcess(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	assert.Equal(x.t, 32768, vars["carried"].(int))
	return model.Vars{}, nil
}

func (x *testMultiworkflowMessagingHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	x.finished <- struct{}{}
}
