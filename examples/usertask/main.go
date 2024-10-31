package main

func main() {}

/*
import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"gitlab.com/shar-workflow/shar/model"
)

var finished = make(chan struct{})

func main() {
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New()
	if err := cl.Dial(ctx, nats.DefaultURL); err != nil {
		panic(err)
	}

	// Register service tasks
	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "task.Prepare.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "task.Prepare.yaml", prepare); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "task.Complete.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "task.Complete.yaml", prepare); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	// Load BPMN workflow
	b, err := os.ReadFile("testdata/usertask.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "UserTaskWorkflowDemo", WorkflowBPMN: b}); err != nil {
		panic(err)
	}

	// Add a hook to watch for completion
	if err := cl.RegisterProcessComplete("TestUserTasks", processEnd); err != nil {
		panic(err)
	}

	// Launch the workflow
	launchVars := model.NewVars()
	launchVars.SetInt64("OrderId", 68)
	if _, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "TestUserTasks", Vars: launchVars}); err != nil {
		panic(err)
	}

	// Listen for service tasks
	go func() {
		if err := cl.Listen(ctx); err != nil {
			panic(err)
		}
	}()

	go func() {
		for {
			tsk, err := cl.ListUserTaskIDs(ctx, "andrei")
			if err == nil && tsk.Id != nil {
				retVars := model.NewVars()
				retVars.SetString("Forename", "Brangelina")
				retVars.SetString("Surname", "Miggins")
				if err2 := cl.CompleteUserTask(ctx, "andrei", tsk.Id[0], retVars); err2 != nil {
					panic(err)
				}
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// wait for the workflow to complete
	<-finished
}

// A "Hello World" service task
func prepare(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Preparing")
	oid, err := vars.GetInt64("OrderId")
	if err != nil {
		return nil, err
	}
	retVars := model.NewVars()
	retVars.SetInt64("OrderId", oid+1)
	return retVars, nil
}

// A "Hello World" service task
func complete(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	oid, err := vars.GetInt64("OrderId")
	if err != nil {
		return nil, err
	}
	forename, err := vars.GetString("Forename")
	if err != nil {
		return nil, err
	}
	surname, err := vars.GetString("Surname")
	if err != nil {
		return nil, err
	}
	fmt.Println("OrderId", oid)
	fmt.Println("Forename", forename)
	fmt.Println("Surname", surname)
	return model.NewVars(), nil
}

func processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	finished <- struct{}{}
}
*/
