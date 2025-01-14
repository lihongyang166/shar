package workflow

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/client/parser"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	model2 "gitlab.com/shar-workflow/shar/internal/model"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/server/option"
	"gitlab.com/shar-workflow/shar/server/services/natz"
	"gitlab.com/shar-workflow/shar/server/vars"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Engine contains the workflow processing functions
type Engine struct {
	closing                 chan struct{}
	natsService             *natz.NatsService
	operations              *Operations
	concurrency             int
	allowOrphanServiceTasks bool
	telCfg                  telemetry.Config
	receiveMiddleware       []middleware.Receive
	exprEngine              expression.Engine
}

// New returns an instance of the core workflow engine.
func New(natsService *natz.NatsService, operations *Operations, options *option.ServerOptions) (*Engine, error) {
	if options.Concurrency < 1 || options.Concurrency > 200 {
		return nil, fmt.Errorf("invalid concurrency: %w", errors2.New("invalid concurrency set"))
	}

	ctx := context.Background()

	e := &Engine{
		natsService:             natsService,
		operations:              operations,
		concurrency:             options.Concurrency,
		allowOrphanServiceTasks: options.AllowOrphanServiceTasks,
		telCfg:                  options.TelemetryConfig,
		closing:                 make(chan struct{}),
		exprEngine:              &expression.ExprEngine{},
	}

	e.receiveMiddleware = append(e.receiveMiddleware, telemetry.NatsMsgToCtxWithSpanMiddleware())

	if err := e.startTelemetry(ctx, namespace.Default); err != nil {
		return nil, fmt.Errorf("start telemetry: %w", err)
	}

	return e, nil
}

// Start sets up the activity and job processors and starts the engine processing workflows.
func (c *Engine) Start(ctx context.Context) error {
	if err := c.StartProcessing(ctx); err != nil {
		return fmt.Errorf("start processing: %w", err)
	}
	return nil
}

// StartProcessing begins listening to all the message processing queues.
func (s *Engine) StartProcessing(ctx context.Context) error {

	if err := s.processTraversals(ctx); err != nil {
		return fmt.Errorf("start traversals handler: %w", err)
	}
	if err := s.processJobAbort(ctx); err != nil {
		return fmt.Errorf("start job abort handler: %w", err)
	}
	if err := s.processGeneralAbort(ctx); err != nil {
		return fmt.Errorf("general abort handler: %w", err)
	}
	if err := s.processTracking(ctx); err != nil {
		return fmt.Errorf("start tracking handler: %w", err)
	}
	if err := s.processWorkflowEvents(ctx); err != nil {
		return fmt.Errorf("start workflow events handler: %w", err)
	}
	if err := s.processMessages(ctx); err != nil {
		return fmt.Errorf("start process messages handler: %w", err)
	}
	if err := s.listenForTimer(ctx, s.natsService.Js, s.closing, 4); err != nil {
		return fmt.Errorf("start timer handler: %w", err)
	}
	if err := s.processCompletedJobs(ctx); err != nil {
		return fmt.Errorf("start completed jobs handler: %w", err)
	}
	if err := s.processActivities(ctx); err != nil {
		return fmt.Errorf("start activities handler: %w", err)
	}
	if err := s.processLaunch(ctx); err != nil {
		return fmt.Errorf("start launch handler: %w", err)
	}
	if err := s.processProcessComplete(ctx); err != nil {
		return fmt.Errorf("start process complete handler: %w", err)
	}
	if err := s.processProcessCompensate(ctx); err != nil {
		return fmt.Errorf("start process compensate handler: %w", err)
	}
	if err := s.processProcessTerminate(ctx); err != nil {
		return fmt.Errorf("start process terminate handler: %w", err)
	}
	if err := s.processGatewayActivation(ctx); err != nil {
		return fmt.Errorf("start gateway execute handler: %w", err)
	}
	if err := s.processAwaitMessageExecute(ctx); err != nil {
		return fmt.Errorf("start await message handler: %w", err)
	}
	if err := s.processGatewayExecute(ctx); err != nil {
		return fmt.Errorf("start gateway execute handler: %w", err)
	}
	if err := s.processFatalError(ctx); err != nil {
		return fmt.Errorf("start fatal error handler: %w", err)
	}

	if err := s.processJobRetry(ctx); err != nil {
		return fmt.Errorf("start job retry handler: %w", err)
	}

	if err := s.processMockServices(ctx); err != nil {
		return fmt.Errorf("start mock services handler: %w", err)
	}
	return nil
}

