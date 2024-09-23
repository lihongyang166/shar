package workflow

import (
	"bytes"
	"context"
	_ "embed"
	errors2 "errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/setup"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	"gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/common/workflow"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/services/cache"
	"gitlab.com/shar-workflow/shar/server/services/natz"
	"gitlab.com/shar-workflow/shar/server/vars"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	maps2 "golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Ops is the interface for the Operations struct
//
//go:generate mockery
type Ops interface {
	GetTaskSpecUID(ctx context.Context, name string) (string, error)
	GetTaskSpecVersions(ctx context.Context, name string) (*model.TaskSpecVersions, error)
	PutTaskSpec(ctx context.Context, spec *model.TaskSpec) (string, error)
	GetTaskSpecByUID(ctx context.Context, uid string) (*model.TaskSpec, error)
	GetTaskSpecUsageByName(ctx context.Context, name string) (*model.TaskSpecUsageReport, error)
	GetExecutableWorkflowIds(ctx context.Context) ([]string, error)
	GetTaskSpecUsage(ctx context.Context, uid []string) (*model.TaskSpecUsageReport, error)
	RecordHistory(ctx context.Context, state *model.WorkflowState, historyType model.ProcessHistoryType) error
	RecordHistoryProcessStart(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryActivityExecute(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryActivityComplete(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryJobExecute(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryJobComplete(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryJobAbort(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryProcessComplete(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryProcessSpawn(ctx context.Context, state *model.WorkflowState, newProcessInstanceID string) error
	RecordHistoryProcessAbort(ctx context.Context, state *model.WorkflowState) error
	RecordHistoryCompensationCheckpoint(ctx context.Context, state *model.WorkflowState) error
	GetProcessHistory(ctx context.Context, processInstanceId string, wch chan<- *model.ProcessHistoryEntry, errs chan<- error)
	GetProcessHistoryItem(ctx context.Context, processInstanceID string, trackingID string, historyType model.ProcessHistoryType) (*model.ProcessHistoryEntry, error)
	LoadWorkflow(ctx context.Context, model *model.Workflow) (string, error)
	Launch(ctx context.Context, processId string, vars []byte, headers []byte) (string, string, error)
	LaunchWithParent(ctx context.Context, processId string, ID common.TrackingID, vrs []byte, headers []byte, parentpiID string, parentElID string) (string, string, error)
	CancelProcessInstance(ctx context.Context, state *model.WorkflowState) error
	CompleteManualTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error
	CompleteServiceTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error
	CompleteSendMessageTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error
	CompleteUserTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error
	ListWorkflows(ctx context.Context, res chan<- *model.ListWorkflowResponse, errs chan<- error)
	StoreWorkflow(ctx context.Context, wf *model.Workflow) (string, error)
	ProcessServiceTasks(ctx context.Context, wf *model.Workflow, svcTaskConsFn ServiceTaskConsumerFn, wfProcessMappingFn WorkflowProcessMappingFn) error
	EnsureServiceTaskConsumer(ctx context.Context, uid string) error
	GetWorkflow(ctx context.Context, workflowID string) (*model.Workflow, error)
	GetWorkflowNameFor(ctx context.Context, processId string) (string, error)
	GetWorkflowVersions(ctx context.Context, workflowName string, wch chan<- *model.WorkflowVersion, errs chan<- error)
	CreateExecution(ctx context.Context, execution *model.Execution) (*model.Execution, error)
	GetExecution(ctx context.Context, executionID string) (*model.Execution, error)
	XDestroyProcessInstance(ctx context.Context, state *model.WorkflowState) error
	GetLatestVersion(ctx context.Context, workflowName string) (string, error)
	CreateJob(ctx context.Context, job *model.WorkflowState) (string, error)
	GetJob(ctx context.Context, trackingID string) (*model.WorkflowState, error)
	DeleteJob(ctx context.Context, trackingID string) error
	ListExecutions(ctx context.Context, workflowName string, wch chan<- *model.ListExecutionItem, errs chan<- error)
	ListExecutionProcesses(ctx context.Context, id string) ([]string, error)
	PublishWorkflowState(ctx context.Context, stateName string, state *model.WorkflowState, opts ...PublishOpt) error
	SignalFatalErrorTeardown(ctx context.Context, state *model.WorkflowState, log *slog.Logger)
	SignalFatalErrorPause(ctx context.Context, state *model.WorkflowState, log *slog.Logger)
	PublishMsg(ctx context.Context, subject string, sharMsg proto.Message) error
	GetElement(ctx context.Context, state *model.WorkflowState) (*model.Element, error)
	HasValidProcess(ctx context.Context, processInstanceId, executionId string) (*model.ProcessInstance, *model.Execution, error)
	HasValidExecution(ctx context.Context, executionId string) (*model.Execution, error)
	GetUserTaskIDs(ctx context.Context, owner string) (*model.UserTasks, error)
	OwnerID(ctx context.Context, name string) (string, error)
	OwnerName(ctx context.Context, id string) (string, error)
	CreateProcessInstance(ctx context.Context, executionId string, parentProcessID string, parentElementID string, processId string, workflowName string, workflowId string, headers []byte) (*model.ProcessInstance, error)
	GetProcessInstance(ctx context.Context, processInstanceID string) (*model.ProcessInstance, error)
	DestroyProcessInstance(ctx context.Context, state *model.WorkflowState, processInstanceId string, executionId string) error
	DeprecateTaskSpec(ctx context.Context, uid []string) error
	CheckProcessTaskDeprecation(ctx context.Context, workflow *model.Workflow, processId string) error
	ListTaskSpecUIDs(ctx context.Context, deprecated bool) ([]string, error)
	GetProcessIdFor(ctx context.Context, startEventMessageName string) (string, error)
	Heartbeat(ctx context.Context, req *model.HeartbeatRequest) error
	Log(ctx context.Context, req *model.LogRequest) error
	DeleteNamespace(ctx context.Context, ns string) error
	ListExecutableProcesses(ctx context.Context, wch chan<- *model.ListExecutableProcessesItem, errs chan<- error)
	StartJob(ctx context.Context, subject string, job *model.WorkflowState, el *model.Element, v []byte, opts ...PublishOpt) error
	GetCompensationInputVariables(ctx context.Context, processInstanceId string, trackingId string) ([]byte, error)
	GetCompensationOutputVariables(ctx context.Context, processInstanceId string, trackingId string) ([]byte, error)
	HandleWorkflowError(ctx context.Context, errorCode string, inVars []byte, state *model.WorkflowState) error
	GetFatalErrors(ctx context.Context, keyPrefix string, fatalErrs chan<- *model.FatalError, errs chan<- error)
	RetryActivity(ctx context.Context, state *model.WorkflowState) error
	PersistFatalError(ctx context.Context, fatalError *model.FatalError) (bool, error)
	TearDownWorkflow(ctx context.Context, state *model.WorkflowState) (bool, error)
	DeleteFatalError(ctx context.Context, state *model.WorkflowState) error
	GetActiveEntries(ctx context.Context, processInstanceID string, result chan<- *model.ProcessHistoryEntry, errs chan<- error)
}

// Operations provides methods for executing and managing workflow processes.
// It provides methods for various tasks such as canceling process instances, completing tasks,
// retrieving workflow-related information, and managing workflow execution.
type Operations struct {
	natsService    *natz.NatsService
	sCache         *cache.SharCache[string, any]
	publishTimeout time.Duration
	sendMiddleware []middleware.Send
	tr             trace.Tracer
	exprEngine     expression.Engine
}

// NewOperations constructs a new Operations instance
func NewOperations(natsService *natz.NatsService) (*Operations, error) {
	ristrettoCache, err := cache.NewRistrettoCacheBackend[string, any]()
	if err != nil {
		return nil, fmt.Errorf("create ristretto cache: %w", err)
	}
	sharCache := cache.NewSharCache[string, any](ristrettoCache)

	return &Operations{
		natsService:    natsService,
		sCache:         sharCache,
		publishTimeout: time.Second * 30,
		sendMiddleware: []middleware.Send{telemetry.CtxSpanToNatsMsgMiddleware()},
		tr:             otel.GetTracerProvider().Tracer("shar", trace.WithInstrumentationVersion(version.Version)),
		exprEngine:     &expression.ExprEngine{},
	}, nil
}

// LoadWorkflow loads a model.Process describing a workflow into the engine ready for execution.
func (c *Operations) LoadWorkflow(ctx context.Context, model *model.Workflow) (string, error) {
	// Store the workflow model and return an ID
	wfID, err := c.StoreWorkflow(ctx, model)
	if err != nil {
		return "", fmt.Errorf("store workflow: %w", err)
	}
	return wfID, nil
}

// Launch starts a new instance of a workflow and returns an execution ID.
func (c *Operations) Launch(ctx context.Context, processId string, vars []byte, headers []byte) (string, string, error) {
	return c.LaunchWithParent(ctx, processId, []string{}, vars, headers, "", "")
}

// LaunchWithParent contains the underlying logic to start a workflow.  It is also called to spawn new instances of child workflows.
func (c *Operations) LaunchWithParent(ctx context.Context, processId string, ID common.TrackingID, vrs []byte, headers []byte, parentpiID string, parentElID string) (string, string, error) {
	var reterr error
	ctx, log := logx.ContextWith(ctx, "engine.launch")

	workflowName, err := c.GetWorkflowNameFor(ctx, processId)
	if err != nil {
		reterr = engineErr(ctx, "get the workflow name for this process name", err,
			slog.String(keys.ParentInstanceElementID, parentElID),
			slog.String(keys.ParentProcessInstanceID, parentpiID),
			slog.String(keys.ProcessId, processId),
		)
		return "", "", reterr
	}

	// get the last ID of the workflow
	wfID, err := c.GetLatestVersion(ctx, workflowName)
	if err != nil {
		reterr = engineErr(ctx, "get latest version of workflow", err,
			slog.String(keys.ParentInstanceElementID, parentElID),
			slog.String(keys.ParentProcessInstanceID, parentpiID),
			slog.String(keys.WorkflowName, workflowName),
		)
		return "", "", reterr
	}

	// get the last version of the workflow
	wf, err := c.GetWorkflow(ctx, wfID)

	if err != nil {
		reterr = engineErr(ctx, "get workflow", err,
			slog.String(keys.ParentInstanceElementID, parentElID),
			slog.String(keys.ParentProcessInstanceID, parentpiID),
			slog.String(keys.WorkflowName, workflowName),
			slog.String(keys.WorkflowID, wfID),
		)
		return "", "", reterr
	}

	if err := c.CheckProcessTaskDeprecation(ctx, wf, processId); err != nil {
		return "", "", fmt.Errorf("check task deprecation: %w", err)
	}

	var executionId string

	if parentpiID == "" {
		e, err := c.CreateExecution(ctx, &model.Execution{
			WorkflowId:   wfID,
			WorkflowName: wf.Name,
		})

		if err != nil {
			reterr = engineErr(ctx, "create execution", err,
				slog.String(keys.ParentInstanceElementID, parentElID),
				slog.String(keys.ParentProcessInstanceID, parentpiID),
				slog.String(keys.WorkflowName, workflowName),
				slog.String(keys.WorkflowID, wfID),
			)
			return "", "", reterr
		}

		defer func() {
			if reterr != nil {
				c.rollBackLaunch(ctx, e)
			}
		}()
		executionId = e.ExecutionId
	} else {
		pi, err := c.GetProcessInstance(ctx, parentpiID)
		if err != nil {
			reterr = fmt.Errorf("launch failed to get process instance for parent: %w", err)
			return "", "", reterr
		}
		headers = pi.Headers
		e, err := c.GetExecution(ctx, pi.ExecutionId)
		if err != nil {
			reterr = fmt.Errorf("launch failed to get execution for parent: %w", err)
			return "", "", reterr
		}

		executionId = e.ExecutionId
	}

	if parentpiID == "" { // only publish on creation of top level process as this is the only place an execution is defined at
		wiState := &model.WorkflowState{
			ExecutionId:  executionId,
			WorkflowId:   wfID,
			WorkflowName: workflowName,
			Vars:         vrs,
			Id:           []string{executionId},
		}

		telemetry.CtxWithTraceParentToWfState(ctx, wiState)

		ctx, log = common.ContextLoggerWithWfState(ctx, wiState)
		//log.Debug("just after adding wfState details to log ctx")

		// fire off the new workflow state
		if err := c.PublishWorkflowState(ctx, messages.WorkflowExecutionExecute, wiState); err != nil {
			reterr = engineErr(ctx, "publish workflow instance execute", err,
				slog.String(keys.WorkflowName, workflowName),
				slog.String(keys.WorkflowID, wfID),
			)
			return "", "", reterr
		}
	}
	partOfCollaboration := false
	collabProcesses := make([]*model.Process, 0, len(wf.Collaboration.Participant))

	for _, i := range wf.Collaboration.Participant {
		if i.ProcessId == processId {
			partOfCollaboration = true
		}
		pr, ok := wf.Process[i.ProcessId]
		if !ok {
			return "", "", fmt.Errorf("find collaboration process with name %s: %w", processId, err)
		}
		collabProcesses = append(collabProcesses, pr)
	}
	var launchProcesses []*model.Process
	if partOfCollaboration {
		launchProcesses = collabProcesses

	} else {
		pr, ok := wf.Process[processId]
		if !ok {
			return "", "", fmt.Errorf("find process with name %s: %w", processId, err)
		}
		launchProcesses = append(launchProcesses, pr)
	}
	for _, pr := range launchProcesses {
		err2 := c.launchProcess(ctx, ID, pr.Id, pr, workflowName, wfID, executionId, vrs, parentpiID, parentElID, headers, log)
		if err2 != nil {
			return "", "", err2
		}
	}

	return executionId, wfID, nil
}

func (c *Operations) launchProcess(ctx context.Context, ID common.TrackingID, prId string, pr *model.Process, workflowName string, wfID string, executionId string, vrs []byte, parentpiID string, parentElID string, headers []byte, log *slog.Logger) error {
	ctx, sp := c.tr.Start(ctx, "launchProcess")
	defer func() {
		sp.End()
	}()
	var reterr error
	var hasStartEvents bool

	testVars, err := vars.Decode(ctx, vrs)
	if err != nil {
		return fmt.Errorf("decode variables during launch: %w", err)
	}

	// Test to see if all variables are present.
	vErr := forEachStartElement(pr, func(el *model.Element) error {
		hasStartEvents = true
		if el.OutputTransform != nil {
			for _, v := range el.OutputTransform {
				evs, err := expression.GetVariables(ctx, c.exprEngine, v)
				if err != nil {
					return fmt.Errorf("extract variables from workflow during launch: %w", err)
				}
				for _, ev := range evs {
					if _, ok := testVars[ev.Name]; !ok {
						return fmt.Errorf("workflow expects variable '%s': %w", ev, errors.ErrExpectedVar)
					}
				}
			}
		}
		return nil
	})

	if vErr != nil {
		reterr = fmt.Errorf("initialize all workflow start events: %w", vErr)
		return reterr
	}

	if hasStartEvents {

		pi, err := c.CreateProcessInstance(ctx, executionId, parentpiID, parentElID, pr.Id, workflowName, wfID, headers)
		if err != nil {
			reterr = fmt.Errorf("launch failed to create new process instance: %w", err)
			return reterr
		}

		errs := make(chan error)

		// for each start element, launch a workflow thread
		startErr := forEachStartElement(pr, func(el *model.Element) error {
			trackingID := ID.Push(pi.ProcessInstanceId).Push(ksuid.New().String())
			exec := &model.WorkflowState{
				ElementType: el.Type,
				ElementId:   el.Id,
				ElementName: el.Name,
				WorkflowId:  wfID,
				//WorkflowInstanceId: wfi.WorkflowInstanceId,
				ExecutionId:       executionId,
				Id:                trackingID,
				Vars:              vrs,
				WorkflowName:      workflowName,
				ProcessId:         prId,
				ProcessInstanceId: pi.ProcessInstanceId,
			}

			if trace.SpanContextFromContext(ctx).IsValid() {
				telemetry.CtxWithTraceParentToWfState(ctx, exec)
			} else {
				// If there is no valid span, then use the traceparent.
				exec.TraceParent = ctx.Value(ctxkey.Traceparent).(string)
			}
			ctx, log := common.ContextLoggerWithWfState(ctx, exec)
			//log.Debug("just prior to publishing start msg")

			processWfState := proto.Clone(exec).(*model.WorkflowState)
			processTrackingId := ID.Push(executionId).Push(pi.ProcessInstanceId)
			processWfState.Id = processTrackingId

			if err := c.PublishWorkflowState(ctx, subj.NS(messages.WorkflowProcessExecute, subj.GetNS(ctx)), processWfState); err != nil {
				return fmt.Errorf("publish workflow timed process execute: %w", err)
			}
			if err := c.RecordHistoryProcessStart(ctx, exec); err != nil {
				log.Error("start events record process start", "error", err)
				return fmt.Errorf("publish initial traversal: %w", err)
			}
			if err := c.PublishWorkflowState(ctx, messages.WorkflowTraversalExecute, exec); err != nil {
				log.Error("publish initial traversal", "error", err)
				return fmt.Errorf("publish initial traversal: %w", err)
			}
			return nil
		})
		if startErr != nil {
			reterr = startErr
			return reterr
		}
		// wait for all paths to be started
		close(errs)
		if err := <-errs; err != nil {
			reterr = engineErr(ctx, "initial traversal", err,
				slog.String(keys.ParentInstanceElementID, parentElID),
				slog.String(keys.ParentProcessInstanceID, parentpiID),
				slog.String(keys.WorkflowName, workflowName),
				slog.String(keys.WorkflowID, wfID),
			)
			return reterr
		}
	}
	return nil
}

func (c *Operations) rollBackLaunch(ctx context.Context, e *model.Execution) {
	log := logx.FromContext(ctx)
	log.Info("rolling back workflow launch")
	err := c.PublishWorkflowState(ctx, messages.WorkflowExecutionAbort, &model.WorkflowState{
		//Id:           []string{e.WorkflowInstanceId},
		Id:           []string{e.ExecutionId},
		ExecutionId:  e.ExecutionId,
		WorkflowName: e.WorkflowName,
		WorkflowId:   e.WorkflowId,
		State:        model.CancellationState_terminated,
	})
	if err != nil {
		log.Error("workflow fatally terminated, however the termination signal could not be sent", "error", err)
	}
}

// forEachStartElement finds all start elements for a given process and executes a function on the element.
func forEachStartElement(pr *model.Process, fn func(element *model.Element) error) error {
	for _, i := range pr.Elements {
		if i.Type == element.StartEvent {
			err := fn(i)
			if err != nil {
				return fmt.Errorf("start event execution: %w", err)
			}
		}
	}
	return nil
}

// CancelProcessInstance cancels a workflow instance with a reason.
func (c *Operations) CancelProcessInstance(ctx context.Context, state *model.WorkflowState) error {
	if state.State == model.CancellationState_executing {
		return fmt.Errorf("executing is an invalid cancellation state: %w", errors.ErrInvalidState)
	}
	if err := c.XDestroyProcessInstance(ctx, state); err != nil {
		return fmt.Errorf("cancel workflow instance failed: %w", errors.ErrCancelFailed)
	}
	return nil
}

// CompleteManualTask completes a manual workflow task
func (c *Operations) CompleteManualTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error {
	el, err := c.GetElement(ctx, job)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	job.Vars = newvars
	err = vars.CheckVars(ctx, c.exprEngine, job, el)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	if err := c.PublishWorkflowState(ctx, messages.WorkflowJobManualTaskComplete, job); err != nil {
		return fmt.Errorf("complete manual task failed to publish manual task complete message: %w", err)
	}
	return nil
}

// CompleteServiceTask completes a workflow service task
func (c *Operations) CompleteServiceTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error {
	if job.State != model.CancellationState_compensating {
		if _, err := c.GetProcessHistoryItem(ctx, job.ProcessInstanceId, common.TrackingID(job.Id).ParentID(), model.ProcessHistoryType_activityExecute); errors2.Is(err, jetstream.ErrKeyNotFound) {
			if err := c.PublishWorkflowState(ctx, subj.NS(messages.WorkflowJobServiceTaskAbort, subj.GetNS(ctx)), job); err != nil {
				return fmt.Errorf("complete service task failed to publish workflow state: %w", err)
			}
			return nil
		} else if err != nil {
			return fmt.Errorf("complete service task failed to get old state: %w", err)
		}
	}
	el, err := c.GetElement(ctx, job)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	job.Vars = newvars
	err = vars.CheckVars(ctx, c.exprEngine, job, el)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	if err := c.PublishWorkflowState(ctx, messages.WorkflowJobServiceTaskComplete, job); err != nil {
		return fmt.Errorf("complete service task failed to publish service task complete message: %w", err)
	}
	return nil
}

// CompleteSendMessageTask completes a send message task
func (c *Operations) CompleteSendMessageTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error {
	_, err := c.GetElement(ctx, job)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	if err := c.PublishWorkflowState(ctx, messages.WorkflowJobSendMessageComplete, job); err != nil {
		return fmt.Errorf("complete send message task failed to publish send message complete nats message: %w", err)
	}
	return nil
}

// CompleteUserTask completes and closes a user task with variables
func (c *Operations) CompleteUserTask(ctx context.Context, job *model.WorkflowState, newvars []byte) error {
	el, err := c.GetElement(ctx, job)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	job.Vars = newvars
	err = vars.CheckVars(ctx, c.exprEngine, job, el)
	if err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	if err := c.PublishWorkflowState(ctx, messages.WorkflowJobUserTaskComplete, job); err != nil {
		return fmt.Errorf("complete user task failed to publish user task complete message: %w", err)
	}
	if err := c.closeUserTask(ctx, common.TrackingID(job.Id).ID()); err != nil {
		return fmt.Errorf("complete user task failed to close user task: %w", err)
	}
	return nil
}

// ListWorkflows returns a list of all the workflows in SHAR.
func (s *Operations) ListWorkflows(ctx context.Context, res chan<- *model.ListWorkflowResponse, errs chan<- error) {

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		err2 := fmt.Errorf("get KVs for ns %s: %w", ns, err)
		log := logx.FromContext(ctx)
		log.Error("ListWorkflows get KVs", "error", err)
		errs <- err2
		return
	}

	ks, err := nsKVs.WfVersion.Keys(ctx)
	if errors2.Is(err, jetstream.ErrNoKeysFound) {
		ks = []string{}
	} else if err != nil {
		errs <- err
		return
	}

	v := &model.WorkflowVersions{}
	for _, k := range ks {
		err := common.LoadObj(ctx, nsKVs.WfVersion, k, v)
		if errors2.Is(err, jetstream.ErrNoKeysFound) {
			continue
		}
		if err != nil {
			errs <- err
		}
		res <- &model.ListWorkflowResponse{
			Name:    k,
			Version: v.Version[len(v.Version)-1].Number,
		}

	}
}

func (s *Operations) validateUniqueProcessIdFor(ctx context.Context, wf *model.Workflow) error {
	existingProcessWorkflowNames := make(map[string]string)

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	for processId := range wf.Process {
		workFlowName, err := nsKVs.WfProcess.Get(ctx, processId)
		if errors2.Is(err, jetstream.ErrKeyNotFound) {
			continue
		}
		wfName := string(workFlowName.Value())
		if wfName != wf.Name {
			existingProcessWorkflowNames[processId] = wfName
		}
	}

	if len(existingProcessWorkflowNames) > 0 {
		s := make([]string, len(existingProcessWorkflowNames))

		for pName, wName := range existingProcessWorkflowNames {
			s = append(s, fmt.Sprintf("[process: %s, workflow: %s]", pName, wName))
		}

		existingProcessWorkflows := strings.Join(s, ",")
		return fmt.Errorf("These process names already exist under different workflows %s", existingProcessWorkflows)
	}

	return nil
}

// StoreWorkflow stores a workflow definition and returns a unique ID
func (s *Operations) StoreWorkflow(ctx context.Context, wf *model.Workflow) (string, error) {
	err := s.validateUniqueProcessIdFor(ctx, wf)
	if err != nil {
		return "", fmt.Errorf("process names are not unique: %w", err)
	}

	err = s.validateUniqueMessageNames(ctx, wf)
	if err != nil {
		return "", fmt.Errorf("message names are not globally unique: %w", err)
	}

	// Populate Metadata
	s.populateMetadata(wf)

	// get this workflow name if it has already been registered
	ns := subj.GetNS(ctx)

	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	_, err = nsKVs.WfName.Get(ctx, wf.Name)
	if errors2.Is(err, jetstream.ErrKeyNotFound) {
		wfNameID := ksuid.New().String()
		_, err = nsKVs.WfName.Put(ctx, wf.Name, []byte(wfNameID))
		if err != nil {
			return "", fmt.Errorf("store the workflow id during store workflow: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("get an existing workflow id: %w", err)
	}

	wfID := ksuid.New().String()

	createWorkflowProcessMappingFn := func(ctx context.Context, wf *model.Workflow, i *model.Process) (uint64, error) {
		ret, err := nsKVs.WfProcess.Put(ctx, i.Id, []byte(wf.Name))
		if err != nil {
			return 0, fmt.Errorf("store the workflow process mapping: %w", err)
		}
		return ret, nil
	}

	err3 := s.ProcessServiceTasks(ctx, wf, s.EnsureServiceTaskConsumer, createWorkflowProcessMappingFn)
	if err3 != nil {
		return "", err3
	}

	hash, err2 := workflow.GetHash(wf)
	if err2 != nil {
		return "", fmt.Errorf("store workflow failed to get the workflow hash: %w", err2)
	}

	var newWf bool
	log := logx.FromContext(ctx)
	if err := common.UpdateObj(ctx, nsKVs.WfVersion, wf.Name, &model.WorkflowVersions{}, func(v *model.WorkflowVersions) (*model.WorkflowVersions, error) {
		n := len(v.Version)
		if v.Version == nil || n == 0 {
			v.Version = make([]*model.WorkflowVersion, 0, 1)
		} else {
			if bytes.Equal(hash, v.Version[n-1].Sha256) {
				wfID = v.Version[n-1].Id
				log.Info("workflow version already exists", keys.WorkflowID, v.Version[n-1].Sha256)
				return v, nil
			}
		}
		newWf = true
		err = common.SaveObj(ctx, nsKVs.Wf, wfID, wf)
		if err != nil {
			return nil, fmt.Errorf("save workflow: %s", wf.Name)
		}
		v.Version = append(v.Version, &model.WorkflowVersion{Id: wfID, Sha256: hash, Number: int32(n) + 1}) //nolint:gosec
		log.Info("workflow version created", keys.WorkflowID, hash)
		return v, nil
	}); err != nil {
		return "", fmt.Errorf("update workflow version for: %s", wf.Name)
	}

	if !newWf {
		return wfID, nil
	}

	if err := s.ensureMessageBuckets(ctx, wf); err != nil {
		return "", fmt.Errorf("create workflow message buckets: %w", err)
	}

	for _, pr := range wf.Process {
		// Start all timed start events.
		vErr := forEachTimedStartElement(pr, func(el *model.Element) error {
			if el.Type == element.TimedStartEvent {
				timer := &model.WorkflowState{
					Id:           []string{},
					WorkflowId:   wfID,
					ExecutionId:  "",
					ElementId:    el.Id,
					ElementName:  el.Name,
					UnixTimeNano: time.Now().UnixNano(),
					Timer: &model.WorkflowTimer{
						LastFired: 0,
						Count:     0,
					},
					Vars:         []byte{},
					WorkflowName: wf.Name,
					ProcessId:    pr.Id,
				}
				if err := s.PublishWorkflowState(ctx, subj.NS(messages.WorkflowTimedExecute, ns), timer); err != nil {
					return fmt.Errorf("publish workflow timed execute: %w", err)
				}
				return nil
			}
			return nil
		})

		if vErr != nil {
			return "", fmt.Errorf("initialize all workflow timed start events for %s: %w", pr.Id, vErr)
		}
	}

	return wfID, nil
}

func (s *Operations) ensureMessageBuckets(ctx context.Context, wf *model.Workflow) error {
	//TODO should this be in natsService???
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	for _, m := range wf.Messages {

		receivers, ok := wf.MessageReceivers[m.Name]
		var rcvrBytes []byte
		if !ok {
			rcvrBytes = []byte{}
		} else {
			var err error
			rcvrBytes, err = proto.Marshal(receivers)
			if err != nil {
				return fmt.Errorf("failed serialising message receivers: %w", err)
			}
		}

		if err := common.Save(ctx, nsKVs.WfMsgTypes, m.Name, rcvrBytes); err != nil {
			return &errors.ErrWorkflowFatal{Err: err}
		}

		ks := ksuid.New()

		//TODO: this should only happen if there is a task associated with message send
		if err := common.Save(ctx, nsKVs.WfClientTask, wf.Name+"_"+m.Name, []byte(ks.String())); err != nil {
			return fmt.Errorf("create a client task during workflow creation: %w", err)
		}

		jxCfg := jetstream.ConsumerConfig{
			Durable:       "ServiceTask_" + subj.GetNS(ctx) + "_" + wf.Name + "_" + m.Name,
			Description:   "",
			FilterSubject: subj.NS(messages.WorkflowJobSendMessageExecute, subj.GetNS(ctx)) + "." + wf.Name + "_" + m.Name,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxAckPending: 65536,
		}
		if _, err := s.natsService.Js.CreateOrUpdateConsumer(ctx, "WORKFLOW", jxCfg); err != nil {
			return fmt.Errorf("add service task consumer: %w", err)
		}
	}
	return nil
}

// ProcessServiceTasks iterates over service tasks in the processes of a given workflow setting, validating them and setting their uid into their element definitions
func (s *Operations) ProcessServiceTasks(ctx context.Context, wf *model.Workflow, svcTaskConsFn ServiceTaskConsumerFn, wfProcessMappingFn WorkflowProcessMappingFn) error {
	for _, i := range wf.Process {
		for _, j := range i.Elements {
			if j.Type == element.ServiceTask {
				id, err := s.GetTaskSpecUID(ctx, j.Execute)
				if err != nil && errors2.Is(err, jetstream.ErrKeyNotFound) {
					return fmt.Errorf("task %s is not registered: %w", j.Execute, err)
				}
				j.Version = &id

				def, err := s.GetTaskSpecByUID(ctx, id)
				if err != nil {
					return fmt.Errorf("look up task spec for '%s': %w", j.Execute, err)
				}

				// Validate the input parameters
				if def.Parameters != nil && def.Parameters.Input != nil {
					for _, val := range def.Parameters.Input {
						if !val.Mandatory {
							continue
						}
						if _, ok := j.InputTransform[val.Name]; !ok {
							return fmt.Errorf("mandatory input parameter %s was expected for service task %s", val.Name, j.Id)
						}
					}
				}

				// Validate the output parameters
				// Collate all the variables from the output transforms
				outVars := make(map[string]struct{})
				for varN, exp := range j.OutputTransform {
					vars, err := expression.GetVariables(ctx, s.exprEngine, exp)
					if err != nil {
						return fmt.Errorf("an error occurred getting the variables from the output expression for %s in service task %s", varN, j.Id)
					}
					// Take the variables and add them to the list
					for _, i := range vars {
						outVars[i.Name] = struct{}{}
					}
				}
				if def.Parameters != nil && def.Parameters.Output != nil {
					for _, val := range def.Parameters.Output {
						if !val.Mandatory {
							continue
						}
						if _, ok := outVars[val.Name]; !ok {
							return fmt.Errorf("mandatory output parameter %s was expected for service task %s", val.Name, j.Id)
						}
					}
				}

				// Merge default retry policy.
				if r := j.RetryBehaviour; r == nil { // No behaviour given in the BPMN
					if def.Behaviour.DefaultRetry != nil { // Behaviour defined by
						j.RetryBehaviour = def.Behaviour.DefaultRetry
					}
				} else {
					retry := j.RetryBehaviour.Number
					j.RetryBehaviour = def.Behaviour.DefaultRetry
					if retry != 0 {
						j.RetryBehaviour.Number = retry
					}
				}

				// Check to make sure errors can't set variable values that are not handled.
				if j.RetryBehaviour.DefaultExceeded.Action == model.RetryErrorAction_SetVariableValue {
					if _, ok := outVars[j.RetryBehaviour.DefaultExceeded.Variable]; !ok {
						return fmt.Errorf("retry exceeded output parameter %s was expected for service task %s, but is not handled", j.RetryBehaviour.DefaultExceeded.VariableValue, j.Id)
					}
				}

				// Check to make sure workflow errors exist and are handled in the service task.
				if j.RetryBehaviour.DefaultExceeded.Action == model.RetryErrorAction_ThrowWorkflowError {
					errCode := j.RetryBehaviour.DefaultExceeded.ErrorCode
					errIndex := slices.IndexFunc(wf.Errors, func(e *model.Error) bool { return e.Code == errCode })
					if errIndex == -1 {
						return fmt.Errorf("%s retry exceeded error code %s is not declared", j.Id, errCode)
					}
					wfErr := wf.Errors[errIndex]
					handleIndex := slices.IndexFunc(j.Errors, func(e *model.CatchError) bool { return e.ErrorId == wfErr.Id })
					if handleIndex == -1 {
						return fmt.Errorf("%s retry exceeded error code %s can be thrown but is not handled", j.Id, errCode)
					}
				}

				if err := svcTaskConsFn(ctx, id); err != nil {
					return fmt.Errorf("ensure consumer for service task %s:%w", j.Execute, err)
				}
			}
		}

		_, err := wfProcessMappingFn(ctx, wf, i)
		if err != nil {
			return fmt.Errorf("store the process to workflow mapping: %w", err)
		}
	}
	return nil
}

// WorkflowProcessMappingFn defines the type of a function that creates a workflow to process mapping
type WorkflowProcessMappingFn func(ctx context.Context, wf *model.Workflow, i *model.Process) (uint64, error)

// NoOpWorkFlowProcessMappingFn no op workflow to process mapping fn
func NoOpWorkFlowProcessMappingFn(_ context.Context, _ *model.Workflow, _ *model.Process) (uint64, error) {
	return 0, nil
}

// ServiceTaskConsumerFn defines the type of a function that ensures existence of a service task consumer
type ServiceTaskConsumerFn func(ctx context.Context, id string) error

// NoOpServiceTaskConsumerFn no op service task consumer fn
func NoOpServiceTaskConsumerFn(_ context.Context, _ string) error {
	return nil
}

// EnsureServiceTaskConsumer creates or updates a service task consumer.
func (s *Operations) EnsureServiceTaskConsumer(ctx context.Context, uid string) error {
	//TODO should this be in natsService???
	ns := subj.GetNS(ctx)
	jxCfg := jetstream.ConsumerConfig{
		Durable:       "ServiceTask_" + ns + "_" + uid,
		Description:   "",
		FilterSubject: subj.NS(messages.WorkflowJobServiceTaskExecute, subj.GetNS(ctx)) + "." + uid,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MemoryStorage: s.natsService.StorageType == jetstream.MemoryStorage,
	}

	if _, err := s.natsService.Js.CreateOrUpdateConsumer(ctx, "WORKFLOW", jxCfg); err != nil {
		return fmt.Errorf("add service task consumer: %w", err)
	}
	return nil
}

// forEachStartElement finds all start elements for a given process and executes a function on the element.
func forEachTimedStartElement(pr *model.Process, fn func(element *model.Element) error) error {
	for _, i := range pr.Elements {
		if i.Type == element.TimedStartEvent {
			err := fn(i)
			if err != nil {
				return fmt.Errorf("timed start event execution: %w", err)
			}
		}
	}
	return nil
}

// GetWorkflow - retrieves a workflow model given its ID
func (s *Operations) GetWorkflow(ctx context.Context, workflowID string) (*model.Workflow, error) {
	getWorkflowFn := func() (*model.Workflow, error) {
		ns := subj.GetNS(ctx)
		nsKVs, err := s.natsService.KvsFor(ctx, ns)
		if err != nil {
			return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
		}

		wf := &model.Workflow{}
		if err := common.LoadObj(ctx, nsKVs.Wf, workflowID, wf); errors2.Is(err, jetstream.ErrKeyNotFound) {
			return nil, fmt.Errorf("get workflow failed to load object: %w", errors.ErrWorkflowNotFound)

		} else if err != nil {
			return nil, fmt.Errorf("load workflow from KV: %w", err)
		}
		return wf, nil
	}

	workflow, err := cache.Cacheable[string, *model.Workflow](workflowID, getWorkflowFn, s.sCache)
	//workflow, err := getWorkflowFn()
	if err != nil {
		return nil, fmt.Errorf("error caching GetWorkflow: %w", err)
	}
	return workflow, nil
}

// GetWorkflowNameFor - get the worflow name a process is associated with
func (s *Operations) GetWorkflowNameFor(ctx context.Context, processId string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if entry, err := common.Load(ctx, nsKVs.WfProcess, processId); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return "", fmt.Errorf(fmt.Sprintf("get workflow name for process %s: %%w", processId), errors.ErrProcessNotFound)
	} else if err != nil {
		return "", fmt.Errorf("load workflow name for process: %w", err)
	} else {
		return string(entry), nil
	}
}

// GetWorkflowVersions - returns a list of versions for a given workflow.
func (s *Operations) GetWorkflowVersions(ctx context.Context, workflowName string, wch chan<- *model.WorkflowVersion, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}

	ver := &model.WorkflowVersions{}
	if err := common.LoadObj(ctx, nsKVs.WfVersion, workflowName, ver); errors2.Is(err, jetstream.ErrKeyNotFound) {
		errs <- fmt.Errorf("load object: %w", errors.ErrWorkflowVersionNotFound)
		return
	} else if err != nil {
		errs <- fmt.Errorf("load workflow from KV: %w", err)
		return
	}
	for _, i := range ver.Version {
		wch <- i
	}
}

// CreateExecution given a workflow, starts a new execution and returns its ID
func (s *Operations) CreateExecution(ctx context.Context, execution *model.Execution) (*model.Execution, error) {
	executionID := ksuid.New().String()
	log := logx.FromContext(ctx)
	log.Info("creating execution", slog.String(keys.ExecutionID, executionID))
	execution.ExecutionId = executionID
	execution.ProcessInstanceId = []string{}

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	wf, err := s.GetWorkflow(ctx, execution.WorkflowId)

	if err := common.SaveObj(ctx, nsKVs.WfExecution, executionID, execution); err != nil {
		return nil, fmt.Errorf("save execution object to KV: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow object from KV: %w", err)
	}
	for _, m := range wf.Messages {
		if err := setup.EnsureBucket(ctx, s.natsService.Js, jetstream.KeyValueConfig{
			Bucket:      subj.NS("MsgTx_%s_", subj.GetNS(ctx)) + m.Name,
			Description: "Message transmit for " + m.Name,
		}, s.natsService.StorageType, func(_ *jetstream.KeyValueConfig) {}); err != nil {
			return nil, fmt.Errorf("ensuring bucket '%s':%w", m.Name, err)
		}
	}
	return execution, nil
}

// GetExecution retrieves an execution given its ID.
func (s *Operations) GetExecution(ctx context.Context, executionID string) (*model.Execution, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	e := &model.Execution{}
	if err := common.LoadObj(ctx, nsKVs.WfExecution, executionID, e); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("get execution failed to load object: %w", errors.ErrExecutionNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("load execution from KV: %w", err)
	}
	return e, nil
}

// XDestroyProcessInstance terminates a running process instance with a cancellation reason and error
func (s *Operations) XDestroyProcessInstance(ctx context.Context, state *model.WorkflowState) error {
	log := logx.FromContext(ctx)
	log.Info("destroying process instance", slog.String(keys.ProcessInstanceID, state.ProcessInstanceId))

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	// TODO: soft error
	execution, err := s.GetExecution(ctx, state.ExecutionId)
	if err != nil {
		return fmt.Errorf("x destroy process instance, get execution: %w", err)
	}
	pi, err := s.GetProcessInstance(ctx, state.ProcessInstanceId)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("x destroy process instance, get process instance: %w", err)
	}
	err = s.DestroyProcessInstance(ctx, state, pi.ProcessInstanceId, execution.ExecutionId)
	if err != nil {
		return fmt.Errorf("x destroy process instance, kill process instance: %w", err)
	}
	// Get the workflow
	wf := &model.Workflow{}
	if execution.WorkflowId != "" {
		if err := common.LoadObj(ctx, nsKVs.Wf, execution.WorkflowId, wf); err != nil {
			log.Warn("fetch workflow definition",
				slog.String(keys.ExecutionID, execution.ExecutionId),
				slog.String(keys.WorkflowID, execution.WorkflowId),
				slog.String(keys.WorkflowName, wf.Name),
			)
		}
	}
	tState := common.CopyWorkflowState(state)

	if tState.Error != nil {
		tState.State = model.CancellationState_errored
	}

	if err := s.deleteExecution(ctx, tState); err != nil {
		return fmt.Errorf("delete workflow state whilst destroying execution: %w", err)
	}
	return nil
}

func (s *Operations) deleteExecution(ctx context.Context, state *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := nsKVs.WfExecution.Delete(ctx, state.ExecutionId); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete workflow instance: %w", err)
	}

	//TODO: Loop through all messages checking for process subscription and remove

	if err := nsKVs.WfTracking.Delete(ctx, state.ExecutionId); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete workflow tracking: %w", err)
	}
	if err := s.PublishWorkflowState(ctx, messages.ExecutionTerminated, state); err != nil {
		return fmt.Errorf("send workflow terminate message: %w", err)
	}
	return nil
}

// GetLatestVersion queries the workflow versions table for the latest entry
func (s *Operations) GetLatestVersion(ctx context.Context, workflowName string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	v := &model.WorkflowVersions{}
	if err := common.LoadObj(ctx, nsKVs.WfVersion, workflowName, v); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return "", fmt.Errorf("get latest workflow version: %w", errors.ErrWorkflowNotFound)
	} else if err != nil {
		return "", fmt.Errorf("load object whist getting latest versiony: %w", err)
	} else {
		return v.Version[len(v.Version)-1].Id, nil
	}
}

// CreateJob stores a workflow task state for user tasks.
func (s *Operations) CreateJob(ctx context.Context, job *model.WorkflowState) (string, error) {
	//TODO move into engine.go
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	tid := ksuid.New().String()
	job.Id = common.TrackingID(job.Id).Push(tid)
	if err := common.SaveObj(ctx, nsKVs.Job, tid, job); err != nil {
		return "", fmt.Errorf("save job to KV: %w", err)
	}
	return tid, nil
}

// GetJob gets a workflow task state.
func (s *Operations) GetJob(ctx context.Context, trackingID string) (*model.WorkflowState, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	job := &model.WorkflowState{}
	if err := common.LoadObj(ctx, nsKVs.Job, trackingID, job); err == nil {
		return job, nil
	} else if errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("get job failed to load workflow object: %w", errors.ErrJobNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("load job from KV: %w", err)
	} else {
		return job, nil
	}
}

// DeleteJob removes a workflow task state.
func (s *Operations) DeleteJob(ctx context.Context, trackingID string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.Delete(ctx, nsKVs.Job, trackingID); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return nil
}

// ListExecutions returns a list of running workflows and versions given a workflow Name
func (s *Operations) ListExecutions(ctx context.Context, workflowName string, wch chan<- *model.ListExecutionItem, errs chan<- error) {
	log := logx.FromContext(ctx)

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		err2 := fmt.Errorf("get KVs for ns %s: %w", ns, err)
		log := logx.FromContext(ctx)
		log.Error("list exec get KVs", "error", err)
		errs <- err2
		return
	}

	wfv := &model.WorkflowVersions{}
	if err := common.LoadObj(ctx, nsKVs.WfVersion, workflowName, wfv); err != nil {
		errs <- err
		return
	}

	ver := make(map[string]*model.WorkflowVersion)
	for _, v := range wfv.Version {
		ver[v.Id] = v
	}

	ks, err := nsKVs.WfExecution.Keys(ctx)
	if errors2.Is(err, jetstream.ErrNoKeysFound) {
		ks = []string{}
	} else if err != nil {
		log := logx.FromContext(ctx)
		log.Error("obtaining keys", "error", err)
		errs <- err
		return
	}
	for _, k := range ks {
		v := &model.Execution{}
		err := common.LoadObj(ctx, nsKVs.WfExecution, k, v)
		if wv, ok := ver[v.WorkflowId]; ok {
			if err != nil && errors2.Is(err, jetstream.ErrKeyNotFound) {
				errs <- err
				log.Error("loading object", "error", err)
				return
			}
			wch <- &model.ListExecutionItem{
				Id:      k,
				Version: wv.Number,
			}
		}
	}
}

// ListExecutionProcesses gets the current processIDs for an execution.
func (s *Operations) ListExecutionProcesses(ctx context.Context, id string) ([]string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	v := &model.Execution{}
	err = common.LoadObj(ctx, nsKVs.WfExecution, id, v)
	if err != nil {
		return nil, fmt.Errorf("load execution from KV: %w", err)
	}
	return v.ProcessInstanceId, nil
}

// PublishWorkflowState publishes a SHAR state object to a given subject
func (s *Operations) PublishWorkflowState(ctx context.Context, stateName string, state *model.WorkflowState, opts ...PublishOpt) error {
	//TODO should this be in natsService???
	c := &publishOptions{}
	for _, i := range opts {
		i.Apply(c)
	}
	state.UnixTimeNano = time.Now().UnixNano()

	msg := nats.NewMsg(subj.NS(stateName, subj.GetNS(ctx)))
	msg.Header.Set("embargo", strconv.Itoa(c.Embargo))
	msg.Header.Set(header.SharNamespace, subj.GetNS(ctx))
	if c.headers != nil {
		for k, v := range c.headers {
			msg.Header.Set(k, v)
		}
	}
	b, err := proto.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal proto during publish workflow state: %w", err)
	}
	msg.Data = b
	if err := header.FromCtxToMsgHeader(ctx, &msg.Header); err != nil {
		return fmt.Errorf("add header to published workflow state: %w", err)
	}

	wrappedMessage := common.NewNatsMsgWrapper(msg)
	for _, i := range s.sendMiddleware {
		if err := i(ctx, wrappedMessage); err != nil {
			return fmt.Errorf("apply middleware %s: %w", reflect.TypeOf(i), err)
		}
	}

	if !trace.SpanContextFromContext(ctx).IsValid() {
		msg.Header.Set("traceparent", state.TraceParent)
	}

	pubCtx, cancel := context.WithTimeout(ctx, s.publishTimeout)
	defer cancel()
	if c.ID == "" {
		c.ID = ksuid.New().String()
	}

	if _, err := s.natsService.TxJS.PublishMsg(pubCtx, msg, jetstream.WithMsgID(c.ID)); err != nil {
		log := logx.FromContext(ctx)
		log.Error("publish message", "error", err, slog.String("nats.msg.id", c.ID), slog.Any("state", state), slog.String("subject", msg.Subject))
		return fmt.Errorf("publish workflow state message: %w", err)
	}
	if stateName == subj.NS(messages.WorkflowJobUserTaskExecute, subj.GetNS(ctx)) {
		for _, i := range append(state.Owners, state.Groups...) {
			if err := s.openUserTask(ctx, i, common.TrackingID(state.Id).ID()); err != nil {
				return fmt.Errorf("open user task during publish workflow state: %w", err)
			}
		}
	}
	return nil
}

// SignalFatalErrorTeardown publishes a FatalError message on FatalErr of a process in a workflow with a Teardown strategy
func (s *Operations) SignalFatalErrorTeardown(ctx context.Context, state *model.WorkflowState, log *slog.Logger) {
	s.signaFatalError(ctx, state, log, model.HandlingStrategy_TearDown)
}

// SignalFatalErrorPause publishes a FatalError message on FatalErr of a process in a workflow with a Pause strategy
func (s *Operations) SignalFatalErrorPause(ctx context.Context, state *model.WorkflowState, log *slog.Logger) {
	s.signaFatalError(ctx, state, log, model.HandlingStrategy_Pause)
}

func (s *Operations) signaFatalError(ctx context.Context, state *model.WorkflowState, log *slog.Logger, handlingStrategy model.HandlingStrategy) {
	fataError := &model.FatalError{
		HandlingStrategy: handlingStrategy,
		WorkflowState:    state,
	}
	err := s.PublishMsg(ctx, subj.NS(messages.WorkflowSystemProcessFatalError, subj.GetNS(ctx)), fataError)
	if err != nil {
		log.Error("failed publishing fatal err", "err", err)
	}
}

// PublishMsg publishes a workflow message.
func (s *Operations) PublishMsg(ctx context.Context, subject string, sharMsg proto.Message) error {
	//TODO should this be in natsService???
	msg := nats.NewMsg(subject)
	b, err := proto.Marshal(sharMsg)
	if err != nil {
		return fmt.Errorf("marshal message for publishing: %w", err)
	}
	msg.Data = b
	if err := header.FromCtxToMsgHeader(ctx, &msg.Header); err != nil {
		return fmt.Errorf("add header to published message: %w", err)
	}
	msg.Header.Set(header.SharNamespace, subj.GetNS(ctx))
	pubCtx, cancel := context.WithTimeout(ctx, s.publishTimeout)
	defer cancel()
	id := ksuid.New().String()
	if _, err := s.natsService.TxJS.PublishMsg(ctx, msg, jetstream.WithMsgID(id)); err != nil {
		log := logx.FromContext(pubCtx)
		log.Error("publish message", "error", err, slog.String("nats.msg.id", id), slog.Any("msg", sharMsg), slog.String("subject", msg.Subject))
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}

// GetElement gets the definition for the current element given a workflow state.
func (s *Operations) GetElement(ctx context.Context, state *model.WorkflowState) (*model.Element, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	wf := &model.Workflow{}
	if err := common.LoadObj(ctx, nsKVs.Wf, state.WorkflowId, wf); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("load object during get element: %w", err)
	}
	els := common.ElementTable(wf)
	if el, ok := els[state.ElementId]; ok {
		return el, nil
	}
	return nil, fmt.Errorf("get element failed to locate %s: %w", state.ElementId, errors.ErrElementNotFound)
}

// HasValidProcess - checks for a valid process and instance for a workflow process and instance ids
func (s *Operations) HasValidProcess(ctx context.Context, processInstanceId, executionId string) (*model.ProcessInstance, *model.Execution, error) {
	execution, err := s.HasValidExecution(ctx, executionId)
	if err != nil {
		return nil, nil, err
	}
	pi, err := s.GetProcessInstance(ctx, processInstanceId)
	if errors2.Is(err, errors.ErrProcessInstanceNotFound) {
		return nil, nil, fmt.Errorf("orphaned activity: %w", err)
	}
	if err != nil {
		return nil, nil, err
	}
	return pi, execution, err
}

// HasValidExecution checks to see whether an execution exists for the executionId
func (s *Operations) HasValidExecution(ctx context.Context, executionId string) (*model.Execution, error) {
	execution, err := s.GetExecution(ctx, executionId)
	if errors2.Is(err, errors.ErrExecutionNotFound) {
		return nil, fmt.Errorf("orphaned activity: %w", err)
	}
	if err != nil {
		return nil, err
	}
	return execution, err
}

func remove[T comparable](slice []T, member T) []T {
	for i, v := range slice {
		if v == member {
			slice = append(slice[:i], slice[i+1:]...)
			break
		}
	}
	return slice
}

// closeUserTask removes a completed user task.
func (s *Operations) closeUserTask(ctx context.Context, trackingID string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	job := &model.WorkflowState{}
	if err := common.LoadObj(ctx, nsKVs.Job, trackingID, job); err != nil {
		return fmt.Errorf("load job when closing user task: %w", err)
	}

	// TODO: abstract group and user names, return all errors
	var retErr error
	allIDs := append(job.Owners, job.Groups...)
	for _, i := range allIDs {
		if err := common.UpdateObj(ctx, nsKVs.WfUserTasks, i, &model.UserTasks{}, func(msg *model.UserTasks) (*model.UserTasks, error) {
			msg.Id = remove(msg.Id, trackingID)
			return msg, nil
		}); err != nil {
			retErr = fmt.Errorf("faiiled to update user tasks object when closing user task: %w", err)
		}
	}
	return retErr
}

func (s *Operations) openUserTask(ctx context.Context, owner string, id string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.UpdateObj(ctx, nsKVs.WfUserTasks, owner, &model.UserTasks{}, func(msg *model.UserTasks) (*model.UserTasks, error) {
		msg.Id = append(msg.Id, id)
		return msg, nil
	}); err != nil {
		return fmt.Errorf("update user task object: %w", err)
	}
	return nil
}

// GetUserTaskIDs gets a list of tasks given an owner.
func (s *Operations) GetUserTaskIDs(ctx context.Context, owner string) (*model.UserTasks, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	ut := &model.UserTasks{}
	if err := common.LoadObj(ctx, nsKVs.WfUserTasks, owner, ut); err != nil {
		return nil, fmt.Errorf("load user task IDs: %w", err)
	}
	return ut, nil
}

// OwnerID gets a unique identifier for a task owner.
func (s *Operations) OwnerID(ctx context.Context, name string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if name == "" {
		name = "AnyUser"
	}
	nm, err := nsKVs.OwnerID.Get(ctx, name)
	if err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return "", fmt.Errorf("get owner id: %w", err)
	}
	if nm == nil {
		id := ksuid.New().String()
		if _, err := nsKVs.OwnerID.Put(ctx, name, []byte(id)); err != nil {
			return "", fmt.Errorf("write owner ID: %w", err)
		}
		if _, err = nsKVs.OwnerName.Put(ctx, id, []byte(name)); err != nil {
			return "", fmt.Errorf("store owner name in kv: %w", err)
		}
		return id, nil
	}
	return string(nm.Value()), nil
}

