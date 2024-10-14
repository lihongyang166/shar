package api

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"log/slog"
)

// Auth is a struct with various methods to provide authentication and authorisation capabilities for the api
type Auth interface {
	authFromExecutionID(ctx context.Context, executionID string) (context.Context, *model.Execution, error)
	authFromProcessInstanceID(ctx context.Context, instanceID string) (context.Context, *model.ProcessInstance, error)
	authForNamedWorkflow(ctx context.Context, name string) (context.Context, error)
	authForWorkflowId(ctx context.Context, workflowId string) (context.Context, error)
	authenticate(ctx context.Context) (context.Context, header.Values, error)
	authorize(ctx context.Context, workflowName string) (context.Context, error)
	authFromJobID(ctx context.Context, trackingID string) (context.Context, *model.WorkflowState, error)
	authForNonWorkflow(ctx context.Context) (context.Context, error)
}

// sharAuth is a struct implementing the Auth interface
type sharAuth struct {
	apiAuthZFn authz.APIFunc
	apiAuthNFn authn.Check
	operations workflow.Ops
}

// NewSharAuth constructs a new Shar Auth instance
func NewSharAuth(apiAuthZFn authz.APIFunc, apiAuthNFn authn.Check, operations workflow.Ops) *sharAuth {
	return &sharAuth{
		apiAuthZFn: apiAuthZFn,
		apiAuthNFn: apiAuthNFn,
		operations: operations,
	}
}

func (a *sharAuth) authenticate(ctx context.Context) (context.Context, header.Values, error) {
	val := ctx.Value(header.ContextKey).(header.Values)
	res, authErr := a.apiAuthNFn(ctx, &model.ApiAuthenticationRequest{Headers: val})
	if authErr != nil || !res.Authenticated {
		slog.Debug("failed to authenticate", "values", val, "func", a.apiAuthNFn, "auth-err", authErr)
		return context.Background(), header.Values{}, fmt.Errorf("authenticate: %w", errors2.ErrApiAuthNFail)
	}
	ctx = context.WithValue(ctx, ctxkey.SharUser, res.User)
	return ctx, val, nil
}

func (a *sharAuth) authorize(ctx context.Context, workflowName string) (context.Context, error) {
	ctx, val, err := a.authenticate(ctx)
	if err != nil {
		return ctx, fmt.Errorf("authenticate: %w", errors2.ErrApiAuthNFail)
	}
	if a.apiAuthZFn == nil {
		return ctx, nil
	}
	if authRes, err := a.apiAuthZFn(ctx, &model.ApiAuthorizationRequest{
		Headers:      val,
		Function:     ctx.Value(ctxkey.APIFunc).(string),
		WorkflowName: workflowName,
		User:         ctx.Value(ctxkey.SharUser).(string),
	}); err != nil || !authRes.Authorized {
		return ctx, fmt.Errorf("authorize: %w", errors2.ErrApiAuthZFail)
	}
	return ctx, nil
}

func (a *sharAuth) authFromJobID(ctx context.Context, trackingID string) (context.Context, *model.WorkflowState, error) {
	job, err := a.operations.GetJob(ctx, trackingID)
	if err != nil {
		return ctx, nil, fmt.Errorf("get job for authorization: %w", err)
	}
	w, err := a.operations.GetWorkflow(ctx, job.WorkflowId)
	if err != nil {
		return ctx, nil, fmt.Errorf("get workflow for authorization: %w", err)
	}
	ctx, auth := a.authorize(ctx, w.Name)
	if auth != nil {
		return ctx, nil, fmt.Errorf("authorize: %w", &errors2.ErrWorkflowFatal{Err: auth})
	}
	return ctx, job, nil
}

func (a *sharAuth) authFromExecutionID(ctx context.Context, executionID string) (context.Context, *model.Execution, error) {
	execution, err := a.operations.GetExecution(ctx, executionID)
	if err != nil {
		return ctx, nil, fmt.Errorf("get execution for authorization: %w", err)
	}
	ctx, auth := a.authorize(ctx, execution.WorkflowName)
	if auth != nil {
		return ctx, nil, fmt.Errorf("authorize: %w", &errors2.ErrWorkflowFatal{Err: auth})
	}
	return ctx, execution, nil
}

func (a *sharAuth) authFromProcessInstanceID(ctx context.Context, instanceID string) (context.Context, *model.ProcessInstance, error) {
	pi, err := a.operations.GetProcessInstance(ctx, instanceID)
	if err != nil {
		return ctx, nil, fmt.Errorf("get workflow instance for authorization: %w", err)
	}
	ctx, auth := a.authorize(ctx, pi.WorkflowName)
	if auth != nil {
		return ctx, nil, fmt.Errorf("authorize: %w", &errors2.ErrWorkflowFatal{Err: auth})
	}
	return ctx, pi, nil
}

func (a *sharAuth) authForNonWorkflow(ctx context.Context) (context.Context, error) {
	ctx, auth := a.authorize(ctx, "")
	if auth != nil {
		return ctx, fmt.Errorf("authorize: %w", &errors2.ErrWorkflowFatal{Err: auth})
	}
	return ctx, nil
}

func (a *sharAuth) authForNamedWorkflow(ctx context.Context, name string) (context.Context, error) {
	ctx, auth := a.authorize(ctx, name)
	if auth != nil {
		return ctx, fmt.Errorf("authorize: %w", &errors2.ErrWorkflowFatal{Err: auth})
	}
	return ctx, nil
}

func (a *sharAuth) authForWorkflowId(ctx context.Context, workflowId string) (context.Context, error) {
	workflow, err := a.operations.GetWorkflow(ctx, workflowId)
	if err != nil {
		return ctx, fmt.Errorf("get workflow for authorization: %w", err)
	}
	ctx, auth := a.authorize(ctx, workflow.Name)
	if auth != nil {
		return ctx, fmt.Errorf("authorize: %w", &errors2.ErrWorkflowFatal{Err: auth})
	}
	return ctx, nil
}
