package workflow

import (
	"bytes"
	"context"
	_ "embed"
	errors2 "errors"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/setup"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/workflow"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/services/cache"
	"gitlab.com/shar-workflow/shar/server/vars"
	"go.opentelemetry.io/otel/trace"
	maps2 "golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// NatsConfig holds the current nats configuration for SHAR.
//
//go:embed nats-config.yaml
var NatsConfig string

// NamespaceKvs defines all of the key value stores shar needs to operate
type NamespaceKvs struct {
	wfExecution       jetstream.KeyValue
	wfProcessInstance jetstream.KeyValue
	wfUserTasks       jetstream.KeyValue
	wfVarState        jetstream.KeyValue
	wfTaskSpec        jetstream.KeyValue
	wfTaskSpecVer     jetstream.KeyValue
	wf                jetstream.KeyValue
	wfVersion         jetstream.KeyValue
	wfTracking        jetstream.KeyValue
	job               jetstream.KeyValue
	ownerName         jetstream.KeyValue
	ownerID           jetstream.KeyValue
	wfClientTask      jetstream.KeyValue
	wfGateway         jetstream.KeyValue
	wfName            jetstream.KeyValue
	wfHistory         jetstream.KeyValue
	wfLock            jetstream.KeyValue
	wfMsgTypes        jetstream.KeyValue
	wfProcess         jetstream.KeyValue
	wfMessages        jetstream.KeyValue
	wfClients         jetstream.KeyValue
}

// Nats contains the engine functions that communicate with NATS.
type Nats struct {
}

// ListWorkflows returns a list of all the workflows in SHAR.
func (s *Engine) ListWorkflows(ctx context.Context, res chan<- *model.ListWorkflowResponse, errs chan<- error) {

	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		err2 := fmt.Errorf("get KVs for ns %s: %w", ns, err)
		log := logx.FromContext(ctx)
		log.Error("ListWorkflows get KVs", "error", err)
		errs <- err2
		return
	}

	ks, err := nsKVs.wfVersion.Keys(ctx)
	if errors2.Is(err, jetstream.ErrNoKeysFound) {
		ks = []string{}
	} else if err != nil {
		errs <- err
		return
	}

	v := &model.WorkflowVersions{}
	for _, k := range ks {
		err := common.LoadObj(ctx, nsKVs.wfVersion, k, v)
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

// NatsConnConfiguration represents the configuration for a NATS connection.
//
// - Conn: The NATS connection.
// - TxConn: The transactional NATS connection.
// - StorageType: The storage type for JetStream.
type NatsConnConfiguration struct {
	Conn        *nats.Conn
	TxConn      *nats.Conn
	StorageType jetstream.StorageType
}

func initNamespacedKvs(ctx context.Context, ns string, js jetstream.JetStream, storageType jetstream.StorageType, config string) (*NamespaceKvs, error) {
	cfg := &setup.NatsConfig{}
	if err := yaml.Unmarshal([]byte(config), cfg); err != nil {
		return nil, fmt.Errorf("initNamespacedKvs - parse nats-config.yaml: %w", err)
	}
	err := setup.EnsureBuckets(ctx, cfg, js, storageType, ns)
	if err != nil {
		return nil, fmt.Errorf("initNamespacedKvs - EnsureBuckets: %w", err)
	}

	nKvs := NamespaceKvs{}
	kvs := make(map[string]*jetstream.KeyValue)

	kvs[namespace.PrefixWith(ns, messages.KvWfName)] = &nKvs.wfName
	kvs[namespace.PrefixWith(ns, messages.KvExecution)] = &nKvs.wfExecution
	kvs[namespace.PrefixWith(ns, messages.KvTracking)] = &nKvs.wfTracking
	kvs[namespace.PrefixWith(ns, messages.KvDefinition)] = &nKvs.wf
	kvs[namespace.PrefixWith(ns, messages.KvJob)] = &nKvs.job
	kvs[namespace.PrefixWith(ns, messages.KvVersion)] = &nKvs.wfVersion
	kvs[namespace.PrefixWith(ns, messages.KvUserTask)] = &nKvs.wfUserTasks
	kvs[namespace.PrefixWith(ns, messages.KvOwnerID)] = &nKvs.ownerID
	kvs[namespace.PrefixWith(ns, messages.KvOwnerName)] = &nKvs.ownerName
	kvs[namespace.PrefixWith(ns, messages.KvClientTaskID)] = &nKvs.wfClientTask
	kvs[namespace.PrefixWith(ns, messages.KvVarState)] = &nKvs.wfVarState
	kvs[namespace.PrefixWith(ns, messages.KvProcessInstance)] = &nKvs.wfProcessInstance
	kvs[namespace.PrefixWith(ns, messages.KvGateway)] = &nKvs.wfGateway
	kvs[namespace.PrefixWith(ns, messages.KvHistory)] = &nKvs.wfHistory
	kvs[namespace.PrefixWith(ns, messages.KvLock)] = &nKvs.wfLock
	kvs[namespace.PrefixWith(ns, messages.KvMessageTypes)] = &nKvs.wfMsgTypes
	kvs[namespace.PrefixWith(ns, messages.KvTaskSpec)] = &nKvs.wfTaskSpec
	kvs[namespace.PrefixWith(ns, messages.KvTaskSpecVersions)] = &nKvs.wfTaskSpecVer
	kvs[namespace.PrefixWith(ns, messages.KvProcess)] = &nKvs.wfProcess
	kvs[namespace.PrefixWith(ns, messages.KvMessages)] = &nKvs.wfMessages
	kvs[namespace.PrefixWith(ns, messages.KvClients)] = &nKvs.wfClients

	for k, v := range kvs {
		kv, err := js.KeyValue(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("open %s KV: %w", k, err)
		}
		*v = kv
	}
	return &nKvs, nil
}

// KvsFor retrieves the shar KVs for a given namespace. If they do not exist for a namespace,
// it will initialise them and store them in a map for future lookup.
func (s *Engine) KvsFor(ctx context.Context, ns string) (*NamespaceKvs, error) {
	s.rwmx.RLock()
	if nsKvs, exists := s.sharKvs[ns]; !exists {
		s.rwmx.RUnlock()
		s.rwmx.Lock()
		kvs, err := initNamespacedKvs(ctx, ns, s.js, s.storageType, NatsConfig)
		if err != nil {
			s.rwmx.Unlock()
			return nil, fmt.Errorf("failed to initialise KVs for namespace %s: %w", ns, err)
		}

		s.sharKvs[ns] = kvs
		s.rwmx.Unlock()
		return kvs, nil
	} else {
		s.rwmx.RUnlock()
		return nsKvs, nil
	}
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
	if err := s.listenForTimer(ctx, s.js, s.closing, 4); err != nil {
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

	return nil
}

func (s *Engine) validateUniqueProcessNameFor(ctx context.Context, wf *model.Workflow) error {
	existingProcessWorkflowNames := make(map[string]string)

	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	for processName := range wf.Process {
		workFlowName, err := nsKVs.wfProcess.Get(ctx, processName)
		if errors2.Is(err, jetstream.ErrKeyNotFound) {
			continue
		}
		wfName := string(workFlowName.Value())
		if wfName != wf.Name {
			existingProcessWorkflowNames[processName] = wfName
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
func (s *Engine) StoreWorkflow(ctx context.Context, wf *model.Workflow) (string, error) {
	err := s.validateUniqueProcessNameFor(ctx, wf)
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

	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	_, err = nsKVs.wfName.Get(ctx, wf.Name)
	if errors2.Is(err, jetstream.ErrKeyNotFound) {
		wfNameID := ksuid.New().String()
		_, err = nsKVs.wfName.Put(ctx, wf.Name, []byte(wfNameID))
		if err != nil {
			return "", fmt.Errorf("store the workflow id during store workflow: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("get an existing workflow id: %w", err)
	}

	wfID := ksuid.New().String()

	createWorkflowProcessMappingFn := func(ctx context.Context, wf *model.Workflow, i *model.Process) (uint64, error) {
		ret, err := nsKVs.wfProcess.Put(ctx, i.Name, []byte(wf.Name))
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
	if err := common.UpdateObj(ctx, nsKVs.wfVersion, wf.Name, &model.WorkflowVersions{}, func(v *model.WorkflowVersions) (*model.WorkflowVersions, error) {
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
		err = common.SaveObj(ctx, nsKVs.wf, wfID, wf)
		if err != nil {
			return nil, fmt.Errorf("save workflow: %s", wf.Name)
		}
		v.Version = append(v.Version, &model.WorkflowVersion{Id: wfID, Sha256: hash, Number: int32(n) + 1})
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
					UnixTimeNano: time.Now().UnixNano(),
					Timer: &model.WorkflowTimer{
						LastFired: 0,
						Count:     0,
					},
					Vars:         []byte{},
					WorkflowName: wf.Name,
					ProcessName:  pr.Name,
				}
				if err := s.PublishWorkflowState(ctx, subj.NS(messages.WorkflowTimedExecute, ns), timer); err != nil {
					return fmt.Errorf("publish workflow timed execute: %w", err)
				}
				return nil
			}
			return nil
		})

		if vErr != nil {
			return "", fmt.Errorf("initialize all workflow timed start events for %s: %w", pr.Name, vErr)
		}
	}

	return wfID, nil
}

// ProcessServiceTasks iterates over service tasks in the processes of a given workflow setting, validating them and setting their uid into their element definitions
func (s *Engine) ProcessServiceTasks(ctx context.Context, wf *model.Workflow, svcTaskConsFn ServiceTaskConsumerFn, wfProcessMappingFn WorkflowProcessMappingFn) error {
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
					vars, err := expression.GetVariables(exp)
					if err != nil {
						return fmt.Errorf("an error occurred getting the variables from the output expression for %s in service task %s", varN, j.Id)
					}
					// Take the variables and add them to the list
					maps.Copy(outVars, vars)
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
func (s *Engine) EnsureServiceTaskConsumer(ctx context.Context, uid string) error {
	ns := subj.GetNS(ctx)
	jxCfg := jetstream.ConsumerConfig{
		Durable:       "ServiceTask_" + ns + "_" + uid,
		Description:   "",
		FilterSubject: subj.NS(messages.WorkflowJobServiceTaskExecute, subj.GetNS(ctx)) + "." + uid,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MemoryStorage: s.storageType == jetstream.MemoryStorage,
	}

	if _, err := s.js.CreateOrUpdateConsumer(ctx, "WORKFLOW", jxCfg); err != nil {
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
func (s *Engine) GetWorkflow(ctx context.Context, workflowID string) (*model.Workflow, error) {
	getWorkflowFn := func() (*model.Workflow, error) {
		ns := subj.GetNS(ctx)
		nsKVs, err := s.KvsFor(ctx, ns)
		if err != nil {
			return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
		}

		wf := &model.Workflow{}
		if err := common.LoadObj(ctx, nsKVs.wf, workflowID, wf); errors2.Is(err, jetstream.ErrKeyNotFound) {
			return nil, fmt.Errorf("get workflow failed to load object: %w", errors.ErrWorkflowNotFound)

		} else if err != nil {
			return nil, fmt.Errorf("load workflow from KV: %w", err)
		}
		return wf, nil
	}

	workflow, err := cache.Cacheable(workflowID, getWorkflowFn, s.sCache)
	if err != nil {
		return nil, fmt.Errorf("error caching GetWorkflow: %w", err)
	}
	return workflow, nil
}

// GetWorkflowNameFor - get the worflow name a process is associated with
func (s *Engine) GetWorkflowNameFor(ctx context.Context, processName string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if entry, err := common.Load(ctx, nsKVs.wfProcess, processName); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return "", fmt.Errorf(fmt.Sprintf("get workflow name for process %s: %%w", processName), errors.ErrProcessNotFound)
	} else if err != nil {
		return "", fmt.Errorf("load workflow name for process: %w", err)
	} else {
		return string(entry), nil
	}
}

// GetWorkflowVersions - returns a list of versions for a given workflow.
func (s *Engine) GetWorkflowVersions(ctx context.Context, workflowName string, wch chan<- *model.WorkflowVersion, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}

	ver := &model.WorkflowVersions{}
	if err := common.LoadObj(ctx, nsKVs.wfVersion, workflowName, ver); errors2.Is(err, jetstream.ErrKeyNotFound) {
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
func (s *Engine) CreateExecution(ctx context.Context, execution *model.Execution) (*model.Execution, error) {
	executionID := ksuid.New().String()
	log := logx.FromContext(ctx)
	log.Info("creating execution", slog.String(keys.ExecutionID, executionID))
	execution.ExecutionId = executionID
	execution.ProcessInstanceId = []string{}

	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	wf, err := s.GetWorkflow(ctx, execution.WorkflowId)

	if err := common.SaveObj(ctx, nsKVs.wfExecution, executionID, execution); err != nil {
		return nil, fmt.Errorf("save execution object to KV: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow object from KV: %w", err)
	}
	for _, m := range wf.Messages {
		if err := setup.EnsureBucket(ctx, s.js, jetstream.KeyValueConfig{
			Bucket:      subj.NS("MsgTx_%s_", subj.GetNS(ctx)) + m.Name,
			Description: "Message transmit for " + m.Name,
		}, s.storageType, func(_ *jetstream.KeyValueConfig) {}); err != nil {
			return nil, fmt.Errorf("ensuring bucket '%s':%w", m.Name, err)
		}
	}
	return execution, nil
}

// GetExecution retrieves an execution given its ID.
func (s *Engine) GetExecution(ctx context.Context, executionID string) (*model.Execution, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	e := &model.Execution{}
	if err := common.LoadObj(ctx, nsKVs.wfExecution, executionID, e); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("get execution failed to load object: %w", errors.ErrExecutionNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("load execution from KV: %w", err)
	}
	return e, nil
}

// XDestroyProcessInstance terminates a running process instance with a cancellation reason and error
func (s *Engine) XDestroyProcessInstance(ctx context.Context, state *model.WorkflowState) error {
	log := logx.FromContext(ctx)
	log.Info("destroying process instance", slog.String(keys.ProcessInstanceID, state.ProcessInstanceId))

	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
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
		if err := common.LoadObj(ctx, nsKVs.wf, execution.WorkflowId, wf); err != nil {
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

func (s *Engine) deleteExecution(ctx context.Context, state *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := nsKVs.wfExecution.Delete(ctx, state.ExecutionId); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete workflow instance: %w", err)
	}

	//TODO: Loop through all messages checking for process subscription and remove

	if err := nsKVs.wfTracking.Delete(ctx, state.ExecutionId); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete workflow tracking: %w", err)
	}
	if err := s.PublishWorkflowState(ctx, messages.ExecutionTerminated, state); err != nil {
		return fmt.Errorf("send workflow terminate message: %w", err)
	}
	return nil
}

// GetLatestVersion queries the workflow versions table for the latest entry
func (s *Engine) GetLatestVersion(ctx context.Context, workflowName string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	v := &model.WorkflowVersions{}
	if err := common.LoadObj(ctx, nsKVs.wfVersion, workflowName, v); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return "", fmt.Errorf("get latest workflow version: %w", errors.ErrWorkflowNotFound)
	} else if err != nil {
		return "", fmt.Errorf("load object whist getting latest versiony: %w", err)
	} else {
		return v.Version[len(v.Version)-1].Id, nil
	}
}

// CreateJob stores a workflow task state for user tasks.
func (s *Engine) CreateJob(ctx context.Context, job *model.WorkflowState) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	tid := ksuid.New().String()
	job.Id = common.TrackingID(job.Id).Push(tid)
	if err := common.SaveObj(ctx, nsKVs.job, tid, job); err != nil {
		return "", fmt.Errorf("save job to KV: %w", err)
	}
	return tid, nil
}

// GetJob gets a workflow task state.
func (s *Engine) GetJob(ctx context.Context, trackingID string) (*model.WorkflowState, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	job := &model.WorkflowState{}
	if err := common.LoadObj(ctx, nsKVs.job, trackingID, job); err == nil {
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
func (s *Engine) DeleteJob(ctx context.Context, trackingID string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.Delete(ctx, nsKVs.job, trackingID); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return nil
}

// ListExecutions returns a list of running workflows and versions given a workflow Name
func (s *Engine) ListExecutions(ctx context.Context, workflowName string, wch chan<- *model.ListExecutionItem, errs chan<- error) {
	log := logx.FromContext(ctx)

	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		err2 := fmt.Errorf("get KVs for ns %s: %w", ns, err)
		log := logx.FromContext(ctx)
		log.Error("list exec get KVs", "error", err)
		errs <- err2
		return
	}

	wfv := &model.WorkflowVersions{}
	if err := common.LoadObj(ctx, nsKVs.wfVersion, workflowName, wfv); err != nil {
		errs <- err
		close(errs)
		return
	}

	ver := make(map[string]*model.WorkflowVersion)
	for _, v := range wfv.Version {
		ver[v.Id] = v
	}

	ks, err := nsKVs.wfExecution.Keys(ctx)
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
		err := common.LoadObj(ctx, nsKVs.wfExecution, k, v)
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
func (s *Engine) ListExecutionProcesses(ctx context.Context, id string) ([]string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	v := &model.Execution{}
	err = common.LoadObj(ctx, nsKVs.wfExecution, id, v)
	if err != nil {
		return nil, fmt.Errorf("load execution from KV: %w", err)
	}
	return v.ProcessInstanceId, nil
}

// GetProcessInstanceStatus returns a list of workflow statuses for the specified process instance ID.
func (s *Engine) GetProcessInstanceStatus(ctx context.Context, id string, wch chan<- *model.WorkflowState, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}

	v := &model.WorkflowState{}
	err = common.LoadObj(ctx, nsKVs.wfProcessInstance, id, v)
	if err != nil {
		errs <- fmt.Errorf("function GetProcessInstanceStatus failed to load from KV: %w", err)
		return
	}
	//TODO: This should be multiple states!
	wch <- v
}

// PublishWorkflowState publishes a SHAR state object to a given subject
func (s *Engine) PublishWorkflowState(ctx context.Context, stateName string, state *model.WorkflowState, opts ...PublishOpt) error {
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

	if _, err := s.txJS.PublishMsg(pubCtx, msg, jetstream.WithMsgID(c.ID)); err != nil {
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

// SignalFatalError publishes a FatalError message on death of a process in a workflow
func (s *Engine) SignalFatalError(ctx context.Context, state *model.WorkflowState, log *slog.Logger) {
	fataError := &model.FatalError{
		HandlingStrategy: 1,
		WorkflowState:    state,
	}

	err := s.PublishMsg(ctx, messages.WorkflowSystemProcessFatalError, fataError)
	if err != nil {
		log.Error("failed publishing fatal err", "err", err)
	}
}

// PublishMsg publishes a workflow message.
func (s *Engine) PublishMsg(ctx context.Context, subject string, sharMsg proto.Message) error {
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
	if _, err := s.txJS.PublishMsg(ctx, msg, jetstream.WithMsgID(id)); err != nil {
		log := logx.FromContext(pubCtx)
		log.Error("publish message", "error", err, slog.String("nats.msg.id", id), slog.Any("msg", sharMsg), slog.String("subject", msg.Subject))
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}

// GetElement gets the definition for the current element given a workflow state.
func (s *Engine) GetElement(ctx context.Context, state *model.WorkflowState) (*model.Element, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	wf := &model.Workflow{}
	if err := common.LoadObj(ctx, nsKVs.wf, state.WorkflowId, wf); errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("load object during get element: %w", err)
	}
	els := common.ElementTable(wf)
	if el, ok := els[state.ElementId]; ok {
		return el, nil
	}
	return nil, fmt.Errorf("get element failed to locate %s: %w", state.ElementId, errors.ErrElementNotFound)
}

func (s *Engine) processTraversals(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "traversal", s.closing, subj.NS(messages.WorkflowTraversalExecute, "*"), "Traversal", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var traversal model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &traversal); err != nil {
			return false, fmt.Errorf("unmarshal traversal proto: %w", err)
		}

		if _, _, err := s.HasValidProcess(ctx, traversal.ProcessInstanceId, traversal.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelInfo, "processTraversals aborted due to a missing process")
			return true, nil
		} else if err != nil {
			return false, err
		}

		activityID := ksuid.New().String()
		if err := s.SaveState(ctx, activityID, &traversal); err != nil {
			return false, err
		}
		if err := s.activityStartProcessor(ctx, activityID, &traversal, false); errors.IsWorkflowFatal(err) {
			logx.FromContext(ctx).Error("workflow fatally terminated whilst processing activity", "error", err, slog.String(keys.ExecutionID, traversal.ExecutionId), slog.String(keys.WorkflowID, traversal.WorkflowId), "error", err, slog.String(keys.ElementID, traversal.ElementId))
			return true, nil
		} else if err != nil {
			return false, fmt.Errorf("process event: %w", err)
		}

		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("traversal processor: %w", err)
	}
	return nil
}

// HasValidProcess - checks for a valid process and instance for a workflow process and instance ids
func (s *Engine) HasValidProcess(ctx context.Context, processInstanceId, executionId string) (*model.ProcessInstance, *model.Execution, error) {
	execution, err := s.hasValidExecution(ctx, executionId)
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

func (s *Engine) hasValidExecution(ctx context.Context, executionId string) (*model.Execution, error) {
	execution, err := s.GetExecution(ctx, executionId)
	if errors2.Is(err, errors.ErrExecutionNotFound) {
		return nil, fmt.Errorf("orphaned activity: %w", err)
	}
	if err != nil {
		return nil, err
	}
	return execution, err
}

func (s *Engine) processTracking(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "tracking", s.closing, "WORKFLOW.>", "Tracking", 1, s.receiveMiddleware, s.track, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("tracking processor: %w", err)
	}
	return nil
}

func (s *Engine) processCompletedJobs(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "completedJob", s.closing, subj.NS(messages.WorkFlowJobCompleteAll, "*"), "JobCompleteConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("unmarshal completed job state: %w", err)
		}
		if _, _, err := s.HasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelInfo, "processCompletedJobs aborted due to a missing process")
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
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("completed job processor: %w", err)
	}
	return nil
}

func (s *Engine) track(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return false, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	sj := msg.Subject()
	switch {
	case
		strings.HasSuffix(sj, ".State.Execution.Execute"),
		strings.HasSuffix(sj, ".State.Process.Execute"),
		strings.HasSuffix(sj, ".State.Traversal.Execute"),
		strings.HasSuffix(sj, ".State.Activity.Execute"),
		strings.Contains(sj, ".State.Job.Execute."):
		st := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), st); err != nil {
			return false, fmt.Errorf("unmarshal failed during tracking 'execute' event: %w", err)
		}
		if err := common.SaveObj(ctx, nsKVs.wfTracking, st.ExecutionId, st); err != nil {
			return false, fmt.Errorf("save tracking information: %w", err)
		}
	case
		strings.HasSuffix(sj, ".State.Execution.Complete"),
		strings.HasSuffix(sj, ".State.Process.Complete"),
		strings.HasSuffix(sj, ".State.Traversal.Complete"),
		strings.HasSuffix(sj, ".State.Activity.Complete"),
		strings.Contains(sj, ".State.Job.Complete."):
		st := &model.WorkflowState{}
		if err := proto.Unmarshal(msg.Data(), st); err != nil {
			return false, fmt.Errorf("unmarshall failed during tracking 'complete' event: %w", err)
		}
		if err := nsKVs.wfTracking.Delete(ctx, st.ExecutionId); err != nil {
			return false, fmt.Errorf("delete workflow instance upon completion: %w", err)
		}
	default:

	}
	return true, nil
}

// Conn returns the active nats connection
func (s *Engine) Conn() common.NatsConn { //nolint:ireturn
	return s.conn
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

// Shutdown signals the engine to stop processing.
func (s *Engine) Shutdown() {
	close(s.closing)
}

func (s *Engine) processWorkflowEvents(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "workflowEvent", s.closing, subj.NS(messages.WorkflowExecutionAll, "*"), "WorkflowConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("load workflow state processing workflow event: %w", err)
		}
		if strings.HasSuffix(msg.Subject(), ".State.Execution.Complete") {
			if _, err := s.hasValidExecution(ctx, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
				log := logx.FromContext(ctx)
				log.Log(ctx, slog.LevelInfo, "processWorkflowEvents aborted due to a missing process")
				return true, nil
			} else if err != nil {
				return false, err
			}
			if err := s.XDestroyProcessInstance(ctx, &job); err != nil {
				return false, fmt.Errorf("destroy process instance whilst processing workflow events: %w", err)
			}
		}
		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("starting workflow event processing: %w", err)
	}
	return nil
}

func (s *Engine) processActivities(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "activity", s.closing, subj.NS(messages.WorkflowActivityAll, "*"), "ActivityConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var activity model.WorkflowState
		switch {
		case strings.HasSuffix(msg.Subject(), ".State.Activity.Execute"):

		case strings.HasSuffix(msg.Subject(), ".State.Activity.Complete"):
			if err := proto.Unmarshal(msg.Data(), &activity); err != nil {
				return false, fmt.Errorf("unmarshal state activity complete: %w", err)
			}
			activityID := common.TrackingID(activity.Id).ID()
			if err := s.activityCompleteProcessor(ctx, &activity); err != nil {
				return false, err
			}
			err := s.deleteSavedState(ctx, activityID)
			if err != nil {
				return true, fmt.Errorf("delete saved state upon activity completion: %w", err)
			}
		}

		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("starting activity processing: %w", err)
	}
	return nil
}

