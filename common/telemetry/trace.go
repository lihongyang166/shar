package telemetry

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

// NewRandomTraceID creates a random OpenTelemetry trace id.
func NewRandomTraceID() ([16]byte, error) {
	b := [16]byte{}
	sl := b[:]
	_, err := rand.Read(sl)
	if err != nil {
		return [16]byte{}, fmt.Errorf("creating random trace id: %w", err)
	}
	return b, nil
}

func EnsureTraceId(ctx context.Context) (context.Context, error) {

	if sCtx := trace.SpanContextFromContext(ctx); !sCtx.HasTraceID() {
		b, err := NewRandomTraceID()
		if err != nil {
			return nil, fmt.Errorf("ensuring trace ID: %w", err)
		}
		ctx = trace.ContextWithSpanContext(ctx, sCtx.WithTraceID(b))
	}
	return ctx, nil
}

func TelemetryContextFromNatsMsg(ctx context.Context, msg *nats.Msg) context.Context {
	carrier := NewNatsMsgCarrier(msg)
	prop := autoprop.NewTextMapPropagator()
	ctx = prop.Extract(ctx, carrier)

	ctx, err := EnsureTraceId(ctx)
	if err != nil {
		slog.Error("ensure traceId ", "error", err)
		return ctx
	}
	return ctx
}

func TelemetryContextToNatsMsg(ctx context.Context, msg *nats.Msg) {
	carrier := NewNatsMsgCarrier(msg)
	prop := autoprop.NewTextMapPropagator()
	prop.Inject(ctx, carrier)
}
