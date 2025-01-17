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
func TestMessaging(t *testing.T) {
	t.Parallel()
	ns := ksuid.New().String()
	ctx := context.Background()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	handlers := &testMessagingHandlerDef{t: t, wg: sync.WaitGroup{}, tst: tst, finished: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step2.yaml", handlers.step2)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/message-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessaging", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterMessageSender(ctx, "TestMessaging", "continueMessage", handlers.sendMessage)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("Process_03llwnm", handlers.processEnd)
	require.NoError(t, err)

	// Launch the processes
	launchVars := model.NewVars()
	launchVars.SetInt64("orderId", 57)
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_0hgpt6k", Vars: launchVars})
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

func TestMessageNameGlobalUniqueness(t *testing.T) {
	t.Parallel()
	ns := ksuid.New().String()
	ctx := context.Background()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	handlers := &testMessagingHandlerDef{t: t, wg: sync.WaitGroup{}, tst: tst, finished: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_step2.yaml", handlers.step2)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/message-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessaging", WorkflowBPMN: b})
	require.NoError(t, err)

	// try to load another bpmn with a message of the same name, should fail
	b, err = os.ReadFile("../../../testdata/message-workflow-duplicate-message.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessagingDupMessage", WorkflowBPMN: b})
	require.ErrorContains(t, err, "these messages already exist for other workflows:")

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

func TestMessageNameGlobalUniquenessAcrossVersions(t *testing.T) {
	t.Parallel()
	ns := ksuid.New().String()
	ctx := context.Background()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	messageEventHandlers := messageStartEventWorkflowEventHandler{
		completed: make(chan struct{}),
		t:         t,
	}

	// reg svc task
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_simple_service_step.yaml", messageEventHandlers.simpleServiceTaskHandler)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0w6dssp", messageEventHandlers.processEnd)
	require.NoError(t, err)

	// load bpmn
	b, err := os.ReadFile("../../../testdata/message-start-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessageStartEvent", WorkflowBPMN: b})
	require.NoError(t, err)

	b, err = os.ReadFile("../../../testdata/message-start-test-v2.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessageStartEvent", WorkflowBPMN: b})
	require.NoError(t, err)
}

func TestMessageStartEvent(t *testing.T) {
	t.Parallel()
	ns := ksuid.New().String()
	ctx := context.Background()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	messageEventHandlers := messageStartEventWorkflowEventHandler{
		completed: make(chan struct{}),
		t:         t,
	}

	// reg svc task
	_, err = support.RegisterTaskYamlFile(ctx, cl, "messaging_test_simple_service_step.yaml", messageEventHandlers.simpleServiceTaskHandler)
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("Process_0w6dssp", messageEventHandlers.processEnd)
	require.NoError(t, err)

	// load bpmn
	b, err := os.ReadFile("../../../testdata/message-start-test.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestMessageStartEvent", WorkflowBPMN: b})
	require.NoError(t, err)

	// send message
	msgVars := model.NewVars()
	msgVars.SetInt64("customerID", 333)
	err = cl.SendMessage(ctx, "startDemoMsg", "", msgVars)
	require.NoError(t, err)

	// listen for events from shar svr
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// wait for completion
	support.WaitForChan(t, messageEventHandlers.completed, time.Second*10)

	// assert empty KV
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testMessagingHandlerDef struct {
	wg       sync.WaitGroup
	tst      *support.Integration
	finished chan struct{}
	t        *testing.T
}

func (x *testMessagingHandlerDef) step1(ctx context.Context, client task.JobClient, _ model.Vars) (model.Vars, error) {
	logger := client.Logger()
	logger.Info("step 1")
	logger.Info("a sample client log")
	return model.NewVars(), nil
}

func (x *testMessagingHandlerDef) step2(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	logger := client.Logger()
	logger.Info("step2")
	x.tst.Mx.Lock()
	x.tst.FinalVars = vars
	x.tst.Mx.Unlock()
	return model.NewVars(), nil
}

func (x *testMessagingHandlerDef) sendMessage(ctx context.Context, client task.MessageClient, vars model.Vars) error {
	logger := client.Logger()
	logger.Info("Sending Message...")
	logger.Info("A sample messaging log")
	newVars := model.NewVars()
	carried, err := vars.GetString("carried")
	require.NoError(x.t, err)
	newVars.SetString("carried", carried)
	if err := client.SendMessage(ctx, "continueMessage", 57, newVars); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

func (x *testMessagingHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	orderId, err := vars.GetInt64("orderId")
	require.NoError(x.t, err)
	assert.Equal(x.t, int64(57), orderId)
	close(x.finished)
}

type messageStartEventWorkflowEventHandler struct {
	completed chan struct{}
	t         *testing.T
}

func (mse *messageStartEventWorkflowEventHandler) simpleServiceTaskHandler(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	logger := client.Logger()
	logger.Info("simpleServiceTaskHandler")
	actualCustomerId, err := vars.GetInt64("customerID")
	require.NoError(mse.t, err)
	assert.Equal(mse.t, int64(333), actualCustomerId)
	return vars, nil
}

func (mse *messageStartEventWorkflowEventHandler) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	customerId, err := vars.GetInt64("customerID")
	require.NoError(mse.t, err)
	assert.Equal(mse.t, int64(333), customerId)
	close(mse.completed)
}