func (s *Engine) deleteSavedState(ctx context.Context, activityID string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.Delete(ctx, nsKVs.wfVarState, activityID); err != nil {
		return fmt.Errorf("delete saved state: %w", err)
	}
	return nil
}

// CloseUserTask removes a completed user task.
func (s *Engine) CloseUserTask(ctx context.Context, trackingID string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	job := &model.WorkflowState{}
	if err := common.LoadObj(ctx, nsKVs.job, trackingID, job); err != nil {
		return fmt.Errorf("load job when closing user task: %w", err)
	}

	// TODO: abstract group and user names, return all errors
	var retErr error
	allIDs := append(job.Owners, job.Groups...)
	for _, i := range allIDs {
		if err := common.UpdateObj(ctx, nsKVs.wfUserTasks, i, &model.UserTasks{}, func(msg *model.UserTasks) (*model.UserTasks, error) {
			msg.Id = remove(msg.Id, trackingID)
			return msg, nil
		}); err != nil {
			retErr = fmt.Errorf("faiiled to update user tasks object when closing user task: %w", err)
		}
	}
	return retErr
}

func (s *Engine) openUserTask(ctx context.Context, owner string, id string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.UpdateObj(ctx, nsKVs.wfUserTasks, owner, &model.UserTasks{}, func(msg *model.UserTasks) (*model.UserTasks, error) {
		msg.Id = append(msg.Id, id)
		return msg, nil
	}); err != nil {
		return fmt.Errorf("update user task object: %w", err)
	}
	return nil
}

