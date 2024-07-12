package error

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/server/messages"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestFatalErrorHandledWithTeardown(t *testing.T) {
	t.Skip("need to revisit this when we introduce the ability to specify the handling strategy" +
		"via either configuration or in the element/service task definition")

	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := fatalErrorHandledHandlerDef{test: t, fatalErr: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", d.willPanicAndCauseWorkflowFatalError)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestHandleFatalError", WorkflowBPMN: b}); err != nil {
		panic(err)
	}

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess"})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	tst.ListenForFatalErr(t, d.fatalErr)

	// wait for the fatal err to appear
	support.WaitForChan(t, d.fatalErr, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

func TestFatalErrorHandledWithPauseHandlingStrategy(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := fatalErrorHandledHandlerDef{test: t, fatalErr: make(chan struct{})}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", d.willPanicAndCauseWorkflowFatalError)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)
	wfName := "TestHandleFatalError"
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, wfName, b); err != nil {
		panic(err)
	}

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	tst.ListenForFatalErr(t, d.fatalErr)

	// wait for the fatal err to appear
	support.WaitForChan(t, d.fatalErr, 20*time.Second)

	expectedFatalErrorKey := fmt.Sprintf("%s.%s.>", wfName, executionId)

	tst.AssertExpectedKVKey(ns, messages.KvFatalError, expectedFatalErrorKey, 20*time.Second, t)
}

//TODO test backoff retry exhaustion

type fatalErrorHandledHandlerDef struct {
	test     *testing.T
	fatalErr chan struct{}
}

func (d *fatalErrorHandledHandlerDef) willPanicAndCauseWorkflowFatalError(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	// panic and cause a WorkflowFatalError
	if true {
		panic(fmt.Errorf("BOOM, cause an ErrWorkflowFatal to be thrown"))
	}

	return model.Vars{"success": true, "myVar": 69}, nil
}
