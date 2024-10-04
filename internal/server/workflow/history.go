package workflow

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"google.golang.org/protobuf/proto"
	"maps"
	"strconv"
	"strings"
)

// RecordHistory records into the history KV.
func (s *Operations) RecordHistory(ctx context.Context, state *model.WorkflowState, historyType model.ProcessHistoryType) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	id := common.TrackingID(state.Id)
	trackingId := id.ID()
	e := &model.ProcessHistoryEntry{
		Id:                          id,
		ItemType:                    historyType,
		WorkflowId:                  &state.WorkflowId,
		ExecutionId:                 &state.ExecutionId,
		ElementId:                   &state.ElementId,
		ElementName:                 &state.ElementName,
		ProcessInstanceId:           &state.ProcessInstanceId,
		CancellationState:           &state.State,
		Vars:                        state.Vars,
		Timer:                       state.Timer,
		Error:                       state.Error,
		UnixTimeNano:                state.UnixTimeNano,
		Execute:                     state.Execute,
		ProcessId:                   state.ProcessId,
		SatisfiesGatewayExpectation: maps.Clone(state.SatisfiesGatewayExpectation),
		GatewayExpectations:         maps.Clone(state.GatewayExpectations),
		WorkflowName:                state.WorkflowName,
		PreviousElement:             state.PreviousElement,
		PreviousActivity:            state.PreviousActivity,
	}
	newId := fmt.Sprintf("%s.%s.%d", state.ProcessInstanceId, trackingId, historyType)
	if err := common.SaveObj(ctx, nsKVs.WfHistory, newId, e); err != nil {
		return &errors.ErrWorkflowFatal{Err: fmt.Errorf("recording history: %w", err)}
	}

	return nil
}

// RecordHistoryProcessStart records the process start into the history object.
func (s *Operations) RecordHistoryProcessStart(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processExecute); err != nil {
		return fmt.Errorf("recordHistoryProcessStart: %w", err)
	}
	return nil
}

// RecordHistoryActivityExecute records the activity execute into the history object.
func (s *Operations) RecordHistoryActivityExecute(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_activityExecute); err != nil {
		return fmt.Errorf("recordHistoryActivityExecute: %w", err)
	}
	return nil
}

// RecordHistoryActivityComplete records the activity completion into the history object.
func (s *Operations) RecordHistoryActivityComplete(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_activityComplete); err != nil {
		return fmt.Errorf("recordHistoryActivityComplete: %w", err)
	}
	return nil
}

// RecordHistoryJobExecute records the job execute into the history object.
func (s *Operations) RecordHistoryJobExecute(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_jobExecute); err != nil {
		return fmt.Errorf("recordHistoryJobExecute: %w", err)
	}
	return nil
}

// RecordHistoryJobComplete records the job completion into the history object.
func (s *Operations) RecordHistoryJobComplete(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_jobComplete); err != nil {
		return fmt.Errorf("recordHistoryJobComplete: %w", err)
	}
	return nil
}

// RecordHistoryJobAbort records the job abort into the history object.
func (s *Operations) RecordHistoryJobAbort(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_jobAbort); err != nil {
		return fmt.Errorf("recordHistoryJobAbort: %w", err)
	}
	return nil
}

// RecordHistoryProcessComplete records the process completion into the history object.
func (s *Operations) RecordHistoryProcessComplete(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processComplete); err != nil {
		return fmt.Errorf("recordHistoryProcessComplete: %w", err)
	}
	return nil
}

// RecordHistoryProcessSpawn records the process spawning a new process into the history object.
func (s *Operations) RecordHistoryProcessSpawn(ctx context.Context, state *model.WorkflowState, newProcessInstanceID string) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processSpawnSync); err != nil {
		return fmt.Errorf("recordHistoryProcessSpawn: %w", err)
	}
	return nil
}

// RecordHistoryProcessAbort records the process aborting into the history object.
func (s *Operations) RecordHistoryProcessAbort(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processAbort); err != nil {
		return fmt.Errorf("recordHistoryProcessAbort: %w", err)
	}
	return nil
}

// RecordHistoryCompensationCheckpoint records the process aborting into the history object.
func (s *Operations) RecordHistoryCompensationCheckpoint(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_compensationCheckpoint); err != nil {
		return fmt.Errorf("recordHistoryCompensationCheckpoint: %w", err)
	}
	return nil
}

