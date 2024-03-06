package middleware

import (
	"context"
	"github.com/nats-io/nats.go/jetstream"
)

// Send is the prototype for a middleware send function.
type Send func(ctx context.Context, msg jetstream.Msg) error

// Receive is the prototype for a middleware receive function.
type Receive func(ctx context.Context, msg jetstream.Msg) (context.Context, error)