// OwnerName retrieves an owner name given an ID.
func (s *Operations) OwnerName(ctx context.Context, id string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	nm, err := nsKVs.OwnerName.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get owner name for id: %w", err)
	}
	return string(nm.Value()), nil
}

// CreateProcessInstance creates a new instance of a process and attaches it to the workflow instance.
func (s *Operations) CreateProcessInstance(ctx context.Context, executionId string, parentProcessID string, parentElementID string, processId string, workflowName string, workflowId string, headers []byte) (*model.ProcessInstance, error) {
	id := ksuid.New().String()
	pi := &model.ProcessInstance{
		ProcessInstanceId: id,
		ProcessId:         processId,
		ParentProcessId:   &parentProcessID,
		ParentElementId:   &parentElementID,
		ExecutionId:       executionId,
		Headers:           headers,
	}

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	wfi, err := s.GetExecution(ctx, executionId)
	//wf, err := s.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("create process instance failed to get workflow: %w", err)
	}
	pi.WorkflowName = workflowName
	pi.WorkflowId = workflowId
	pi.Headers = headers
	err = common.SaveObj(ctx, nsKVs.WfProcessInstance, pi.ProcessInstanceId, pi)
	if err != nil {
		return nil, fmt.Errorf("create process instance failed to save process instance: %w", err)
	}

	err = common.UpdateObj(ctx, nsKVs.WfExecution, executionId, wfi, func(v *model.Execution) (*model.Execution, error) {
		v.ProcessInstanceId = append(v.ProcessInstanceId, pi.ProcessInstanceId)
		return v, nil
	})
	if err != nil {
		return nil, fmt.Errorf("create process instance failed to update workflow instance: %w", err)
	}
	return pi, nil
}