// GetUserTaskIDs gets a list of tasks given an owner.
func (s *Engine) GetUserTaskIDs(ctx context.Context, owner string) (*model.UserTasks, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	ut := &model.UserTasks{}
	if err := common.LoadObj(ctx, nsKVs.wfUserTasks, owner, ut); err != nil {
		return nil, fmt.Errorf("load user task IDs: %w", err)
	}
	return ut, nil
}

// OwnerID gets a unique identifier for a task owner.
func (s *Engine) OwnerID(ctx context.Context, name string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if name == "" {
		name = "AnyUser"
	}
	nm, err := nsKVs.ownerID.Get(ctx, name)
	if err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return "", fmt.Errorf("get owner id: %w", err)
	}
	if nm == nil {
		id := ksuid.New().String()
		if _, err := nsKVs.ownerID.Put(ctx, name, []byte(id)); err != nil {
			return "", fmt.Errorf("write owner ID: %w", err)
		}
		if _, err = nsKVs.ownerName.Put(ctx, id, []byte(name)); err != nil {
			return "", fmt.Errorf("store owner name in kv: %w", err)
		}
		return id, nil
	}
	return string(nm.Value()), nil
}

// OwnerName retrieves an owner name given an ID.
func (s *Engine) OwnerName(ctx context.Context, id string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	nm, err := nsKVs.ownerName.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get owner name for id: %w", err)
	}
	return string(nm.Value()), nil
}