// traverse traverses all outbound connections provided the conditions passed if available.
func (c *Engine) traverse(ctx context.Context, _ *model.ProcessInstance, trackingID common.TrackingID, outbound *model.Targets, el map[string]*model.Element, state *model.WorkflowState) error {
	state.PreviousActivity = common.TrackingID(state.Id).ID()
	state.PreviousElement = state.ElementId

	ctx, log := logx.ContextWith(ctx, "engine.traverse")
	if outbound == nil {
		return nil
	}
	targets := make(map[string]string, len(outbound.Target))
	// Traverse along all outbound edges
	for ord, t := range outbound.Target {
		if ord == int(outbound.DefaultTarget) {
			continue
		}
		ws := proto.Clone(state).(*model.WorkflowState)
		// Evaluate conditions
		ok := true
		for _, ex := range t.Conditions {
			// TODO: Cache compilation.
			exVars := model2.NewServerVars()
			if err := exVars.Decode(ctx, ws.Vars); err != nil {
				return fmt.Errorf("decode variables for condition evaluation: %w", err)
			}

			// evaluate the condition
			res, err := expression.Eval[bool](ctx, c.exprEngine, ex, exVars.Vals)
			if err != nil {
				return &errors.ErrWorkflowFatal{Err: err}
			}
			if !res {
				ok = false
				break
			}
		}
		if ok {
			targets[t.Id] = t.Target
		}
	}

	if len(targets) == 0 && outbound.DefaultTarget != -1 {
		def := outbound.Target[outbound.DefaultTarget]
		targets[def.Id] = def.Target
	}

	elem := el[state.ElementId]

	reciprocatedDivergentGateway := elem.Type == element.Gateway && elem.Gateway.Direction == model.GatewayDirection_divergent && elem.Gateway.ReciprocalId != ""

	divergentGatewayReciprocalInstanceId := ksuid.New().String()

	// Check traversals from a reciprocated divergent gateway
	if reciprocatedDivergentGateway {
		expectedPaths := make([]string, 0, len(targets))
		for k := range targets {
			expectedPaths = append(expectedPaths, k)
		}
		if state.GatewayExpectations == nil {
			state.GatewayExpectations = make(map[string]*model.GatewayExpectations)
		}
		state.GatewayExpectations[divergentGatewayReciprocalInstanceId] = &model.GatewayExpectations{
			ExpectedPaths: expectedPaths,
		}

		gw := &model.Gateway{
			MetExpectations: make(map[string]string),
			Vars:            [][]byte{state.Vars},
			Visits:          0,
		}
		gwIID := divergentGatewayReciprocalInstanceId

		ns := subj.GetNS(ctx)
		nsKVs, err := c.natsService.KvsFor(ctx, ns)
		if err != nil {
			return fmt.Errorf("traverse get kvs for ns: %w", err)
		}
		if err := common.SaveObj(ctx, nsKVs.WfGateway, gwIID, gw); err != nil {
			return fmt.Errorf("%s failed to save gateway to KV: %w", errors.Fn(), err)
		}

	}

	for branchID, elID := range targets {
		ws := proto.Clone(state).(*model.WorkflowState)
		newID := ksuid.New().String()
		tID := trackingID.Push(newID)
		target := el[elID]
		ws.Id = tID
		ws.ElementType = target.Type
		ws.ElementId = target.Id

		if wf, err := c.operations.GetWorkflow(ctx, ws.WorkflowId); err != nil {
			return &errors.ErrWorkflowFatal{Err: fmt.Errorf("get workflow: %w", err)}
		} else {
			els := map[string]*model.Element{}
			common.IndexProcessElements(wf.Process[ws.ProcessId].Elements, els)
			ws.ElementName = els[ws.ElementId].Name
		}

		// Check traversals that lead to solitary convergent gateways
		if target.Type == element.Gateway && target.Gateway.Direction == model.GatewayDirection_convergent && target.Gateway.ReciprocalId == "" {
			if ws.SatisfiesGatewayExpectation == nil {
				ws.SatisfiesGatewayExpectation = make(map[string]*model.SatisfiesGateway)
			}
			if _, ok := ws.SatisfiesGatewayExpectation[target.Id]; !ok {
				ws.SatisfiesGatewayExpectation[target.Id] = &model.SatisfiesGateway{InstanceTracking: make([]string, 0)}
			}
			ws.SatisfiesGatewayExpectation[target.Id].InstanceTracking = append(ws.SatisfiesGatewayExpectation[target.Id].InstanceTracking, "-,"+branchID)
		}

		if reciprocatedDivergentGateway {
			if ws.SatisfiesGatewayExpectation == nil {
				ws.SatisfiesGatewayExpectation = make(map[string]*model.SatisfiesGateway)
			}
			if _, ok := ws.SatisfiesGatewayExpectation[elem.Gateway.ReciprocalId]; !ok {
				ws.SatisfiesGatewayExpectation[elem.Gateway.ReciprocalId] = &model.SatisfiesGateway{InstanceTracking: make([]string, 0)}
			}
			ws.SatisfiesGatewayExpectation[elem.Gateway.ReciprocalId].InstanceTracking = append(ws.SatisfiesGatewayExpectation[elem.Gateway.ReciprocalId].InstanceTracking, divergentGatewayReciprocalInstanceId+","+branchID)
		}

		if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowTraversalExecute, ws); err != nil {
			log.Error("publish workflow state", "error", err)
			return fmt.Errorf("publish workflow state: %w", err)
		}
	}
	return nil
}

