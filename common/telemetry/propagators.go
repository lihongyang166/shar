package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/model"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"log/slog"
)

func CtxToNatsMsg(ctx context.Context, config *Config, msg *nats.Msg) {
	if config.Enabled {
		// If telemetry is enabled in the host application
		// We can utilise it to inject the traceID and spanID
		car := NewNatsCarrier(msg)
		prop := autoprop.NewTextMapPropagator()
		prop.Inject(ctx, car)
	} else {
		// The property injector will fail silently, so we need to carry these ourselves
		// traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
		traceParent := ctx.Value("traceparent")
		if traceParent != nil {
			msg.Header.Set("traceparent", traceParent.(string))
		}
	}
}

type propagateOptions struct {
	detaultTraceId []byte
}

// PropagateOption is the prototype for the propagate option function
type PropagateOption func(*propagateOptions)

func WithNewDefaultTraceId() PropagateOption {
	return func(o *propagateOptions) {
		o.detaultTraceId = newTraceID()
	}
}

func newTraceID() []byte {
	var traceID = make([]byte, 16)
	if _, err := rand.Read(traceID); err != nil {
		slog.Error("new trace parent: crypto get bytes: %w", err)
	}
	return traceID
}
func NatsMsgToCtx(ctx context.Context, config *Config, msg *nats.Msg, opts ...PropagateOption) context.Context {

	o := propagateOptions{}
	for _, i := range opts {
		i(&o)
	}
	if config.Enabled {
		// If telemetry is enabled in the host application
		// We can utilise it to inject the traceID and spanID
		if msg.Header.Get("traceparent") == "" {
			slog.Error("trace enabled, but missing traceparent header from server")
			msg.Header.Set("traceparent", NewTraceParent(newTraceID()))
		}
		car := NewNatsCarrier(msg)
		prop := autoprop.NewTextMapPropagator()
		return prop.Extract(ctx, car)
	} else {
		// The property injector will fail silently, so we need to carry these ourselves
		// using https://www.w3.org/TR/trace-context/
		// e.g. traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
		traceContext := msg.Header.Get("traceparent")
		if traceContext == "" && len(o.detaultTraceId) == 16 {
			traceContext = NewTraceParent(o.detaultTraceId)
		}
		ctx = context.WithValue(ctx, "traceparent", traceContext)
		return ctx
	}
}

func NewTraceParent(traceID []byte) string {
	return "00-" + hex.EncodeToString(traceID) + "-1000000000000001-00"
}

type TraceParams struct {
	TraceParent string
}

func CtxToTraceParams(ctx context.Context, config *Config) *TraceParams {
	ret := &TraceParams{}
	if config.Enabled {
		// If telemetry is enabled in the host application
		// We can utilise it to inject the traceID and spanID
		car := NewMapCarrier()
		prop := autoprop.NewTextMapPropagator()
		prop.Inject(ctx, car)

		if tp, ok := car.Map["traceparent"]; ok {
			ret.TraceParent = tp
		}
	} else {
		// The property injector will fail silently, so we need to carry these ourselves
		// traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
		traceParent := ctx.Value("traceparent")
		if traceParent != nil {
			ret.TraceParent = traceParent.(string)
		}
	}
	return ret
}

func TraceParamsToCtx(ctx context.Context, config *Config, tp *TraceParams) context.Context {
	if config.Enabled {
		// If telemetry is enabled in the host application
		// We can utilise it to inject the traceID and spanID
		car := NewMapCarrier()
		car.Map["traceparent"] = tp.TraceParent
		prop := autoprop.NewTextMapPropagator()
		return prop.Extract(ctx, car)
	} else {
		// The property injector will fail silently, so we need to carry these ourselves
		// traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
		ctx = context.WithValue(ctx, "traceparent", tp.TraceParent)
		return ctx
	}
}

func CtxToWfState(ctx context.Context, config *Config, state *model.WorkflowState) {
	tp := CtxToTraceParams(ctx, config)
	state.TraceParent = tp.TraceParent
}
