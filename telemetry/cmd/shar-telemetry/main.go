package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/telemetry/config"
	"gitlab.com/shar-workflow/shar/telemetry/server"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"os"
	"time"
)

func main() {

	// Get the configuration
	cfg, err := config.GetEnvironment()
	if err != nil {
		panic(err)
	}

	// Connect to nats
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		panic(err)
	}

	// Get Jetstream
	js, err := nc.JetStream()
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "--remove" {
		// Attempt both in case one failed last time, and deal with errors after
		err1 := js.DeleteConsumer("WORKFLOW-TELEMETRY", "Tracing")
		err2 := js.DeleteKeyValue(messages.KvTracking)
		if err1 != nil {
			panic(err1)
		}
		if err2 != nil {
			panic(err2)
		}
		return
	}

	ctx := context.Background()

	exp, err := exporterFor(ctx, cfg)
	if err != nil {
		panic(err)
	}

	// Start the server
	svr := server.New(ctx, nc, js, nats.FileStorage, exp)
	if err := svr.Listen(); err != nil {
		panic(err)
	}
	time.Sleep(100 * time.Hour)
}

// nolint:ireturn
func exporterFor(ctx context.Context, cfg *config.Settings) (server.Exporter, error) {
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.OTLPEndpoint)}
	if !cfg.OTLPEndpointIsSecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error constructing oltp exporter: %w", err)
	}
	return exporter, nil
}
