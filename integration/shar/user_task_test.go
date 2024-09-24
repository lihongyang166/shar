package intTest

import (
	"context"
	"fmt"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"os"
	"sync"
	"testing"
	"time"
)

func TestUserTasks(t *testing.T) {
	t.Parallel()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	if err := cl.Dial(ctx, tst.NatsURL); err != nil {
		require.NoError(t, err)
	}

	//sub := tracer.Trace(NatsURL)
	//defer sub.Drain()

	d := &testUserTaskHandlerDef{finished: make(chan struct{}), t: t}
	d.finalVars = model.NewVars()

	// Register service tasks
	_, err := support.RegisterTaskYamlFile(ctx, cl, "user_task_test_Prepare.yaml", d.prepare)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "user_task_test_Complete.yaml", d.complete)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/usertask.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "TestUserTasks", WorkflowBPMN: b})
	require.NoError(t, err)

	err = cl.RegisterProcessComplete("TestUserTasks", d.processEnd)
	require.NoError(t, err)
	// Launch the workflow
	launchVars := model.NewVars()
	launchVars.SetInt64("OrderId", 68)
	_, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "TestUserTasks", Vars: launchVars})
	if err != nil {
		panic(err)
	}

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	go func() {
		for {
			tsk, err := cl.ListUserTaskIDs(ctx, "andrei")
			require.NoError(t, err)
			if err == nil && tsk.Id != nil {
				td, _, gErr := cl.GetUserTask(ctx, "andrei", tsk.Id[0])
				assert.NoError(t, gErr)
				fmt.Printf("%+v\n", td)
				fmt.Println("Name:", td.Name)
				fmt.Println("Description:", td.Description)
				newVars := model.NewVars()
				newVars.SetString("Forename", "Brangelina")
				newVars.SetString("Surname", "Miggins")
				cErr := cl.CompleteUserTask(ctx, "andrei", tsk.Id[0], newVars)
				assert.NoError(t, cErr)
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()

	support.WaitForChan(t, d.finished, 50*time.Second)

	et, err := cl.ListUserTaskIDs(ctx, "andrei")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(et.Id))
	d.lock.Lock()
	defer d.lock.Unlock()
	forename, err := d.finalVars.GetString("Forename")
	require.NoError(t, err)
	surname, err := d.finalVars.GetString("Surname")
	require.NoError(t, err)
	orderId, err := d.finalVars.GetInt64("OrderId")
	require.NoError(t, err)
	carried, err := d.finalVars.GetInt64("carried")
	require.NoError(t, err)
	assert.Equal(t, "Brangelina", forename)
	assert.Equal(t, "Miggins", surname)
	assert.Equal(t, int64(69), orderId)
	assert.Equal(t, int64(32767), carried)
	tst.AssertCleanKV(ns, t, tst.Cooldown)
}

type testUserTaskHandlerDef struct {
	finalVars model.Vars
	lock      sync.Mutex
	finished  chan struct{}
	t         *testing.T
}

// A "Hello World" service task
func (d *testUserTaskHandlerDef) prepare(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Preparing")
	orderId, err := vars.GetInt64("OrderId")
	require.NoError(d.t, err)
	newVars := model.NewVars()
	newVars.SetInt64("OrderId", orderId+1)
	return newVars, nil
}

// A "Hello World" service task
func (d *testUserTaskHandlerDef) complete(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Completed")
	orderId, err := vars.GetInt64("OrderId")
	require.NoError(d.t, err)
	forename, err := vars.GetString("Forename")
	require.NoError(d.t, err)
	surname, err := vars.GetString("Surname")
	require.NoError(d.t, err)
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	fmt.Println("OrderId", orderId)
	fmt.Println("Forename", forename)
	fmt.Println("Surname", surname)
	fmt.Println("carried", carried)
	d.lock.Lock()
	defer d.lock.Unlock()
	d.finalVars = vars
	return model.NewVars(), nil
}

func (d *testUserTaskHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}
