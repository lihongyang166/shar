package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	options2 "gitlab.com/shar-workflow/shar/server/server/option"
	"testing"
)

func TestFatalErrorKeyPrefixBuilderWithAllSegments(t *testing.T) {
	wfName := "wfName"
	workflowId := "workflowId"
	executionId := "executionId"
	processInstanceId := "processInstanceId"

	req := &model.GetFatalErrorRequest{
		WfName:            wfName,
		WfId:              workflowId,
		ExecutionId:       executionId,
		ProcessInstanceId: processInstanceId,
	}

	keyPrefix := fatalErrorKeyPrefixBuilder(req)

	assert.Equal(t, fmt.Sprintf("%s.%s.%s.%s", wfName, workflowId, executionId, processInstanceId), keyPrefix)
}

func TestFatalErrorKeyPrefixBuilderWithPartialSegments(t *testing.T) {
	wfName := "wfName"
	executionId := ""
	processInstanceId := "processInstanceId"

	req := &model.GetFatalErrorRequest{
		WfName:            wfName,
		ExecutionId:       executionId,
		ProcessInstanceId: processInstanceId,
	}

	keyPrefix := fatalErrorKeyPrefixBuilder(req)

	assert.Equal(t, fmt.Sprintf("%s.%s.%s.%s", wfName, "*", "*", processInstanceId), keyPrefix)
}

func TestGetFatalErrorValidation(t *testing.T) {
	cases := map[string]struct {
		getFatalErrorRequest *model.GetFatalErrorRequest
		expectedError        string
	}{
		"AllParamsEmptyString": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "",
				WfId:              "",
				ExecutionId:       "",
				ProcessInstanceId: "",
			},
			expectedError: invalidGetFatalErrorRequestEmptyParams,
		},
		"AllParamsWildcardString": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "*",
				WfId:              "*",
				ExecutionId:       "*",
				ProcessInstanceId: "*",
			},
			expectedError: invalidGetFatalErrorRequestWildcardedParams,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			mockOperations := &workflow.MockOps{}
			mockAuth := &MockAuth{}

			endpoints, _ := New(mockOperations, nil, mockAuth, &options2.ServerOptions{})

			req := tc.getFatalErrorRequest

			respChan := make(chan *model.FatalError)
			errChan := make(chan error)

			//when
			go func() {
				endpoints.getFatalErrors(ctx, req, respChan, errChan)
			}()

			expectedErr := <-errChan
			//then
			assert.ErrorContains(t, expectedErr, tc.expectedError)

			mockOperations.AssertNotCalled(t, "GetFatalErrors")
		})
	}

}

type USERNAME string

func TestFatalErrorAuth(t *testing.T) {
	cases := map[string]struct {
		getFatalErrorRequest *model.GetFatalErrorRequest

		authMethodCallMockFn func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest) context.Context
	}{
		"WorkflowNameAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "workFlowName",
				ExecutionId:       "",
				ProcessInstanceId: "",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest) context.Context {
				authCtx := context.WithValue(ctx, USERNAME("userName"), "foo-user")
				mockAuth.On("authForNamedWorkflow", ctx, req.WfName).Return(authCtx, nil)
				return authCtx
			},
		},
		"WorkflowIdAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "",
				WfId:              "workflowId",
				ExecutionId:       "",
				ProcessInstanceId: "",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest) context.Context {
				authCtx := context.WithValue(ctx, USERNAME("userName"), "foo-user")
				mockAuth.On("authForWorkflowId", ctx, req.WfId).Return(authCtx, nil)
				return authCtx
			},
		},
		"ExecutionIdAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "",
				ExecutionId:       "ExecutionId",
				ProcessInstanceId: "",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest) context.Context {
				authCtx := context.WithValue(ctx, USERNAME("userName"), "foo-user")
				mockAuth.On("authFromExecutionID", ctx, req.ExecutionId).Return(authCtx, nil, nil)
				return authCtx
			},
		},
		"ProcessInstanceIdAuth": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "",
				ExecutionId:       "",
				ProcessInstanceId: "processInstanceId",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest) context.Context {
				authCtx := context.WithValue(ctx, USERNAME("userName"), "foo-user")
				mockAuth.On("authFromProcessInstanceID", ctx, req.ProcessInstanceId).Return(authCtx, nil, nil)
				return authCtx
			},
		},
		"OnlyAuthAgainstOneParameter": {
			getFatalErrorRequest: &model.GetFatalErrorRequest{
				WfName:            "wfName",
				ExecutionId:       "executionId",
				ProcessInstanceId: "processInstanceId",
			},
			authMethodCallMockFn: func(mockAuth *MockAuth, ctx context.Context, req *model.GetFatalErrorRequest) context.Context {
				authCtx := context.WithValue(ctx, USERNAME("userName"), "foo-user")
				mockAuth.On("authForNamedWorkflow", ctx, req.WfName).Return(authCtx, nil)
				return authCtx
			},
		},
	}

	for name, testParams := range cases {
		t.Run(name, func(t *testing.T) {
			mockOperations := &workflow.MockOps{}
			mockAuth := &MockAuth{}

			endpoints, _ := New(mockOperations, nil, mockAuth, &options2.ServerOptions{})

			ctx := context.Background()
			respCh := make(chan *model.FatalError)
			errsCh := make(chan error)

			req := testParams.getFatalErrorRequest

			authCtx := testParams.authMethodCallMockFn(mockAuth, ctx, req)

			expectedFatalErr := &model.FatalError{}
			mockOperations.On("GetFatalErrors", authCtx, fatalErrorKeyPrefixBuilder(req), (chan<- *model.FatalError)(respCh), (chan<- error)(errsCh)).
				Run(func(args mock.Arguments) {
					rCh := args.Get(2).(chan<- *model.FatalError)
					rCh <- expectedFatalErr
				})

			go func() {
				endpoints.getFatalErrors(ctx, req, respCh, errsCh)
			}()

			actualFatalErrResponse := <-respCh

			mockAuth.AssertExpectations(t)
			mockOperations.AssertExpectations(t)

			assert.Equal(t, expectedFatalErr, actualFatalErrResponse)
		})
	}
}

func TestReturnsAuthErrorWhenAuthFails(t *testing.T) {
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

			endpoints, _ := New(mockOperations, nil, mockAuth, &options2.ServerOptions{})

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

