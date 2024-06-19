package workflow

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/model"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"time"
)

func (s *Engine) processTelemetryTimer(ctx context.Context) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}
	consumer, err := s.natsService.Js.CreateConsumer(ctx, "WORKFLOW", jetstream.ConsumerConfig{
		Name:          "server_telemetry_trigger",
		Durable:       "server_telemetry_trigger",
		FilterSubject: messages.WorkflowTelemetryTimer,
	})
	if err != nil {
		return fmt.Errorf("processTelemetryTimer: create consumer for ns %s: %w", ns, err)
	}

	go func() {
		for {
			select {
			case <-s.closing:
				return
			default:
				var telMsg *nats.Msg
				msgs, err := consumer.Fetch(1)
				if err != nil || len(msgs.Messages()) == 0 {
					log := logx.FromContext(ctx)
					if log.Enabled(ctx, errors2.TraceLevel) {
						log.Log(ctx, errors2.TraceLevel, "pulling client telemetry message")
					}
					time.Sleep(20 * time.Second)
					continue
				}
				msg := <-msgs.Messages()
				clientKeys, err := nsKVs.WfClients.Keys(nats.Context(ctx))
				var (
					b    []byte
					body *model.TelemetryClients
				)
				if errors.Is(err, jetstream.ErrNoKeysFound) {
					goto continueLoop
				}

				if errors.Is(err, jetstream.ErrNoKeysFound) {
					clientKeys = []string{}
				} else if err != nil {
					slog.Error("get client KV keys", "error", err)
					goto continueLoop
				}

				body = &model.TelemetryClients{Count: int32(len(clientKeys))}
				b, err = proto.Marshal(body)
				if err != nil {
					slog.Error("marshal client telemetry", "error", err)
					goto continueLoop
				}
				telMsg = nats.NewMsg(messages.WorkflowTelemetryClientCount)
				telMsg.Data = b
				if err := s.natsService.Conn.PublishMsg(telMsg); err != nil {
					slog.Error("publish client telemetry", "error", err)
				}
			continueLoop:
				if err := msg.NakWithDelay(time.Second * 1); err != nil {
					slog.Warn("message nak: " + err.Error())
				}
			}
		}
	}()
	return nil
}

func (s *Engine) startTelemetry(ctx context.Context, ns string) error {
	msg := nats.NewMsg(messages.WorkflowTelemetryTimer)
	msg.Header.Set(header.SharNamespace, ns)

	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	if err := common.PublishOnce(ctx, s.natsService.Js, nsKVs.WfLock, "WORKFLOW", "TelemetryTimerConsumer", msg); err != nil {
		return fmt.Errorf("ensure telemetry timer message: %w", err)
	}
	if err := s.processTelemetryTimer(ctx); err != nil {
		return fmt.Errorf("run telemetry timer: %w", err)
	}
	return nil
}