// GetProcessInstance returns a process instance for a given process ID
func (s *Operations) GetProcessInstance(ctx context.Context, processInstanceID string) (*model.ProcessInstance, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	pi := &model.ProcessInstance{}
	err = common.LoadObj(ctx, nsKVs.WfProcessInstance, processInstanceID, pi)
	if errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("get process instance failed to load instance: %w", errors.ErrProcessInstanceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get process instance failed to load instance: %w", err)
	}
	return pi, nil
}

// DestroyProcessInstance deletes a process instance and removes the workflow instance dependent on all process instances being satisfied.
func (s *Operations) DestroyProcessInstance(ctx context.Context, state *model.WorkflowState, processInstanceId string, executionId string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	e := &model.Execution{}
	err = common.UpdateObj(ctx, nsKVs.WfExecution, executionId, e, func(v *model.Execution) (*model.Execution, error) {
		v.ProcessInstanceId = remove(v.ProcessInstanceId, processInstanceId)
		return v, nil
	})
	if len(e.ProcessInstanceId) == 0 {
		if err := common.Delete(ctx, nsKVs.WfExecution, executionId); err != nil {
			return fmt.Errorf("destroy process instance delete execution: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("destroy process instance failed to update execution: %w", err)
	}
	err = common.Delete(ctx, nsKVs.WfProcessInstance, processInstanceId)
	// TODO: Key not found
	if err != nil {
		return fmt.Errorf("destroy process instance failed to delete process instance: %w", err)
	}

	if err := s.PublishWorkflowState(ctx, messages.WorkflowProcessTerminated, state); err != nil {
		return fmt.Errorf("destroy process instance failed initiaite completing workflow instance: %w", err)
	}
	if len(e.ProcessInstanceId) == 0 {
		if err := s.PublishWorkflowState(ctx, messages.WorkflowExecutionComplete, state); err != nil {
			return fmt.Errorf("destroy process instance failed initiaite completing workflow instance: %w", err)
		}
	}
	return nil
}

// DeprecateTaskSpec deprecates one or more task specs by ID.
func (s *Operations) DeprecateTaskSpec(ctx context.Context, uid []string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	for _, u := range uid {
		ts := &model.TaskSpec{}
		err := common.UpdateObj(ctx, nsKVs.WfTaskSpec, u, ts, func(v *model.TaskSpec) (*model.TaskSpec, error) {
			if v.Behaviour == nil {
				v.Behaviour = &model.TaskBehaviour{}
			}
			v.Behaviour.Deprecated = true
			return v, nil
		})
		if err != nil {
			return fmt.Errorf("deprecate task spec update task: %w", err)
		}
	}
	return nil
}

func (s *Operations) populateMetadata(wf *model.Workflow) {
	for _, process := range wf.Process {
		process.Metadata = &model.Metadata{}
		for _, elem := range process.Elements {
			if elem.Type == element.TimedStartEvent {
				process.Metadata.TimedStart = true
			}
		}
	}
}

func (s *Operations) validateUniqueMessageNames(ctx context.Context, wf *model.Workflow) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	existingMessageTypes := map[string]struct{}{}
	for _, msgType := range wf.Messages {
		messageReceivers := &model.MessageReceivers{}
		err := common.LoadObj(ctx, nsKVs.WfMsgTypes, msgType.Name, messageReceivers)

		if err == nil && messageReceivers.AssociatedWorkflowName != wf.Name {
			existingMessageTypes[msgType.Name] = struct{}{}
		}
	}
	existingMessages := strings.Join(maps2.Keys(existingMessageTypes), ",")

	if len(existingMessageTypes) > 0 {
		return fmt.Errorf("these messages already exist for other workflows: %s", existingMessages)
	}

	return nil
}

// CheckProcessTaskDeprecation checks if all the tasks in a process have not been deprecated.
func (s *Operations) CheckProcessTaskDeprecation(ctx context.Context, workflow *model.Workflow, processId string) error {
	pr := workflow.Process[processId]
	for _, el := range pr.Elements {
		if el.Type == element.ServiceTask {
			st, err := s.GetTaskSpecByUID(ctx, *el.Version)
			if err != nil {
				return fmt.Errorf("get task spec by uid: %w", err)
			}
			if st.Behaviour != nil && st.Behaviour.Deprecated {
				return fmt.Errorf("process %s contains deprecated task %s", processId, el.Execute)
			}
		}
	}
	return nil
}

// ListTaskSpecUIDs lists UIDs of active (and optionally deprecated) tasks specs.
func (s *Operations) ListTaskSpecUIDs(ctx context.Context, deprecated bool) ([]string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	ret := make([]string, 0, 50)
	vers, err := nsKVs.WfTaskSpecVer.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get task spec version keys: %w", err)
	}
	for _, v := range vers {
		ver := &model.TaskSpecVersions{}
		err := common.LoadObj(ctx, nsKVs.WfTaskSpecVer, v, ver)
		if err != nil {
			return nil, fmt.Errorf("get task spec version: %w", err)
		}
		latestVer := ver.Id[len(ver.Id)-1]
		latest, err := s.GetTaskSpecByUID(ctx, latestVer)
		if err != nil {
			return nil, fmt.Errorf("get task spec: %w", err)
		}
		if deprecated || (latest.Behaviour != nil && !latest.Behaviour.Deprecated) {
			ret = append(ret, latestVer)
		}
	}
	return ret, nil
}

