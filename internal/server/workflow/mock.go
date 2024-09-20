package workflow

import (
	"context"
	errors2 "errors"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/workflow"
	"gitlab.com/shar-workflow/shar/internal/common/client"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"log/slog"
	"sync/atomic"
	"time"
)

type jobClient struct {
	jobID             string
	processInstanceID string
	activityID        string
	executionID       string
	originalInputs    map[string]interface{}
	originalOutputs   map[string]interface{}
}

func (c *jobClient) Headers(ctx context.Context) (map[string]string, error) {
	panic("implement me")
}

// Log logs to the span related to this jobClient instance.
func (c *jobClient) Log(ctx context.Context, level slog.Level, message string, attrs map[string]string) error {
	flat := []any{}
	for k, v := range attrs {
		flat = append(flat, k, v)
	}
	slog.Log(ctx, level, message, flat...)
	return fmt.Errorf("%w", errors2.New(message))
}

func (c *jobClient) Logger() *slog.Logger {
	return c.LoggerWith(slog.Default())
}

func (c *jobClient) LoggerWith(logger *slog.Logger) *slog.Logger {
	return slogWith(logger, c.executionID, c.processInstanceID, c.activityID, c.jobID)
}

func slogWith(logger *slog.Logger, executionID, processInstanceID, activityID, jobID string) *slog.Logger {
	return logger.With(
		slog.String(keys.ExecutionID, executionID),
		slog.String(keys.ProcessInstanceID, processInstanceID),
		slog.String(keys.ActivityID, activityID),
		slog.String(keys.JobID, jobID),
	)
}
func (c *jobClient) OriginalVars() (inputVars model.Vars, outputVars model.Vars) {
	inputVars = c.originalInputs
	outputVars = c.originalOutputs
	return
}