// GetOldState gets a task state given its tracking ID.
func (s *Engine) GetOldState(ctx context.Context, id string) (*model.WorkflowState, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	oldState := &model.WorkflowState{}
	err = common.LoadObj(ctx, nsKVs.wfVarState, id, oldState)
	if err == nil {
		return oldState, nil
	} else if errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("get old state failed to load object: %w", errors.ErrStateNotFound)
	}
	return nil, fmt.Errorf("retrieving task state: %w", err)
}

func (s *Engine) processLaunch(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "launch", s.closing, subj.NS(messages.WorkflowJobLaunchExecute, "*"), "LaunchConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("unmarshal during process launch: %w", err)
		}
		if _, _, err := s.HasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelInfo, "processLaunch aborted due to a missing process")
			return true, err
		} else if err != nil {
			return false, err
		}
		if err := s.launchProcessor(ctx, &job); err != nil {
			return false, fmt.Errorf("execute launch function: %w", err)
		}
		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("start process launch processor: %w", err)
	}
	return nil
}

func (s *Engine) processJobAbort(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "abort", s.closing, subj.NS(messages.WorkFlowJobAbortAll, "*"), "JobAbortConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("job abort consumer failed to unmarshal state: %w", err)
		}
		if _, _, err := s.HasValidProcess(ctx, state.ProcessInstanceId, state.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelInfo, "processJobAbort aborted due to a missing process")
			return true, err
		} else if err != nil {
			return false, err
		}
		//TODO: Make these idempotently work given missing values
		switch {
		case strings.Contains(msg.Subject(), ".State.Job.Abort.ServiceTask"), strings.Contains(msg.Subject(), ".State.Job.Abort.Gateway"):
			if err := s.deleteJob(ctx, &state); err != nil {
				return false, fmt.Errorf("delete job during service task abort: %w", err)
			}
			if err := s.RecordHistoryJobComplete(ctx, &state); err != nil {
				return true, fmt.Errorf("complete job processor failed to record history job complete: %w", err)
			}
		default:
			return true, nil
		}
		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("start job abort processor: %w", err)
	}
	return nil
}

