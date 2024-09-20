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

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMultiWorkflow1", WorkflowBPMN: b})
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMultiWorkflow2", WorkflowBPMN: b2})
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
			launchVars := model.NewVars()
			launchVars.SetInt64("orderId", int64(inst))
			// Launch the processes
			if wfiID, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_03llwnm", Vars: launchVars}); err != nil {
				require.NoError(t, err)
			} else {
				mx.Lock()
				instances[wfiID] = struct{}{}
				mx.Unlock()
			}

			if wfiID2, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"}); err != nil {
				require.NoError(t, err)
			} else {
				mx.Lock()
				instances[wfiID2] = struct{}{}
				mx.Unlock()
			}
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
	carried, err := vars.GetString("carried")
	require.NoError(x.t, err)
	assert.Equal(x.t, "carried1value", carried)
	carried2, err := vars.GetString("carried2")
	require.NoError(x.t, err)
	assert.Equal(x.t, "carried2value", carried2)
	return model.Vars{}, nil
}

func (x *testMultiworkflowMessagingHandlerDef) sendMessage(ctx context.Context, cmd task.MessageClient, vars model.Vars) error {
	orderId, err := vars.GetInt64("orderId")
	require.NoError(x.t, err)
	carried, err := vars.GetString("carried")
	require.NoError(x.t, err)
	newVars := model.NewVars()
	newVars.SetString("carried", carried)
	if err := cmd.SendMessage(ctx, "continueMessage", orderId, newVars); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

// A "Hello World" service task
func (x *testMultiworkflowMessagingHandlerDef) simpleProcess(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	carried, err := vars.GetInt64("carried")
	require.NoError(x.t, err)
	assert.Equal(x.t, int64(32768), carried)
	return model.Vars{}, nil
}

func (x *testMultiworkflowMessagingHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	x.finished <- struct{}{}
}
