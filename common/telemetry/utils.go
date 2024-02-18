package telemetry

import (
	"fmt"
	"go.opentelemetry.io/otel/trace"
)

// SetTraceParentSpanID sets the span portion of a W3C traceparent and returns a new W3C traceparent
func SetTraceParentSpanID(traceparent string, spanID string) string {
	base := []rune(traceparent)
	copy(base[36:52], []rune(spanID))
	return string(base)
}

// GetTraceparentTraceAndSpan returns a trace and span from a W3C traceparent
func GetTraceparentTraceAndSpan(traceparent string) (string, string) {
	return traceparent[3:35], traceparent[36:52]
}

func TraceIDFromTraceparent(traceparent string) (trace.TraceID, error) {
	traceID, err := trace.TraceIDFromHex(traceparent[3:35])
	if err != nil {
		return trace.TraceID{}, fmt.Errorf("trace id from hex: %w", err)
	}
	return traceID, nil
}
