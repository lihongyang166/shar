package client

import "github.com/nats-io/nats.go"

// ConnectOptions represents the options for connecting to a NATS server.
// It includes the NATS options and the JetStream domain.
type ConnectOptions struct {
	natsOptions     []nats.Option
	jetStreamDomain string
}

// ConnectOption is a function type that modifies a ConnectOptions struct.
type ConnectOption func(*ConnectOptions)

// WithNatsOption allows nats options to be provided to a connection
func WithNatsOption(opt nats.Option) ConnectOption {
	return func(o *ConnectOptions) {
		o.natsOptions = append(o.natsOptions, opt)
	}
}

// WithJetStreamDomain sets the JetStream domain for ConnectOptions.
func WithJetStreamDomain(domain string) ConnectOption {
	return func(o *ConnectOptions) {
		o.jetStreamDomain = domain
	}
}
