package simple

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSimple(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
	require.NoError(t, err)

	go func() {
		tst.TrackingUpdatesFor(ns, executionId, d.trackingReceived, 20*time.Second, t)
	}()

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, d.trackingReceived, 20*time.Second)
	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

const TICKET_ID = 555

func TestVarsWithAtCharacters(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "create-halo-ticket-task.yaml", d.createHaloTicket)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "var-with-at-symbol-task.yaml", d.sendEmail100)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "send-sms-task.yaml", d.sendSMS)
	require.NoError(t, err)
	
	processId := "demoWorkflow-1-0-1-process-1"
	err = cl.RegisterProcessComplete(processId, func(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
		close(d.finished)
	})
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/demoWorkflow-1-0-1-diagram.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "ProcessWithSvcTaskAtSymbol", b)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, processId, model.Vars{
		"ClientID_number":     123,
		"TicketTypeID_number": 456,
		"Summary_string":      "summaryString",
		"FirstName_string":    "firstName",
		"LastName_string":     "lastName",
		"Priority_number":     4,
		"CFdetails_string":    "CDdetails",
		"CFticketDT_string":   "ticketDt",
		"CFincidentDT_string": "fincidentDt",
		"CFopenReason_string": "openReason",
	})
	require.NoError(t, err)

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, d.finished, 20*time.Second)
	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSimpleHandlerDef struct {
	t                *testing.T
	finished         chan struct{}
	trackingReceived chan struct{}
}

func (d *testSimpleHandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["localVar"].(int))
	vars["Success"] = true
	return vars, nil
}

func (d *testSimpleHandlerDef) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["processVar"].(int))
	close(d.finished)
}

func (d *testSimpleHandlerDef) sendEmail100(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	assert.Equal(d.t, "matthew.brazel@vitrifi.net, keshava.gavigowda@vitrifi.net", vars["To"].(string))
	assert.Equal(d.t, fmt.Sprintf(`Halo ticket created %d`, TICKET_ID), vars["Subject"].(string))
	assert.Equal(d.t, fmt.Sprintf(`Halo ticket created %d`, TICKET_ID), vars["Body"].(string))
	vars["Success"] = true

	return vars, nil
}

func (d *testSimpleHandlerDef) createHaloTicket(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	vars["ticketID"] = TICKET_ID
	return vars, nil
}

func (d *testSimpleHandlerDef) sendSMS(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	return vars, nil
}
