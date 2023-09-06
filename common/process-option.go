package common

import (
	"github.com/nats-io/nats.go"
	"time"
)

type ProcessOpts struct {
	BackoffCalc BackoffFn
}

type ProcessOption interface {
	Set(opts *ProcessOpts)
}

type BackoffFn func(msg *nats.Msg) (time.Time, error)

type backoffProcessOption struct {
	fn BackoffFn
}

func (b backoffProcessOption) Set(opts *ProcessOpts) {
	opts.BackoffCalc = b.fn
}

func WithBackoffFn(fn BackoffFn) ProcessOption {
	return backoffProcessOption{fn: fn}
}