// GetProcessIdFor retrieves the processId that a begun by a message start event
func (s *Operations) GetProcessIdFor(ctx context.Context, startEventMessageName string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	messageReceivers := &model.MessageReceivers{}
	err = common.LoadObj(ctx, nsKVs.WfMsgTypes, startEventMessageName, messageReceivers)

	if errors2.Is(err, jetstream.ErrKeyNotFound) || messageReceivers.MessageReceiver == nil || len(messageReceivers.MessageReceiver) == 0 {
		return "", fmt.Errorf("no message receivers for %q: %w", startEventMessageName, err)
	}

	for _, recvr := range messageReceivers.MessageReceiver {
		if recvr.ProcessIdToStart != "" {
			return recvr.ProcessIdToStart, nil
		}
	}

	return "", fmt.Errorf("no message receivers for %q: %w", startEventMessageName, err)
}

// Heartbeat saves a client status to the client KV.
func (s *Operations) Heartbeat(ctx context.Context, req *model.HeartbeatRequest) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.SaveObj(ctx, nsKVs.WfClients, req.Host+"-"+req.Id, req); err != nil {
		return fmt.Errorf("heartbeat write to kv: %w", err)
	}
	return nil
}

// Log publishes LogRequest to WorkflowTelemetry Logs subject
func (s *Operations) Log(ctx context.Context, req *model.LogRequest) error {
	if err := common.PublishObj(ctx, s.natsService.Conn, messages.WorkflowTelemetryLog, req, nil); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	return nil
}

