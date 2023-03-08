package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	"os"
)

var cl *client.Client

var finished = make(chan struct{})

func main() {
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl = client.New()
	if err := cl.Dial(nats.DefaultURL); err != nil {
		panic(err)
	}

	// Load BPMN workflow
	b, err := os.ReadFile("testdata/message-manual-workflow.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, "MessageManualDemo", b); err != nil {
		panic(err)
	}

	// Register a service task
	err = cl.RegisterServiceTask(ctx, "step1", step1)
	if err != nil {
		panic(err)
	}
	err = cl.RegisterServiceTask(ctx, "step2", step2)
	if err != nil {
		panic(err)
	}
	err = cl.RegisterProcessComplete("Process_03llwnm", processEnd)
	if err != nil {
		panic(err)
	}

	// Launch the workflow
	_, _, err = cl.LaunchWorkflow(ctx, "MessageManualDemo", model.Vars{"orderId": 57})
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

	// wait for the workflow to complete
	<-finished
}

func step1(ctx context.Context, _ client.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 1")
	fmt.Println("Sending Message...")
	if err := cl.SendMessage(ctx, "", "continueMessage", 57, model.Vars{"success": 32768}); err != nil {
		return nil, fmt.Errorf("send continue message failed: %w", err)
	}
	return model.Vars{}, nil
}

func step2(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Step 2")
	fmt.Println(vars["success"])
	return model.Vars{}, nil
}

func processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	finished <- struct{}{}
}
