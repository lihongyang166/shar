package simple

import (
	"context"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/server/tools/tracer"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
)

func TestBankAccountNoCompensation(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	s := tracer.Trace(tst.NatsURL)
	defer s.Close()

	// Register a service task
	d := &testBankAccount{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "deductFromPayee.yaml", d.deductFromPayee)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "applyToRecipient.yaml", d.applyToRecipient)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "compensatePayee.yaml", d.compensatePayee)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "compensateRecipient.yaml", d.compensateRecipient)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("BankTransfer", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/bankTransfer.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "BankTransfer", WorkflowBPMN: b})
	require.NoError(t, err)

	launchVars := model.NewVars()
	launchVars.SetString("approved", "No")
	launchVars.SetFloat64("transferAmount", 6.50)
	launchVars.SetFloat64("payeeAccountBalance", 125.00)
	launchVars.SetFloat64("recipientAccountBalance", 100.00)

	// Launch the workflow
	executionId, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "BankTransfer", Vars: launchVars})
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
	assert.Equal(t, 125.0, d.finalPayeeBalance)
	assert.Equal(t, 100.0, d.finalRecipientBalance)
}

func TestBankAccountCompensation(t *testing.T) {
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	s := tracer.Trace(tst.NatsURL)
	defer s.Close()

	// Register a service task
	d := &testBankAccount{t: t, finished: make(chan struct{}), trackingReceived: make(chan struct{}, 1)}

	_, err = support.RegisterTaskYamlFile(ctx, cl, "deductFromPayee.yaml", d.deductFromPayee)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "applyToRecipient.yaml", d.applyToRecipient)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "compensatePayee.yaml", d.compensatePayee)
	require.NoError(t, err)
	_, err = support.RegisterTaskYamlFile(ctx, cl, "compensateRecipient.yaml", d.compensateRecipient)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("BankTransfer", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/bankTransfer.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "BankTransfer", WorkflowBPMN: b})
	require.NoError(t, err)

	launchVars := model.NewVars()
	launchVars.SetString("approved", "Yes")
	launchVars.SetFloat64("transferAmount", 6.50)
	launchVars.SetFloat64("payeeAccountBalance", 125.00)
	launchVars.SetFloat64("recipientAccountBalance", 100.00)

	// Launch the workflow
	executionID, _, err := cl.LaunchProcess(ctx, client.LaunchParams{ProcessID: "BankTransfer", Vars: launchVars})
	require.NoError(t, err)

	go func() {
		tst.TrackingUpdatesFor(ns, executionID, d.trackingReceived, 20*time.Second, t)
	}()

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()

	support.WaitForChan(t, d.trackingReceived, 20*time.Second)
	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
	assert.Equal(t, 118.5, d.finalPayeeBalance)
	assert.Equal(t, 106.5, d.finalRecipientBalance)
}

type testBankAccount struct {
	t                     *testing.T
	finished              chan struct{}
	trackingReceived      chan struct{}
	finalPayeeBalance     float64
	finalRecipientBalance float64
}

func (d *testBankAccount) applyToRecipient(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	slog.Info("applyToRecipient")
	recipientAccountBalance, err := vars.GetFloat64("recipientAccountBalance")
	require.NoError(d.t, err)
	transferAmount, err := vars.GetFloat64("transferAmount")
	require.NoError(d.t, err)
	vars.SetFloat64("recipientAccountBalance", recipientAccountBalance+transferAmount)
	return vars, nil
}

func (d *testBankAccount) deductFromPayee(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	payeeAccountBalance, err := vars.GetFloat64("payeeAccountBalance")
	require.NoError(d.t, err)
	transferAmount, err := vars.GetFloat64("transferAmount")
	require.NoError(d.t, err)
	vars.SetFloat64("payeeAccountBalance", payeeAccountBalance-transferAmount)
	return vars, nil
}
func (d *testBankAccount) compensateRecipient(_ context.Context, c task.JobClient, vars model.Vars) (model.Vars, error) {
	inputs, _ := c.OriginalVars()
	balance, err := vars.GetFloat64("recipientAccountBalance")
	require.NoError(d.t, err)
	amount, err := inputs.GetFloat64("transferAmount")
	require.NoError(d.t, err)
	vars.SetFloat64("recipientAccountBalance", balance-amount)
	return vars, nil
}
func (d *testBankAccount) compensatePayee(_ context.Context, c task.JobClient, vars model.Vars) (model.Vars, error) {
	inputs, _ := c.OriginalVars()
	balance, err := vars.GetFloat64("payeeAccountBalance")
	require.NoError(d.t, err)
	amount, err := inputs.GetFloat64("transferAmount")
	require.NoError(d.t, err)
	vars.SetFloat64("payeeAccountBalance", balance+amount)
	return vars, nil
}
func (d *testBankAccount) processEnd(_ context.Context, vars model.Vars, _ *model.Error, _ model.CancellationState) {
	var err error
	d.finalPayeeBalance, err = vars.GetFloat64("finalAccountBalance")
	require.NoError(d.t, err)
	d.finalRecipientBalance, err = vars.GetFloat64("finalRecipientAccountBalance")
	require.NoError(d.t, err)
	assert.Equal(d.t, 2, vars.Len())
	close(d.finished)
}
