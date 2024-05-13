package messaging

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"log/slog"
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
	subscAckSubj(ns)
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, "nats://127.0.0.1:4222")
	//err := cl.Dial(ctx, tst.NatsURL)
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

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestConcurrentMessaging", b)
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	handlers.instComplete = make(map[string]struct{})
	n := 100
	tm := time.Now()
	for inst := 0; inst < n; inst++ {
		go func(inst int) {
			// Launch the processes
			if _, _, err := cl.LaunchProcess(ctx, "Process_0hgpt6k", model.Vars{"orderId": inst}); err != nil {
				panic(err)
			} else {
				handlers.mx.Lock()
				handlers.instComplete[strconv.Itoa(inst)] = struct{}{}
				handlers.mx.Unlock()
			}
		}(inst)
		time.Sleep(500 * time.Millisecond)
	}

	support.WaitForExpectedCompletions(t, n, handlers.finished, 60*time.Second)
	time.Sleep(5 * time.Second)

	fmt.Println("Stopwatch:", -time.Until(tm))
	time.Sleep(10 * time.Millisecond)
	//tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.Equal(t, n, handlers.received)
	assert.Equal(t, 0, len(handlers.instComplete))

	slog.Info("$$$instComplete", "v", handlers.instComplete)
	time.Sleep(5 * time.Second)
}

func subscAckSubj(ns string) {
	fmt.Println("enter subscAckSubj")
	nc, _ := nats.Connect("nats://127.0.0.1:4222")
	_, err := nc.Subscribe(fmt.Sprintf("$JS.ACK.WORKFLOW.ProcessTerminateConsumer_%s.>", ns), func(msg *nats.Msg) {
		fmt.Println(msg.Subject)
		fmt.Println(fmt.Sprintf("ackdata: %s", string(msg.Data)))
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("exit subscAckSubj")
}

type testConcurrentMessagingHandlerDef struct {
	mx           sync.Mutex
	test         *testing.T
	received     int
	finished     chan struct{}
	instComplete map[string]struct{}
}

func (x *testConcurrentMessagingHandlerDef) step1(_ context.Context, _ client.JobClient, _ model.Vars) (model.Vars, error) {
	return model.Vars{}, nil
}

func (x *testConcurrentMessagingHandlerDef) step2(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	ks := make([]string, len(vars))
	i := 0
	for k := range vars {
		ks[i] = k
		i++
	}

	orderId := vars["orderId"].(int)
	slog.Info("###svc task step 2", "orderId", orderId)
	assert.Equal(x.test, "carried1value", vars["carried"])
	assert.Equal(x.test, "carried2value", vars["carried2"])
	return model.Vars{}, nil
}

func (x *testConcurrentMessagingHandlerDef) sendMessage(ctx context.Context, cmd client.MessageClient, vars model.Vars) error {
	if err := cmd.SendMessage(ctx, "continueMessage", vars["orderId"], model.Vars{"carried": vars["carried"]}); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

func (x *testConcurrentMessagingHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	x.mx.Lock()
	orderId := vars["orderId"].(int)
	slog.Info("### processEnd", "orderId", orderId)
	if _, ok := x.instComplete[strconv.Itoa(orderId)]; !ok {
		slog.Info("### again received", "orderId", orderId)
		//panic("too many calls")
	} else {
		slog.Info("### deleting received", "orderId", orderId)
		delete(x.instComplete, strconv.Itoa(orderId))
	}
	x.received++
	x.mx.Unlock()
	x.finished <- struct{}{}
}