// activityStartProcessor handles the behaviour of each BPMN element
func (c *Engine) activityStartProcessor(ctx context.Context, newActivityID string, traversal *model.WorkflowState, traverseOnly bool) error {
	ctx, log := logx.ContextWith(ctx, "engine.activityStartProcessor")
	// set the default status to be 'executing'
	status := model.CancellationState_executing

	// get the corresponding process instance
	pi, err := c.operations.GetProcessInstance(ctx, traversal.ProcessInstanceId)
	if errors2.Is(err, errors.ErrProcessInstanceNotFound) || errors2.Is(err, jetstream.ErrKeyNotFound) {
		// if the workflow instance has been removed kill any activity and exit
		log.Warn("process instance not found, cancelling activity", "error", err, slog.String(keys.ProcessInstanceID, traversal.ProcessInstanceId))
		return nil
	} else if err != nil {
		return engineErr(ctx, "get process instance", err,
			slog.String(keys.ProcessInstanceID, traversal.ProcessInstanceId),
		)
	}

	// get the corresponding workflow definition
	workflow, err := c.operations.GetWorkflow(ctx, pi.WorkflowId)
	if err != nil {
		return engineErr(ctx, "get workflow", err,
			slog.String(keys.ExecutionID, pi.ExecutionId),
			slog.String(keys.WorkflowID, pi.WorkflowId),
		)
	}

	// create an indexed map of elements
	els := common.ElementTable(workflow)
	el := els[traversal.ElementId]

	// force traversal will not process the event, and will just traverse instead.
	if traverseOnly {
		el.Type = "forceTraversal"
	}

	activityID := common.TrackingID(traversal.Id).Pop().Push(newActivityID)

	newState := common.CopyWorkflowState(traversal)
	newState.Id = activityID
	// tell the world we are going to execute an activity
	if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowActivityExecute, newState); err != nil {
		return engineErr(ctx, "publish workflow status", err, apErrFields(pi.ExecutionId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
	}
	// log this with history
	if err := c.operations.RecordHistoryActivityExecute(ctx, newState); err != nil {
		return engineErr(ctx, "publish process history", err, apErrFields(pi.ExecutionId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
	}

	// tell the world we have safely completed the traversal
	if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowTraversalComplete, traversal); err != nil {
		return engineErr(ctx, "publish traversal status", err, apErrFields(pi.ExecutionId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
	}

	//Start any timers
	if el.BoundaryTimer != nil || len(el.BoundaryTimer) > 0 {
		for _, i := range el.BoundaryTimer {
			ai := i
			timerState := common.CopyWorkflowState(newState)
			timerState.Execute = &ai.Target
			timerState.UnixTimeNano = time.Now().UnixNano()
			timerState.Timer = &model.WorkflowTimer{LastFired: 0, Count: 0}
			v := model2.NewServerVars()
			err := v.Decode(ctx, traversal.Vars)
			if err != nil {
				return fmt.Errorf("decode boundary timer variable: %w", err)
			}
			res, err := expression.EvalAny(ctx, c.exprEngine, i.Duration, v.Vals)
			if err != nil {
				return fmt.Errorf("evaluate boundary timer expression: %w", err)
			}
			ut := time.Now()
			switch x := res.(type) {
			case int:
				ut = ut.Add(time.Duration(x))
			case string:
				dur, err := parser.ParseISO8601(x)
				if err != nil {
					return fmt.Errorf("parse ISO8601 boundary timer value: %w", err)
				}
				ut = dur.Shift(ut)
			}
			err = c.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowElementTimedExecute, subj.GetNS(ctx)), timerState, WithEmbargo(int(ut.UnixNano())))
			if err != nil {
				return fmt.Errorf("publish timed execute during activity start: %w", err)
			}
		}
	}

	// process any supported events
	switch el.Type {
	case element.StartEvent:
		initVars := make([]byte, 0)
		err := vars.OutputVars(ctx, c.exprEngine, traversal.Vars, &initVars, el.OutputTransform)
		if err != nil {
			return fmt.Errorf("get output vars for start event: %w", err)
		}
		traversal.Vars = initVars
		newState.State = status
		newState.Vars = traversal.Vars
		if err := c.completeActivity(ctx, newState); err != nil {
			return fmt.Errorf("start event complete activity: %w", err)
		}
	case element.Gateway:
		completeActivityState := common.CopyWorkflowState(newState)
		completeActivityState.State = status
		//
		if el.Gateway.Direction == model.GatewayDirection_convergent {
			if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowJobGatewayTaskActivate, completeActivityState); err != nil {
				return fmt.Errorf("%s failed to activate gateway: %w", errors.Fn(), err)
			}
		} else {
			if err := c.completeActivity(ctx, completeActivityState); err != nil {
				return fmt.Errorf("complete activity for exclusive gateway: %w", err)
			}
		}
	case element.ServiceTask:
		if el.Version == nil {
			v, err := c.operations.GetTaskSpecUID(ctx, el.Execute)
			if errors2.Is(err, jetstream.ErrKeyNotFound) {
				return fmt.Errorf("engine failed to get task spec id: %w", &errors.ErrWorkflowFatal{Err: err})
			}
			el.Version = &v
		}
		if err != nil {
			return fmt.Errorf("get service task routing key during activity start processor: %w", err)
		}
		if err := c.operations.startJob(ctx, messages.WorkflowJobServiceTaskExecute+"."+*el.Version, newState, el, traversal.Vars); err != nil {
			return engineErr(ctx, "start service task job", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	case element.UserTask:
		if err := c.operations.startJob(ctx, messages.WorkflowJobUserTaskExecute, newState, el, traversal.Vars); err != nil {
			return engineErr(ctx, "start user task job", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	case element.ManualTask:
		if err := c.operations.startJob(ctx, messages.WorkflowJobManualTaskExecute, newState, el, traversal.Vars); err != nil {
			return engineErr(ctx, "start manual task job", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	case element.MessageIntermediateThrowEvent:
		wf, err := c.operations.GetWorkflow(ctx, pi.WorkflowId)
		if err != nil {
			return fmt.Errorf("get workflow for intermediate throw event: %w", err)
		}
		ix := -1
		for i, v := range wf.Messages {
			if v.Name == el.Execute {
				ix = i
				break
			}
		}
		if ix == -1 {
			// TODO: Fatal workflow error - we shouldn't allow to send unknown messages in parser
			return &errors.ErrWorkflowFatal{Err: fmt.Errorf("unknown workflow message name: %s", el.Execute)}
		}
		msgState := common.CopyWorkflowState(newState)
		msgState.Condition = &wf.Messages[ix].Execute //this is the correlation key

		if err := c.operations.startJob(ctx, messages.WorkflowJobSendMessageExecute+"."+pi.WorkflowName+"_"+el.Execute, newState, el, traversal.Vars); err != nil {
			return engineErr(ctx, "start message job", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	case element.CallActivity:
		if err := c.operations.startJob(ctx, subj.NS(messages.WorkflowJobLaunchExecute, subj.GetNS(ctx)), newState, el, traversal.Vars); err != nil {
			return engineErr(ctx, "start message launch", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	case element.MessageIntermediateCatchEvent:
		awaitMsg := common.CopyWorkflowState(newState)
		awaitMsg.Execute = &el.Execute
		awaitMsg.Condition = &el.Msg
		if err := c.operations.startJob(ctx, messages.WorkflowJobAwaitMessageExecute, awaitMsg, el, awaitMsg.Vars); err != nil {
			return engineErr(ctx, "start await message task job", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	case element.TimerIntermediateCatchEvent:
		varmap := model2.NewServerVars()
		err := varmap.Decode(ctx, traversal.Vars)
		if err != nil {
			return &errors.ErrWorkflowFatal{Err: err}
		}
		ret, err := expression.EvalAny(ctx, c.exprEngine, el.Execute, varmap.Vals)
		if err != nil {
			return &errors.ErrWorkflowFatal{Err: err}
		}
		var embargo int
		switch em := ret.(type) {
		case string:
			if v, err := strconv.Atoi(em); err == nil {
				embargo = v + int(time.Now().UnixNano())
				break
			}
			pem, err := parser.ParseISO8601(em)
			if err != nil {
				return &errors.ErrWorkflowFatal{Err: err}
			}
			embargo = int(pem.Shift(time.Now()).UnixNano())
		case int:
			embargo = em + int(time.Now().UnixNano())
		default:
			return errors.ErrFatalBadDuration
		}
		newState.Id = common.TrackingID(newState.Id).Push(ksuid.New().String())
		if err := c.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowJobTimerTaskExecute, subj.GetNS(ctx)), newState); err != nil {
			return fmt.Errorf("publish timer task execute job: %w", err)
		}
		if err := c.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowJobTimerTaskComplete, subj.GetNS(ctx)), newState, WithEmbargo(embargo)); err != nil {
			return fmt.Errorf("publish timer task execute complete: %w", err)
		}
	case element.EndEvent:
		if pi.ParentProcessId == nil || *pi.ParentProcessId == "" {
			if len(el.Errors) == 0 {
				status = model.CancellationState_completed
			} else {
				status = model.CancellationState_errored
			}
		}
		newState.State = status
		if err := c.completeActivity(ctx, newState); err != nil {
			return fmt.Errorf("complete activity for end event: %w", err)
		}
	case element.LinkIntermediateThrowEvent:
		newState.State = status
		if err := c.completeActivity(ctx, newState); err != nil {
			return fmt.Errorf("default complete activity: %w", err)
		}
	case element.CompensateEndEvent:
		if err := c.Compensate(ctx, newState); err != nil {
			return fmt.Errorf("initializing compensation: %w", err)
		}
	default:
		// if we don't support the event, just traverse to the next element
		newState.State = status
		if err := c.completeActivity(ctx, newState); err != nil {
			return fmt.Errorf("default complete activity: %w", err)
		}
	}

	// if the workflow is complete, send an instance complete message to trigger tidy up
	if status == model.CancellationState_completed || status == model.CancellationState_errored || status == model.CancellationState_terminated {
		newState.Id = []string{pi.ProcessInstanceId}
		newState.State = status
		newState.Error = el.Error
		// If the workflow completed successfully, transform result if necessary.
		if newState.State == model.CancellationState_completed {
			outputTransform := els[newState.ElementId].OutputTransform
			if len(outputTransform) > 0 {
				// Transform if requested
				finalVars := make([]byte, 0)
				if err := vars.OutputVars(ctx, c.exprEngine, newState.Vars, &finalVars, els[newState.ElementId].OutputTransform); err != nil {
					return fmt.Errorf("transform output vars: %w", err)
				}
				newState.Vars = finalVars
			}
		}
		if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowProcessComplete, newState); err != nil {
			return engineErr(ctx, "publish workflow status", err, apErrFields(pi.ProcessInstanceId, pi.WorkflowId, el.Id, el.Name, el.Type, workflow.Name)...)
		}
	}
	return nil
}

func (c *Engine) completeActivity(ctx context.Context, state *model.WorkflowState) error {
	// tell the world that we processed the activity
	common.DropStateParams(state)
	if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowActivityComplete, state); err != nil {
		return engineErr(ctx, "publish workflow cancellationState", err)
		//TODO: report this without process: apErrFields(wfi.WorkflowInstanceId, wfi.WorkflowId, el.Id, el.Name, el.Type, process.Name)
	}
	if err := c.operations.RecordHistoryActivityComplete(ctx, state); err != nil {
		return engineErr(ctx, "record history activity complete", &errors.ErrWorkflowFatal{Err: err})
	}
	return nil
}

// apErrFields writes out the common error fields for an application error
func apErrFields(executionID, workflowID, elementID, elementName, elementType, workflowName string, extraFields ...any) []any {
	fields := []any{
		slog.String(keys.ExecutionID, executionID),
		slog.String(keys.WorkflowID, workflowID),
		slog.String(keys.ElementID, elementID),
		slog.String(keys.ElementName, elementName),
		slog.String(keys.ElementType, elementType),
		slog.String(keys.WorkflowName, workflowName),
	}
	if len(extraFields) > 0 {
		fields = append(fields, extraFields...)
	}
	return fields
}

// completeJobProcessor processes completed jobs
func (c *Engine) completeJobProcessor(ctx context.Context, job *model.WorkflowState) error {
	ctx, log := logx.ContextWith(ctx, "engine.completeJobProcessor")
	// Validate if it safe to end this job
	// get the saved job state
	if _, err := c.operations.GetProcessHistoryItem(ctx, job.ProcessInstanceId, common.TrackingID(job.Id).ParentID(), model.ProcessHistoryType_activityExecute); errors2.Is(err, jetstream.ErrKeyNotFound) {
		// We can't find the job's old state
		return nil
	} else if err != nil {
		return fmt.Errorf("get old state for complete job processor: %w", err)
	}

	// TODO: CHeck Workflow instance exists
	// get the relevant workflow instance
	pi, err := c.operations.GetProcessInstance(ctx, job.ProcessInstanceId)
	if errors2.Is(err, errors.ErrProcessInstanceNotFound) {
		// if the instance has been deleted quash this activity
		log.Warn("process instance not found, cancelling job processing", "error", err, slog.String(keys.ExecutionID, job.ExecutionId))
		return nil
	} else if err != nil {
		log.Warn("get process instance for job", "error", err,
			slog.String(keys.JobType, job.ElementType),
			slog.String(keys.JobID, common.TrackingID(job.Id).ID()),
		)
		return fmt.Errorf("get workflow instance for job: %w", err)
	}

	// get the relevant workflow
	wf, err := c.operations.GetWorkflow(ctx, pi.WorkflowId)
	if err != nil {
		return engineErr(ctx, "fetch job workflow", err,
			slog.String(keys.JobType, job.ElementType),
			slog.String(keys.JobID, common.TrackingID(job.Id).ID()),
			slog.String(keys.ExecutionID, pi.ExecutionId),
			slog.String(keys.WorkflowID, pi.WorkflowId),
		)
	}
	// build element table
	els := common.ElementTable(wf)
	el := els[job.ElementId]
	newID := common.TrackingID(job.Id).Pop()
	activityStart, err := c.operations.GetProcessHistoryItem(ctx, job.ProcessInstanceId, newID.ID(), model.ProcessHistoryType_activityExecute)
	if errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("complete job processor failed to get old state: %w", err)
	}
	oldState := workfFlowStateFrom(activityStart)

	if err := vars.OutputVars(ctx, c.exprEngine, job.Vars, &oldState.Vars, el.OutputTransform); err != nil {
		return fmt.Errorf("complete job processor failed to transform variables: %w", err)
	}
	completeActivityState := common.CopyWorkflowState(oldState)
	completeActivityState.Id = newID
	completeActivityState.State = job.State
	completeActivityState.ElementId = el.Id
	completeActivityState.ElementType = el.Type
	if err := c.completeActivity(ctx, completeActivityState); err != nil {
		return fmt.Errorf("complete job processor failed to complete activity: %w", err)
	}

	if err := c.operations.RecordHistoryJobComplete(ctx, job); err != nil {
		return fmt.Errorf("complete job processor failed to record history job complete: %w", err)
	}

	if err := c.operations.DeleteJob(ctx, common.TrackingID(job.Id).ID()); err != nil {
		return fmt.Errorf("complete job processor failed to delete job: %w", err)
	}
	return nil
}

func workfFlowStateFrom(activityStart *model.ProcessHistoryEntry) *model.WorkflowState {
	return &model.WorkflowState{
		Id:                          activityStart.Id,
		WorkflowId:                  *activityStart.WorkflowId,
		ExecutionId:                 *activityStart.ExecutionId,
		ElementId:                   *activityStart.ElementId,
		ProcessInstanceId:           *activityStart.ProcessInstanceId,
		State:                       *activityStart.CancellationState,
		Vars:                        activityStart.Vars,
		Timer:                       activityStart.Timer,
		Error:                       activityStart.Error,
		UnixTimeNano:                activityStart.UnixTimeNano,
		Execute:                     activityStart.Execute,
		ProcessId:                   activityStart.ProcessId,
		SatisfiesGatewayExpectation: activityStart.SatisfiesGatewayExpectation,
		GatewayExpectations:         activityStart.GatewayExpectations,
		WorkflowName:                activityStart.WorkflowName,
	}
}

func engineErr(ctx context.Context, msg string, err error, z ...any) error {
	log := logx.FromContext(ctx)
	z = append(z, "error", err.Error())
	log.Error(msg, z...)

	return fmt.Errorf("engine-error: %w", err)
}

func (c *Engine) activityCompleteProcessor(ctx context.Context, state *model.WorkflowState) error {
	ctx, log := logx.ContextWith(ctx, "engine.activityCompleteProcessor")
	var old *model.ProcessHistoryEntry
	var err error
	if old, err = c.operations.GetProcessHistoryItem(ctx, state.ProcessInstanceId, common.TrackingID(state.Id).ID(), model.ProcessHistoryType_activityExecute); errors2.Is(err, jetstream.ErrKeyNotFound) {
		log.Warn("old var state not found", slog.Any("error", err))
	} else if err != nil {
		return fmt.Errorf("activity complete processor failed to get old state: %w", err)
	} else if *old.CancellationState == model.CancellationState_obsolete && state.State == model.CancellationState_obsolete {
		return nil
	}

	pi, pierr := c.operations.GetProcessInstance(ctx, state.ProcessInstanceId)
	if errors2.Is(pierr, errors.ErrProcessInstanceNotFound) {
		errTxt := "process instance not found"
		log.Warn(errTxt, slog.String(keys.ProcessInstanceID, state.ProcessInstanceId))
	} else if pierr != nil {
		return fmt.Errorf("activity complete processor failed to get process instance: %w", pierr)
	}

	wf, err := c.operations.GetWorkflow(ctx, state.WorkflowId)
	if err != nil {
		return fmt.Errorf("activity complete processor failed to get workflow: %w", err)
	}

	els := common.ElementTable(wf)
	newID := common.TrackingID(state.Id).Pop()
	el := els[state.ElementId]
	// intermediateLinkCatchEvent - manually attach outbound connections.
	if el.Type == element.LinkIntermediateThrowEvent {
		var target *model.Element
		for _, elTest := range els {
			if elTest.Type == element.LinkIntermediateCatchEvent && elTest.Execute == el.Execute {
				target = elTest
				break
			}
		}
		if target == nil {
			return &errors.ErrWorkflowFatal{Err: errors2.New("corresponding catch not found")}
		}
		el.Outbound = &model.Targets{Target: []*model.Target{{Target: target.Id}}}
	}
	if pierr == nil {
		if err = c.traverse(ctx, pi, newID, el.Outbound, els, state); errors.IsWorkflowFatal(err) {
			log.Error("workflow fatally terminated whilst traversing", "error", err, slog.String(keys.ProcessInstanceID, pi.ProcessInstanceId), slog.String(keys.WorkflowID, pi.WorkflowId), slog.String(keys.ElementID, state.ElementId))
			return nil
		} else if err != nil {
			return fmt.Errorf("activity complete processor traversal attempt: %w", err)
		}
	}
	switch state.ElementType {
	case element.EndEvent, element.CompensateEndEvent:
		if len(state.Id) > 2 {
			jobID := common.TrackingID(state.Id).Ancestor(2)
			// If we are a sub workflow then complete the parent job
			if jobID != state.ExecutionId {
				j, joberr := c.operations.GetJob(ctx, jobID)
				if errors2.Is(joberr, errors.ErrJobNotFound) {
					log.Warn("job not found " + jobID + " : " + joberr.Error())
				} else if joberr != nil {
					return fmt.Errorf("activity complete processor failed to get job: %w", joberr)
				}
				if joberr == nil {
					j.Vars = state.Vars
					j.Error = state.Error
					if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowJobLaunchComplete, j); err != nil {
						return fmt.Errorf("activity complete processor failed to publish job launch complete: %w", err)
					}
				}
				if err := c.operations.DeleteJob(ctx, jobID); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
					return fmt.Errorf("activity complete processor failed to delete job %s: %w", jobID, err)
				}
				execution, eerr := c.operations.GetExecution(ctx, state.ExecutionId)
				if eerr != nil && !errors2.Is(eerr, jetstream.ErrKeyNotFound) {
					return fmt.Errorf("activity complete processor failed to get execution: %w", err)
				}
				if pierr == nil {
					if err := c.operations.destroyProcessInstance(ctx, state, pi.ProcessInstanceId, execution.ExecutionId); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
						return fmt.Errorf("activity complete processor failed to destroy execution: %w", err)
					}
				}
				if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowJobLaunchComplete, j); err != nil {
					return fmt.Errorf("activity complete processor failed to publish job launch complete: %w", err)
				}
			}
		}
	}

	return nil
}

func (c *Engine) launchProcessor(ctx context.Context, state *model.WorkflowState) error {
	wf, err := c.operations.GetWorkflow(ctx, state.WorkflowId)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: errors.ErrWorkflowNotFound}
	}
	els := common.ElementTable(wf)
	ctx = context.WithValue(ctx, ctxkey.Traceparent, state.TraceParent)
	pi, err := c.operations.GetProcessInstance(ctx, state.ProcessInstanceId)
	if err != nil {
		return fmt.Errorf("get process instance: %w", err)
	}
	if _, _, err := c.operations.LaunchWithParent(ctx, els[state.ElementId].Execute, state.Id, state.Vars, pi.Headers, state.ProcessInstanceId, state.ElementId); err != nil {
		return engineErr(ctx, "launch child workflow", &errors.ErrWorkflowFatal{Err: err})
	}
	return nil
}

