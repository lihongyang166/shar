package workflow

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/internal/common/client"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"log/slog"
	"sync/atomic"
	"time"
)

type jobClient struct {
	trackingID        string
	processInstanceId string
	originalInputs    map[string]interface{}
	originalOutputs   map[string]interface{}
}

// Log logs to the span related to this jobClient instance.
func (c *jobClient) Log(ctx context.Context, level slog.Level, message string, attrs map[string]string) error {
	flat := []any{}
	for k, v := range attrs {
		flat = append(flat, k, v)
	}
	slog.Log(ctx, level, message, flat...)
	return fmt.Errorf(message)
}

func (c *jobClient) OriginalVars() (inputVars map[string]interface{}, outputVars map[string]interface{}) {
	inputVars = c.originalInputs
	outputVars = c.originalOutputs
	return
}

func (s *Engine) processMockServices(ctx context.Context) error {

	ackTimeout := time.Second * 30
	counter := atomic.Int64{}

	svcFnExecutor := func(ctx context.Context, trackingID string, job *model.WorkflowState, svcFn task.ServiceFn, inVars model.Vars) (model.Vars, error) {
		pidCtx := context.WithValue(ctx, client.InternalProcessInstanceId, job.ProcessInstanceId)
		pidCtx = client.ReParentSpan(pidCtx, job)
		jc := &jobClient{trackingID: trackingID, processInstanceId: job.ProcessInstanceId}
		if job.State == model.CancellationState_compensating {
			var err error
			ins, err := s.GetCompensationInputVariables(ctx, job.ProcessInstanceId, trackingID)
			if err != nil {
				return nil, fmt.Errorf("get input variables: %w", err)
			}
			outs, err := s.GetCompensationOutputVariables(ctx, job.ProcessInstanceId, trackingID)
			if err != nil {
				return nil, fmt.Errorf("get output variables: %w", err)
			}
			iVars, err := vars.Decode(ctx, ins)
			if err != nil {
				return nil, fmt.Errorf("decode input variables: %w", err)
			}
			oVars, err := vars.Decode(ctx, outs)
			if err != nil {
				return nil, fmt.Errorf("decode output variables: %w", err)
			}
			jc.originalInputs, jc.originalOutputs = iVars, oVars
			if err != nil {
				return make(model.Vars), fmt.Errorf("get compensation variables: %w", err)
			}
		}
		v, err := svcFn(pidCtx, jc, inVars)
		if err != nil {
			return v, fmt.Errorf("execute service task: %w", err)
		}
		return v, nil
	}

	msgFnExecutor := func(ctx context.Context, trackingID string, job *model.WorkflowState, fn task.SenderFn, inVars model.Vars) error {
		// Call a message function
		return nil
	}

	svcFnLocator := func(job *model.WorkflowState) (task.ServiceFn, error) {
		return mockServiceFunction, nil
	}

	msgFnLocator := func(job *model.WorkflowState) (task.SenderFn, error) {
		return mockMessageFunction, nil
	}

	svcTaskCompleter := func(ctx context.Context, trackingID string, newVars model.Vars, compensating bool) error {
		return nil
	}

	msgSendCompleter := func(ctx context.Context, trackingID string, newVars model.Vars) error {
		return nil
	}

	wfErrHandler := func(ctx context.Context, ns string, trackingID string, errorCode string, binVars []byte) (*model.HandleWorkflowErrorResponse, error) {
		return nil, nil
	}

	piErrHandler := func(ctx context.Context, processInstanceID string, wfe *model.Error) error {
		return nil
	}

	params := client.ServiceTaskProcessParams{
		SvcFnExecutor:    svcFnExecutor,
		MsgFnExecutor:    msgFnExecutor,
		SvcFnLocator:     svcFnLocator,
		MsgFnLocator:     msgFnLocator,
		SvcTaskCompleter: svcTaskCompleter,
		MsgSendCompleter: msgSendCompleter,
		WfErrorHandler:   wfErrHandler,
		PiErrorCanceller: piErrHandler,
	}

	subject := messages.WorkflowJobServiceTaskExecute + ".*.Mock"

	err := common.Process(ctx, s.js, "WORKFLOW", "traversal", s.closing, subj.NS(subject, "*"), "Traversal", s.concurrency, s.receiveMiddleware, client.ClientProcessFn(ackTimeout, &counter, s, params))
	if err != nil {
		return fmt.Errorf("traversal processor: %w", err)
	}
	return nil
}

func mockServiceFunction(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	return model.Vars{}, nil
}

func mockMessageFunction(ctx context.Context, client task.MessageClient, vars model.Vars) error {
	return nil
}
