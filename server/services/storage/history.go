package storage

import (
	"context"
	"fmt"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
)

// RecordHistory records into the history KV.
func (s *Nats) RecordHistory(ctx context.Context, state *model.WorkflowState, historyType model.ProcessHistoryType) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("RecordHistoryProcessStart - failed getting KVs for ns %s: %w", ns, err)
	}

	e := &model.ProcessHistoryEntry{
		ItemType:          historyType,
		WorkflowId:        &state.WorkflowId,
		ExecutionId:       &state.ExecutionId,
		CancellationState: &state.State,
		Vars:              state.Vars,
		Timer:             state.Timer,
		Error:             state.Error,
		UnixTimeNano:      state.UnixTimeNano,
		Execute:           state.Execute,
	}
	if err := common.SaveObj(ctx, nsKVs.wfHistory, state.ProcessInstanceId+"."+ksuid.New().String(), e); err != nil {
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

// GetProcessHistory fetches the history object for a process.
func (s *Nats) GetProcessHistory(ctx context.Context, processInstanceId string) ([]*model.ProcessHistoryEntry, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}
	wfHistory := nsKVs.wfHistory
	keys, err := common.KeyPrefixSearch(ctx, s.js, wfHistory, processInstanceId, common.KeyPrefixResultOpts{Sort: true})
	if err != nil {
		return nil, fmt.Errorf("get process history: %w", err)
	}

	ph := &model.ProcessHistory{
		Item: make([]*model.ProcessHistoryEntry, 0, len(keys)),
	}
	for _, key := range keys {
		entry := &model.ProcessHistoryEntry{}
		err := common.LoadObj(ctx, nsKVs.wfHistory, key, entry)
		if err != nil {
			return nil, fmt.Errorf("get process history item: %w", err)
		}
		ph.Item = append(ph.Item, entry)
	}
	return ph.Item, nil
}