func (s *Engine) processMockServices(ctx context.Context) error {

	ackTimeout := time.Second * 30
	counter := atomic.Int64{}

	svcFnExecutor := func(ctx context.Context, trackingID string, job *model.WorkflowState, svcFn *task.FnDef, inVars model.Vars) (model.Vars, error) {
		id := common.TrackingID(job.Id)
		pidCtx := context.WithValue(ctx, client.InternalProcessInstanceId, job.ProcessInstanceId)
		pidCtx = context.WithValue(pidCtx, client.InternalExecutionId, job.ExecutionId)
		pidCtx = context.WithValue(pidCtx, client.InternalActivityId, id.ParentID())
		pidCtx = context.WithValue(pidCtx, client.InternalTaskId, id.ID())
		pidCtx = client.ReParentSpan(pidCtx, job)
		pidCtx = context.WithValue(pidCtx, keys.ContextKey("taskDef"), job.ExecuteVersion)
		jc := &jobClient{jobID: trackingID, processInstanceID: job.ProcessInstanceId}
		if job.State == model.CancellationState_compensating {
			var err error
			ins, err := s.operations.GetCompensationInputVariables(ctx, job.ProcessInstanceId, trackingID)
			if err != nil {
				return nil, fmt.Errorf("get input variables: %w", err)
			}
			outs, err := s.operations.GetCompensationOutputVariables(ctx, job.ProcessInstanceId, trackingID)
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
		v, err := svcFn.Fn.(task.ServiceFn)(pidCtx, jc, inVars)
		if err != nil {
			return v, fmt.Errorf("execute service task: %w", err)
		}
		return v, nil
	}

	msgFnExecutor := func(ctx context.Context, trackingID string, job *model.WorkflowState, fn *task.FnDef, inVars model.Vars) error {
		// Call a message function
		return nil
	}

	svcFnLocator := func(job *model.WorkflowState) (*task.FnDef, error) {
		var fn task.ServiceFn = s.mockServiceFunction
		return &task.FnDef{Fn: fn}, nil
	}

	msgFnLocator := func(job *model.WorkflowState) (*task.FnDef, error) {
		var fn task.SenderFn = s.mockMessageFunction
		return &task.FnDef{Fn: fn}, nil
	}

	svcTaskCompleter := func(ctx context.Context, trackingID string, newVars model.Vars, compensating bool) error {
		job, err := s.operations.GetJob(ctx, trackingID)
		if err != nil {
			return fmt.Errorf("get job: %w", err)
		}
		b, err := vars.Encode(ctx, newVars)
		if err != nil {
			return fmt.Errorf("encode vars: %w", err)
		}
		err = s.operations.CompleteServiceTask(ctx, job, b)
		if err != nil {
			return fmt.Errorf("complete service task: %w", err)
		}
		return nil
	}

	msgSendCompleter := func(ctx context.Context, trackingID string, newVars model.Vars) error {
		return nil
	}

	wfErrHandler := func(ctx context.Context, ns string, trackingID string, errorCode string, binVars []byte) (*model.HandleWorkflowErrorResponse, error) {
		state, err := s.operations.GetJob(ctx, trackingID)
		if err != nil {
			return nil, fmt.Errorf("get job: %w", err)
		}
		if err := s.operations.HandleWorkflowError(ctx, errorCode, binVars, state); errors2.Is(err, errors.ErrUnhandledWorkflowError) {
			return &model.HandleWorkflowErrorResponse{Handled: false}, nil
		} else if err != nil {
			return nil, fmt.Errorf("handle workflow error: %w", err)
		}
		return &model.HandleWorkflowErrorResponse{Handled: true}, nil
	}

	piErrHandler := func(ctx context.Context, processInstanceID string, wfe *model.Error) error {
		pi, err := s.operations.GetProcessInstance(ctx, processInstanceID)
		if err != nil {
			return fmt.Errorf("get process instance: %w", err)
		}
		state := &model.WorkflowState{
			ExecutionId:       pi.ExecutionId,
			ProcessInstanceId: pi.ProcessInstanceId,
			State:             model.CancellationState_errored,
			Error:             wfe,
		}
		if err := s.operations.CancelProcessInstance(ctx, state); err != nil {
			return fmt.Errorf("cancel process instance: %w", err)
		}
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

	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "mockTask", s.closing, subj.NS(subject, "*"), "MockTaskConsumer", s.concurrency, s.receiveMiddleware, client.ClientProcessFn(ackTimeout, &counter, false, s.operations, params), s.operations.SignalFatalError)
	if err != nil {
		return fmt.Errorf("traversal processor: %w", err)
	}
	return nil
}

func (s *Engine) mockServiceFunction(ctx context.Context, client task.JobClient, vars model.Vars) (model.Vars, error) {
	newVars := model.NewVars()
	ts, err := s.operations.GetTaskSpecByUID(ctx, ctx.Value(keys.ContextKey("taskDef")).(string))
	if err != nil {
		return newVars, fmt.Errorf("get task spec: %w", err)
	}
	fatalError := false
	wfError := ""
	if ts.Behaviour != nil && ts.Behaviour.MockBehaviour != nil {
		var err error
		if b := ts.Behaviour.MockBehaviour.ErrorCodeExpr; b != "" {
			if wfError, err = expression.Eval[string](ctx, s.exprEngine, b, vars); err != nil {
				return newVars, fmt.Errorf("evaluate mock workflow error expression: %w", err)
			}
		}
		if b := ts.Behaviour.MockBehaviour.FatalErrorExpr; b != "" {
			if fatalError, err = expression.Eval[bool](ctx, s.exprEngine, b, vars); err != nil {
				return newVars, fmt.Errorf("evaluate mock fatal error expression: %w", err)
			}
		}

	}
	if fatalError {
		return newVars, &errors.ErrWorkflowFatal{Err: fmt.Errorf("mock fatal error")}
	}
	for _, outParam := range ts.Parameters.Output {
		example := outParam.Example
		if example != "" {
			v, err := expression.EvalAny(ctx, s.exprEngine, example, vars)
			if err != nil {
				return newVars, &errors.ErrWorkflowFatal{Err: fmt.Errorf("eval example expression: %w", err)}
			}
			newVars[outParam.Name] = v
		} else {
			var v any
			switch outParam.Type {
			case "string":
				v = ""
			case "bool":
				v = false
			case "float":
				v = 0.0
			case "int":
				v = 0
			}
			newVars[outParam.Name] = v
		}
	}
	if wfError != "" {
		return newVars, &workflow.Error{Code: wfError, WrappedError: errors2.New("simulated mock error")}
	}
	return newVars, nil
}

func (s *Engine) mockMessageFunction(ctx context.Context, client task.MessageClient, vars model.Vars) error {
	return nil
}
