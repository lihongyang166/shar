package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	"go.uber.org/zap"
	"os"
)

var cl *client.Client

func main() {
	// Create a starting context
	ctx := context.Background()

	// Create logger
	log, _ := zap.NewDevelopment()

	defer func() {
		if err := log.Sync(); err != nil {
		}
	}()

	// Dial shar
	cl = client.New(log)
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
	// A hook to watch for completion
	complete := make(chan *model.WorkflowInstanceComplete, 100)
	cl.RegisterWorkflowInstanceComplete(complete)

	// Launch the workflow
	wfiID, err := cl.LaunchWorkflow(ctx, "MessageManualDemo", model.Vars{"orderId": 57})
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
	for i := range complete {
		if i.WorkflowInstanceId == wfiID {
			break
		}
	}
}

func step1(ctx context.Context, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 1")
	fmt.Println("Sending Message...")
	if err := cl.SendMessage(ctx, "", "continueMessage", 57, model.Vars{"success": 32768}); err != nil {
		return nil, err
	}
	return model.Vars{}, nil
}

func step2(_ context.Context, vars model.Vars) (model.Vars, error) {
	fmt.Println("Step 2")
	fmt.Println(vars["success"])
	return model.Vars{}, nil
}
