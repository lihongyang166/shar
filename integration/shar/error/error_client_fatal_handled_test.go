package error

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"log/slog"
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
	_, err = support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", willPanicAndCauseWorkflowFatalError)
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

	tst.ListenForFatalErr(t, d.fatalErr, ns)

	// wait for the fatal err to appear
	support.WaitForChan(t, d.fatalErr, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type executionParams struct {
	startVars        model.Vars
	expectingFailure bool
}

func TestFatalErrorPersistedWithPauseHandlingStrategy(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	execParams := []executionParams{
		{startVars: model.Vars{}, expectingFailure: true},
		{startVars: model.Vars{}, expectingFailure: true},
		{startVars: model.Vars{}, expectingFailure: true},
	}

	wfName, executionIds, _ := runExecutions(t, ctx, cl, ns, execParams, willPanicAndCauseWorkflowFatalError)

	assert.Eventually(t, func() bool {
		fatalErrors, err := cl.GetFatalErrors(ctx, fmt.Sprintf("%s.>", wfName))
		require.NoError(t, err)
		return len(fatalErrors) == len(executionIds)
	}, 3*time.Second, 100*time.Millisecond)

	for _, executionId := range executionIds {
		assert.Eventually(t, func() bool {
			fatalErrors, err := cl.GetFatalErrors(ctx, fmt.Sprintf("%s.%s.>", wfName, executionId))
			require.NoError(t, err)
			return len(fatalErrors) == 1 && fatalErrors[0].WorkflowState.ExecutionId == executionId
		}, 3*time.Second, 100*time.Millisecond)
	}

	assert.Eventually(t, func() bool {
		fatalErrors, err := cl.GetFatalErrors(ctx, fmt.Sprintf("%s.*.*", wfName))
		require.NoError(t, err)
		return len(fatalErrors) == len(executionIds)
	}, 3*time.Second, 100*time.Millisecond)
}

func TestRetryFatalErroredProcess(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	pId1 := 1
	pId2 := 2
	execParams := []executionParams{
		{startVars: model.Vars{"blowUp": false, "pId": pId1}, expectingFailure: false},
		{startVars: model.Vars{"blowUp": true, "pId": pId2}, expectingFailure: true},
	}

	wfName, executionIds, completionChan := runExecutions(t, ctx, cl, ns, execParams, willConditionallyPanicDependingOnVar)

	assert.Eventually(t, func() bool {
		successExecutionId := executionIds[0]
		fatalErrors, err := cl.GetFatalErrors(ctx, fmt.Sprintf("%s.%s.>", wfName, successExecutionId))
		require.NoError(t, err)
		return len(fatalErrors) == 0
	}, 3*time.Second, 100*time.Millisecond)

	var fatalErrors []*model.FatalError
	failedExecutionId := executionIds[1]
	require.Eventually(t, func() bool {
		fatalErrors, err = cl.GetFatalErrors(ctx, fmt.Sprintf("%s.%s.>", wfName, failedExecutionId))
		require.NoError(t, err)
		return len(fatalErrors) == 1 && fatalErrors[0].WorkflowState.ExecutionId == failedExecutionId
	}, 3*time.Second, 100*time.Millisecond)

	//attempt the retry here!!!
	wfState := fatalErrors[0].WorkflowState

	decodedVars, err := vars.Decode(ctx, wfState.Vars)
	require.NoError(t, err)

	decodedVars["blowUp"] = false
	encodedVars, err := vars.Encode(ctx, decodedVars)
	require.NoError(t, err)

	wfState.Vars = encodedVars

	err = cl.Retry(ctx, wfState)
	require.NoError(t, err)

	//assert receipt of completion msg for both original and retried executions
	support.WaitForChanZ(t, completionChan, time.Second*20, func(ele processCompletion) {
		pId := ele.vars["pId"]
		assert.Equal(t, pId1, pId)
	})

	support.WaitForChanZ(t, completionChan, time.Second*20, func(ele processCompletion) {
		pId := ele.vars["pId"]
		assert.Equal(t, pId2, pId)
	})

	//assert non existence of FatalError
	assert.Eventually(t, func() bool {
		fatalErrors, err := cl.GetFatalErrors(ctx, fmt.Sprintf("%s.%s.>", wfName, failedExecutionId))
		require.NoError(t, err)
		return len(fatalErrors) == 0
	}, 3*time.Second, 100*time.Millisecond)
}

type processCompletion struct {
	vars model.Vars
}

func runExecutions(t *testing.T, ctx context.Context, cl *client.Client, ns string, executionParams []executionParams, svcFn task.ServiceFn) (string, map[int]string, chan processCompletion) {
	d := fatalErrorHandledHandlerDef{test: t, fatalErr: make(chan struct{})}

	// Register service tasks
	_, err := support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test.yaml", svcFn)
	require.NoError(t, err)

	processId := "SimpleProcess"

	processCompletionChannel := make(chan processCompletion)
	err = cl.RegisterProcessComplete(processId, func(ctx context.Context, vars model.Vars, wfError *model.Error, endState model.CancellationState) {
		processCompletionChannel <- processCompletion{vars}
	})
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)
	wfName := "TestHandleFatalError"
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, wfName, b); err != nil {
		panic(err)
	}

	executionIds := make(map[int]string, len(executionParams))

	for i, executionParam := range executionParams {
		// Launch the workflow
		executionId, _, err := cl.LaunchProcess(ctx, processId, executionParam.startVars)
		require.NoError(t, err)

		executionIds[i] = executionId

		// Listen for service tasks
		go func() {
			err := cl.Listen(ctx)
			require.NoError(t, err)
		}()

		if executionParam.expectingFailure {
			tst.ListenForFatalErr(t, d.fatalErr, ns)
			// wait for the fatal err to appear
			support.WaitForChan(t, d.fatalErr, 20*time.Second)

			expectedFatalErrorKey := fmt.Sprintf("%s.%s.>", wfName, executionId)
			tst.AssertExpectedKVKey(ns, messages.KvFatalError, expectedFatalErrorKey, 20*time.Second, t)
		}
	}
	return wfName, executionIds, processCompletionChannel
}

