package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	"go.uber.org/zap"
	"os"
	"sync"
	"testing"
	"time"
)

func TestTimedStart(t *testing.T) {
	//	if os.Getenv("INT_TEST") != "true" {
	//		t.Skip("Skipping integration test " + t.Name())
	//	}

	tst := &integration{}
	tst.setup(t)
	defer tst.teardown()

	// Create a starting context
	ctx := context.Background()

	// Create logger
	log, _ := zap.NewDevelopment()

	// Dial shar
	cl := client.New(log, client.WithEphemeralStorage())
	err := cl.Dial(natsURL)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../testdata/timed-start-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TimedStartTest", b)
	require.NoError(t, err)

	d := &timedStartHandlerDef{}

	// Register a service task
	err = cl.RegisterServiceTask(ctx, "SimpleProcess", d.integrationSimple)
	require.NoError(t, err)

	// Launch the workflow
	if _, err := cl.LaunchWorkflow(ctx, "TimedStartTest", model.Vars{}); err != nil {
		panic(err)
	}

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(5 * time.Second)
	d.mx.Lock()
	defer d.mx.Unlock()
	assert.Equal(t, 3, d.count)
}

type timedStartHandlerDef struct {
	mx    sync.Mutex
	count int
}

func (d *timedStartHandlerDef) integrationSimple(ctx context.Context, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	d.mx.Lock()
	defer d.mx.Unlock()
	d.count++
	return vars, nil
}