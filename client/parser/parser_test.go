package parser

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/model"
	"os"
	"slices"
	"testing"
)

//goland:noinspection GoNilness
func TestParseWorkflowDuration(t *testing.T) {
	ctx := context.Background()
	eng := &expression.ExprEngine{}
	b, err := os.ReadFile("../../testdata/test-timer-parse-duration.bpmn")
	require.NoError(t, err)
	p, err := Parse(ctx, eng, "Test", bytes.NewBuffer(b))
	require.NoError(t, err)
	assert.Equal(t, "2000000000", p.Process["Process_0cxoltv"].Elements[1].Execute)
}

func TestMessageStart(t *testing.T) {
	ctx := context.Background()
	eng := &expression.ExprEngine{}
	b, err := os.ReadFile("../../testdata/message-start-test.bpmn")
	require.NoError(t, err)
	workflowName := "MessageStart"
	wf, err := Parse(ctx, eng, workflowName, bytes.NewBuffer(b))
	require.NoError(t, err)

	processIdWithMessageStartEvent := "Process_0w6dssp"
	processElements := wf.Process[processIdWithMessageStartEvent].Elements

	expectedMessageStartEventId := "StartEvent_1"
	expectedType := element.StartEvent
	expectedMessageName := "startDemoMsg"

	require.True(t, slices.ContainsFunc(
		processElements,
		func(e *model.Element) bool {
			return e.Id == expectedMessageStartEventId &&
				e.Type == expectedType &&
				e.Msg == expectedMessageName
		},
	),
		"does not contain the expected start event message '%s', type '%s' and messageName '%s'", expectedMessageStartEventId, expectedType, expectedMessageName)

	startMessageMessageReceivers := wf.MessageReceivers[expectedMessageName]

	require.True(t, slices.ContainsFunc(
		startMessageMessageReceivers.MessageReceiver,
		func(e *model.MessageReceiver) bool {
			return e.Id == expectedMessageStartEventId &&
				e.ProcessIdToStart == processIdWithMessageStartEvent
		},
	),
		"message receiver did not have expected Id and ProcessIdToStart")

	assert.Equal(t, workflowName, startMessageMessageReceivers.AssociatedWorkflowName)
}

func TestExclusiveParse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	eng := &expression.ExprEngine{}
	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/gateway-exclusive-out-and-in-test.bpmn")
	require.NoError(t, err)

	wf, err := Parse(ctx, eng, "SimpleWorkflowTest", bytes.NewBuffer(b))
	require.NoError(t, err)

	els := make(map[string]*model.Element)
	common.IndexProcessElements(wf.Process["Process_0ljss15"].Elements, els)
	assert.Equal(t, model.GatewayType_exclusive, els["Gateway_01xjq2a"].Gateway.Type)
	assert.Equal(t, model.GatewayDirection_divergent, els["Gateway_01xjq2a"].Gateway.Direction)
	assert.Equal(t, model.GatewayType_exclusive, els["Gateway_1ps8xyt"].Gateway.Type)
	assert.Equal(t, model.GatewayDirection_convergent, els["Gateway_1ps8xyt"].Gateway.Direction)
	assert.Equal(t, "Gateway_01xjq2a", els["Gateway_1ps8xyt"].Gateway.ReciprocalId)
	assert.Equal(t, "Gateway_1ps8xyt", els["Gateway_01xjq2a"].Gateway.ReciprocalId)
}

func TestNestedExclusiveParse(t *testing.T) {
	t.Parallel()

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/gateway-multi-exclusive-out-and-in-test.bpmn")
	require.NoError(t, err)

	ctx := context.Background()
	eng := &expression.ExprEngine{}
	wf, err := Parse(ctx, eng, "SimpleWorkflowTest", bytes.NewBuffer(b))
	require.NoError(t, err)

	els := make(map[string]*model.Element)
	common.IndexProcessElements(wf.Process["Process_0ljss15"].Elements, els)
	assert.Equal(t, "Gateway_0bcqcrc", els["Gateway_1ucd1b5"].Gateway.ReciprocalId)
	assert.Equal(t, "Gateway_1ps8xyt", els["Gateway_01xjq2a"].Gateway.ReciprocalId)

	assert.Equal(t, "=value1", els["Gateway_1ps8xyt"].OutputTransform["value1"])
	assert.Equal(t, "=value2", els["Gateway_1ps8xyt"].OutputTransform["value2"])
	assert.Equal(t, "=val11", els["Gateway_1ps8xyt"].OutputTransform["val11"])
	assert.Equal(t, "=val12", els["Gateway_1ps8xyt"].OutputTransform["val12"])
}

func TestConvergentInclusiveGatewayHasOutputTransform(t *testing.T) {
	b, err := os.ReadFile("../../testdata/gateway-inclusive-complex-out-and-in-test.bpmn")
	require.NoError(t, err)
	workflowName := "GatewayInclusive"
	ctx := context.Background()
	eng := &expression.ExprEngine{}
	wf, err := Parse(ctx, eng, workflowName, bytes.NewBuffer(b))
	require.NoError(t, err)

	elements := wf.Process["Process_0ljss15"].Elements
	convergentGatewayIdx := slices.IndexFunc(elements, func(e *model.Element) bool {
		return e.Id == "Gateway_1ps8xyt"
	})
	convergentGatewayEl := elements[convergentGatewayIdx]

	assert.Equal(t, "=value1", convergentGatewayEl.OutputTransform["value1"])
	assert.Equal(t, "=value2", convergentGatewayEl.OutputTransform["value2"])
	assert.Equal(t, "=value4", convergentGatewayEl.OutputTransform["value4"])
}
