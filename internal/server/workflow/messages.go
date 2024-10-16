package workflow

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/subj"
	model2 "gitlab.com/shar-workflow/shar/internal/model"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strings"
)

const (
	senderParty   = "sender"
	receiverParty = "receiver"
)

func (s *Engine) processMessages(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "message", s.closing, subj.NS(messages.WorkflowMessage, "*"), "Message", s.concurrency, s.receiveMiddleware, s.processMessage, s.operations.SignalFatalErrorTeardown)
	if err != nil {
		return fmt.Errorf("start message processor: %w", err)
	}
	return nil
}

func (s *Engine) processMessage(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	// Unpack the message Instance
	instance := &model.MessageInstance{}
	if err := proto.Unmarshal(msg.Data(), instance); err != nil {
		return false, fmt.Errorf("unmarshal message proto: %w", err)
	}

	sender := &model.Sender{Vars: instance.Vars, CorrelationKey: instance.CorrelationKey}
	exchange := &model.Exchange{Sender: sender}

	setPartyFn := func(exch *model.Exchange) (*model.Exchange, error) {
		exch.Sender = sender
		return exch, nil
	}

	if err2 := s.handleMessageExchange(ctx, senderParty, setPartyFn, "", exchange, instance.Name, instance.CorrelationKey); err2 != nil {
		return false, err2
	}

	return true, nil
}

type setPartyFn func(exch *model.Exchange) (*model.Exchange, error)

func (s *Engine) handleMessageExchange(ctx context.Context, party string, setPartyFn setPartyFn, elementId string, exchange *model.Exchange, messageName string, correlationKey string) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	messageKey := messageKeyFrom([]string{messageName, correlationKey})

	exchangeProto, err3 := proto.Marshal(exchange)
	if err3 != nil {
		return fmt.Errorf("error serialising "+party+" message %w", err3)
	}
	//use optimistic locking capabilities on create/update to ensure no lost writes...
	_, createErr := nsKVs.WfMessages.Create(ctx, messageKey, exchangeProto)

	if errors2.Is(createErr, jetstream.ErrKeyExists) {
		err3 := common.UpdateObj(ctx, nsKVs.WfMessages, messageKey, &model.Exchange{}, setPartyFn)
		if err3 != nil {
			return fmt.Errorf("failed to update exchange with %s: %w", party, err3)
		}
	} else if createErr != nil {
		return fmt.Errorf("error creating message exchange: %w", createErr)
	}

	updatedExchange := &model.Exchange{}
	if err3 = common.LoadObj(ctx, nsKVs.WfMessages, messageKey, updatedExchange); err3 != nil {
		return fmt.Errorf("failed to retrieve exchange: %w", err3)
	} else if err3 = s.attemptMessageDelivery(ctx, updatedExchange, elementId, party, messageName, correlationKey); err3 != nil {
		return fmt.Errorf("failed attempted delivery: %w", err3)
	}
	//}
	return nil
}

func (s *Engine) hasAllReceivers(ctx context.Context, exchange *model.Exchange, messageName string) (bool, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return false, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	expectedMessageReceivers := &model.MessageReceivers{}
	err = common.LoadObj(ctx, nsKVs.WfMsgTypes, messageName, expectedMessageReceivers)
	if err != nil {
		return false, fmt.Errorf("load expected receivers: %w", err)
	}

	var allMessagesReceived bool
	for _, receiver := range expectedMessageReceivers.MessageReceiver {
		_, ok := exchange.Receivers[receiver.Id]
		if ok {
			allMessagesReceived = true
		} else {
			allMessagesReceived = false
			break
		}
	}
	return allMessagesReceived, nil
}

