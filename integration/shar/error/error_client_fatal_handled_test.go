package error

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/common/subj"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

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
		{startVars: model.NewVars(), expectingFailure: true},
		{startVars: model.NewVars(), expectingFailure: true},
		{startVars: model.NewVars(), expectingFailure: true},
	}

	wfName, workflowId, executionIds, _ := runExecutions(t, ctx, cl, ns, execParams, willPanicAndCauseWorkflowFatalError)

	assert.Eventually(t, func() bool {
		fatalErrors, err := cl.GetFatalErrors(ctx, wfName, workflowId, "", "")
		require.NoError(t, err)
		return len(fatalErrors) == len(executionIds)
	}, 3*time.Second, 100*time.Millisecond)

	for _, executionId := range executionIds {
		assert.Eventually(t, func() bool {
			fatalErrors, err := cl.GetFatalErrors(ctx, wfName, workflowId, executionId, "")
			require.NoError(t, err)
			return len(fatalErrors) == 1 && fatalErrors[0].WorkflowState.ExecutionId == executionId
		}, 3*time.Second, 100*time.Millisecond)
	}

	assert.Eventually(t, func() bool {
		fatalErrors, err := cl.GetFatalErrors(ctx, wfName, workflowId, "*", "*")
		require.NoError(t, err)
		return len(fatalErrors) == len(executionIds)
	}, 3*time.Second, 100*time.Millisecond)
}

func getWorkflowIdFrom(t *testing.T, ctx context.Context, cl *client.Client, name string) string {
	versions, err := cl.GetWorkflowVersions(ctx, name)
	require.NoError(t, err)
	require.Len(t, versions, 1) //should only be 1 version
	latestVersion := versions[len(versions)-1]
	return latestVersion.Id
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

	pId1 := int64(1)
	pId2 := int64(2)

	newVars1 := model.NewVars()
	newVars1.SetBool("blowUp", false)
	newVars1.SetInt64("pId", int64(pId1))

	newVars2 := model.NewVars()
	newVars2.SetBool("blowUp", true)
	newVars2.SetInt64("pId", int64(pId2))

	execParams := []executionParams{
		{startVars: newVars1, expectingFailure: false},
		{startVars: newVars2, expectingFailure: true},
	}

	wfName, workflowId, executionIds, completionChan := runExecutions(t, ctx, cl, ns, execParams, willConditionallyPanicDependingOnVar)

	assert.Eventually(t, func() bool {
		successExecutionId := executionIds[0]
		fatalErrors, err := cl.GetFatalErrors(ctx, wfName, workflowId, successExecutionId, "")
		require.NoError(t, err)
		return len(fatalErrors) == 0
	}, 3*time.Second, 100*time.Millisecond)

	var fatalErrors []*model.FatalError
	failedExecutionId := executionIds[1]
	require.Eventually(t, func() bool {
		fatalErrors, err = cl.GetFatalErrors(ctx, wfName, workflowId, failedExecutionId, "")
		require.NoError(t, err)
		return len(fatalErrors) == 1 && fatalErrors[0].WorkflowState.ExecutionId == failedExecutionId
	}, 3*time.Second, 100*time.Millisecond)

	//attempt the retry here!!!
	wfState := fatalErrors[0].WorkflowState

	decodedVars := model.NewVars()
	err = decodedVars.Decode(ctx, wfState.Vars)
	require.NoError(t, err)

	decodedVars.SetBool("blowUp", false)
	encodedVars, err := decodedVars.Encode(ctx)
	require.NoError(t, err)

	wfState.Vars = encodedVars

	err = cl.Retry(ctx, wfState)
	require.NoError(t, err)

	//assert receipt of completion msg for both original and retried executions
	support.WaitForChan(t, completionChan, time.Second*20, func(ele processCompletion) {
		pId, err := ele.vars.GetInt64("pId")
		assert.NoError(t, err)
		assert.Equal(t, pId1, pId)
	})

	support.WaitForChan(t, completionChan, time.Second*20, func(ele processCompletion) {
		pId, err := ele.vars.GetInt64("pId")
		assert.NoError(t, err)
		assert.Equal(t, pId2, pId)
	})

	//assert non existence of FatalError
	assert.Eventually(t, func() bool {
		fatalErrors, err := cl.GetFatalErrors(ctx, wfName, workflowId, failedExecutionId, "")
		require.NoError(t, err)
		return len(fatalErrors) == 0
	}, 3*time.Second, 100*time.Millisecond)
}

type processCompletion struct {
	vars model.Vars
}

func runExecutions(t *testing.T, ctx context.Context, cl *client.Client, ns string, executionParams []executionParams, svcFn task.ServiceFn) (string, string, map[int]string, chan processCompletion) {
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
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: wfName, WorkflowBPMN: b}); err != nil {
		panic(err)
	}
	workflowId := getWorkflowIdFrom(t, ctx, cl, wfName)

	executionIds := make(map[int]string, len(executionParams))

	for i, executionParam := range executionParams {
		// Launch the workflow
		executionId, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: processId, Vars: executionParam.startVars})
		require.NoError(t, err)

		executionIds[i] = executionId

		// Listen for service tasks
		go func() {
			err := cl.Listen(ctx)
			require.NoError(t, err)
		}()

		if executionParam.expectingFailure {
			support.ListenForMsg(support.ExpectedMessage{
				T:             t,
				NatsUrl:       tst.NatsURL,
				Subject:       subj.NS(messages.WorkflowSystemProcessFatalError, ns),
				CreateModelFn: func() proto.Message { return &model.FatalError{} },
				MsgReceived:   d.fatalErr,
			})
			// wait for the fatal err to appear
			support.WaitForChan(t, d.fatalErr, 20*time.Second)

			expectedFatalErrorKey := fmt.Sprintf("%s.%s.%s.>", wfName, workflowId, executionId)
			tst.AssertExpectedKVKey(ns, messages.KvFatalError, expectedFatalErrorKey, 20*time.Second, t)
		}
	}
	return wfName, workflowId, executionIds, processCompletionChannel
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
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: wfName, WorkflowBPMN: b}); err != nil {
		panic(err)
	}

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess", Vars: model.NewVars()})
	require.NoError(t, err)

	workflowId := getWorkflowIdFrom(t, ctx, cl, wfName)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.ListenForMsg(support.ExpectedMessage{
		T:             t,
		NatsUrl:       tst.NatsURL,
		Subject:       subj.NS(messages.WorkflowSystemProcessFatalError, ns),
		CreateModelFn: func() proto.Message { return &model.FatalError{} },
		MsgReceived:   d.fatalErr,
	})

	// wait for the fatal err to appear
	support.WaitForChan(t, d.fatalErr, 20*time.Second)

	expectedFatalErrorKey := fmt.Sprintf("%s.%s.%s.>", wfName, workflowId, executionId)
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
	retVars := model.NewVars()
	retVars.SetBool("success", true)
	retVars.SetInt64("myVar", 69) // nolint:ireturn
	return retVars, nil
}

func (d *fatalErrorHandledHandlerDef) willResultInRetryExhaustion(ctx context.Context, jobClient task.JobClient, vars model.Vars) (model.Vars, error) {
	return nil, fmt.Errorf("test error: %w", errors.New("I will cause retry exhaustion"))
}

func willConditionallyPanicDependingOnVar(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("in willConditionallyPanicDependingOnVar", "vars", vars)
	blowUp, _ := vars.GetBool("blowUp")
	if blowUp {
		panic(fmt.Errorf("BLOW UP, cause an ErrWorkflowFatal to be thrown"))
	}

	return vars, nil
}
