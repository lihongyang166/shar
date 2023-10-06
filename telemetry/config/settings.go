package config

import (
	"fmt"
	"github.com/caarlos0/env/v6"
)

// These constants define the valid trace export formats
const (
	Otlp   = "otlp"
	Jaeger = "jaeger"
)

// Settings is the settings provider for telemetry.
type Settings struct {
	NatsURL              string `env:"NATS_URL" envDefault:"nats://127.0.0.1:4222"`
	JaegerURL            string `env:"JAEGER_URL" envDefault:"http://localhost:14268/api/traces"`
	LogLevel             string `env:"SHAR_LOG_LEVEL" envDefault:"debug"`
	TraceDataFormat      string `env:"TRACE_DATA_FORMAT"`
	OTLPEndpoint         string `env:"OTLP_URL" envDefault:"localhost:4318"`
	OTLPEndpointIsSecure bool   `env:"OTLP_IS_SECURE" envDefault:"false"`
}

// GetEnvironment pulls the active settings into a settings struct.
func GetEnvironment() (*Settings, error) {
	cfg := &Settings{TraceDataFormat: Jaeger}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse environment settings: %w", err)
	}
	return cfg, nil
}
