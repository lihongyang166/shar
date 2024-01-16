package telemetry

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"time"
)

func SetUpHTTP(ctx context.Context, spanOtlUri string, resourceName string) (sdktrace.SpanProcessor, error) {

	if spanOtlUri != "" {
		res, err := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceName(resourceName),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("creating resource: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		// Set up a trace exporter
		traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(spanOtlUri))
		if err != nil {
			return nil, fmt.Errorf("creating trace exporter: %w", err)
		}

		// Register the trace exporter with a TracerProvider, using a batch
		// span processor to aggregate spans before export.
		bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
			sdktrace.WithSpanProcessor(bsp),
		)
		otel.SetTracerProvider(tracerProvider)
		return bsp, nil
	}
	return nil, nil
}