func (c *Engine) timedExecuteProcessor(ctx context.Context, state *model.WorkflowState, execution *model.Execution, due int64) (bool, int, error) {
	ctx, log := logx.ContextWith(ctx, "engine.timedExecuteProcessor")
	slog.Info("timedExecuteProcessor")
	wf, err := c.operations.GetWorkflow(ctx, state.WorkflowId)
	if err != nil {
		log.Error("get timer proto workflow", "error", err)
		return true, 0, fmt.Errorf("get timer proto workflow: %w", err)
	}

	els := common.ElementTable(wf)
	el := els[state.ElementId]

	now := time.Now().UnixNano()
	lastFired := state.Timer.LastFired
	elapsed := now - lastFired
	count := state.Timer.Count
	repeat := el.Timer.Repeat
	value := el.Timer.Value
	fireNext := lastFired + (value * 2)

	newTimer := proto.Clone(state).(*model.WorkflowState)

	newTimer.Timer = &model.WorkflowTimer{
		LastFired: now,
		Count:     count + 1,
	}

	var (
		isTimer    bool
		shouldFire bool
	)

	switch el.Timer.Type {
	case model.WorkflowTimerType_fixed:
		isTimer = true
		shouldFire = value <= now
	case model.WorkflowTimerType_duration:
		if repeat != 0 && count >= repeat {
			return true, 0, nil
		}
		isTimer = true
		shouldFire = elapsed >= value
	}

	if isTimer {
		if shouldFire {
			exec, err := c.operations.createExecution(ctx, &model.Execution{
				WorkflowId:   state.WorkflowId,
				WorkflowName: state.WorkflowName,
			})
			if err != nil {
				log.Error("creating execution instance", "error", err)
				return false, 0, fmt.Errorf("creating timed workflow instance: %w", err)
			}

			pi, err := c.operations.createProcessInstance(ctx, exec.ExecutionId, "", "", state.ProcessId, wf.Name, state.WorkflowId, wf.AutoLaunchHeaders)
			if err != nil {
				log.Error("creating timed process instance", "error", err)
				return false, 0, fmt.Errorf("creating timed workflow instance: %w", err)
			}
			state.ExecutionId = pi.ExecutionId
			state.ProcessInstanceId = pi.ProcessInstanceId

			processWfState := proto.Clone(state).(*model.WorkflowState)
			processWfState.TraceParent = telemetry.NewTraceParentWithEmptySpan(telemetry.NewTraceID())
			processTrackingId := common.TrackingID([]string{}).Push(state.ExecutionId).Push(state.ProcessInstanceId)
			processWfState.Id = processTrackingId

			if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowProcessExecute, processWfState); err != nil {
				log.Error("spawning process", "error", err)
				return false, 0, nil
			}
			if err := c.operations.RecordHistoryProcessStart(ctx, processWfState); err != nil {
				log.Error("start events record process start", "error", err)
				return false, 0, fmt.Errorf("publish initial traversal: %w", err)
			}
			if err := vars.OutputVars(ctx, c.exprEngine, newTimer.Vars, &newTimer.Vars, el.OutputTransform); err != nil {
				log.Error("merging variables", "error", err)
				return false, 0, nil
			}
			if err := c.traverse(ctx, pi, []string{ksuid.New().String()}, el.Outbound, els, state); err != nil {
				log.Error("traversing for timed workflow instance", "error", err)
				return false, 0, nil
			}
			if err := c.operations.PublishWorkflowState(ctx, messages.WorkflowTimedExecute, newTimer); err != nil {
				log.Error("publishing timer", "error", err)
				return false, int(fireNext), nil
			}
		} else if el.Timer.Type == model.WorkflowTimerType_duration {
			return false, int(fireNext - now), nil
		}
	}
	return true, 0, nil
}