// DeleteNamespace deletes the key-value store for the specified namespace in SHAR.
// It iterates over all the key-value stores and deletes them one by one.
// The function returns nil if all key-value stores are successfully deleted.
func (s *Operations) DeleteNamespace(ctx context.Context, ns string) error {
	lister := s.natsService.Js.KeyValueStores(ctx)
	toDelete := make([]string, 0)
	for i := range lister.Status() {
		toDelete = append(toDelete, i.Bucket())
	}
	for _, bucket := range toDelete {
		err := s.natsService.Js.DeleteKeyValue(ctx, bucket)
		if err != nil {
			return fmt.Errorf("delete key value %s: %w", bucket, err)
		}
	}
	return nil
}

// ListExecutableProcesses returns a list of all the executable processes in SHAR.
// It retrieves the current SHAR namespace from the context and fetches the workflow versions
// for that namespace from the key-value store. It then iterates through each workflow version
// and loads the corresponding workflow. For each process in the workflow, it creates a
// ListExecutableProcessesItem object and populates it with the process name, workflow name,
// and the executable start parameters obtained from the workflow's start events. It sends
// each ListExecutableProcessesItem object to the wch channel.
//
// Parameters:
// - ctx: The context containing the SHAR namespace.
// - wch: The channel for sending the list of executable processes.
// - errs: The channel for sending any errors that occur.
//
// Returns: Nothing. Errors are sent to the errs channel if encountered.
func (s *Operations) ListExecutableProcesses(ctx context.Context, wch chan<- *model.ListExecutableProcessesItem, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}
	wfVerKeys, err := nsKVs.WfVersion.Keys(ctx)
	if err != nil {
		errs <- fmt.Errorf("get workflow keys: %w", err)
		return
	}
	for _, k := range wfVerKeys {
		wfv := &model.WorkflowVersions{}
		if err := common.LoadObj(ctx, nsKVs.WfVersion, k, wfv); err != nil {
			if errors2.Is(err, jetstream.ErrKeyNotFound) {
				continue
			}
			errs <- fmt.Errorf("load workflow versions: %w", err)
		}
		latest := wfv.Version[len(wfv.Version)-1]
		wf := &model.Workflow{}
		if err := common.LoadObj(ctx, nsKVs.Wf, latest.Id, wf); err != nil {
			if errors2.Is(err, jetstream.ErrKeyNotFound) {
				continue
			}
			errs <- fmt.Errorf("load workflow: %w", err)
		}
		for _, p := range wf.Process {
			ret := &model.ListExecutableProcessesItem{Parameter: make([]*model.ExecutableStartParameter, 0)}
			ret.ProcessId = p.Id
			ret.WorkflowName = wf.Name
			startParam := make(map[string]struct{})
			for _, el := range p.Elements {
				if el.Type == element.StartEvent {
					for _, ex := range el.OutputTransform {
						v, err := expression.GetVariables(ctx, s.exprEngine, ex)
						if err != nil {
							errs <- fmt.Errorf("get expression variables: %w", err)
							return
						}
						for _, n := range v {
							startParam[n.Name] = struct{}{}
						}
					}
				}
				for n := range startParam {
					ret.Parameter = append(ret.Parameter, &model.ExecutableStartParameter{Name: n})
				}
			}
			wch <- ret
		}
	}
}

