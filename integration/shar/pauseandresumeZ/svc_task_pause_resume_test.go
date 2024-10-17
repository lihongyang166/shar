package simple

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestPauseAndResumeServiceTask(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	slog.Info("###", "tst.NatsURL", tst.NatsURL)
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	//TODO try pause the svc task here
	// try launching 10 wfs in a loop
	// on fifth iteration, pause the svc task,
	// continue launching

	//try pausing svc task here in other test
	pauseConsumer(t, ctx)
	<-time.After(time.Second * 120)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	//check the state of the consumer

	//TODO assert that svc tasks have been received but then none are received after
	//N secs

	//TODO resume svc task here

	//TODO assert 5 more svc task has been called and wf completes

	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

func TestPauseAndResumeVersionedServiceTask(t *testing.T) {
	t.Fatal("not implemented")
}

type testSimpleHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testSimpleHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	localVar, err := vars.GetInt64("localVar")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(42), localVar)
	vars.SetBool("Success", true)
	return vars, nil
}

func (d *testSimpleHandlerDef) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	processVar, err := vars.GetInt64("processVar")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(42), processVar)
	close(d.finished)
}

func pauseConsumer(t *testing.T, ctx context.Context) {
	js, err := tst.GetJetstream()
	require.NoError(t, err)

	stream := "WORKFLOW"
	s, err := js.Stream(ctx, stream)
	require.NoError(t, err)
	consumerInfoListener := s.ListConsumers(ctx)
	consInfoCh := consumerInfoListener.Info()

	var consumerName string
	for consInf := range consInfoCh {
		if strings.HasPrefix(consInf.Name, "ServiceTask_") {
			consumerName = consInf.Name
			slog.Info("%%%", "consumerName", consumerName)
		}
	}

	pauseExpiry := time.Now().Add(time.Second * 240)

	//consumerName := "ServiceTask_2nTgfChtw9IkGnBqhKHWRaIl1cW_YKk60mHqUWfPevXHOlL3338B7sGrxxefJAx7ak3Wny1"
	//config := jetstream.ConsumerConfig{
	//	Name:       consumerName,
	//	PauseUntil: &pauseExpiry,
	//}
	//_, err = js.CreateOrUpdateConsumer(ctx, stream, config)
	//require.NoError(t, err)

	timeout, _ := context.WithTimeout(ctx, time.Second*10)
	_, err = s.PauseConsumer(timeout, consumerName, pauseExpiry)
	require.NoError(t, err)

	consumer, err := js.Consumer(ctx, stream, consumerName)
	require.NoError(t, err)

	info, err := consumer.Info(ctx)
	require.NoError(t, err)

	slog.Info("$$$", "consumerInfo", info)
}
