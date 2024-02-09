package middleware

import (
	"context"
	"github.com/nats-io/nats.go"
)

type Send func(ctx context.Context, msg *nats.Msg) error
type Receive func(ctx context.Context, msg *nats.Msg) (context.Context, error)
