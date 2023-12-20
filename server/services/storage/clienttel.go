package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"time"
)

func (s *Nats) processTelemetryTimer(ctx context.Context) error {
	sub, err := s.js.PullSubscribe(messages.WorkflowTelemetryTimer, "server_telemetry_trigger")
	if err != nil {
		return fmt.Errorf("creating message kick subscription: %w", err)
	}
	go func() {
		for {
			select {
			case <-s.closing:
				return
			default:
				var telmsg *nats.Msg
				pctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				msgs, err := sub.Fetch(1, nats.Context(pctx))
				if err != nil || len(msgs) == 0 {
					slog.Warn("pulling kick message")
					cancel()
					time.Sleep(20 * time.Second)
					continue
				}
				msg := msgs[0]
				clientKeys, err := s.wfClients.Keys(nats.Context(ctx))
				var (
					b    []byte
					body *model.TelemetryClients
				)
				if errors.Is(err, nats.ErrNoKeysFound) {
					goto continueLoop
				}

				if errors.Is(err, nats.ErrNoKeysFound) {
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
				telmsg = nats.NewMsg(messages.WorkflowTelemetryClientCount)
				telmsg.Data = b
				if err := s.conn.PublishMsg(telmsg); err != nil {
					slog.Error("publish client telemetry", "error", err)
				}
			continueLoop:
				if err := msg.NakWithDelay(time.Second * 1); err != nil {
					slog.Warn("message nak: " + err.Error())
				}
				cancel()
			}
		}
	}()
	return nil
}

func (s *Nats) startTelemetry(ctx context.Context) error {
	msg := nats.NewMsg(messages.WorkflowTelemetryTimer)
	msg.Header.Set(header.SharNamespace, "*")
	if err := common.PublishOnce(s.js, s.wfLock, "WORKFLOW", "TelemetryTimerConsumer", msg); err != nil {
		return fmt.Errorf("ensure telemetry timer message: %w", err)
	}
	if err := s.processTelemetryTimer(ctx); err != nil {
		return fmt.Errorf("run telemetry timer: %w", err)
	}
	return nil
}