func (s *Engine) processProcessComplete(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "processComplete", s.closing, subj.NS(messages.WorkflowProcessComplete, "*"), "ProcessCompleteConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("unmarshal during general abort processor: %w", err)
		}
		pi, execution, err := s.HasValidProcess(ctx, state.ProcessInstanceId, state.ExecutionId)
		if errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelInfo, "processProcessComplete aborted due to a missing process")
			return true, err
		} else if err != nil {
			return false, err
		}
		state.State = model.CancellationState_completed
		if err := s.DestroyProcessInstance(ctx, &state, pi.ProcessInstanceId, execution.ExecutionId); err != nil {
			return false, fmt.Errorf("delete prcess: %w", err)
		}
		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("start general abort processor: %w", err)
	}
	return nil

}

func (s *Engine) processProcessTerminate(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "processTerminate", s.closing, subj.NS(messages.WorkflowProcessTerminated, "*"), "ProcessTerminateConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
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
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("start process terminate processor: %w", err)
	}
	return nil

}

func (s *Engine) processGeneralAbort(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "abort", s.closing, subj.NS(messages.WorkflowGeneralAbortAll, "*"), "GeneralAbortConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var state model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &state); err != nil {
			return false, fmt.Errorf("unmarshal during general abort processor: %w", err)
		}
		//TODO: Make these idempotently work given missing values
		switch {
		case strings.HasSuffix(msg.Subject(), ".State.Activity.Abort"):
			if err := s.deleteActivity(ctx, &state); err != nil {
				return false, fmt.Errorf("delete activity during general abort processor: %w", err)
			}
		case strings.HasSuffix(msg.Subject(), ".State.Execution.Abort"):
			abortState := common.CopyWorkflowState(&state)
			abortState.State = model.CancellationState_terminated
			if err := s.XDestroyProcessInstance(ctx, &state); err != nil {
				return false, fmt.Errorf("delete process instance during general abort processor: %w", err)
			}
		default:
			return true, nil
		}
		return true, nil
	}, s.SignalFatalError)
	if err != nil {
		return fmt.Errorf("start general abort processor: %w", err)
	}
	return nil
}

