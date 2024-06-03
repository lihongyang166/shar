package error

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestFatalErrorHandled(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	if err := cl.Dial(ctx, tst.NatsURL); err != nil {
		panic(err)
	}

	d := fatalErrorHandledHandlerDef{test: t, fatalErr: make(chan struct{})}

	// Register service tasks
	_, err := support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", d.willPanicAndCauseWorkflowFatalError)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, "TestHandleFatalError", b); err != nil {
		panic(err)
	}

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
	if err != nil {
		panic(err)
	}

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		if err != nil {
			panic(err)
		}
	}()

	tst.ListenForFatalErr(t, d.fatalErr)

	// wait for the fatal err to appear
	support.WaitForChan(t, d.fatalErr, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

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
