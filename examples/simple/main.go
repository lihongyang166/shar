package main

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"os"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
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

	// Register a service task
	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "./examples/simple/task.SimpleProcess.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "./examples/simple/task.SimpleProcess.yaml", simpleProcess); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	// Load BPMN workflow
	b, err := os.ReadFile("testdata/simple-workflow.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowDemo", b); err != nil {
		panic(err)
	}

	if err := cl.RegisterProcessComplete("SimpleProcess", processEnd); err != nil {
		panic(err)
	}

	// A hook to watch for completion

	// Launch the workflow
	if _, _, err = cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{}); err != nil {
		panic(err)
	}

	// Listen for service tasks
	go func() {
		if err := cl.Listen(ctx); err != nil {
			panic(err)
		}
	}()

	// wait for the workflow to complete
	<-finished
}

// A "Hello World" service task
func simpleProcess(_ context.Context, _ client.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Hello World")
	return model.Vars{}, nil
}

func processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	finished <- struct{}{}
}
