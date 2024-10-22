package error

import (
	"context"
	"errors"
	"fmt"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"os"
	"testing"
	"time"
)

const instanceNameVarName = "instanceName"
const instanceNameOne = "instanceOne"
const instanceNameTwo = "instanceTwo"

func TestBackoffRetryExhausted(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	h := &backoffRetryHandlerDef{t: t, instanceCompletionInvocations: make(map[string]int), instanceSvcTaskInvocations: make(map[string]int)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_svc_task.yaml", h.svcTask)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", h.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-svc-task-backoff-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	instanceNames := []string{instanceNameOne, instanceNameTwo}

	for _, instanceName := range instanceNames {
		go func() {
			// Launch the workflow
			vars, err := model.NewVarsFromMap(map[string]any{instanceNameVarName: instanceName})
			require.NoError(t, err)
			_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "SimpleProcess", Vars: vars})
			require.NoError(t, err)
		}()
	}

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	assert.Eventually(t, func() bool {
		expectedInvokationCount := 3
		i := h.instanceSvcTaskInvocations[instanceNameOne]
		return i == expectedInvokationCount
	}, time.Second*10, time.Second, "expected invokation count with retries not reached")

	assert.Eventually(t, func() bool {
		expectedInvokationCount := 1
		return h.instanceSvcTaskInvocations[instanceNameTwo] == expectedInvokationCount
	}, time.Second*10, time.Second, "expected single invokation")

	assert.Never(t, func() bool {
		expectedInvokationCount := 1
		return h.instanceCompletionInvocations[instanceNameOne] == expectedInvokationCount
	}, time.Second*10, time.Second, "instance one should never have completed")

	assert.Eventually(t, func() bool {
		expectedInvokationCount := 1
		return h.instanceCompletionInvocations[instanceNameTwo] == expectedInvokationCount
	}, time.Second*10, time.Second, "expected invokation count with retries not reached")

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type backoffRetryHandlerDef struct {
	t                             *testing.T
	instanceSvcTaskInvocations    map[string]int
	instanceCompletionInvocations map[string]int
}

func (h *backoffRetryHandlerDef) svcTask(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	instanceName, err := vars.GetString(instanceNameVarName)
	require.NoError(h.t, err)
	incrementInvokationCounter(instanceName, h.instanceSvcTaskInvocations)

	if instanceName == instanceNameOne {
		return nil, fmt.Errorf("i want to trigger backoff: %w", errors.New("to backoff exhaustion"))
	}
	return vars, nil
}

func (h *backoffRetryHandlerDef) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	instanceName, err := vars.GetString(instanceNameVarName)
	require.NoError(h.t, err)
	incrementInvokationCounter(instanceName, h.instanceCompletionInvocations)
}

func incrementInvokationCounter(instanceName string, counterMap map[string]int) {
	instanceNameCount, hasEntry := counterMap[instanceName]
	if !hasEntry {
		counterMap[instanceName] = 1
	} else {
		counterMap[instanceName] = instanceNameCount + 1
	}
}
