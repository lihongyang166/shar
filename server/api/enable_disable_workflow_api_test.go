package api

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
)

func TestReturnsAuthErrorWhenDisableWorkflowAuthFails(t *testing.T) {
	ops := &workflow.MockOps{}
	auth := &MockAuth{}
	e := NewEndpoints(ops, auth, nil)

	ctx := context.Background()
	workflowName := "wfName"
	req := &model.DisableWorkflowRequest{WorkflowName: workflowName}

	expectedErrMessage := "failed to auth"
	auth.On("authForNamedWorkflow", ctx, workflowName).Return(nil, errors.New(expectedErrMessage))

	_, err := e.disableWorkflow(ctx, req)

	auth.AssertExpectations(t)
	ops.AssertNotCalled(t, "DisableWorkflow", ctx, workflowName)

	assert.ErrorContains(t, err, expectedErrMessage)

}

func TestDisableWorkflow(t *testing.T) {
	ops := &workflow.MockOps{}
	auth := &MockAuth{}
	e := NewEndpoints(ops, auth, nil)

	ctx := context.Background()
	wfName := "wfName"
	req := &model.DisableWorkflowRequest{WorkflowName: wfName}

	authCtx := context.Background()
	auth.On("authForNamedWorkflow", ctx, wfName).Return(authCtx, nil)
	ops.On("DisableWorkflow", authCtx, wfName).Return(nil)

	_, err := e.disableWorkflow(ctx, req)

	auth.AssertExpectations(t)
	ops.AssertExpectations(t)

	assert.NoError(t, err)
}
