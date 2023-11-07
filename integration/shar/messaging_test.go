package intTest

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"os"
	"sync"
	"testing"
	"time"
)

type MessagingTestSuite struct {
	suite.Suite
	integrationSupport *support.Integration
	ctx                context.Context
	client             *client.Client
}

func (suite *MessagingTestSuite) SetupTest() {
	suite.integrationSupport = &support.Integration{}
	//tst.WithTrace = true
	suite.integrationSupport.Setup(suite.T(), nil, nil)
	suite.ctx = context.Background()

	suite.client = client.New(client.WithEphemeralStorage(), client.WithConcurrency(10))
	err := suite.client.Dial(suite.ctx, suite.integrationSupport.NatsURL)
	require.NoError(suite.T(), err)
}

func (suite *MessagingTestSuite) TearDownTest() {
	suite.integrationSupport.Teardown()
}

func TestMessagingTestSuite(t *testing.T) {
	suite.Run(t, new(MessagingTestSuite))
}

//goland:noinspection GoNilness
func (suite *MessagingTestSuite) TestMessaging() {
	t := suite.T()
	tst := suite.integrationSupport
	ctx := suite.ctx
	cl := suite.client

	handlers := &testMessagingHandlerDef{t: t, wg: sync.WaitGroup{}, tst: tst, finished: make(chan struct{})}

	// Register service tasks
	err := taskutil.RegisterTaskYamlFile(ctx, cl, "messaging_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	err = taskutil.RegisterTaskYamlFile(ctx, cl, "messaging_test_step2.yaml", handlers.step2)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/message-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestMessaging", b)
	require.NoError(t, err)

	err = cl.RegisterMessageSender(ctx, "TestMessaging", "continueMessage", handlers.sendMessage)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("Process_03llwnm", handlers.processEnd)
	require.NoError(t, err)

	// Launch the processes
	_, _, err = cl.LaunchProcess(ctx, "Process_0hgpt6k", model.Vars{"orderId": 57})
	if err != nil {
		t.Fatal(err)
		return
	}

	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, handlers.finished, 20*time.Second)

	tst.AssertCleanKV()
}

func (suite *MessagingTestSuite) TestMessageNameGlobalUniqueness() {
	t := suite.T()
	ctx := suite.ctx
	cl := suite.client
	tst := suite.integrationSupport

	handlers := &testMessagingHandlerDef{t: t, wg: sync.WaitGroup{}, tst: tst, finished: make(chan struct{})}

	// Register service tasks
	err := taskutil.RegisterTaskYamlFile(ctx, cl, "messaging_test_step1.yaml", handlers.step1)
	require.NoError(t, err)
	err = taskutil.RegisterTaskYamlFile(ctx, cl, "messaging_test_step2.yaml", handlers.step2)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/message-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestMessaging", b)
	require.NoError(t, err)

	// try to load another bpmn with a message of the same name, should fail
	b, err = os.ReadFile("../../testdata/message-workflow-duplicate-message.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "TestMessagingDupMessage", b)
	require.ErrorContains(t, err, "These messages already exist for other workflows:")

	tst.AssertCleanKV()
}

type testMessagingHandlerDef struct {
	wg       sync.WaitGroup
	tst      *support.Integration
	finished chan struct{}
	t        *testing.T
}

func (x *testMessagingHandlerDef) step1(ctx context.Context, client client.JobClient, _ model.Vars) (model.Vars, error) {
	if err := client.Log(ctx, messages.LogInfo, -1, "Step 1", nil); err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	return model.Vars{}, nil
}

func (x *testMessagingHandlerDef) step2(ctx context.Context, client client.JobClient, vars model.Vars) (model.Vars, error) {
	if err := client.Log(ctx, messages.LogInfo, -1, "Step 2", nil); err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	x.tst.Mx.Lock()
	x.tst.FinalVars = vars
	x.tst.Mx.Unlock()
	return model.Vars{}, nil
}

func (x *testMessagingHandlerDef) sendMessage(ctx context.Context, client client.MessageClient, vars model.Vars, executionId string, elementId string) error {
	if err := client.Log(ctx, messages.LogDebug, -1, "Sending Message...", nil); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	if err := client.SendMessage(ctx, "continueMessage", 57, model.Vars{"carried": vars["carried"]}, executionId, elementId); err != nil {
		return fmt.Errorf("send continue message: %w", err)
	}
	return nil
}

func (x *testMessagingHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {

	assert.Equal(x.t, "carried1value", vars["carried"])
	assert.Equal(x.t, "carried2value", vars["carried2"])
	close(x.finished)
}
