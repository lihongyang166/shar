package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/client/parser"
	task2 "gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/logx"
	middleware2 "gitlab.com/shar-workflow/shar/common/middleware"
	ns "gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/setup"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/structs"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/task"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	version2 "gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/common/workflow"
	api2 "gitlab.com/shar-workflow/shar/internal/client/api"
	"gitlab.com/shar-workflow/shar/internal/common/client"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"google.golang.org/protobuf/proto"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// HeartBeatInterval defines the time between client heartbeats.
const HeartBeatInterval = 1 * time.Second

type jobClient struct {
	cl                *Client
	jobID             string
	activityID        string
	processInstanceID string
	executionID       string
	originalInputs    map[string]interface{}
	originalOutputs   map[string]interface{}
}

// Log logs to the span related to this jobClient instance.
// Deprecated: Use Logger or LoggerWith methods to obtain a logger instance.
func (c *jobClient) Log(ctx context.Context, level slog.Level, message string, attrs map[string]string) error {
	return c.cl.clientLog(ctx, c.jobID, level, message, attrs)
}

func (c *jobClient) Logger() *slog.Logger {
	return c.LoggerWith(slog.Default())
}

func (c *jobClient) LoggerWith(logger *slog.Logger) *slog.Logger {
	return slogWith(logger, c.executionID, c.processInstanceID, c.activityID, c.jobID)
}

func (c *jobClient) OriginalVars() (inputVars map[string]interface{}, outputVars map[string]interface{}) {
	inputVars = c.originalInputs
	outputVars = c.originalOutputs
	return
}

func slogWith(logger *slog.Logger, executionID, processInstanceID, activityID, jobID string) *slog.Logger {
	return logger.With(
		slog.String(keys.ExecutionID, executionID),
		slog.String(keys.ProcessInstanceID, processInstanceID),
		slog.String(keys.ActivityID, activityID),
		slog.String(keys.JobID, jobID),
	)
}

type messageClient struct {
	cl                *Client
	jobID             string
	activityID        string
	processInstanceID string
	executionID       string
}

// SendMessage sends a Workflow Message into the SHAR engine
func (c *messageClient) SendMessage(ctx context.Context, name string, key any, vars model.Vars) error {
	return c.cl.SendMessage(ctx, name, key, vars)
}

// Log logs to the span related to this messageClient instance.
// Deprecated: Use Logger or LoggerWith methods to obtain a logger instance.
func (c *messageClient) Log(ctx context.Context, level slog.Level, message string, attrs map[string]string) error {
	return c.cl.clientLog(ctx, c.jobID, level, message, attrs)
}

func (c *messageClient) Logger() *slog.Logger {
	return c.LoggerWith(slog.Default())
}

func (c *messageClient) LoggerWith(logger *slog.Logger) *slog.Logger {
	return slogWith(logger, c.executionID, c.processInstanceID, c.activityID, c.jobID)
}

// Client implements a SHAR client capable of listening for service task activations, listening for Workflow Messages, and integrating with the API
type Client struct {
	id                              string
	host                            string
	js                              jetstream.JetStream
	SvcTasks                        map[string]*task2.FnDef
	con                             *nats.Conn
	MsgSender                       map[string]*task2.FnDef
	storageType                     jetstream.StorageType
	ns                              string
	listenTasks                     map[string]struct{}
	msgListenTasks                  map[string]struct{}
	proCompleteTasks                map[string]*task2.FnDef
	txJS                            jetstream.JetStream
	txCon                           *nats.Conn
	concurrency                     int
	ExpectedCompatibleServerVersion *version.Version
	ExpectedServerVersion           *version.Version
	version                         *version.Version
	noRecovery                      bool
	closer                          chan struct{}
	shutdownOnce                    sync.Once
	sig                             chan os.Signal
	processing                      atomic.Int64
	noOSSig                         bool
	telemetryConfig                 telemetry.Config
	SendMiddleware                  []middleware2.Send
	ReceiveMiddleware               []middleware2.Receive
}

// New creates a new SHAR client instance
func New(option ...ConfigurationOption) *Client {
	ver, err := version.NewVersion(version2.Version)
	if err != nil {
		panic(err)
	}
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	c := &Client{
		id:                              ksuid.New().String(),
		host:                            host,
		storageType:                     jetstream.FileStorage,
		SvcTasks:                        make(map[string]*task2.FnDef),
		MsgSender:                       make(map[string]*task2.FnDef),
		listenTasks:                     make(map[string]struct{}),
		msgListenTasks:                  make(map[string]struct{}),
		proCompleteTasks:                make(map[string]*task2.FnDef),
		ns:                              ns.Default,
		concurrency:                     10,
		version:                         ver,
		ExpectedCompatibleServerVersion: upgrader.GetCompatibleVersion(),
		closer:                          make(chan struct{}),
		sig:                             make(chan os.Signal),
		telemetryConfig:                 telemetry.Config{Enabled: false},
		SendMiddleware:                  make([]middleware2.Send, 0),
		ReceiveMiddleware:               make([]middleware2.Receive, 0),
	}
	for _, i := range option {
		i.configure(c)
	}
	return c
}

