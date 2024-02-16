package telemetry

import (
	"context"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/middleware"
)

// SendMessageTelemetry attaches telemetry to outgoing messages if the caller is configured for telemetry.
func SendMessageTelemetry(cfg Config) middleware.Send {
	if cfg.Enabled {
		return func(ctx context.Context, msg *nats.Msg) error {
			CtxToNatsMsg(ctx, &cfg, msg)
			return nil
		}
	} else {
		return func(ctx context.Context, msg *nats.Msg) error {
			return nil
		}
	}
}

// ReceiveMessageTelemetry returns a middleware function which extracts telemetry from incoming messages.
func ReceiveMessageTelemetry(cfg Config) middleware.Receive {
	if cfg.Enabled {
		return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
			return NatsMsgToCtx(ctx, &cfg, msg), nil
		}
	} else {
		return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
			return ctx, nil
		}
	}
}

// ReceiveAPIMessageTelemetry returns a middleware function which extracts telemetry from incoming messages for Request/Reply calls.
func ReceiveAPIMessageTelemetry(cfg Config) middleware.Receive {
	return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
		return NatsMsgToCtx(ctx, &cfg, msg, WithNewDefaultTraceId()), nil
	}
}

// SendServerMessageTelemetry returns a middleware function which attaches telemetry to outgoing server messages.
func SendServerMessageTelemetry(cfg Config) middleware.Send {
	return func(ctx context.Context, msg *nats.Msg) error {
		if v := ctx.Value(ctxkey.Traceparent); v != nil {
			msg.Header.Set("traceparent", v.(string))
			return nil
		}
		CtxToNatsMsg(ctx, &cfg, msg)
		return nil
	}
}

// ReceiveServerMessageTelemetry returns a middleware function which extracts telemetry from incoming messages for server to server calls.
func ReceiveServerMessageTelemetry(cfg Config) middleware.Receive {
	return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
		return NatsMsgToCtx(ctx, &cfg, msg), nil
	}
}