// StartJob launches a user/service task
func (s *Operations) StartJob(ctx context.Context, subject string, job *model.WorkflowState, el *model.Element, v []byte, opts ...PublishOpt) error {
	job.Execute = &el.Execute
	// el.Version is only used for versioned tasks such as service tasks
	if el.Version != nil {
		job.ExecuteVersion = *el.Version
	}
	// skip if this job type requires no input transformation
	if el.Type != element.MessageIntermediateCatchEvent {
		job.Vars = nil
		if err := vars.InputVars(ctx, s.exprEngine, v, &job.Vars, el); err != nil {
			return &errors.ErrWorkflowFatal{Err: fmt.Errorf("start job failed to get input variables: %w", err)}
		}
	}
	// if this is a user task, find out who can perform it
	if el.Type == element.UserTask {
		vx, err := vars.Decode(ctx, v)
		if err != nil {
			return &errors.ErrWorkflowFatal{Err: fmt.Errorf("start job failed to decode input variables: %w", err)}
		}

		owners, err := s.evaluateOwners(ctx, el.Candidates, vx)
		if err != nil {
			return &errors.ErrWorkflowFatal{Err: fmt.Errorf("start job: evaluate owners: %w", err)}
		}
		groups, err := s.evaluateOwners(ctx, el.CandidateGroups, vx)
		if err != nil {
			return &errors.ErrWorkflowFatal{Err: fmt.Errorf("start job failed to evaluate groups: %w", err)}
		}

		job.Owners = owners
		job.Groups = groups
	}
	if el.Type == element.ServiceTask {
		def, err := s.GetTaskSpecByUID(ctx, *el.Version)
		if err != nil {
			return fmt.Errorf("get task spec by uid: %w", err)
		}
		if def.Behaviour != nil && def.Behaviour.Mock {
			subject += ".Mock"
		}
	}

	// create the job
	_, err := s.CreateJob(ctx, job)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	/*
		//Save Iterator State
		common.SaveLargeObj

		// Multi-instance
		if el.Iteration != nil {
			if el.Iteration.Execute == model.ThreadingType_Sequential {
				// Launch as usual, just with iteration parameters
				seqVars, err := vars.Decode(ctx, job.Vars)
				if err != nil {
					return &errors.ErrWorkflowFatal{Err: fmt.Errorf("start job failed to decode input variables: %w", err)}
				}
				collection, ok := seqVars[el.Iteration.Collection]
				if !ok {
					return &errors.ErrWorkflowFatal{Err: fmt.Errorf("start job failed to decode input variables: %w", err)}
				}
				seqVars[el.Iteration.Iterator] = getCollectionIndex[collection]
			} else if model.ThreadingType_Parallel {

			}
		}
	*/
	// Single instance launch
	if el.Iteration == nil {
		if err := s.PublishWorkflowState(ctx, subj.NS(subject, subj.GetNS(ctx)), job, opts...); err != nil {
			return fmt.Errorf("start job failed to publish: %w", err)
		}
		if err := s.RecordHistoryJobExecute(ctx, job); err != nil {
			return fmt.Errorf("job start failed to record history: %w", err)
		}
		// finally tell the engine that the job is ready for a client
		return nil
	}
	return nil
}