// Dial instructs the client to connect to a NATS server.
func (c *Client) Dial(ctx context.Context, natsURL string, opts ...nats.Option) error {

	if c.telemetryConfig.Enabled {
		c.SendMiddleware = append(c.SendMiddleware,
			telemetry.CtxSpanToNatsMsgMiddleware(),
		)
		c.ReceiveMiddleware = append(c.ReceiveMiddleware,
			telemetry.NatsMsgToCtxWithSpanMiddleware(),
		)
	}

	n, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return c.clientErr(context.Background(), err)
	}
	if err := common.CheckVersion(ctx, n); err != nil {
		return fmt.Errorf("check NATS version: %w", err)
	}
	txnc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return c.clientErr(context.Background(), err)
	}
	js, err := jetstream.New(n)
	if err != nil {
		return c.clientErr(context.Background(), err)
	}
	txJS, err := jetstream.New(txnc)
	if err != nil {
		return c.clientErr(context.Background(), err)
	}
	c.js = js
	c.txJS = txJS
	c.con = n
	c.txCon = txnc
	_, err = c.GetServerVersion(ctx)
	if err != nil {
		return fmt.Errorf("server version: %w", err)
	}

	cdef := &jetstream.ConsumerConfig{
		Durable:       "ProcessTerminateConsumer_" + c.ns,
		Description:   "Processing queue for process end",
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		FilterSubject: subj.NS(messages.WorkflowProcessTerminated, c.ns),
		MaxAckPending: 65535,
	}
	if err := setup.EnsureConsumer(ctx, js, "WORKFLOW", *cdef, false, c.storageType); err != nil {
		return fmt.Errorf("setting up end event queue")
	}

	return nil
}

// DeprecateTaskSpec deprecates a task spec by name.
func (c *Client) DeprecateTaskSpec(ctx context.Context, name string) error {
	req := &model.DeprecateServiceTaskRequest{
		Name: name,
	}
	res := &model.DeprecateServiceTaskResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIDeprecateServiceTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	if !res.Success {
		return &ErrTaskInUse{Err: fmt.Errorf("attempt to deprectate a task in use"), Usage: res.Usage}
	}
	return nil
}

// StoreTask stores a task specification, and assigns the generated ID to the task metadata.
func (c *Client) StoreTask(ctx context.Context, spec *model.TaskSpec) error {
	id, err := c.registerServiceTask(ctx, spec)
	if err != nil {
		return fmt.Errorf("store task: %w", err)
	}
	spec.Metadata.Uid = id
	slog.Info("stored task", "type", spec.Metadata.Type, "id", spec.Metadata.Uid)
	return nil
}

// RegisterTaskFunction registers a service task function.
// If the service task spec has no UID then it will be calculated and written to the Metadata.Uid field.
func (c *Client) RegisterTaskFunction(ctx context.Context, spec *model.TaskSpec, fn task2.ServiceFn) error {
	def := &task2.FnDef{
		Type: task2.ExecutionTypeVars,
	}
	if fn != nil {
		def.Fn = fn
	}
	if err := c.registerTaskFunction(ctx, spec, def); err != nil {
		return fmt.Errorf("register task function: %w", err)
	}
	return nil
}

func (c *Client) registerTaskFunction(ctx context.Context, spec *model.TaskSpec, def *task2.FnDef) error {
	if spec.Metadata == nil {
		return fmt.Errorf("task metadata is nil")
	}
	if spec.Metadata.Uid == "" {
		uid, err := task.CreateUID(spec)
		if err != nil {
			return fmt.Errorf("create uid: %w", err)
		}
		spec.Metadata.Uid = uid
	}
	if def.Fn != nil {
		if _, ok := c.SvcTasks[spec.Metadata.Uid]; ok {
			return fmt.Errorf("service task '%s' already registered: %w", spec.Metadata.Type, errors2.ErrServiceTaskAlreadyRegistered)
		}
		c.SvcTasks[spec.Metadata.Uid] = def
		c.listenTasks[spec.Metadata.Uid] = struct{}{}
	}
	return nil
}

// RegisterMessageSender registers a function that requires support for sending Workflow Messages
func (c *Client) RegisterMessageSender(ctx context.Context, workflowName string, messageName string, sender task2.SenderFn) error {
	if _, ok := c.MsgSender[workflowName+"_"+messageName]; ok {
		return fmt.Errorf("message sender '%s' already registered: %w", messageName, errors2.ErrMessageSenderAlreadyRegistered)
	}
	c.MsgSender[workflowName+"_"+messageName] = &task2.FnDef{
		Type: task2.ExecutionTypeVars,
		Fn:   sender,
	}
	c.msgListenTasks[workflowName+"_"+messageName] = struct{}{}
	return nil
}

