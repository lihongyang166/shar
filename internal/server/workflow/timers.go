package workflow

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/internal/common/natsobject"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

func (s *Engine) listenForTimer(sCtx context.Context, js jetstream.JetStream, closer chan struct{}, concurrency int) error {
	log := logx.FromContext(sCtx)
	subject := subj.NS("WORKFLOW.%s.Timers.>", "*")
	durable := "workflowTimers"
	consumerCfg := jetstream.ConsumerConfig{
		Name:          durable,
		Durable:       durable,
		FilterSubject: subject,
	}

	consumer, err := js.CreateOrUpdateConsumer(sCtx, natsobject.WORKFLOW_STREAM, consumerCfg)
	if err != nil {
		return fmt.Errorf("get consumer for %s: %w", durable, err)
	}
	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				select {
				case <-closer:
					return
				default:
				}
				msg, err := consumer.Fetch(1)
				if err != nil {
					if errors2.Is(err, context.DeadlineExceeded) {
						continue
					}
					if err.Error() == "nats: Server Shutdown" || err.Error() == "nats: connection closed" {
						continue
					}
					// Log Error
					log.Error("message fetch error", "error", err)
					continue
				}
				for m := range msg.Messages() {
					//				log.Debug("Process:"+traceName, slog.String("subject", msg[0].Subject))
					embargoA := m.Headers().Get("embargo")
					if embargoA == "" {
						embargoA = "0"
					}
					embargo, err := strconv.Atoi(embargoA)
					if err != nil {
						log.Error("bad embargo value", "error", err)
						continue
					}
					if embargo != 0 {
						offset := time.Duration(int64(embargo) - time.Now().UnixNano())
						if offset > 0 {
							if err := m.NakWithDelay(offset); err != nil {
								log.Warn("nak with delay")
							}
							continue
						}
					}

					state := &model.WorkflowState{}
					err = proto.Unmarshal(m.Data(), state)
					if err != nil {
						log.Error("unmarshal timer proto: %s", "error", err)
						err := m.Ack()
						if err != nil {
							log.Error("dispose of timer message after unmarshal error: %s", "error", err)
						}
						continue
					}

					var cid string
					if cid = m.Headers().Get(logx.CorrelationHeader); cid == "" {
						log.Error("correlation key missing", "error", errors.ErrMissingCorrelation)
						continue
					}

					ctx, log := logx.NatsMessageLoggingEntrypoint(sCtx, "shar-server", m.Headers())
					ctx, err = header.FromMsgHeaderToCtx(ctx, m.Headers())
					ctx = subj.SetNS(ctx, m.Headers().Get(header.SharNamespace))
					if err != nil {
						log.Error("get header values from incoming process message", slog.Any("error", &errors.ErrWorkflowFatal{Err: err}))
						if err := m.Ack(); err != nil {
							log.Error("processing failed to ack", "error", err)
						}
						continue
					}
					if strings.HasSuffix(m.Subject(), ".Timers.ElementExecute") {
						_, err := s.operations.hasValidExecution(sCtx, state.ExecutionId)
						if errors2.Is(err, errors.ErrExecutionNotFound) {
							log := logx.FromContext(sCtx)
							log.Log(sCtx, slog.LevelInfo, "listenForTimer aborted due to a missing instance")
							continue
						} else if err != nil {
							continue
						}

						pi, err := s.operations.GetProcessInstance(ctx, state.ProcessInstanceId)
						if errors2.Is(err, errors.ErrProcessInstanceNotFound) {
							if err := m.Ack(); err != nil {
								log.Error("ack message after process instance not found", "error", err)
								continue
							}
							continue
						}
						wf, err := s.operations.GetWorkflow(ctx, pi.WorkflowId)
						if err != nil {
							log.Error("get workflow", "error", err)
							continue
						}
						activityID := common.TrackingID(state.Id).ID()
						_, err = s.operations.GetProcessHistoryItem(ctx, state.ProcessInstanceId, activityID, model.ProcessHistoryType_activityExecute)
						if errors2.Is(err, jetstream.ErrKeyNotFound) {
							if err := m.Ack(); err != nil {
								log.Error("ack message after state not found", "error", err)
								continue
							}
						}
						if err != nil {
							return
						}
						els := common.ElementTable(wf)
						parent := common.TrackingID(state.Id).Pop()
						if err := s.traverse(ctx, pi, parent, &model.Targets{Target: []*model.Target{{Id: "timer-target", Target: *state.Execute}}}, els, state); err != nil {
							log.Error("traverse", "error", err)
							continue
						}
						if err := s.operations.PublishWorkflowState(ctx, subj.NS(messages.WorkflowActivityAbort, subj.GetNS(ctx)), state); err != nil {
							if err != nil {
								continue
							}
						}

						if err = m.Ack(); err != nil {
							log.Warn("ack after timer redirect", "error", err)
						}
						continue
					}
					ack, delay, err := s.timedExecuteProcessor(ctx, state, nil, int64(embargo))
					if err != nil {
						if errors.IsWorkflowFatal(err) {
							if err := m.Ack(); err != nil {
								log.Error("ack after a fatal error in message processing", "error", err)
							}
							log.Error("a fatal error occurred processing a message", "error", err)
							continue
						}
						log.Error("an error occurred processing a message", "error", err)
						continue
					}
					if ack {
						err := m.Ack()
						if err != nil {
							log.Error("ack after message processing", "error", err)
							continue
						}
					} else {
						if delay > 0 {
							err := m.NakWithDelay(time.Duration(delay))
							if err != nil {
								log.Error("nak message with delay: %s", "error", err)
								continue
							}
						} else {
							err := m.Nak()
							if err != nil {
								log.Error("nak message: %s", "error", err)
								continue
							}
						}
					}

				}
			}
		}()
	}
	return nil
}