func (s *Engine) processFatalError(ctx context.Context) error {
	err := common.Process(ctx, s.js, "WORKFLOW", "fatalError", s.closing, messages.WorkflowSystemProcessFatalError, "FatalErrorConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		var fatalErr model.FatalError
		if err := proto.Unmarshal(msg.Data(), &fatalErr); err != nil {
			return false, fmt.Errorf("unmarshal during fatal error processor: %w", err)
		}

		switch fatalErr.WorkflowState.ElementType {
		case element.ServiceTask, element.MessageIntermediateCatchEvent:
			// These are both job based, so we need to abort the job
			if err := s.PublishWorkflowState(ctx, messages.WorkflowJobServiceTaskAbort, fatalErr.WorkflowState); err != nil {
				log := logx.FromContext(ctx)
				log.Error("publish workflow state for service task abort", "error", err)
				return false, fmt.Errorf("publish abort task for handle fatal workflow error: %w", err)
			}
		default:
			//add more cases as more element types need to be supported for cleaning down fatal errs
			//just remove the process/executions below
		}

		//get the execution
		execution, err2 := s.GetExecution(ctx, fatalErr.WorkflowState.ExecutionId)
		if err2 != nil {
			return false, fmt.Errorf("error retrieving execution when processing fatal err: %w", err2)
		}
		//loop over the process instance ids to tear them down
		for _, processInstanceId := range execution.ProcessInstanceId {
			fatalErr.WorkflowState.State = model.CancellationState_terminated
			if err := s.DestroyProcessInstance(ctx, fatalErr.WorkflowState, processInstanceId, execution.ExecutionId); err != nil {
				log := logx.FromContext(ctx)
				log.Error("failed destroying process instance", "err", err)
			}
		}

		return true, nil
	}, nil)

	if err != nil {
		return fmt.Errorf("start process fatal error processor: %w", err)
	}
	return nil
}