// Listen starts processing the client message queues.
func (c *Client) Listen(ctx context.Context) error {
	if err := c.listen(ctx); err != nil {
		return c.clientErr(ctx, err)
	}
	c.listenTerm(ctx)
	if err := c.listenProcessTerminate(ctx); err != nil {
		return c.clientErr(ctx, err)
	}
	if err := c.startHeart(ctx); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

func (c *Client) listen(ctx context.Context) error {
	ctx = context.WithValue(ctx, ctxkey.SharNamespace, c.ns)
	tasks := make(map[string]string)
	for i := range c.listenTasks {
		tasks[i] = subj.NS(messages.WorkflowJobServiceTaskExecute+"."+i, c.ns)
	}
	for i := range c.msgListenTasks {
		tasks[i] = subj.NS(messages.WorkflowJobSendMessageExecute+"."+i, c.ns)
	}

	svcFnExecutor := func(ctx context.Context, trackingID string, job *model.WorkflowState, def *task2.FnDef, inVars model.Vars) (model.Vars, error) {
		switch def.Type {
		case task2.ExecutionTypeVars:
			id := common.TrackingID(job.Id)
			pidCtx := context.WithValue(ctx, client.InternalProcessInstanceId, job.ProcessInstanceId)
			pidCtx = context.WithValue(pidCtx, client.InternalExecutionId, job.ExecutionId)
			pidCtx = context.WithValue(pidCtx, client.InternalActivityId, id.ParentID())
			pidCtx = context.WithValue(pidCtx, client.InternalTaskId, id.ID())
			pidCtx = client.ReParentSpan(pidCtx, job)
			jc := &jobClient{
				cl:                c,
				activityID:        id.ParentID(),
				executionID:       job.ExecutionId,
				processInstanceID: job.ProcessInstanceId,
				jobID:             id.ID(),
			}
			if job.State == model.CancellationState_compensating {
				var err error
				jc.originalInputs, jc.originalOutputs, err = c.getCompensationVariables(ctx, job.ProcessInstanceId, job.Compensation.ForTrackingId)
				if err != nil {
					return make(model.Vars), fmt.Errorf("get compensation variables: %w", err)
				}
			}
			svcFn := def.Fn.(task2.ServiceFn)
			v, err := svcFn(pidCtx, jc, inVars)
			if err != nil {
				return v, fmt.Errorf("execute service task: %w", err)
			}
			return v, nil
		case task2.ExecutionTypeTyped:
			id := common.TrackingID(job.Id)
			pidCtx := context.WithValue(ctx, client.InternalProcessInstanceId, job.ProcessInstanceId)
			pidCtx = context.WithValue(pidCtx, client.InternalExecutionId, job.ExecutionId)
			pidCtx = context.WithValue(pidCtx, client.InternalActivityId, id.ParentID())
			pidCtx = context.WithValue(pidCtx, client.InternalTaskId, id.ID())
			pidCtx = client.ReParentSpan(pidCtx, job)
			revMapping := make(map[string]string, len(def.OutMapping))
			for k, v := range def.OutMapping {
				revMapping[v] = k
			}
			fn := reflect.TypeOf(def.Fn)
			x := fn.In(2)
			t := coerceVarsToType(x, inVars, def.InMapping)
			em := t.Elem().Interface()
			fmt.Println(em)
			vl := reflect.ValueOf(def.Fn)
			jc := &jobClient{
				cl:                c,
				jobID:             id.ID(),
				processInstanceID: job.ProcessInstanceId,
				activityID:        id.ParentID(),
				executionID:       job.ExecutionId,
			}
			params := []reflect.Value{
				reflect.ValueOf(pidCtx),
				reflect.ValueOf(jc),
				reflect.ValueOf(em),
			}
			out := vl.Call(params)
			str := out[0].Interface()
			if err := out[1].Interface(); err != nil {
				return model.Vars{}, fmt.Errorf("execute task: %w", err.(error))
			}
			newVars := model.Vars{}
			outType := reflect.TypeOf(str)
			nf := outType.NumField()
			for i := 0; i < nf; i++ {
				typ := outType.Field(i)
				nm := typ.Name
				var v any
				if typ.Type.Kind() == reflect.Struct {
					v = structs.Map(reflect.ValueOf(str).Field(i).Interface())

				} else {
					v = out[i].Field(i).Interface()
				}
				newVars[revMapping[nm]] = v
			}
			return newVars, nil
		default:
			return model.Vars{}, fmt.Errorf("bad execution type: %d", def.Type)
		}
	}

	svcFnLocator := func(job *model.WorkflowState) (*task2.FnDef, error) {
		fnDef, ok := c.SvcTasks[job.ExecuteVersion]
		if !ok {
			err := fmt.Errorf("service task '%s' not found", job.ExecuteVersion)
			slog.Error("find service function", "error", err, slog.String("fn", *job.Execute))
			return nil, fmt.Errorf("find service task function: %w", errors2.ErrWorkflowFatal{Err: err})
		}
		return fnDef, nil
	}

	msgFnLocator := func(job *model.WorkflowState) (*task2.FnDef, error) {
		sendFn, ok := c.MsgSender[job.WorkflowName+"_"+*job.Execute]
		if !ok {
			return nil, fmt.Errorf("msg send task '%s' not found", *job.Execute)
		}
		return sendFn, nil
	}

	msgFnExecutor := func(ctx context.Context, trackingID string, job *model.WorkflowState, def *task2.FnDef, inVars model.Vars) error {
		switch def.Type {
		case task2.ExecutionTypeVars:
			fn := def.Fn.(task2.SenderFn)
			if err := fn(ctx, &messageClient{
				cl: c, jobID: trackingID,
				executionID:       job.ExecutionId,
				processInstanceID: job.ProcessInstanceId,
				activityID:        common.TrackingID(job.Id).ParentID(),
			}, inVars); err != nil {
				slog.Warn("nats listener", "error", err)
				return err
			}
			return nil
		default:
			return fmt.Errorf("bad execution type: %d", def.Type)
		}
	}

	workflowErrorHandler := func(ctx context.Context, ns string, trackingID string, errorCode string, binVars []byte) (*model.HandleWorkflowErrorResponse, error) {
		res := &model.HandleWorkflowErrorResponse{}
		req := &model.HandleWorkflowErrorRequest{TrackingId: trackingID, ErrorCode: errorCode, Vars: binVars}
		ctx = subj.SetNS(ctx, ns)
		if err2 := api2.Call(ctx, c.txCon, messages.APIHandleWorkflowError, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err2 != nil {
			// TODO: This isn't right.  If this call fails it assumes it is handled!
			retErr := fmt.Errorf("handle workflow error: %w", err2)
			return nil, logx.Err(ctx, "handle a workflow error", retErr, slog.Any("workflowError", errorCode))
		}
		return res, nil
	}

	params := client.ServiceTaskProcessParams{
		SvcFnExecutor:    svcFnExecutor,
		SvcFnLocator:     svcFnLocator,
		MsgFnLocator:     msgFnLocator,
		SvcTaskCompleter: c.completeServiceTask,
		MsgSendCompleter: c.completeSendMessage,
		MsgFnExecutor:    msgFnExecutor,
		WfErrorHandler:   workflowErrorHandler,
		PiErrorCanceller: c.cancelProcessInstanceWithError,
	}

	for k, v := range tasks {
		cName := "ServiceTask_" + c.ns + "_" + k
		slog.Info("listening for tasks", "subject", cName)
		consumer, err := c.js.Consumer(ctx, "WORKFLOW", cName)
		if err != nil {
			return fmt.Errorf("get consumer '%s': %w", cName, err)
		}
		cInf, err := consumer.Info(ctx)
		if err != nil {
			return fmt.Errorf("listen obtaining consumer info for %s: %w", cName, err)
		}
		ackTimeout := cInf.Config.AckWait
		err = common.Process(ctx, c.js, "WORKFLOW", "jobExecute", c.closer, v, cName, c.concurrency, c.ReceiveMiddleware, client.ClientProcessFn(ackTimeout, &c.processing, c.noRecovery, c, params), c.signalFatalErr, common.WithBackoffFn(c.backoff))
		if err != nil {
			return fmt.Errorf("connect to service task consumer: %w", err)
		}
	}
	return nil
}

func (c *Client) signalFatalErr(ctx context.Context, state *model.WorkflowState, log *slog.Logger) {
	res := &model.HandleWorkflowFatalErrorResponse{}
	req := &model.HandleWorkflowFatalErrorRequest{WorkflowState: state}
	ctx = subj.SetNS(ctx, c.ns)

	if err2 := api2.Call(ctx, c.txCon, messages.APIHandleWorkflowFatalError, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err2 != nil {
		retErr := fmt.Errorf("handle workflow fatal error: %w", err2)
		log.Error("handle a workflow fatal error call failed", "err", retErr)
	}
}

func (c *Client) listenProcessTerminate(ctx context.Context) error {
	closer := make(chan struct{}, 1)
	err := common.Process(ctx, c.js, "WORKFLOW", "ProcessTerminateConsumer_"+c.ns, closer, subj.NS(messages.WorkflowProcessTerminated, c.ns), "ProcessTerminateConsumer_"+c.ns, 4, c.ReceiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		st := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), st); err != nil {
			log.Error("proto unmarshal error", "error", err)
			return true, fmt.Errorf("listenProcessTerminate unmarshalling proto: %w", err)
		}
		callCtx := context.WithValue(ctx, keys.ContextKey(keys.ProcessInstanceID), st.ProcessInstanceId)
		v, err := vars.Decode(callCtx, st.Vars)
		if err != nil {
			return true, fmt.Errorf("listenProcessTerminate decoding vars: %w", err)
		}
		if def, ok := c.proCompleteTasks[st.ProcessId]; ok {
			switch def.Type {
			case task2.ExecutionTypeVars:
				fn := def.Fn.(task2.ProcessTerminateFn)
				fn(callCtx, v, st.Error, st.State)
			case task2.ExecutionTypeTyped:
				typ := reflect.TypeOf(def.Fn).In(1)
				val := coerceVarsToType(typ, v, def.InMapping)
				params := []reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(val.Elem().Interface()),
					reflect.ValueOf(st.Error),
					reflect.ValueOf(st.State),
				}
				fn := reflect.ValueOf(def.Fn)
				fn.Call(params)
			default:
				return true, fmt.Errorf("unknown processterminate function type: %d", def.Type)
			}
		}
		return true, nil
	}, nil)
	if err != nil {
		return fmt.Errorf("listen workflow complete process: %w", err)
	}
	return nil
}

