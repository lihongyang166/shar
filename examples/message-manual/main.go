package main

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"os"
	"time"

	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	zensvr "gitlab.com/shar-workflow/shar/zen-shar/server"
)

var cl *client.Client

var finished = make(chan struct{})

func main() {
	ss, ns, err := zensvr.GetServers(8, nil, nil)
	defer ss.Shutdown()
	defer ns.Shutdown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl = client.New()

	if err := cl.Dial(ctx, fmt.Sprintf("nats://%s", ns.GetEndPoint())); err != nil {
		panic(err)
	}

	// Register a service task
	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "./examples/message-manual/task.step1.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "./examples/message-manual/task.step1.yaml", step1); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	// Register a service task
	if _, err := taskutil.LoadTaskFromYamlFile(ctx, cl, "./examples/message-manual/task.step2.yaml"); err != nil {
		panic(fmt.Errorf("load service task: %w", err))
	}
	if _, err := taskutil.RegisterTaskFunctionFromYamlFile(ctx, cl, "./examples/message-manual/task.step2.yaml", step2); err != nil {
		panic(fmt.Errorf("register service task function: %w", err))
	}

	workflowName := "MessageManualDemo"

	if err := cl.RegisterMessageSender(ctx, workflowName, "continueMessage", sendMessage); err != nil {
		panic(fmt.Errorf("failed to register message sender:  %w", err))
	}

	// Register a completion hook
	if err := cl.RegisterProcessComplete("Process_03llwnm", processEnd); err != nil {
		panic(err)
	}

	// Load BPMN workflow
	b, err := os.ReadFile("./testdata/message-manual-workflow.bpmn")
	if err != nil {
		panic(err)
	}
	if _, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: workflowName, WorkflowBPMN: b}); err != nil {
		panic(err)
	}

	launchVars := model.NewVars()
	launchVars.SetInt64("orderId", 57)
	launchVars.SetInt64("carried", 128)
	// Launch the workflow
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
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		panic("nope")

	}
}

func step1(ctx context.Context, _ task.JobClient, _ model.Vars) (model.Vars, error) {
	fmt.Println("Step 1")
	fmt.Println("Sending Message...")
	return model.NewVars(), nil
}

func step2(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Step 2")
	fmt.Println(vars.GetInt64("success"))
	return model.NewVars(), nil
}

func sendMessage(ctx context.Context, cl task.MessageClient, vars model.Vars) error {
	orderId, err := vars.GetString("orderId")
	if err != nil {
		return err
	}
	msgVars := model.NewVars()
	msgVars.SetInt64("success", 32768)
	if err := cl.SendMessage(ctx, "continueMessage", orderId, msgVars); err != nil {
		return fmt.Errorf("send continue message failed: %w", err)
	}
	return nil
}

func processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	finished <- struct{}{}
}
