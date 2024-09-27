package simple

import (
	"context"
	"fmt"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestTaskVersion(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSTVersionDef{t: t, finished: make(chan struct{})}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "GetCapitalData.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("GetCapitalData_test", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("GetCapitalData_test.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "GetCapitalData", WorkflowBPMN: b})
	require.NoError(t, err)

	newVars := model.NewVars()
	newVars.SetString("city", "Dublin")
	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "GetCapitalData_test", Vars: newVars})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)
	cl.Shutdown()
	cl = client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err = cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "GetCapitalDataV2.yaml", d.integrationSimple)
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "GetCapitalData", WorkflowBPMN: b})
	require.NoError(t, err)

	// Launch the workflow
	launchVars := model.NewVars()
	launchVars.SetString("city", "Dublin")
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "GetCapitalData_test", Vars: launchVars})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSTVersionDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testSTVersionDef) integrationSimple(ctx context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	vars.SetString("region", "ireland")
	vars.SetInt64("population", 3)
	vars.SetString("language", "english")
	vars.SetFloat64("latitude", 50.342)
	vars.SetFloat64("longitude", 1.345)
	return vars, nil
}

func (d *testSTVersionDef) processEnd(_ context.Context, _ model.Vars, _ *model.Error, _ model.CancellationState) {
	d.finished <- struct{}{}
}