// Shutdown signals the engine to stop processing.
func (s *Engine) Shutdown() {
	close(s.closing)
}

func (s *Engine) processTraversals(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "traversal", s.closing, subj.NS(messages.WorkflowTraversalExecute, "*"), "Traversal", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var traversal model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &traversal); err != nil {
			return false, fmt.Errorf("unmarshal traversal proto: %w", err)
		}

		if _, _, err := s.operations.hasValidProcess(ctx, traversal.ProcessInstanceId, traversal.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelDebug, "processTraversals aborted due to a missing process")
			return true, nil
		} else if err != nil {
			return false, err
		}

		activityID := ksuid.New().String()
		if err := s.activityStartProcessor(ctx, activityID, &traversal, false); errors.IsWorkflowFatal(err) {
			logx.FromContext(ctx).Error("workflow fatally terminated whilst processing activity", "error", err, slog.String(keys.ExecutionID, traversal.ExecutionId), slog.String(keys.WorkflowID, traversal.WorkflowId), "error", err, slog.String(keys.ElementID, traversal.ElementId))
			return true, nil
		} else if err != nil {
			return false, fmt.Errorf("process event: %w", err)
		}

		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("traversal processor: %w", err)
	}
	return nil
}

func (s *Engine) processTracking(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "tracking", s.closing, "WORKFLOW.>", "Tracking", 1, s.receiveMiddleware, s.track, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("tracking processor: %w", err)
	}
	return nil
}

