package api

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
)

func TestDisableWorkflowAuth(t *testing.T) {

}

func TestReturnsAuthErrorWhenDisableWorkflowAuthFails(t *testing.T) {
	cases := map[string]struct {
		getFatalErrorRequest *model.GetFatalErrorRequest

		authMethodCallMockFn func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest, expectedErr error)
	}{
		"WorkflowNameAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "workFlowName",
				ExecutionId:       "",
				ProcessInstanceId: "",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest, expectedErr error) {
				mockAuth.On("authForNamedWorkflow", ctx, req.WfName).Return(nil, expectedErr)
			},
		},
		"ExecutionIdAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "",
				ExecutionId:       "ExecutionId",
				ProcessInstanceId: "",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest, expectedErr error) {
				mockAuth.On("authFromExecutionID", ctx, req.ExecutionId).Return(nil, nil, expectedErr)
			},
		},
		"ProcessInstanceIdAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "",
				ExecutionId:       "",
				ProcessInstanceId: "processInstanceId",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest, expectedErr error) {
				mockAuth.On("authFromProcessInstanceID", ctx, req.ProcessInstanceId).Return(nil, nil, expectedErr)
			},
		},
	}

	for name, testParams := range cases {
		t.Run(name, func(t *testing.T) {
			mockOperations := &workflow.MockOps{}
			mockAuth := &MockAuth{}

			endpoints := newEndpoints(mockOperations, mockAuth)

			ctx := context.Background()
			respCh := make(chan *model.FatalError)
			errsCh := make(chan error)

			req := testParams.getFatalErrorRequest

			expectedErr := errors.New("auth error")
			testParams.authMethodCallMockFn(mockAuth, ctx, req, expectedErr)

			go func() {
				endpoints.getFatalErrors(ctx, req, respCh, errsCh)
			}()

			actualErrResponse := <-errsCh

			mockAuth.AssertExpectations(t)
			mockOperations.AssertNotCalled(t, "GetFatalErrors")

			assert.True(t, errors.Is(actualErrResponse, expectedErr))
		})
	}
}