func TestFatalErrorPersistedWhenRetriesAreExhaustedAndErrorActionIsPause(t *testing.T) {
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
	_, err = support.RegisterTaskYamlFile(ctx, cl, "../simple/simple_test_pause_on_retry_exhausted.yaml", d.willResultInRetryExhaustion)
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

	tst.ListenForFatalErr(t, d.fatalErr, ns)

	// wait for the fatal err to appear
	support.WaitForChan(t, d.fatalErr, 20*time.Second)

	expectedFatalErrorKey := fmt.Sprintf("%s.%s.>", wfName, executionId)
	tst.AssertExpectedKVKey(ns, messages.KvFatalError, expectedFatalErrorKey, 20*time.Second, t)
}

type fatalErrorHandledHandlerDef struct {
	test     *testing.T
	fatalErr chan struct{}
}

func willPanicAndCauseWorkflowFatalError(_ context.Context, _ task.JobClient, v model.Vars) (model.Vars, error) {
	// panic and cause a WorkflowFatalError
	if true {
		panic(fmt.Errorf("BOOM, cause an ErrWorkflowFatal to be thrown"))
	}

	return model.Vars{"success": true, "myVar": 69}, nil
}

func (d *fatalErrorHandledHandlerDef) willResultInRetryExhaustion(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	return nil, fmt.Errorf("test error: %w", errors.New("I will cause retry exhaustion"))
}

func willConditionallyPanicDependingOnVar(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("in willConditionallyPanicDependingOnVar", "vars", vars)
	blowUp, _ := vars["blowUp"].(bool)
	if blowUp {
		panic(fmt.Errorf("BLOW UP, cause an ErrWorkflowFatal to be thrown"))
	}

	return vars, nil
}
