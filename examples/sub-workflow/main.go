package main

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"os"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/tools/tracer"
)

var finished = make(chan struct{})

func main() {
	// Create a starting context
	ctx := context.Background()

	t := tracer.Trace("127.0.0.1:4222")
	defer t.Close()
	// Dial shar
	cl := client.New(client.WithNamespace("testns"))
	if err := cl.Dial(ctx, nats.DefaultURL); err != nil {
		panic(err)
	}

	// Register the service tasks
	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "./examples/sub-workflow/task.BeforeCallingSubProcess.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "./examples/sub-workflow/task.BeforeCallingSubProcess.yaml", beforeCallingSubProcess); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "./examples/sub-workflow/task.DuringSubProcess.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "./examples/sub-workflow/task.DuringSubProcess.yaml", duringSubProcess); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "./examples/sub-workflow/task.AfterCallingSubProcess.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "./examples/sub-workflow/task.AfterCallingSubProcess.yaml", afterCallingSubProcess); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	// Load the workflows
	w1, _ := os.ReadFile("testdata/sub-workflow-parent.bpmn")
	w2, _ := os.ReadFile("testdata/sub-workflow-child.bpmn")
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "MasterWorkflowDemo", WorkflowBPMN: w1}); err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SubWorkflowDemo", WorkflowBPMN: w2}); err != nil {
		panic(err)
	}

	// Add a hook to watch for completion
	if err := cl.RegisterProcessComplete("WorkflowDemo", processEnd); err != nil {
		panic(err)
	}

	// Launch the workflow
	if _, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "WorkflowDemo"}); err != nil {
		panic(err)
	}
	go func() {
		if err := cl.Listen(ctx); err != nil {
			panic(err)
		}
	}()

	// wait for the workflow to complete
	<-finished
}

func afterCallingSubProcess(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println(vars.GetInt64("x"))
	return vars, nil
}

func duringSubProcess(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	z, err := vars.GetInt64("z")
	if err != nil {
		return nil, err
	}
	retVars := model.NewVars()
	retVars.SetInt64("z", z+41)
	return retVars, nil
}

func beforeCallingSubProcess(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	retVars := model.NewVars()
	retVars.SetInt64("x", 1)
	return retVars, nil
}

func processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	finished <- struct{}{}
}
