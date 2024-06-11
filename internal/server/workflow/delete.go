package workflow

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
)

func (s *Engine) processDeleteCommand(ctx context.Context) error {
	if err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "deleteCommand", s.closing, subj.NS(messages.WorkflowCommandDelete, "*"), "DeleteCommandConsumer", s.concurrency, s.receiveMiddleware, s.operations.deleteCommandProcessor, s.operations.SignalFatalError); err != nil {
		return fmt.Errorf("start process launch processor: %w", err)
	}
	return nil
}

// DeleteCommandWithState publishes a delete command to the workflow system, this command is used to delete process instances and executions.
func (o *Operations) DeleteCommandWithState(ctx context.Context, typ model.DeleteCommandType, state *model.WorkflowState, id string) error {
	cmd := &model.DeleteCommand{
		DeleteType: typ,
		State:      state,
		Id:         id,
	}
	if err := o.publishDeleteCommandMessage(ctx, cmd); err != nil {
		return fmt.Errorf("publish delete command: %w", err)
	}
	return nil
}

// DeleteCommand publishes a delete command to the workflow system.
func (o *Operations) DeleteCommand(ctx context.Context, typ model.DeleteCommandType, id string) error {
	cmd := &model.DeleteCommand{
		DeleteType: typ,
		State:      nil,
		Id:         id,
	}
	if err := o.publishDeleteCommandMessage(ctx, cmd); err != nil {
		return fmt.Errorf("publish delete command: %w", err)
	}
	return nil
}

func (s *Operations) deleteCommandProcessor(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	cmd := &model.DeleteCommand{}
	err := proto.Unmarshal(msg.Data(), cmd)
	if err != nil {
		return true, fmt.Errorf("unmarshal delete command: %w", err)
	}
	kvs, err := s.natsService.KvsFor(ctx, subj.GetNS(ctx))
	if err != nil {
		return false, fmt.Errorf("get kvs for namespace: %w", err)
	}

	switch cmd.DeleteType {
	case model.DeleteCommandType_DeleteExecution:
		b, err := s.deleteCmdExecution(ctx, cmd.State, cmd.Id, kvs.WfExecution, kvs.WfTracking)
		if err != nil {
			return b, fmt.Errorf("delete execution: %w", err)
		}
	case model.DeleteCommandType_DeleteProcessInstance:
		b, err := s.deleteCmdProcessInstance(ctx, cmd.State, cmd.Id, kvs.WfExecution, kvs.WfProcessInstance)
		if err != nil {
			return b, fmt.Errorf("delete process: %w", err)
		}
	case model.DeleteCommandType_DeleteProcessHistory:
		b, err := s.deleteCmdProcessHistory(ctx, cmd.Id, kvs.WfHistory)
		if err != nil {
			return b, fmt.Errorf("delete process: %w", err)
		}
	case model.DeleteCommandType_DeleteJob:
		b, err := s.deleteCmdJob(ctx, cmd.Id, kvs.Job)
		if err != nil {
			return b, fmt.Errorf("delete process: %w", err)
		}
	case model.DeleteCommandType_DeleteGateway:
		b, err := s.deleteCmdGateway(ctx, cmd.Id, kvs.WfGateway)
		if err != nil {
			return b, fmt.Errorf("delete process: %w", err)
		}
	}

	return true, nil
}

func (s *Operations) deleteCmdExecution(ctx context.Context, state *model.WorkflowState, executionId string, wfExecution, wfTracking jetstream.KeyValue) (bool, error) {
	// Get Execution
	exec, err := s.GetExecution(ctx, executionId)
	if errors2.IsJetStreamNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get execution: %w", err)
	}

	// Launch Delete Processes
	for _, i := range exec.ProcessInstanceId {
		if err := s.DeleteCommandWithState(ctx, model.DeleteCommandType_DeleteProcessInstance, state, i); err != nil {
			return false, fmt.Errorf("call delete processes: %w", err)
		}
	}

	if err := common.Delete(ctx, wfTracking, executionId); errors2.IsJetStreamNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("delete workflow tracking: %w", err)
	}

	// Delete Execution
	if err := common.Delete(ctx, wfExecution, executionId); errors2.IsJetStreamNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("delete execution: %w", err)
	}
	if err := s.PublishWorkflowState(ctx, messages.ExecutionTerminated, state); err != nil {
		return true, fmt.Errorf("send workflow terminate message: %w", err)
	}

	return true, nil
}