// GetProcessHistory fetches the history object for a process.
func (s *Operations) GetProcessHistory(ctx context.Context, processInstanceId string, wch chan<- *model.ProcessHistoryEntry, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}
	wfHistory := nsKVs.WfHistory
	keys, err := common.KeyPrefixSearch(ctx, s.natsService.Js, wfHistory, processInstanceId, common.KeyPrefixResultOpts{Sort: true})
	if err != nil {
		errs <- fmt.Errorf("get process history: %w", err)
		return
	}

	for _, key := range keys {
		entry := &model.ProcessHistoryEntry{}
		err := common.LoadObj(ctx, nsKVs.WfHistory, key, entry)
		if err != nil {
			errs <- fmt.Errorf("get process history item: %w", err)
			return
		}
		wch <- entry
	}
}

// GetProcessHistoryItem retrieves a process history entry based on the given process instance ID, tracking ID, and history type.
// If the entry is successfully retrieved, it is unmarshaled into a model.ProcessHistoryEntry object and returned.
// If an error occurs during the retrieval or unmarshaling process, an error is returned.
func (s *Operations) GetProcessHistoryItem(ctx context.Context, processInstanceID string, trackingID string, historyType model.ProcessHistoryType) (*model.ProcessHistoryEntry, error) {
	ns := subj.GetNS(ctx)
	kvs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get key value stores for namespace: %w", err)
	}
	entry, err := kvs.WfHistory.Get(ctx, processInstanceID+"."+trackingID+"."+strconv.Itoa(int(historyType)))
	if err != nil {
		return nil, fmt.Errorf("get history entry: %w", err)
	}
	ret := &model.ProcessHistoryEntry{}
	err = proto.Unmarshal(entry.Value(), ret)
	if err != nil {
		return nil, fmt.Errorf("unmarshal history entry: %w", err)
	}
	return ret, nil
}

var (
	activityExecute  = processHistoryTypeString(model.ProcessHistoryType_activityExecute)
	activityComplete = processHistoryTypeString(model.ProcessHistoryType_activityComplete)
	activityAbort    = processHistoryTypeString(model.ProcessHistoryType_activityAbort)
	jobExecute       = processHistoryTypeString(model.ProcessHistoryType_jobExecute)
	jobComplete      = processHistoryTypeString(model.ProcessHistoryType_jobComplete)
	jobAbort         = processHistoryTypeString(model.ProcessHistoryType_jobAbort)
)

func processHistoryTypeString(typeName model.ProcessHistoryType) string {
	return fmt.Sprintf("%d", typeName)
}

// GetActiveEntries returns a list of workflow statuses for the specified process instance ID.
func (s *Operations) GetActiveEntries(ctx context.Context, processInstanceID string, result chan<- *model.ProcessHistoryEntry, errs chan<- error) {

	ns := subj.GetNS(ctx)
	kvs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get kvs for namespace: %w", err)
		return
	}
	keys, err := common.KeyPrefixSearchMap(ctx, s.natsService.Js, kvs.WfHistory, processInstanceID, common.KeyPrefixResultOpts{})
	if err != nil {
		errs <- fmt.Errorf("key prefix search in history for processinstance ID: %w", err)
		return
	}
	for k := range keys {
		dot := strings.LastIndexByte(k, '.')
		state := k[dot+1:]
		entryId := k[:dot]
		select {
		case <-ctx.Done():
			return
		default:
		}
		switch state {
		case activityExecute:
			if _, ok := keys[entryId+"."+activityComplete]; ok {
				continue
			}
			if _, ok := keys[entryId+"."+activityAbort]; ok {
				continue
			}
		case jobExecute:
			if _, ok := keys[entryId+"."+jobComplete]; ok {
				continue
			}
			if _, ok := keys[entryId+"."+jobAbort]; ok {
				continue
			}
		default:
			continue
		}
		item := &model.ProcessHistoryEntry{}
		if err := common.LoadObj(ctx, kvs.WfHistory, k, item); err != nil {
			errs <- fmt.Errorf("get process history item: %w", err)
			return
		} else {
			result <- item
		}
	}
}