// ListUserTaskIDs returns a list of user tasks for a particular owner
func (c *Client) ListUserTaskIDs(ctx context.Context, owner string) (*model.UserTasks, error) {
	res := &model.UserTasks{}
	req := &model.ListUserTasksRequest{Owner: owner}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIListUserTaskIDs, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res, nil
}

// GetTaskSpecVersions returns the version IDs associated with the named task spec.
func (c *Client) GetTaskSpecVersions(ctx context.Context, name string) ([]string, error) {
	res := &model.GetTaskSpecVersionsResponse{}
	req := &model.GetTaskSpecVersionsRequest{Name: name}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetTaskSpecVersions, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res.Versions.Id, nil
}

// CompleteUserTask completes a task and sends the variables back to the workflow
func (c *Client) CompleteUserTask(ctx context.Context, owner string, trackingID string, newVars model.Vars) error {
	ev, err := vars.Encode(ctx, newVars)
	if err != nil {
		return fmt.Errorf("decode variables for complete user task: %w", err)
	}
	res := &model.CompleteUserTaskResponse{}
	req := &model.CompleteUserTaskRequest{Owner: owner, TrackingId: trackingID, Vars: ev}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APICompleteUserTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

func (c *Client) completeServiceTask(ctx context.Context, trackingID string, newVars model.Vars, compensating bool) error {
	ev, err := vars.Encode(ctx, newVars)
	if err != nil {
		return fmt.Errorf("decode variables for complete service task: %w", err)
	}
	res := &model.CompleteServiceTaskResponse{}
	req := &model.CompleteServiceTaskRequest{TrackingId: trackingID, Vars: ev, Compensating: compensating}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APICompleteServiceTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

func (c *Client) completeSendMessage(ctx context.Context, trackingID string, newVars model.Vars) error {
	ev, err := vars.Encode(ctx, newVars)
	if err != nil {
		return fmt.Errorf("decode variables for complete send message: %w", err)
	}
	res := &model.CompleteSendMessageResponse{}
	req := &model.CompleteSendMessageRequest{TrackingId: trackingID, Vars: ev}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APICompleteSendMessageTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

// LoadBPMNWorkflowFromBytes loads, parses, and stores a BPMN workflow in SHAR. Returns the uuid uniquely identifying the workflow.
func (c *Client) LoadBPMNWorkflowFromBytes(ctx context.Context, name string, b []byte) (string, error) {
	rdr := bytes.NewReader(b)
	wf, err := parser.Parse(ctx, &expression.ExprEngine{}, name, rdr)

	if err != nil {
		return "", c.clientErr(ctx, err)
	}
	rdr = bytes.NewReader(b)
	compressed := &bytes.Buffer{}
	archiver := gzip.NewWriter(compressed)
	if _, err := io.Copy(archiver, rdr); err != nil {
		return "", fmt.Errorf("fasiled to compress source: %w", err)
	}
	if err := archiver.Close(); err != nil {
		return "", fmt.Errorf("fasiled to complete source compression: %w", err)
	}
	wf.GzipSource = compressed.Bytes()

	res := &model.StoreWorkflowResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIStoreWorkflow, c.ExpectedCompatibleServerVersion, c.SendMiddleware, &model.StoreWorkflowRequest{Workflow: wf}, res); err != nil {
		return "", c.clientErr(ctx, err)
	}
	return res.WorkflowId, nil
}

// HasWorkflowDefinitionChanged - given a workflow name and a BPMN xml, return true if the resulting definition is different.
func (c *Client) HasWorkflowDefinitionChanged(ctx context.Context, name string, b []byte) (bool, error) {
	versions, err := c.GetWorkflowVersions(ctx, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return true, nil
		}
		return false, err
	}
	rdr := bytes.NewReader(b)
	wf, err := parser.Parse(ctx, &expression.ExprEngine{}, name, rdr)
	if err != nil {
		return false, c.clientErr(ctx, err)
	}

	wf, err = c.ResolveWorkflow(ctx, wf)
	if err != nil {
		return false, c.clientErr(ctx, err)
	}

	hash, err := workflow.GetHash(wf)
	if err != nil {
		return false, c.clientErr(ctx, err)
	}
	return !bytes.Equal(versions[len(versions)-1].Sha256, hash), nil
}

// GetWorkflowVersions - returns a list of versions for a given workflow.
func (c *Client) GetWorkflowVersions(ctx context.Context, name string) ([]*model.WorkflowVersion, error) {
	req := &model.GetWorkflowVersionsRequest{
		Name: name,
	}
	res := &model.WorkflowVersion{}
	ctx = subj.SetNS(ctx, c.ns)
	result := make([]*model.WorkflowVersion, 0)
	err := api2.CallReturnStream(ctx, c.txCon, messages.APIGetWorkflowVersions, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res, func(val *model.WorkflowVersion) error {
		result = append(result, val)
		return nil
	})
	if err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return result, nil
}

// GetWorkflow - retrieves a workflow model given its ID
func (c *Client) GetWorkflow(ctx context.Context, id string) (*model.Workflow, error) {
	req := &model.GetWorkflowRequest{
		Id: id,
	}
	res := &model.GetWorkflowResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetWorkflow, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res.Definition, nil
}

// GetTaskSpecUsage returns a report outlining task spec usage in executable and executing workflows.
func (c *Client) GetTaskSpecUsage(ctx context.Context, id string) (*model.TaskSpecUsageReport, error) {
	req := &model.GetTaskSpecUsageRequest{
		Id: id,
	}
	res := &model.TaskSpecUsageReport{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetTaskSpecUsage, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res, nil
}

// CancelProcessInstance cancels a running Process Instance.
func (c *Client) CancelProcessInstance(ctx context.Context, processInstanceId string) error {
	return c.cancelProcessInstanceWithError(ctx, processInstanceId, nil)
}

func (c *Client) cancelProcessInstanceWithError(ctx context.Context, processInstanceID string, wfe *model.Error) error {
	res := &model.CancelProcessInstanceResponse{}
	req := &model.CancelProcessInstanceRequest{
		Id:    processInstanceID,
		State: model.CancellationState_errored,
		Error: wfe,
	}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APICancelProcessInstance, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

// LaunchProcess launches a new process within a workflow/BPMN definition. It returns the execution Id of the launched process and the workflow id of the
// BPMN definition containing the process
func (c *Client) LaunchProcess(ctx context.Context, processId string, mvars model.Vars) (executionId string, workflowId string, er error) {
	ev, err := vars.Encode(ctx, mvars)
	if err != nil {
		er = fmt.Errorf("encode variables for launch workflow: %w", err)
		return
	}
	req := &model.LaunchWorkflowRequest{ProcessId: processId, Vars: ev}
	res := &model.LaunchWorkflowResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APILaunchProcess, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		er = c.clientErr(ctx, err)
		return
	}
	executionId = res.ExecutionId
	workflowId = res.WorkflowId
	return
}

// ListExecution gets a list of running executions by workflow name.
func (c *Client) ListExecution(ctx context.Context, name string) ([]*model.ListExecutionItem, error) {
	req := &model.ListExecutionRequest{WorkflowName: name}
	res := &model.ListExecutionItem{}
	ctx = subj.SetNS(ctx, c.ns)
	result := make([]*model.ListExecutionItem, 0)
	err := api2.CallReturnStream(ctx, c.txCon, messages.APIListExecution, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res, func(val *model.ListExecutionItem) error {
		result = append(result, val)
		return nil
	})

	if err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return result, nil
}

// ListExecutableProcesses gets a list of executable processes.
func (c *Client) ListExecutableProcesses(ctx context.Context) ([]*model.ListExecutableProcessesItem, error) {
	req := &model.ListExecutableProcessesRequest{}
	res := &model.ListExecutableProcessesItem{}
	ctx = subj.SetNS(ctx, c.ns)
	result := make([]*model.ListExecutableProcessesItem, 0)
	err := api2.CallReturnStream(ctx, c.txCon, messages.APIListExecutableProcess, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res, func(val *model.ListExecutableProcessesItem) error {
		result = append(result, val)
		return nil
	})

	if err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return result, nil
}

// ListWorkflows gets a list of launchable workflow in SHAR.
func (c *Client) ListWorkflows(ctx context.Context) ([]*model.ListWorkflowResponse, error) {
	req := &model.ListWorkflowsRequest{}
	res := &model.ListWorkflowResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	result := make([]*model.ListWorkflowResponse, 0)
	err := api2.CallReturnStream(ctx, c.txCon, messages.APIListWorkflows, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res, func(val *model.ListWorkflowResponse) error {
		result = append(result, val)
		return nil
	})

	if err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return result, nil
}

// ListExecutionProcesses lists the current process IDs for an Execution.
func (c *Client) ListExecutionProcesses(ctx context.Context, id string) (*model.ListExecutionProcessesResponse, error) {
	req := &model.ListExecutionProcessesRequest{Id: id}
	res := &model.ListExecutionProcessesResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIListExecutionProcesses, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res, nil
}

// GetProcessInstanceStatus lists the current workflow states for a process instance.
func (c *Client) GetProcessInstanceStatus(ctx context.Context, id string) ([]*model.WorkflowState, error) {
	req := &model.GetProcessInstanceStatusRequest{Id: id}
	res := &model.WorkflowState{}
	ctx = subj.SetNS(ctx, c.ns)

	result := make([]*model.WorkflowState, 0)
	err := api2.CallReturnStream(ctx, c.txCon, messages.APIGetProcessInstanceStatus, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res, func(val *model.WorkflowState) error {
		result = append(result, val)
		return nil
	})

	if err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return result, nil
}

// GetUserTask fetches details for a user task based upon an ID obtained from, ListUserTasks
func (c *Client) GetUserTask(ctx context.Context, owner string, trackingID string) (*model.GetUserTaskResponse, model.Vars, error) {
	req := &model.GetUserTaskRequest{Owner: owner, TrackingId: trackingID}
	res := &model.GetUserTaskResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetUserTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, nil, c.clientErr(ctx, err)
	}
	v, err := vars.Decode(ctx, res.Vars)
	if err != nil {
		return nil, nil, c.clientErr(ctx, err)
	}
	return res, v, nil
}

// SendMessage sends a Workflow Message to a specific workflow instance
func (c *Client) SendMessage(ctx context.Context, name string, key any, mvars model.Vars) error {
	skey := fmt.Sprintf("%+v", key)
	b, err := vars.Encode(ctx, mvars)
	if err != nil {
		return fmt.Errorf("encode variables for send message: %w", err)
	}
	req := &model.SendMessageRequest{Name: name, CorrelationKey: skey, Vars: b}
	res := &model.SendMessageResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APISendMessage, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

func (c *Client) clientErr(_ context.Context, err error) error {
	return fmt.Errorf("client error: %w", err)
}

// RegisterProcessComplete registers a function to be executed when a shar workflow process terminates.
func (c *Client) RegisterProcessComplete(processId string, fn task2.ProcessTerminateFn) error {
	c.proCompleteTasks[processId] = &task2.FnDef{Fn: fn, Type: task2.ExecutionTypeVars}
	return nil
}

// GetProcessHistory gets the history for a process.
func (c *Client) GetProcessHistory(ctx context.Context, processInstanceId string) ([]*model.ProcessHistoryEntry, error) {
	req := &model.GetProcessHistoryRequest{Id: processInstanceId}
	res := &model.ProcessHistoryEntry{}
	ctx = subj.SetNS(ctx, c.ns)
	result := make([]*model.ProcessHistoryEntry, 0)
	err := api2.CallReturnStream(ctx, c.txCon, messages.APIGetProcessHistory, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res, func(val *model.ProcessHistoryEntry) error {
		result = append(result, val)
		return nil
	})

	if err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return result, nil
}

func (c *Client) clientLog(ctx context.Context, trackingID string, level slog.Level, message string, attrs map[string]string) error {
	k := common.KSuidTo128bit(trackingID)
	req := &model.LogRequest{
		Hostname:   c.host,
		ClientId:   c.id,
		TrackingId: k[:],
		Level:      int32(level),
		Time:       time.Now().UnixMicro(),
		Source:     model.LogSource_logSourceClient,
		Message:    message,
		Attributes: attrs,
	}
	res := &model.LogResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	vals := make([]any, 0)
	for key, v := range attrs {
		vals = append(vals, slog.String(key, v))
	}
	slog.Log(ctx, level, message, vals...)
	if err := api2.Call(ctx, c.txCon, messages.APILog, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

// GetJob returns a Job given a tracking ID
func (c *Client) GetJob(ctx context.Context, id string) (*model.WorkflowState, error) {
	req := &model.GetJobRequest{JobId: id}
	res := &model.GetJobResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetJob, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res.Job, nil
}

// GetServerVersion returns the current server version
func (c *Client) GetServerVersion(ctx context.Context) (*version.Version, error) {
	req := &model.GetVersionInfoRequest{
		ClientVersion:     c.version.String(),
		CompatibleVersion: c.ExpectedCompatibleServerVersion.String(),
	}
	res := &model.GetVersionInfoResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.con, messages.APIGetVersionInfo, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, fmt.Errorf("get version info: %w", err)
	}

	sv, err := version.NewVersion(res.ServerVersion)
	if err != nil {
		return nil, fmt.Errorf("get server version info: %w", err)
	}
	cv, err := version.NewVersion(res.MinCompatibleVersion)
	if err != nil {
		return nil, fmt.Errorf("get server version info: %w", err)
	}
	c.ExpectedServerVersion = sv

	if !res.Connect {
		return sv, fmt.Errorf("incompatible client version: client must be " + cv.String())
	}

	ok, cv2 := upgrader.IsCompatible(cv)
	if !ok {
		return sv, fmt.Errorf("incompatible server version: " + sv.String() + " server must be " + cv2.String())
	}
	return sv, nil
}

func (c *Client) registerServiceTask(ctx context.Context, spec *model.TaskSpec) (string, error) {
	req := &model.RegisterTaskRequest{
		Spec: spec,
	}
	res := &model.RegisterTaskResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIRegisterTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return "", c.clientErr(ctx, err)
	}
	return res.Uid, nil
}

// GetTaskSpecByUID gets a versioned task spec by its UID
func (c *Client) GetTaskSpecByUID(ctx context.Context, uid string) (*model.TaskSpec, error) {
	req := &model.GetTaskSpecRequest{
		Uid: uid,
	}
	res := &model.GetTaskSpecResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetTaskSpec, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res.Spec, nil
}

// ListTaskSpecs lists active and optionally deprecated task specs.
func (c *Client) ListTaskSpecs(ctx context.Context, includeDeprecated bool) ([]*model.TaskSpec, error) {
	req := &model.ListTaskSpecUIDsRequest{
		IncludeDeprecated: includeDeprecated,
	}
	res := &model.ListTaskSpecUIDsResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIListTaskSpecUIDs, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}

	ret := make([]*model.TaskSpec, 0, len(res.Uid))

	for _, i := range res.Uid {
		ts, err := c.GetTaskSpecByUID(ctx, i)
		if err != nil {
			return nil, fmt.Errorf("list task specs: get task spec '%s': %w", i, err)
		}
		ret = append(ret, ts)
	}
	return ret, nil
}

// ResolveWorkflow - returns a list of versions for a given workflow.
func (c *Client) ResolveWorkflow(ctx context.Context, workflow *model.Workflow) (*model.Workflow, error) {
	req := &model.ResolveWorkflowRequest{
		Workflow: workflow,
	}
	res := &model.ResolveWorkflowResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIResolveWorkflow, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, c.clientErr(ctx, err)
	}
	return res.Workflow, nil
}

