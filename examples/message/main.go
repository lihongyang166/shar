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
	cl := client.New(log)
	if err := cl.Dial(nats.DefaultURL); err != nil {
		panic(err)
	}

	// Load BPMN workflow
	b, err := os.ReadFile("testdata/message-workflow.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, "MessageDemo", b); err != nil {
		panic(err)
	}

	// Register a service task
	cl.RegisterServiceTask("step1", step1)
	cl.RegisterServiceTask("step2", step2)
	cl.RegisterMessageSender("continueMessage", sendMessage)

	// A hook to watch for completion
	complete := make(chan *model.WorkflowInstanceComplete, 100)
	cl.RegisterWorkflowInstanceComplete(complete)

	// Launch the workflow
	wfiID, err := cl.LaunchWorkflow(ctx, "MessageDemo", model.Vars{"orderId": 57})
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

func step1(_ context.Context, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 1")
	return model.Vars{}, nil
}

func step2(_ context.Context, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 2")
	return model.Vars{}, nil
}

func sendMessage(ctx context.Context, cmd *client.Command, _ model.Vars) error {
	fmt.Println("Sending Message...")
	return cmd.SendMessage(ctx, "continueMessage", 57, model.Vars{})
}