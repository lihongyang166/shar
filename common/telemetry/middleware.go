package middleware

import (
	"context"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/telemetry"
)

type TelemetryConfigurable interface {
	GetTelemetryConfig() telemetry.Config
}

func SendMessageTelemetry(client TelemetryConfigurable) Send {
	if telConf := client.GetTelemetryConfig(); telConf.Enabled {
		return func(ctx context.Context, msg *nats.Msg) error {
			telemetry.CtxToNatsMsg(ctx, &telConf, msg)
			return nil
		}
	} else {
		return func(ctx context.Context, msg *nats.Msg) error {
			return nil
		}
	}
}

func ReceiveMessageTelemetry(client TelemetryConfigurable) Receive {
	if telConf := client.GetTelemetryConfig(); telConf.Enabled {
		return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
			return telemetry.NatsMsgToCtx(ctx, &telConf, msg), nil
		}
	} else {
		return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
			return ctx, nil
		}
	}
}

func ReceiveAPIMessageTelemetry(client TelemetryConfigurable) Receive {
	if telConf := client.GetTelemetryConfig(); telConf.Enabled {
		return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
			return telemetry.NatsMsgToCtx(ctx, &telConf, msg, telemetry.WithNewDefaultTraceId()), nil
		}
	} else {
		return func(ctx context.Context, msg *nats.Msg) (context.Context, error) {
			return ctx, nil
		}
	}
}