func (s *Engine) track(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return false, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	sj := msg.Subject()

	switch {
	case
		strings.HasSuffix(sj, messages.StateExecutionExecute),
		strings.HasSuffix(sj, messages.StateProcessExecute),
		strings.HasSuffix(sj, messages.StateTraversalExecute),
		strings.HasSuffix(sj, messages.StateActivityExecute),
		strings.Contains(sj, messages.StateJobExecute):
		st := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), st); err != nil {
			return false, fmt.Errorf("unmarshal failed during tracking 'execute' event: %w", err)
		}
		if err := common.SaveObj(ctx, nsKVs.WfTracking, st.ExecutionId, st); err != nil {
			return false, fmt.Errorf("save tracking information: %w", err)
		}
	case
		strings.HasSuffix(sj, messages.StateExecutionComplete),
		strings.HasSuffix(sj, messages.StateProcessComplete),
		strings.HasSuffix(sj, messages.StateTraversalComplete),
		strings.HasSuffix(sj, messages.StateActivityComplete),
		strings.Contains(sj, messages.StateJobComplete):
		st := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), st); err != nil {
			return false, fmt.Errorf("unmarshall failed during tracking 'complete' event: %w", err)
		}
		if err := nsKVs.WfTracking.Delete(ctx, st.ExecutionId); err != nil {
			return false, fmt.Errorf("delete workflow instance upon completion: %w", err)
		}
	default:

	}
	return true, nil
}

