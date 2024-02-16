package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/model"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"log/slog"
)

// CtxToNatsMsg injects traceID and spanID into a NATS message based on the provided context and configuration.
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
		traceParent := ctx.Value(ctxkey.Traceparent)
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

// WithNewDefaultTraceId returns a PropagateOption function that sets the default trace ID to a newly generated trace ID.
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

// NatsMsgToCtx injects traceID and spanID into a context based on the provided NATS message, configuration, and options.
// If telemetry is enabled in the host application, it utilizes the NATS message headers to inject the traceID and spanID.
// If the traceparent header is missing from the message and telemetry is enabled, it logs an error and creates a new traceparent header using a new traceID.
// If telemetry is not enabled, it carries the traceparent header manually based on the traceparent header present in the message or the default trace ID specified in the options.
// The function returns the updated context with the traceID and spanID injected.
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
		ctx = context.WithValue(ctx, ctxkey.Traceparent, traceContext)
		return ctx
	}
}

// NewTraceParent generates a traceparent string based on the provided trace ID.
// The traceparent format follows the W3C Trace Context specification (https://www.w3.org/TR/trace-context/).
// Format: 00-{traceID}-{spanID}-00
func NewTraceParent(traceID []byte) string {
	return "00-" + hex.EncodeToString(traceID) + "-1000000000000001-00"
}

// TraceParams represents parameters related to tracing
type TraceParams struct {
	TraceParent string
}

// CtxToTraceParams extracts traceParent value from the context and returns a TraceParams object containing the traceParent.
// If telemetry is enabled in the host application, it injects the traceID and spanID into the TraceParams object.
// It utilizes a MapCarrier to inject the traceID and spanID into the context using autoprop.TextMapPropagator.
// If telemetry is not enabled, it retrieves the traceParent value from the context and sets it in the TraceParams object.
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
		traceParent := ctx.Value(ctxkey.Traceparent)
		if traceParent != nil {
			ret.TraceParent = traceParent.(string)
		}
	}
	return ret
}

// TraceParamsToCtx injects traceID and spanID into a context based on the provided configuration and trace parameters.
// If telemetry is enabled in the host application, it utilizes a MapCarrier to inject the traceparent into the context.
// Otherwise, it manually adds the traceparent value to the context.
// Returns the updated context.
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
		ctx = context.WithValue(ctx, ctxkey.Traceparent, tp.TraceParent)
		return ctx
	}
}

// CtxToWfState injects traceID and spanID from the context into a WorkflowState object based on the provided configuration.
// If telemetry is enabled in the configuration, the traceID and spanID will be extracted from the context and assigned to the TraceParent field of the WorkflowState object.
// Otherwise, the traceID and spanID are expected to be stored in the context under ctxkey.Traceparent, and they will be assigned to the TraceParent field of the WorkflowState object.
func CtxToWfState(ctx context.Context, config *Config, state *model.WorkflowState) {
	tp := CtxToTraceParams(ctx, config)
	state.TraceParent = tp.TraceParent
}
