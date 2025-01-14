package main

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"os"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"gitlab.com/shar-workflow/shar/model"
	zensvr "gitlab.com/shar-workflow/shar/zen-shar/server"
)

var finished = make(chan struct{})

func main() {
	ss, ns, err := zensvr.GetServers(8, nil, nil)
	defer ss.Shutdown()
	defer ns.Shutdown()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New()
	if err := cl.Dial(ctx, nats.DefaultURL); err != nil {
		panic(err)
	}

	// Load BPMN workflow
	b, err := os.ReadFile("testdata/message-workflow.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "MessageDemo", WorkflowBPMN: b}); err != nil {
		panic(err)
	}

	// Register a service task
	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "task.step1.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "task.step1.yaml", step1); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "task.step2.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "task.step2.yaml", step2); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	if err := cl.RegisterMessageSender(ctx, "MessageDemo", "continueMessage", sendMessage); err != nil {
		panic(err)
	}
	// A hook to watch for completion
	if err := cl.RegisterProcessComplete("Process_03llwnm", processEnd); err != nil {
		panic(err)
	}

	// Launch the workflow
	launchVars := model.NewVars()
	launchVars.SetInt64("orderId", 57)
	launchVars.SetString("carried", "payload")
	if _, _, err = cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "Process_03llwnm", Vars: launchVars}); err != nil {
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

func step1(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 1")
	return model.NewVars(), nil
}

func step2(_ context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 2")
	return model.NewVars(), nil
}

func sendMessage(ctx context.Context, cmd task.MessageClient, vars model.Vars) error {
	fmt.Println("Sending Message...")
	msgVars := model.NewVars()
	carried, err := vars.GetString("carried")
	if err != nil {
		return err
	}
	msgVars.SetString("carried", carried)
	return cmd.SendMessage(ctx, "continueMessage", 57, msgVars)
}

func processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	fmt.Println(vars)
	finished <- struct{}{}
}