// GetTaskUIDFromSpec returns a UID (string) based on a deterministic algorithm from a TaskSpec.
func (c *Client) GetTaskUIDFromSpec(spec *model.TaskSpec) (string, error) {
	uid, err := task.CreateUID(spec)
	if err != nil {
		return "", fmt.Errorf("create uid: %w", err)
	}
	return uid, nil
}

func (c *Client) heartbeat(ctx context.Context) error {
	req := &model.HeartbeatRequest{
		Host: c.host,
		Id:   c.id,
		Time: time.Now().UnixMilli(),
	}
	res := &model.HeartbeatResponse{}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIHeartbeat, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return c.clientErr(ctx, err)
	}
	return nil
}

func (c *Client) startHeart(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-c.closer:
				return
			default:

			}
			if err := c.heartbeat(ctx); err != nil {
				slog.Error("heartbeat", "error", err)
			}
			time.Sleep(HeartBeatInterval)
		}
	}()
	return nil
}

// Shutdown stops message processing and waits for processing messages gracefully.
func (c *Client) Shutdown() {
	c.shutdownOnce.Do(func() {
		close(c.closer)
		for {
			if c.processing.Load() == 0 {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	})
}

func (c *Client) listenTerm(ctx context.Context) {
	if !testing.Testing() && !c.noOSSig {
		signal.Notify(c.sig, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			for {
				select {
				case <-c.closer:
					return
				case <-c.sig:
					c.Shutdown()
					os.Exit(0)
				}
			}
		}()
	}
}

func (c *Client) getCompensationVariables(ctx context.Context, processInstanceId string, trackingId string) (map[string]interface{}, map[string]interface{}, error) {
	req1 := &model.GetCompensationInputVariablesRequest{
		ProcessInstanceId: processInstanceId,
		TrackingId:        trackingId,
	}
	res1 := &model.GetCompensationInputVariablesResponse{}

	req2 := &model.GetCompensationOutputVariablesRequest{
		ProcessInstanceId: processInstanceId,
		TrackingId:        trackingId,
	}
	res2 := &model.GetCompensationOutputVariablesResponse{}

	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.Call(ctx, c.txCon, messages.APIGetCompensationInputVariables, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req1, res1); err != nil {
		return nil, nil, c.clientErr(ctx, err)
	}
	if err := api2.Call(ctx, c.txCon, messages.APIGetCompensationOutputVariables, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req2, res2); err != nil {
		return nil, nil, c.clientErr(ctx, err)
	}
	in, err := vars.Decode(ctx, res1.Vars)
	if err != nil {
		return nil, nil, c.clientErr(ctx, err)
	}
	out, err := vars.Decode(ctx, res2.Vars)
	if err != nil {
		return nil, nil, c.clientErr(ctx, err)
	}
	return in, out, nil
}