func (s *Engine) processCompletedJobs(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "completedJob", s.closing, subj.NS(messages.WorkFlowJobCompleteAll, "*"), "JobCompleteConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("unmarshal completed job state: %w", err)
		}
		if _, _, err := s.operations.hasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelDebug, "processCompletedJobs aborted due to a missing process")
			return true, nil
		} else if err != nil {
			return false, err
		}
		if job.State != model.CancellationState_compensating {

			if err := s.completeJobProcessor(ctx, &job); err != nil {
				return false, err
			}

		} else {
			if err := s.compensationJobComplete(ctx, &job); err != nil {
				return false, fmt.Errorf("complete compensation: %w", err)
			}
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("completed job processor: %w", err)
	}
	return nil
}

func (s *Engine) processWorkflowEvents(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "workflowEvent", s.closing, subj.NS(messages.WorkflowExecutionAll, "*"), "WorkflowConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("load workflow state processing workflow event: %w", err)
		}
		if strings.HasSuffix(msg.Subject(), messages.StateExecutionComplete) {
			if _, err := s.operations.hasValidExecution(ctx, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
				log := logx.FromContext(ctx)
				log.Log(ctx, slog.LevelDebug, "processWorkflowEvents aborted due to a missing process")
				return true, nil
			} else if err != nil {
				return false, err
			}
			if err := s.operations.xDestroyProcessInstance(ctx, &job); err != nil {
				return false, fmt.Errorf("destroy process instance whilst processing workflow events: %w", err)
			}
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("starting workflow event processing: %w", err)
	}
	return nil
}

func (s *Engine) processActivities(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "activity", s.closing, subj.NS(messages.WorkflowActivityAll, "*"), "ActivityConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var activity model.WorkflowState
		switch {
		case strings.HasSuffix(msg.Subject(), messages.StateActivityExecute):

		case strings.HasSuffix(msg.Subject(), messages.StateActivityComplete):
			if err := proto.Unmarshal(msg.Data(), &activity); err != nil {
				return false, fmt.Errorf("unmarshal state activity complete: %w", err)
			}
			if err := s.activityCompleteProcessor(ctx, &activity); err != nil {
				return false, err
			}
		}

		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("starting activity processing: %w", err)
	}
	return nil
}

func (s *Engine) processLaunch(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "launch", s.closing, subj.NS(messages.WorkflowJobLaunchExecute, "*"), "LaunchConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("unmarshal during process launch: %w", err)
		}
		if _, _, err := s.operations.hasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelDebug, "processLaunch aborted due to a missing process")
			return true, err
		} else if err != nil {
			return false, err
		}
		if err := s.launchProcessor(ctx, &job); err != nil {
			return false, fmt.Errorf("execute launch function: %w", err)
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("start process launch processor: %w", err)
	}
	return nil
}

func (s *Engine) processJobAbort(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "abort", s.closing, subj.NS(messages.WorkFlowJobAbortAll, "*"), "JobAbortConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("job abort consumer failed to unmarshal state: %w", err)
		}
		if _, _, err := s.operations.hasValidProcess(ctx, state.ProcessInstanceId, state.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelDebug, "processJobAbort aborted due to a missing process")
			return true, err
		} else if err != nil {
			return false, err
		}
		//TODO: Make these idempotently work given missing values

		switch {
		case strings.Contains(msg.Subject(), messages.StateJobAbortServiceTask), strings.Contains(msg.Subject(), messages.StateJobAbortGateway):
			if err := s.deleteJob(ctx, &state); err != nil {
				return false, fmt.Errorf("delete job during service task abort: %w", err)
			}
			if err := s.operations.RecordHistoryJobComplete(ctx, &state); err != nil {
				return true, fmt.Errorf("complete job processor failed to record history job complete: %w", err)
			}
		default:
			return true, nil
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("start job abort processor: %w", err)
	}
	return nil
}

