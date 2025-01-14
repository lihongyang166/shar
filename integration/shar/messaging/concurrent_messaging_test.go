package messaging

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"strconv"
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
func TestConcurrentMessaging(t *testing.T) {
	t.Parallel()

	handlers := &testConcurrentMessagingHandlerDef{finished: make(chan struct{}), test: t}
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "concurrent_messaging_2_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "concurrent_messaging_2_test_step2.yaml", handlers.step2)
	require.NoError(t, err)
	err = cl.RegisterMessageSender(ctx, "TestConcurrentMessaging", "continueMessage", handlers.sendMessage)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("Process_03llwnm", handlers.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/message-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestConcurrentMessaging", WorkflowBPMN: b})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	handlers.instComplete = make(map[string]struct{})
	n := 200
	tm := time.Now()
	for inst := 0; inst < n; inst++ {
		go func(inst int) {
			// Launch the processes
			launchVars := model.NewVars()
			launchVars.SetInt64("orderId", int64(inst))
			if _, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_0hgpt6k", Vars: launchVars}); err != nil {
				require.NoError(t, err)
			} else {
				handlers.mx.Lock()
				handlers.instComplete[strconv.Itoa(inst)] = struct{}{}
				handlers.mx.Unlock()
			}
		}(inst)
	}

	support.WaitForExpectedCompletions(t, n, handlers.finished, 60*time.Second)

	fmt.Println("Stopwatch:", -time.Until(tm))
	tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.Equal(t, n, handlers.received)
	assert.Equal(t, 0, len(handlers.instComplete))
}

type testConcurrentMessagingHandlerDef struct {
	mx           sync.Mutex
	test         *testing.T
	received     int
	finished     chan struct{}
	instComplete map[string]struct{}
}

func (x *testConcurrentMessagingHandlerDef) step1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	return model.NewVars(), nil
}

func (x *testConcurrentMessagingHandlerDef) step2(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	carried, err := vars.GetString("carried")
	require.NoError(x.test, err)
	carried2, err := vars.GetString("carried2")
	require.NoError(x.test, err)
	assert.Equal(x.test, "carried1value", carried)
	assert.Equal(x.test, "carried2value", carried2)
	return model.NewVars(), nil
}

func (x *testConcurrentMessagingHandlerDef) sendMessage(ctx context.Context, cmd task.MessageClient, vars model.Vars) error {
	orderId, err := vars.GetInt64("orderId")
	require.NoError(x.test, err)
	carried, err := vars.GetString("carried")
	require.NoError(x.test, err)
	newVars := model.NewVars()
	newVars.SetString("carried", carried)
	if err := cmd.SendMessage(ctx, "continueMessage", orderId, newVars); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

func (x *testConcurrentMessagingHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	x.mx.Lock()
	orderId, err := vars.GetInt64("orderId")
	require.NoError(x.test, err)
	if _, ok := x.instComplete[strconv.Itoa(int(orderId))]; !ok {
		x.test.Fatal("too many calls")
	}
	delete(x.instComplete, strconv.Itoa(int(orderId)))
	x.received++
	x.mx.Unlock()
	x.finished <- struct{}{}
}