func (s *Operations) deleteCmdProcessInstance(ctx context.Context, state *model.WorkflowState, processInstanceId string, wfExecution, wfProcessInstance jetstream.KeyValue) (bool, error) {
	// Get Process Instance
	pi, err := s.GetProcessInstance(ctx, processInstanceId)
	if errors2.IsJetStreamNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get process instance: %w", err)
	}

	if state.Error != nil {
		state.State = model.CancellationState_errored
	}

	var orphanProcessInstance bool
	exec, err := s.GetExecution(ctx, pi.ExecutionId)
	if errors2.IsJetStreamNotFound(err) {
		orphanProcessInstance = true
	}
	if err != nil {
		return false, fmt.Errorf("get execution: %w", err)
	}

	if !orphanProcessInstance {
		if err := common.UpdateObj(ctx, wfExecution, pi.ExecutionId, exec, func(v *model.Execution) (*model.Execution, error) {
			v.ProcessInstanceId = remove(v.ProcessInstanceId, processInstanceId)
			return v, nil
		}); err != nil {
			return false, fmt.Errorf("update execution: %w", err)
		}
	}

	if len(exec.ProcessInstanceId) == 0 {
		// This results in delete execution being called twice for the last process.
		// This is harmless yet inefficient.
		if err := s.DeleteCommandWithState(ctx, model.DeleteCommandType_DeleteExecution, state, pi.ExecutionId); err != nil {
			return false, fmt.Errorf("call delete empty execution")
		}
		if err := s.PublishWorkflowState(ctx, messages.WorkflowExecutionComplete, state); err != nil {
			return false, fmt.Errorf("publish WorkflowExecutionComplete message: %w", err)
		}
	}

	if err := s.DeleteCommand(ctx, model.DeleteCommandType_DeleteProcessHistory, pi.ProcessInstanceId); err != nil {
		return false, fmt.Errorf("call delete process history")
	}

	if err := common.Delete(ctx, wfProcessInstance, pi.ProcessInstanceId); errors2.IsJetStreamNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("delete process instance: %w", err)
	}
	if err := s.PublishWorkflowState(ctx, messages.WorkflowProcessTerminated, state); err != nil {
		return false, fmt.Errorf("publish WorkflowProcessTerminated message: %w", err)
	}
	return true, nil
}

// deleteProcessHistory deletes the process history for a given process ID in A SHAR namespace, process history gets spooled pre-deletion.
func (s *Operations) deleteCmdProcessHistory(ctx context.Context, processInstanceId string, wfHistory jetstream.KeyValue) (bool, error) {
	ks, err := common.KeyPrefixSearch(ctx, s.natsService.Js, wfHistory, processInstanceId, common.KeyPrefixResultOpts{})
	if errors2.IsJetStreamNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("keyPrefixSearch: %w", err)
	}
	for _, k := range ks {
		if item, err := wfHistory.Get(ctx, k); errors2.IsJetStreamNotFound(err) {
			continue
		} else if err != nil {
			return false, fmt.Errorf("get workflow history item: %w", err)
		} else {
			msg := nats.NewMsg(messages.WorkflowSystemHistoryArchive)
			msg.Header.Add("KEY", k)
			msg.Data = item.Value()
			if err := s.natsService.Conn.PublishMsg(msg); err != nil {
				return false, fmt.Errorf("publish workflow history archive item: %w", err)
			}
		}
		if err := common.Delete(ctx, wfHistory, k); errors2.IsJetStreamNotFound(err) {
			slog.Warn("key already deleted", "key", k)
		} else if err != nil {
			return false, fmt.Errorf("delete key %s: %w", k, err)
		}
	}
	return true, nil
}

// deleteCmdJob deletes the job for a given job ID in A SHAR namespace.
func (s *Operations) deleteCmdJob(ctx context.Context, jobId string, wfJob jetstream.KeyValue) (bool, error) {
	if err := common.Delete(ctx, wfJob, jobId); errors2.IsJetStreamNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("delete job: %w", err)
	}
	return true, nil
}

// deleteCmdGateway deletes the gateway instance for a given gateway instance ID in A SHAR namespace.
func (s *Operations) deleteCmdGateway(ctx context.Context, gatewayInstanceId string, wfGateway jetstream.KeyValue) (bool, error) {
	if err := common.Delete(ctx, wfGateway, gatewayInstanceId); errors2.IsJetStreamNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("delete gateway instance: %w", err)
	}
	return true, nil
}

func (s *Operations) publishDeleteCommandMessage(ctx context.Context, cmd *model.DeleteCommand, opts ...PublishOpt) error {
	msg := nats.NewMsg(subj.NS(messages.WorkflowCommandDelete, subj.GetNS(ctx)))
	msg.Header.Set(header.SharNamespace, subj.GetNS(ctx))
	b, err := proto.Marshal(cmd)
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

	if !trace.SpanContextFromContext(ctx).IsValid() && cmd.State != nil {
		msg.Header.Set("traceparent", cmd.State.TraceParent)
	}

	pubCtx, cancel := context.WithTimeout(ctx, s.publishTimeout)
	defer cancel()
	msgId := ksuid.New().String()
	if _, err := s.natsService.TxJS.PublishMsg(pubCtx, msg, jetstream.WithMsgID(msgId)); err != nil {
		log := logx.FromContext(ctx)
		log.Error("publish delete command", "error", err, slog.String("nats.msg.id", msgId), slog.Any("state", cmd.State), slog.String("subject", msg.Subject))
		return fmt.Errorf("publish workflow state message: %w", err)
	}
	return nil
}