func (s *Engine) attemptMessageDelivery(ctx context.Context, exchange *model.Exchange, receiverName string, justArrivedParty string, messageName string, correlationKey string) error {
	slog.Debug("attemptMessageDelivery", "exchange", exchange, "messageName", messageName, "correlationKey", correlationKey)

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if exchange.Sender != nil && exchange.Receivers != nil {
		var receivers []*model.Receiver
		if justArrivedParty == senderParty { // deliver to all receivers
			receivers = maps.Values(exchange.Receivers)
		} else { // deliver only to the receiver that has just arrived
			receivers = []*model.Receiver{exchange.Receivers[receiverName]}
		}

		for _, recvr := range receivers {
			job, err := s.operations.GetJob(ctx, recvr.Id)
			if errors2.Is(err, jetstream.ErrKeyNotFound) {
				return nil
			} else if err != nil {
				return err
			}

			job.Vars = exchange.Sender.Vars
			if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowJobAwaitMessageComplete, job); err != nil {
				return fmt.Errorf("publishing complete message job: %w", err)
			}
		}
		hasAllReceivers, err := s.hasAllReceivers(ctx, exchange, messageName)
		if err != nil {
			return fmt.Errorf("failed has expected receivers: %w", err)
		}

		if exchange.Sender != nil && hasAllReceivers {
			messageKey := messageKeyFrom([]string{messageName, correlationKey})
			err := nsKVs.WfMessages.Delete(ctx, messageKey)
			if err != nil {
				return fmt.Errorf("error deleting exchange %w", err)
			}
		}

	}

	return nil
}

func (s *Engine) processAwaitMessageExecute(ctx context.Context) error {
	if err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "messageExecute", s.closing, subj.NS(messages.WorkflowJobAwaitMessageExecute, "*"), "AwaitMessageConsumer", s.concurrency, s.receiveMiddleware, s.awaitMessageProcessor, s.operations.SignalFatalErrorTeardown); err != nil {
		return fmt.Errorf("start process launch processor: %w", err)
	}
	return nil
}

func messageKeyFrom(keyElements []string) string {
	return strings.Join(keyElements, "-")
}

// awaitMessageProcessor waits for WORKFLOW.*.State.Job.AwaitMessage.Execute job and executes a delivery
func (s *Engine) awaitMessageProcessor(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	job := &model.WorkflowState{}
	if err := proto.Unmarshal(msg.Data(), job); err != nil {
		return false, fmt.Errorf("unmarshal during process launch: %w", err)
	}

	_, _, err := s.operations.hasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId)
	if errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
		log := logx.FromContext(ctx)
		log.Log(ctx, slog.LevelDebug, "processLaunch aborted due to a missing process")
		return true, err
	} else if err != nil {
		return false, err
	}

	el, err := s.operations.getElement(ctx, job)
	if errors2.Is(err, jetstream.ErrKeyNotFound) {
		return true, &errors.ErrWorkflowFatal{Err: fmt.Errorf("finding associated element: %w", err), State: job}
	} else if err != nil {
		return false, fmt.Errorf("get message element: %w", err)
	}

	vrs := model2.NewServerVars()
	if err := vrs.Decode(ctx, job.Vars); err != nil {
		return false, &errors.ErrWorkflowFatal{Err: fmt.Errorf("decoding vars for message correlation: %w", err), State: job}
	}
	resAny, err := expression.EvalAny(ctx, s.exprEngine, "= "+el.Execute, vrs.Vals)
	if err != nil || resAny == nil {
		return false, &errors.ErrWorkflowFatal{Err: fmt.Errorf("message correlation expression evaluation errored or was empty '=%s'=%v : %w", el.Execute, resAny, err), State: job}
	}

	correlationKey := fmt.Sprintf("%+v", resAny)

	elementId := job.ElementId
	receiver := &model.Receiver{Id: common.TrackingID(job.Id).ID(), CorrelationKey: correlationKey}
	exchange := &model.Exchange{Receivers: map[string]*model.Receiver{elementId: receiver}}

	setPartyFn := func(exch *model.Exchange) (*model.Exchange, error) {
		if exch.Receivers == nil {
			exch.Receivers = map[string]*model.Receiver{elementId: receiver}
		} else {
			exch.Receivers[elementId] = receiver
		}
		return exch, nil
	}

	messageName := el.Msg
	if err2 := s.handleMessageExchange(ctx, receiverParty, setPartyFn, elementId, exchange, messageName, correlationKey); err2 != nil {
		return false, fmt.Errorf("failed to handle receiver message: %w", err2)
	}

	return true, nil
}
