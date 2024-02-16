package telemetry

import (
	"context"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/middleware"
)

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

func ReceiveAPIMessageTelemetry(cfg Config) middleware.Receive {
	return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
		return NatsMsgToCtx(ctx, &cfg, msg, WithNewDefaultTraceId()), nil
	}
}

func SendServerMessageTelemetry(cfg Config) middleware.Send {
	return func(ctx context.Context, msg *nats.Msg) error {
		if v := ctx.Value("traceparent"); v != nil {
			msg.Header.Set("traceparent", v.(string))
			return nil
		}
		CtxToNatsMsg(ctx, &cfg, msg)
		return nil
	}
}

func ReceiveServerMessageTelemetry(cfg Config) middleware.Receive {
	return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
		return NatsMsgToCtx(ctx, &cfg, msg), nil
	}
}