// evaluateOwners builds a list of groups
func (s *Operations) evaluateOwners(ctx context.Context, owners string, vars model.Vars) ([]string, error) {
	jobGroups := make([]string, 0)
	groups, err := expression.Eval[interface{}](ctx, s.exprEngine, owners, vars)
	if err != nil {
		return nil, &errors.ErrWorkflowFatal{Err: err}
	}
	switch groups := groups.(type) {
	case string:
		jobGroups = append(jobGroups, groups)
	case []string:
		jobGroups = append(jobGroups, groups...)
	}
	for i, v := range jobGroups {
		id, err := s.OwnerID(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("evaluate owners failed to get owner ID: %w", err)
		}
		jobGroups[i] = id
	}
	return jobGroups, nil
}

// GetCompensationInputVariables is a method of the Operations struct that retrieves the original input variables
// for a specific process instance and tracking ID. It returns the variables in byte array format.
func (s *Operations) GetCompensationInputVariables(ctx context.Context, processInstanceId string, trackingId string) ([]byte, error) {
	entry, err := s.GetProcessHistoryItem(ctx, processInstanceId, trackingId, model.ProcessHistoryType_jobExecute)
	if err != nil {
		return nil, fmt.Errorf("get compensation history entry: %w", err)
	}
	return entry.Vars, nil
}

