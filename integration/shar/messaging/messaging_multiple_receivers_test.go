package messaging

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
func TestMessagingMultipleReceivers(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	handlers := &testMessagingMultiReceiverHandlerDef{t: t, wg: sync.WaitGroup{}, tst: tst, finished: make(chan struct{})}

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step2.yaml", handlers.step2)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step3.yaml", handlers.step3)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/message-multiple-receivers-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessaging", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterMessageSender(ctx, "TestMessaging", "continueMessage", handlers.sendMessage)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("Process_03llwnm", handlers.processEnd)
	require.NoError(t, err)

	newVars := model.NewVars()
	newVars.SetInt64("orderId", 57)

	// Launch the processes
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_0hgpt6k", Vars: newVars})
	if err != nil {
		t.Fatal(err)
		return
	}

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, handlers.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testMessagingMultiReceiverHandlerDef struct {
	wg       sync.WaitGroup
	tst      *support.Integration
	finished chan struct{}
	t        *testing.T
}

func (x *testMessagingMultiReceiverHandlerDef) step1(ctx context.Context, client task.JobClient, _ model.Vars) (model.Vars, error) {
	logger := client.Logger()
	logger.Info("Step 1")

	return model.NewVars(), nil
}

func (x *testMessagingMultiReceiverHandlerDef) step2(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	logger := client.Logger()
	logger.Info("Step 2")
	x.tst.Mx.Lock()
	x.tst.FinalVars = vars
	x.tst.Mx.Unlock()
	return model.NewVars(), nil
}

func (x *testMessagingMultiReceiverHandlerDef) step3(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	logger := client.Logger()
	logger.Info("step 3")
	return model.NewVars(), nil
}

func (x *testMessagingMultiReceiverHandlerDef) sendMessage(ctx context.Context, client task.MessageClient, vars model.Vars) error {
	logger := client.Logger()
	logger.Info("Sending Message...")
	carried, err := vars.GetString("carried")
	require.NoError(x.t, err)
	newVars := model.NewVars()
	newVars.SetString("carried", carried)
	if err := client.SendMessage(ctx, "continueMessage", 57, newVars); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

func (x *testMessagingMultiReceiverHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	orderId, err := vars.GetInt64("orderId")
	require.NoError(x.t, err)
	assert.Equal(x.t, int64(57), orderId)
	assert.Equal(x.t, 1, len(vars))
	close(x.finished)
}
