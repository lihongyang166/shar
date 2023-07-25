package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"gitlab.com/shar-workflow/shar/model"
	zensvr "gitlab.com/shar-workflow/shar/zen-shar/server"
	"os"
	"time"
)

var cl *client.Client

var finished = make(chan struct{})

func main() {
	ss, ns, err := zensvr.GetServers("127.0.0.1", 4222, 8, nil, nil)
	defer ss.Shutdown()
	defer ns.Shutdown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl = client.New()
	if err := cl.Dial(ctx, nats.DefaultURL); err != nil {
		panic(err)
	}

	// Register a service task
	if err := taskutil.RegisterTaskYamlFile(ctx, cl, "task.step1.yaml", step1); err != nil {
		panic(err)
	}
	if err := taskutil.RegisterTaskYamlFile(ctx, cl, "task.step2.yaml", step2); err != nil {
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

	// Register a completion hook
	if err := cl.RegisterProcessComplete("Process_03llwnm", processEnd); err != nil {
		panic(err)
	}

	// Launch the workflow
	if _, _, err = cl.LaunchWorkflow(ctx, "MessageManualDemo", model.Vars{"orderId": 57, "carried": 128}); err != nil {
		panic(err)
	}

	// Listen for service tasks
	go func() {
		if err := cl.Listen(ctx); err != nil {
			panic(err)
		}
	}()

	// wait for the workflow to complete
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		panic("nope")

	}
}

func step1(ctx context.Context, _ client.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 1")
	fmt.Println("Sending Message...")
	if err := cl.SendMessage(ctx, "continueMessage", 57, model.Vars{"success": 32768}); err != nil {
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