func (s *Engine) processProcessComplete(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "processComplete", s.closing, subj.NS(messages.WorkflowProcessComplete, "*"), "ProcessCompleteConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("unmarshal during general abort processor: %w", err)
		}
		pi, execution, err := s.operations.hasValidProcess(ctx, state.ProcessInstanceId, state.ExecutionId)
		if errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelDebug, "processProcessComplete aborted due to a missing process")
			return true, err
		} else if err != nil {
			return false, err
		}
		state.State = model.CancellationState_completed
		if err := s.operations.destroyProcessInstance(ctx, &state, pi.ProcessInstanceId, execution.ExecutionId); err != nil {
			return false, fmt.Errorf("delete prcess: %w", err)
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("start general abort processor: %w", err)
	}
	return nil

}

func (s *Engine) processProcessTerminate(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "processTerminate", s.closing, subj.NS(messages.WorkflowProcessTerminated, "*"), "ProcessTerminateConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("unmarshal during general abort processor: %w", err)
		}
		if err := s.deleteProcessHistory(ctx, state.ProcessInstanceId); err != nil {
			if !errors2.Is(err, jetstream.ErrKeyNotFound) {
				return false, fmt.Errorf("delete process history: %w", err)
			}
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("start process terminate processor: %w", err)
	}
	return nil

}

func (s *Engine) processGeneralAbort(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "abort", s.closing, subj.NS(messages.WorkflowGeneralAbortAll, "*"), "GeneralAbortConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("unmarshal during general abort processor: %w", err)
		}
		//TODO: Make these idempotently work given missing values
		switch {
		case strings.HasSuffix(msg.Subject(), messages.StateExecutionAbort):
			abortState := common.CopyWorkflowState(&state)
			abortState.State = model.CancellationState_terminated
			if err := s.operations.xDestroyProcessInstance(ctx, &state); err != nil {
				return false, fmt.Errorf("delete process instance during general abort processor: %w", err)
			}
		default:
			return true, nil
		}
		return true, nil
	}, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("start general abort processor: %w", err)
	}
	return nil
}

func (s *Engine) processFatalError(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "fatalError", s.closing, subj.NS(messages.WorkflowSystemProcessFatalError, "*"), "FatalErrorConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var fatalErr model.FatalError
		if err := proto.Unmarshal(msg.Data(), &fatalErr); err != nil {
			return false, fmt.Errorf("unmarshal during fatal error processor: %w", err)
		}

		switch fatalErr.HandlingStrategy {
		case model.HandlingStrategy_TearDown:
			ack, err := s.operations.tearDownWorkflow(ctx, fatalErr.WorkflowState)
			if err != nil {
				return ack, err
			}
		case model.HandlingStrategy_Pause:
			ack, err := s.operations.PersistFatalError(ctx, &fatalErr)
			if err != nil {
				return ack, err
			}
		default:
			log := logx.FromContext(ctx)
			log.Debug("no fatal error handling strategy specified")
		}

		return true, nil
	}, nil)

	if err != nil {
		return fmt.Errorf("start process fatal error processor: %w", err)
	}
	return nil
}

func (s *Engine) processJobRetry(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "jobRetry", s.closing, subj.NS(messages.WorkflowJobRetry, "*"), "JobRetryConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		state := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), state); err != nil {
			return false, fmt.Errorf("unmarshal during fatal error processor: %w", err)
		}

		if err := s.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowJobServiceTaskAbort, subj.GetNS(ctx)), state); err != nil {
			return false, fmt.Errorf("job retry old service task abort failed to publish workflow state: %w", err)
		}

		retryWs := proto.Clone(state).(*model.WorkflowState)
		retryWsId := common.TrackingID(retryWs.Id).Pop()
		retryWs.Id = retryWsId

		if err := s.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowTraversalExecute, subj.GetNS(ctx)), retryWs); err != nil {
			return false, fmt.Errorf("job retry failed to publish workflow state: %w", err)
		}

		// delete the FatalError from the fatal err KV!
		if err := s.operations.DeleteFatalError(ctx, state); err != nil {
			return false, fmt.Errorf("job retry failed to delete fatal err: %w", err)
		}

		return true, nil
	}, nil)

	if err != nil {
		return fmt.Errorf("start process job retry processor: %w", err)
	}
	return nil
}

func (s *Engine) deleteJob(ctx context.Context, state *model.WorkflowState) error {
	if err := s.operations.DeleteJob(ctx, common.TrackingID(state.Id).ID()); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete job: %w", err)
	}
	if activityStart, err := s.operations.GetProcessHistoryItem(ctx, state.ProcessInstanceId, common.TrackingID(state.Id).Pop().ID(), model.ProcessHistoryType_activityExecute); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("fetch old state during delete job: %w", err)
	} else if err == nil {
		activityState := workfFlowStateFrom(activityStart)
		if err := s.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowActivityAbort, subj.GetNS(ctx)), activityState); err != nil {
			return fmt.Errorf("publish activity abort during delete job: %w", err)
		}
	}
	return nil
}

// deleteProcessHistory deletes the process history for a given process ID in A SHAR namespace, process history gets spooled to a.
func (s *Engine) deleteProcessHistory(ctx context.Context, processId string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)

	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}
	ks, err := common.KeyPrefixSearch(ctx, s.natsService.Js, nsKVs.WfHistory, processId, common.KeyPrefixResultOpts{})
	if err != nil {
		return fmt.Errorf("keyPrefixSearch: %w", err)
	}
	for _, k := range ks {
		if item, err := nsKVs.WfHistory.Get(ctx, k); err != nil {
			return fmt.Errorf("get workflow history item: %w", err)
		} else {
			msg := nats.NewMsg(messages.WorkflowSystemHistoryArchive)
			msg.Header.Add("KEY", k)
			msg.Data = item.Value()
			if err := s.natsService.Conn.PublishMsg(msg); err != nil {
				return fmt.Errorf("publish workflow history archive item: %w", err)
			}
		}
		if err := nsKVs.WfHistory.Delete(ctx, k); errors2.Is(err, jetstream.ErrKeyNotFound) {
			slog.Warn("key already deleted", "key", k)
		} else if err != nil {
			return fmt.Errorf("delete key %s: %w", k, err)
		}
	}
	return nil
}
