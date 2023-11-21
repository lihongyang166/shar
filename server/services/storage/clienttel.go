package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
	"log/slog"
)

func (s *Nats) processTelemetryTimer(ctx context.Context) error {
	if err := common.Process(ctx, s.js, "server_telemetry_trigger", s.closing, messages.WorkflowTelemetryTimer, "", 1, func(ctx context.Context, log *slog.Logger, msg *nats.Msg) (bool, error) {
		select {
		case <-s.closing:
			return true, nil
		case <-ctx.Done():
			return true, nil
		default:
		}
		clientKeys, err := s.wfClients.Keys(nats.Context(ctx))
		if errors.Is(err, nats.ErrNoKeysFound) {
			return false, nil
		}
		if err != nil {
			slog.Error("get client KV keys", "error", err)
			return false, fmt.Errorf("get client KV keys: %w", err)
		}
		body := &model.TelemetryClients{Count: int32(len(clientKeys))}
		b, err := proto.Marshal(body)
		if err != nil {
			slog.Error("marshal client telemetry", "error", err)
			return false, fmt.Errorf("marshal client telemetry: %w", err)
		}

		if _, err := s.js.Publish(messages.WorkflowTelemetryClientCount, b); err != nil {
			slog.Error("publish client telemetry", "error", err)
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("start server telemetry processing: %w", err)
	}
	return nil
}

func (s *Nats) startTelemetry(ctx context.Context) error {
	msg := nats.NewMsg(messages.WorkflowTelemetryTimer)
	if err := common.PublishOnce(s.js, s.wfLock, "WORKFLOW", "TelemetryTimerConsumer", msg); err != nil {
		return fmt.Errorf("ensure telemetry timer message: %w", err)
	}
	if err := s.processTelemetryTimer(ctx); err != nil {
		return fmt.Errorf("run telemetry timer: %w", err)
	}
	return nil
}
