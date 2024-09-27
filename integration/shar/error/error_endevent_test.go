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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestEndEventError(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	d := &testErrorEndEventHandlerDef{finished: make(chan struct{}), t: t}

	// Register service tasks
	_, err = support.RegisterTaskYamlFile(ctx, cl, "error_endevent_test_couldThrowError.yaml", d.mayFail3)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "error_endevent_test_fixSituation.yaml", d.fixSituation)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/errors.bpmn")
	require.NoError(t, err)
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestEndEventError", WorkflowBPMN: b}); err != nil {
		panic(err)
	}

	err = cl.RegisterProcessComplete("Process_07lm3kx", d.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_07lm3kx"})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	// wait for the workflow to complete
	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testErrorEndEventHandlerDef struct {
	finished chan struct{}
	t        *testing.T
}

// A "Hello World" service task
func (d *testErrorEndEventHandlerDef) mayFail3(ctx context.Context, client task.JobClient, _ model.Vars) (model.Vars, error) { // nolint:ireturn
	logger := client.Logger()
	logger.Info("service task completed successfully")
	retVars := model.NewVars()
	retVars.SetBool("success", true)
	return retVars, nil
}

// A "Hello World" service task
func (d *testErrorEndEventHandlerDef) fixSituation(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	carried, err := vars.GetInt64("carried")
	if err != nil {
		return nil, fmt.Errorf("get carried: %w", err)
	}
	fmt.Println("carried", carried)
	d.t.Fatal("this event should not fire")
	return nil, nil
}

func (d *testErrorEndEventHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	assert.Equal(d.t, "103", wfError.Code)
	assert.Equal(d.t, "CatastrophicError", wfError.Name)
	assert.Equal(d.t, model.CancellationState_completed, state)
	close(d.finished)
}