// GetCompensationOutputVariables is a method of the Operations struct that retrieves the original output variables
// of a compensation history entry for a specific process instance and tracking ID.
// It returns the output variables as a byte array.
func (s *Operations) GetCompensationOutputVariables(ctx context.Context, processInstanceId string, trackingId string) ([]byte, error) {
	entry, err := s.GetProcessHistoryItem(ctx, processInstanceId, trackingId, model.ProcessHistoryType_jobComplete)
	if err != nil {
		return nil, fmt.Errorf("get compensation history entry: %w", err)
	}
	return entry.Vars, nil
}

// HandleWorkflowError handles a workflow error by looking up the error definitions in the workflow,
// determining the appropriate action to take, and publishing the necessary workflow state updates.
// It returns an error if there was an issue retrieving the workflow definition, if the workflow
// doesn't support the specified error code.
func (c *Operations) HandleWorkflowError(ctx context.Context, errorCode string, inVars []byte, state *model.WorkflowState) error {
	// Get the workflow, so we can look up the error definitions
	wf, err := c.GetWorkflow(ctx, state.WorkflowId)
	if err != nil {
		return fmt.Errorf("get workflow definition for handle workflow error: %w", err)
	}

	// Get the element corresponding to the state
	els := common.ElementTable(wf)

	// Get the current element
	el := els[state.ElementId]

	// Get the errors supported by this workflow
	var found bool
	wfErrs := make(map[string]*model.Error)
	for _, v := range wf.Errors {
		if v.Code == errorCode {
			found = true
		}
		wfErrs[v.Id] = v
	}
	if !found {
		werr := &errors.ErrWorkflowFatal{Err: fmt.Errorf("workflow-fatal: can't handle error code %s as the workflow doesn't support it: %w", errorCode, errors.ErrWorkflowErrorNotFound)}
		// TODO: This always assumes service task.  Wrong!
		if err := c.PublishWorkflowState(ctx, subj.NS(messages.WorkflowJobServiceTaskAbort, subj.GetNS(ctx)), state); err != nil {
			return fmt.Errorf("cencel state: %w", werr)
		}

		cancelState := common.CopyWorkflowState(state)
		cancelState.State = model.CancellationState_errored
		cancelState.Error = &model.Error{
			Id:   "UNKNOWN",
			Name: "UNKNOWN",
			Code: errorCode,
		}
		if err := c.CancelProcessInstance(ctx, cancelState); err != nil {
			return fmt.Errorf("cancel workflow instance: %w", werr)
		}
		return fmt.Errorf("workflow halted: %w", werr)
	}

	// Get the errors associated with this element
	var errDef *model.Error
	var caughtError *model.CatchError
	for _, v := range el.Errors {
		wfErr := wfErrs[v.ErrorId]
		if errorCode == wfErr.Code {
			errDef = wfErr
			caughtError = v
			break
		}
	}

	if errDef == nil {
		return errors.ErrUnhandledWorkflowError
	}

	// Get the target workflow activity
	target := els[caughtError.Target]

	activityStart, err := c.GetProcessHistoryItem(ctx, state.ProcessInstanceId, common.TrackingID(state.Id).Pop().ID(), model.ProcessHistoryType_activityExecute)
	if err != nil {
		return fmt.Errorf("get activity start state for handle workflow error: %w", err)
	}
	if err := vars.OutputVars(ctx, c.exprEngine, inVars, &activityStart.Vars, caughtError.OutputTransform); err != nil {
		return &errors.ErrWorkflowFatal{Err: err}
	}
	if err := c.PublishWorkflowState(ctx, messages.WorkflowTraversalExecute, &model.WorkflowState{
		ElementType: target.Type,
		ElementId:   target.Id,
		ElementName: target.Name,
		WorkflowId:  state.WorkflowId,
		//WorkflowInstanceId: state.WorkflowInstanceId,
		ExecutionId:       state.ExecutionId,
		Id:                common.TrackingID(state.Id).Pop().Pop(),
		Vars:              activityStart.Vars,
		WorkflowName:      wf.Name,
		ProcessInstanceId: state.ProcessInstanceId,
		ProcessId:         state.ProcessId,
	}); err != nil {
		log := logx.FromContext(ctx)
		log.Error("publish workflow state", "error", err)
		return fmt.Errorf("publish traversal for handle workflow error: %w", err)
	}
	// TODO: This always assumes service task.  Wrong!
	if err := c.PublishWorkflowState(ctx, messages.WorkflowJobServiceTaskAbort, &model.WorkflowState{
		ElementType: target.Type,
		ElementId:   target.Id,
		ElementName: target.Name,
		WorkflowId:  state.WorkflowId,
		//WorkflowInstanceId: state.WorkflowInstanceId,
		ExecutionId:       state.ExecutionId,
		Id:                state.Id,
		Vars:              state.Vars,
		WorkflowName:      wf.Name,
		ProcessInstanceId: state.ProcessInstanceId,
		ProcessId:         state.ProcessId,
	}); err != nil {
		log := logx.FromContext(ctx)
		log.Error("publish workflow state", "error", err)
		// We have already traversed so returning an error here would be incorrect.
		// It would force reprocessing and possibly double traversing
		// TODO: develop an idempotent behaviour based upon hash nats message ids + deduplication
		return fmt.Errorf("publish abort task for handle workflow error: %w", err)
	}
	return nil
}

// GetFatalErrors queries the fatal error KV with a key prefix of the format <workflowName>.<executionId>.<processInstanceId>.
// and sends the results down the provided fatalErrs channel.
func (s *Operations) GetFatalErrors(ctx context.Context, keyPrefix string, fatalErrs chan<- *model.FatalError, errs chan<- error) {
	ns := subj.GetNS(ctx)
	kvs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get kvs for fatal errors: %w", err)
		return
	}

	kvFatalError := kvs.WfFatalError
	matchingKeys, err := common.KeyPrefixSearch(ctx, s.natsService.Js, kvFatalError, keyPrefix, common.KeyPrefixResultOpts{Sort: true, ExcludeDeleted: true})
	if err != nil {
		errs <- fmt.Errorf("get fatal errors: %w", err)
		return
	}

	for _, matchingKey := range matchingKeys {
		fatalErr := &model.FatalError{}
		err := common.LoadObj(ctx, kvFatalError, matchingKey, fatalErr)
		if err != nil {
			errs <- fmt.Errorf("load fatal errors: %w", err)
			return
		}
		fatalErrs <- fatalErr
	}
}

// RetryActivity publishes the state from a prior FatalError to attempt a retry
func (s *Operations) RetryActivity(ctx context.Context, state *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	kvs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get kvs for fatal errors: %w", err)
	}

	k := fatalErrorKey(state.WorkflowName, state.WorkflowId, state.ExecutionId, state.ProcessInstanceId, state.ElementId)
	fatalErr := &model.FatalError{}
	err = common.LoadObj(ctx, kvs.WfFatalError, k, fatalErr)
	if err != nil {
		return fmt.Errorf("unable to find fatal errored workflow to retry: %w", err)
	}
	fatalErr.WorkflowState.Vars = state.Vars

	if err := s.PublishWorkflowState(ctx, subj.NS(messages.WorkflowJobRetry, subj.GetNS(ctx)), fatalErr.WorkflowState); err != nil {
		return fmt.Errorf("failed publishing to job retry: %w", err)
	}

	return nil
}

// PersistFatalError saves a fatal error to the kv
func (s *Operations) PersistFatalError(ctx context.Context, fatalError *model.FatalError) (bool, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return false, fmt.Errorf("persistFatalError get KVs for ns %s: %w", ns, err)
	}
	workFlowName := fatalError.WorkflowState.WorkflowName
	k := fatalErrorKey(workFlowName, fatalError.WorkflowState.WorkflowId, fatalError.WorkflowState.ExecutionId, fatalError.WorkflowState.ProcessInstanceId, fatalError.WorkflowState.ElementId)
	if err := common.SaveObj(ctx, nsKVs.WfFatalError, k, fatalError); err != nil {
		return false, fmt.Errorf("save fatal error: %w", err)
	}

	return true, nil
}

func fatalErrorKey(workFlowName, workflowId, executionId, processInstanceId, elementId string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", workFlowName, workflowId, executionId, processInstanceId, elementId)
}

// TearDownWorkflow removes any state associated with a fatal errored workflow
func (s *Operations) TearDownWorkflow(ctx context.Context, state *model.WorkflowState) (bool, error) {
	switch state.ElementType {
	case element.ServiceTask, element.MessageIntermediateCatchEvent:
		// These are both job based, so we need to abort the job
		if err := s.PublishWorkflowState(ctx, messages.WorkflowJobServiceTaskAbort, state); err != nil {
			log := logx.FromContext(ctx)
			log.Error("publish workflow state for service task abort", "error", err)
			return false, fmt.Errorf("publish abort task for handle fatal workflow error: %w", err)
		}
	default:
		//add more cases as more element types need to be supported for cleaning down fatal errs
		//just remove the process/executions below
	}

	//get the execution
	execution, err2 := s.GetExecution(ctx, state.ExecutionId)
	if err2 != nil {
		return false, fmt.Errorf("error retrieving execution when processing fatal err: %w", err2)
	}
	//loop over the process instance ids to tear them down
	for _, processInstanceId := range execution.ProcessInstanceId {
		state.State = model.CancellationState_terminated
		if err := s.DestroyProcessInstance(ctx, state, processInstanceId, execution.ExecutionId); err != nil {
			log := logx.FromContext(ctx)
			log.Error("failed destroying process instance", "err", err)
		}
	}
	return true, nil
}

// DeleteFatalError removes the fatal error for a given workflow state
func (s *Operations) DeleteFatalError(ctx context.Context, state *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	kvsFor, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get kvs for delete fatal error: %w", err)
	}

	err = common.Delete(ctx, kvsFor.WfFatalError, fatalErrorKey(state.WorkflowName, state.WorkflowId, state.ExecutionId, state.ProcessInstanceId, state.ElementId))
	if err != nil {
		return fmt.Errorf("delete fatal error: %w", err)
	}

	return nil
}
