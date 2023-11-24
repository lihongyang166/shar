package logx

import (
	"context"
	"fmt"
	"github.com/agoda-com/opentelemetry-go/otelslog"
	"github.com/agoda-com/opentelemetry-logs-go/exporters/otlp/otlplogs"
	sdk "github.com/agoda-com/opentelemetry-logs-go/sdk/logs"
	"github.com/go-logr/logr"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"os"
)

// ContextKey is a custom type to avoid context collision.
type ContextKey string

const (
	CorrelationHeader     = "cid"             // CorrelationHeader is the name of the nats message header for transporting the correlationID.
	CorrelationContextKey = ContextKey("cid") // CorrelationContextKey is the name of the context key used to store the correlationID.
	EcoSystemLoggingKey   = "eco"             // EcoSystemLoggingKey is the name of the logging key used to store the current ecosystem.
	SubsystemLoggingKey   = "sub"             // SubsystemLoggingKey is the name of the logging key used to store the current subsystem.
	CorrelationLoggingKey = "cid"             // CorrelationLoggingKey is the name of the logging key used to store the correlation id.
	AreaLoggingKey        = "loc"             // AreaLoggingKey is the name of the logging key used to store the functional area.
)

// Err will output error message to the log and return the error with additional attributes.
func Err(ctx context.Context, message string, err error, atts ...any) error {
	l, err2 := logr.FromContext(ctx)
	if err2 != nil {
		return fmt.Errorf("error: %w", err)
	}
	if l.Enabled() {
		l.Error(err, message, atts)
	}
	return fmt.Errorf(message+" %s : %w", fmt.Sprint(atts...), err)
}

func newResource() *resource.Resource {
	hostName, _ := os.Hostname()
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("shar-server"),
		semconv.ServiceVersion("1.0.0"),
		semconv.HostName(hostName),
	)
}

type LoggingSpan struct {
	trace.Span
	IsRecordin    bool
	SpanCtx       trace.SpanContext
	TracerProvidr trace.TracerProvider
}

func (ls *LoggingSpan) End(_ ...trace.SpanEndOption) {

}

func (ls *LoggingSpan) AddEvent(_ string, _ ...trace.EventOption) {

}

func (ls *LoggingSpan) IsRecording() bool {
	return ls.IsRecordin
}

func (ls *LoggingSpan) RecordError(_ error, _ ...trace.EventOption) {

}

func (ls *LoggingSpan) SpanContext() trace.SpanContext {
	return ls.SpanCtx
}

func (ls *LoggingSpan) SetStatus(_ codes.Code, _ string) {

}

func (ls *LoggingSpan) SetName(_ string) {

}

func (ls *LoggingSpan) SetAttributes(_ ...attribute.KeyValue) {

}

func (ls *LoggingSpan) TracerProvider() trace.TracerProvider {
	return ls.TracerProvidr
}

func SetDefault(handler string, level slog.Level, addSource bool, ecosystem string) func() error {
	var h slog.Handler
	var shutdownFn func() error

	switch handler {
	case "otel":
		ctx := context.Background()

		// configure opentelemetry logger provider
		logExporter, _ := otlplogs.NewExporter(ctx)
		loggerProvider := sdk.NewLoggerProvider(
			sdk.WithBatcher(logExporter),
			sdk.WithResource(newResource()),
		)
		// gracefully shutdown logger to flush accumulated signals before program finish
		shutdownFn = func() error { return loggerProvider.Shutdown(ctx) }

		h = otelslog.NewOtelHandler(loggerProvider, &otelslog.HandlerOptions{})

	default:
		o := &slog.HandlerOptions{
			AddSource:   addSource,
			Level:       level,
			ReplaceAttr: nil,
		}
		h = slog.NewTextHandler(os.Stdout, o)
		shutdownFn = func() error { return nil } //noop for shutting down text handler
	}

	slog.SetDefault(slog.New(h).With(slog.String(EcoSystemLoggingKey, ecosystem)))

	return shutdownFn
}

// NatsMessageLoggingEntrypoint returns a new logger and a context containing the logger for use when a new NATS message arrives.
func NatsMessageLoggingEntrypoint(ctx context.Context, subsystem string, hdr nats.Header) (context.Context, *slog.Logger) {
	cid := hdr.Get(CorrelationHeader)
	return loggingEntrypoint(ctx, subsystem, cid)
}

type contextLoggerKey string

var ctxLogKey contextLoggerKey = "__log"

// ContextWith obtains a new logger with an area parameter.  Typically it should be used when obtaining a logger within a programmatic boundary.
func ContextWith(ctx context.Context, area string) (context.Context, *slog.Logger) {
	logger := FromContext(ctx).With(AreaLoggingKey, area)
	return NewContext(ctx, logger), logger
}

// NewContext creates a new context with the specified logger
func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLogKey, logger)
}

// FromContext obtains a logger from the context or takes the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	var cl *slog.Logger
	l := ctx.Value(ctxLogKey)
	if l == nil {
		cl = slog.Default()
	} else {
		cl = l.(*slog.Logger)
	}
	return cl
}

func loggingEntrypoint(ctx context.Context, subsystem string, correlationId string) (context.Context, *slog.Logger) {
	logger := FromContext(ctx).With(slog.String(SubsystemLoggingKey, subsystem), slog.String(CorrelationLoggingKey, correlationId))
	ctx = NewContext(ctx, logger)
	ctx = context.WithValue(ctx, CorrelationContextKey, correlationId)
	return ctx, logger
}
