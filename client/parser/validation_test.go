package parser

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/server/errors"
	"os"
	"testing"
)

func Test_MissingServiceTaskExecute(t *testing.T) {

	b, err := os.ReadFile("../../testdata/bad/missing-servicetask-definition.bpmn")
	require.NoError(t, err)
	_, err = Parse("TestWorkflow", bytes.NewBuffer(b))
	assert.ErrorIs(t, err, errors.ErrMissingServiceTaskDefinition)
}

func TestValidateAgainstInputVariableReferencingNonExistentVar(t *testing.T) {
	b, err := os.ReadFile("../../testdata/bad/gateway-inclusive-input-ref-undefined-var.bpmn")
	require.NoError(t, err)
	_, err = Parse("TestWorkflow", bytes.NewBuffer(b))
	assert.ErrorIs(t, err, errors.ErrUndefinedVariable)
}

func TestValidateAgainstConditionReferencingNonExistentVar(t *testing.T) {
	b, err := os.ReadFile("../../testdata/bad/gateway-inclusive-cond-ref-undefined-var.bpmn")
	require.NoError(t, err)
	_, err = Parse("TestWorkflow", bytes.NewBuffer(b))
	assert.ErrorIs(t, err, errors.ErrUndefinedVariable)
}

func TestValidateAgainstInputReferencingNonExistentErrorVar(t *testing.T) {
	b, err := os.ReadFile("../../testdata/bad/errors-ref-undefined-var.bpmn")
	require.NoError(t, err)
	_, err = Parse("TestWorkflow", bytes.NewBuffer(b))
	assert.ErrorIs(t, err, errors.ErrUndefinedVariable)
}

func TestValidateSucceedsWhenInputReferencesErrorVar(t *testing.T) {
	b, err := os.ReadFile("../../testdata/errors.bpmn")
	require.NoError(t, err)
	_, err = Parse("TestWorkflow", bytes.NewBuffer(b))
	assert.NoError(t, err)
}

func TestValidationSucceedsWhenParallelGatewayDefinesDistinctVarsOnDistinctBranches(t *testing.T) {
	b, err := os.ReadFile("../../testdata/gateway-parallel-out-and-in-test.bpmn")
	require.NoError(t, err)
	_, err = Parse("TestWorkflow", bytes.NewBuffer(b))
	assert.NoError(t, err)
}
