package storage

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"google.golang.org/protobuf/proto"
	"strconv"
)

// RecordHistory records into the history KV.
func (s *Nats) RecordHistory(ctx context.Context, state *model.WorkflowState, historyType model.ProcessHistoryType) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	id := common.TrackingID(state.Id)
	trackingId := id.ID()
	e := &model.ProcessHistoryEntry{
		Id:                id,
		ItemType:          historyType,
		WorkflowId:        &state.WorkflowId,
		ExecutionId:       &state.ExecutionId,
		ElementId:         &state.ElementId,
		ProcessInstanceId: &state.ProcessInstanceId,
		CancellationState: &state.State,
		Vars:              state.Vars,
		Timer:             state.Timer,
		Error:             state.Error,
		UnixTimeNano:      state.UnixTimeNano,
		Execute:           state.Execute,
	}
	newId := fmt.Sprintf("%s.%s.%d", state.ProcessInstanceId, trackingId, historyType)
	if err := common.SaveObj(ctx, nsKVs.wfHistory, newId, e); err != nil {
		return &errors.ErrWorkflowFatal{Err: fmt.Errorf("recording history: %w", err)}
	}

	return nil
}

// RecordHistoryProcessStart records the process start into the history object.
func (s *Nats) RecordHistoryProcessStart(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processExecute); err != nil {
		return fmt.Errorf("recordHistoryProcessStart: %w", err)
	}
	return nil
}

// RecordHistoryActivityExecute records the activity execute into the history object.
func (s *Nats) RecordHistoryActivityExecute(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_activityExecute); err != nil {
		return fmt.Errorf("recordHistoryActivityExecute: %w", err)
	}
	return nil
}

// RecordHistoryActivityComplete records the activity completion into the history object.
func (s *Nats) RecordHistoryActivityComplete(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_activityComplete); err != nil {
		return fmt.Errorf("recordHistoryActivityComplete: %w", err)
	}
	return nil
}

// RecordHistoryJobExecute records the job execute into the history object.
func (s *Nats) RecordHistoryJobExecute(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_jobExecute); err != nil {
		return fmt.Errorf("recordHistoryJobExecute: %w", err)
	}
	return nil
}

// RecordHistoryJobComplete records the job completion into the history object.
func (s *Nats) RecordHistoryJobComplete(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_jobComplete); err != nil {
		return fmt.Errorf("recordHistoryJobComplete: %w", err)
	}
	return nil
}

// RecordHistoryJobAbort records the job abort into the history object.
func (s *Nats) RecordHistoryJobAbort(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_jobAbort); err != nil {
		return fmt.Errorf("recordHistoryJobAbort: %w", err)
	}
	return nil
}

// RecordHistoryProcessComplete records the process completion into the history object.
func (s *Nats) RecordHistoryProcessComplete(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processComplete); err != nil {
		return fmt.Errorf("recordHistoryProcessComplete: %w", err)
	}
	return nil
}

// RecordHistoryProcessSpawn records the process spawning a new process into the history object.
func (s *Nats) RecordHistoryProcessSpawn(ctx context.Context, state *model.WorkflowState, newProcessInstanceID string) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processSpawnSync); err != nil {
		return fmt.Errorf("recordHistoryProcessSpawn: %w", err)
	}
	return nil
}

// RecordHistoryProcessAbort records the process aborting into the history object.
func (s *Nats) RecordHistoryProcessAbort(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_processAbort); err != nil {
		return fmt.Errorf("recordHistoryProcessAbort: %w", err)
	}
	return nil
}

// RecordHistoryCompensationCheckpoint records the process aborting into the history object.
func (s *Nats) RecordHistoryCompensationCheckpoint(ctx context.Context, state *model.WorkflowState) error {
	if err := s.RecordHistory(ctx, state, model.ProcessHistoryType_compensationCheckpoint); err != nil {
		return fmt.Errorf("recordHistoryCompensationCheckpoint: %w", err)
	}
	return nil
}

// GetProcessHistory fetches the history object for a process.
func (s *Nats) GetProcessHistory(ctx context.Context, processInstanceId string, wch chan<- *model.ProcessHistoryEntry, errs chan<- error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		errs <- fmt.Errorf("get KVs for ns %s: %w", ns, err)
		return
	}
	wfHistory := nsKVs.wfHistory
	keys, err := common.KeyPrefixSearch(ctx, s.js, wfHistory, processInstanceId, common.KeyPrefixResultOpts{Sort: true})
	if err != nil {
		errs <- fmt.Errorf("get process history: %w", err)
		return
	}

	for _, key := range keys {
		entry := &model.ProcessHistoryEntry{}
		err := common.LoadObj(ctx, nsKVs.wfHistory, key, entry)
		if err != nil {
			errs <- fmt.Errorf("get process history item: %w", err)
			return
		}
		wch <- entry
	}
	close(errs)
}

// GetProcessHistoryItem retrieves a process history entry based on the given process instance ID, tracking ID, and history type.
// If the entry is successfully retrieved, it is unmarshaled into a model.ProcessHistoryEntry object and returned.
// If an error occurs during the retrieval or unmarshaling process, an error is returned.
func (s *Nats) GetProcessHistoryItem(ctx context.Context, processInstanceID string, trackingID string, historyType model.ProcessHistoryType) (*model.ProcessHistoryEntry, error) {
	ns := subj.GetNS(ctx)
	kvs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get key value stores for namespace: %w", err)
	}
	entry, err := kvs.wfHistory.Get(ctx, processInstanceID+"."+trackingID+"."+strconv.Itoa(int(historyType)))
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