func (s *Engine) deleteActivity(ctx context.Context, state *model.WorkflowState) error {
	if err := s.deleteSavedState(ctx, common.TrackingID(state.Id).ID()); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete activity: %w", err)
	}
	return nil
}

func (s *Engine) deleteJob(ctx context.Context, state *model.WorkflowState) error {
	if err := s.DeleteJob(ctx, common.TrackingID(state.Id).ID()); err != nil && !errors2.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete job: %w", err)
	}
	if activityState, err := s.GetOldState(ctx, common.TrackingID(state.Id).Pop().ID()); err != nil && !errors2.Is(err, errors.ErrStateNotFound) {
		return fmt.Errorf("fetch old state during delete job: %w", err)
	} else if err == nil {
		if err := s.PublishWorkflowState(ctx, subj.NS(messages.WorkflowActivityAbort, subj.GetNS(ctx)), activityState); err != nil {
			return fmt.Errorf("publish activity abort during delete job: %w", err)
		}
	}
	return nil
}

// SaveState saves the task state.
func (s *Engine) SaveState(ctx context.Context, id string, state *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	saveState := proto.Clone(state).(*model.WorkflowState)
	saveState.Id = common.TrackingID(saveState.Id).Pop().Push(id)
	data, err := proto.Marshal(saveState)
	if err != nil {
		return fmt.Errorf("unmarshal saved state: %w", err)
	}
	if err := common.Save(ctx, nsKVs.wfVarState, id, data); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

// CreateProcessInstance creates a new instance of a process and attaches it to the workflow instance.
func (s *Engine) CreateProcessInstance(ctx context.Context, executionId string, parentProcessID string, parentElementID string, processName string, workflowName string, workflowId string) (*model.ProcessInstance, error) {
	id := ksuid.New().String()
	pi := &model.ProcessInstance{
		ProcessInstanceId: id,
		ProcessName:       processName,
		ParentProcessId:   &parentProcessID,
		ParentElementId:   &parentElementID,
		ExecutionId:       executionId,
	}

	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
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
	err = common.SaveObj(ctx, nsKVs.wfProcessInstance, pi.ProcessInstanceId, pi)
	if err != nil {
		return nil, fmt.Errorf("create process instance failed to save process instance: %w", err)
	}

	err = common.UpdateObj(ctx, nsKVs.wfExecution, executionId, wfi, func(v *model.Execution) (*model.Execution, error) {
		v.ProcessInstanceId = append(v.ProcessInstanceId, pi.ProcessInstanceId)
		return v, nil
	})
	if err != nil {
		return nil, fmt.Errorf("create process instance failed to update workflow instance: %w", err)
	}
	return pi, nil
}

// GetProcessInstance returns a process instance for a given process ID
func (s *Engine) GetProcessInstance(ctx context.Context, processInstanceID string) (*model.ProcessInstance, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	pi := &model.ProcessInstance{}
	err = common.LoadObj(ctx, nsKVs.wfProcessInstance, processInstanceID, pi)
	if errors2.Is(err, jetstream.ErrKeyNotFound) {
		return nil, fmt.Errorf("get process instance failed to load instance: %w", errors.ErrProcessInstanceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get process instance failed to load instance: %w", err)
	}
	return pi, nil
}

// DestroyProcessInstance deletes a process instance and removes the workflow instance dependent on all process instances being satisfied.
func (s *Engine) DestroyProcessInstance(ctx context.Context, state *model.WorkflowState, processInstanceId string, executionId string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	e := &model.Execution{}
	err = common.UpdateObj(ctx, nsKVs.wfExecution, executionId, e, func(v *model.Execution) (*model.Execution, error) {
		v.ProcessInstanceId = remove(v.ProcessInstanceId, processInstanceId)
		return v, nil
	})
	if len(e.ProcessInstanceId) == 0 {
		if err := common.Delete(ctx, nsKVs.wfExecution, executionId); err != nil {
			return fmt.Errorf("destroy process instance delete execution: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("destroy process instance failed to update execution: %w", err)
	}
	err = common.Delete(ctx, nsKVs.wfProcessInstance, processInstanceId)
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
func (s *Engine) DeprecateTaskSpec(ctx context.Context, uid []string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	for _, u := range uid {
		ts := &model.TaskSpec{}
		err := common.UpdateObj(ctx, nsKVs.wfTaskSpec, u, ts, func(v *model.TaskSpec) (*model.TaskSpec, error) {
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

func (s *Engine) populateMetadata(wf *model.Workflow) {
	for _, process := range wf.Process {
		process.Metadata = &model.Metadata{}
		for _, elem := range process.Elements {
			if elem.Type == element.TimedStartEvent {
				process.Metadata.TimedStart = true
			}
		}
	}
}

func (s *Engine) validateUniqueMessageNames(ctx context.Context, wf *model.Workflow) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	existingMessageTypes := map[string]struct{}{}
	for _, msgType := range wf.Messages {
		messageReceivers := &model.MessageReceivers{}
		err := common.LoadObj(ctx, nsKVs.wfMsgTypes, msgType.Name, messageReceivers)

		if err == nil && messageReceivers.AssociatedWorkflowName != wf.Name {
			existingMessageTypes[msgType.Name] = struct{}{}
		}
	}
	existingMessages := strings.Join(maps2.Keys(existingMessageTypes), ",")

	if len(existingMessageTypes) > 0 {
		return fmt.Errorf(fmt.Sprintf("These messages already exist for other workflows: \"%s\"", existingMessages))
	}

	return nil
}

// CheckProcessTaskDeprecation checks if all the tasks in a process have not been deprecated.
func (s *Engine) CheckProcessTaskDeprecation(ctx context.Context, workflow *model.Workflow, processName string) error {
	pr := workflow.Process[processName]
	for _, el := range pr.Elements {
		if el.Type == element.ServiceTask {
			st, err := s.GetTaskSpecByUID(ctx, *el.Version)
			if err != nil {
				return fmt.Errorf("get task spec by uid: %w", err)
			}
			if st.Behaviour != nil && st.Behaviour.Deprecated {
				return fmt.Errorf("process %s contains deprecated task %s", processName, el.Execute)
			}
		}
	}
	return nil
}

// ListTaskSpecUIDs lists UIDs of active (and optionally deprecated) tasks specs.
func (s *Engine) ListTaskSpecUIDs(ctx context.Context, deprecated bool) ([]string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	ret := make([]string, 0, 50)
	vers, err := nsKVs.wfTaskSpecVer.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get task spec version keys: %w", err)
	}
	for _, v := range vers {
		ver := &model.TaskSpecVersions{}
		err := common.LoadObj(ctx, nsKVs.wfTaskSpecVer, v, ver)
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
func (s *Engine) GetProcessIdFor(ctx context.Context, startEventMessageName string) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	messageReceivers := &model.MessageReceivers{}
	err = common.LoadObj(ctx, nsKVs.wfMsgTypes, startEventMessageName, messageReceivers)

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
func (s *Engine) Heartbeat(ctx context.Context, req *model.HeartbeatRequest) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.SaveObj(ctx, nsKVs.wfClients, req.Host+"-"+req.Id, req); err != nil {
		return fmt.Errorf("heartbeat write to kv: %w", err)
	}
	return nil
}

// Log publishes LogRequest to WorkflowTelemetry Logs subject
func (s *Engine) Log(ctx context.Context, req *model.LogRequest) error {
	if err := common.PublishObj(ctx, s.conn, messages.WorkflowTelemetryLog, req, nil); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	return nil
}

// deleteProcessHistory deletes the process history for a given process ID in A SHAR namespace, process history gets spooled to a.
func (s *Engine) deleteProcessHistory(ctx context.Context, processId string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	log := logx.FromContext(ctx)
	log.Debug("delete process history", keys.ProcessInstanceID, processId)

	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}
	ks, err := common.KeyPrefixSearch(ctx, s.js, nsKVs.wfHistory, processId, common.KeyPrefixResultOpts{})
	if err != nil {
		return fmt.Errorf("keyPrefixSearch: %w", err)
	}
	for _, k := range ks {
		if item, err := nsKVs.wfHistory.Get(ctx, k); err != nil {
			return fmt.Errorf("get workflow history item: %w", err)
		} else {
			msg := nats.NewMsg(messages.WorkflowSystemHistoryArchive)
			msg.Header.Add("KEY", k)
			msg.Data = item.Value()
			if err := s.conn.PublishMsg(msg); err != nil {
				return fmt.Errorf("publish workflow history archive item: %w", err)
			}
		}
		if err := nsKVs.wfHistory.Delete(ctx, k); errors2.Is(err, jetstream.ErrKeyNotFound) {
			slog.Warn("key already deleted", "key", k)
		} else if err != nil {
			return fmt.Errorf("delete key %s: %w", k, err)
		}
	}
	return nil
}

// DeleteNamespace deletes the key-value store for the specified namespace in SHAR.
// It iterates over all the key-value stores and deletes them one by one.
// The function returns nil if all key-value stores are successfully deleted.
func (s *Engine) DeleteNamespace(ctx context.Context, ns string) error {
	lister := s.js.KeyValueStores(ctx)
	toDelete := make([]string, 0)
	for i := range lister.Status() {
		toDelete = append(toDelete, i.Bucket())
	}
	for _, bucket := range toDelete {
		err := s.js.DeleteKeyValue(ctx, bucket)
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
func (s *Engine) ListExecutableProcesses(ctx context.Context, wch chan<- *model.ListExecutableProcessesItem, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}
	wfVerKeys, err := nsKVs.wfVersion.Keys(ctx)
	if err != nil {
		errs <- fmt.Errorf("get workflow keys: %w", err)
		return
	}
	for _, k := range wfVerKeys {
		wfv := &model.WorkflowVersions{}
		if err := common.LoadObj(ctx, nsKVs.wfVersion, k, wfv); err != nil {
			if errors2.Is(err, jetstream.ErrKeyNotFound) {
				continue
			}
			errs <- fmt.Errorf("load workflow versions: %w", err)
		}
		latest := wfv.Version[len(wfv.Version)-1]
		wf := &model.Workflow{}
		if err := common.LoadObj(ctx, nsKVs.wf, latest.Id, wf); err != nil {
			if errors2.Is(err, jetstream.ErrKeyNotFound) {
				continue
			}
			errs <- fmt.Errorf("load workflow: %w", err)
		}
		for _, p := range wf.Process {
			ret := &model.ListExecutableProcessesItem{Parameter: make([]*model.ExecutableStartParameter, 0)}
			ret.ProcessName = p.Name
			ret.WorkflowName = wf.Name
			startParam := make(map[string]struct{})
			for _, el := range p.Elements {
				if el.Type == element.StartEvent {
					for _, ex := range el.OutputTransform {
						v, err := expression.GetVariables(ex)
						if err != nil {
							errs <- fmt.Errorf("get expression variables: %w", err)
							return
						}
						for n := range v {
							startParam[n] = struct{}{}
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
func (s *Engine) StartJob(ctx context.Context, subject string, job *model.WorkflowState, el *model.Element, v []byte, opts ...PublishOpt) error {
	job.Execute = &el.Execute

	// el.Version is only used for versioned tasks such as service tasks
	if el.Version != nil {
		job.ExecuteVersion = *el.Version
	}
	// skip if this job type requires no input transformation
	if el.Type != element.MessageIntermediateCatchEvent {
		job.Vars = nil
		if err := vars.InputVars(ctx, v, &job.Vars, el); err != nil {
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
func (s *Engine) evaluateOwners(ctx context.Context, owners string, vars model.Vars) ([]string, error) {
	jobGroups := make([]string, 0)
	groups, err := expression.Eval[interface{}](ctx, owners, vars)
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
