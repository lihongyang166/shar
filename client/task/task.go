package task

import (
	"context"
	"gitlab.com/shar-workflow/shar/model"
	"log/slog"
)

// JobClient represents a client that is sent to all service tasks to facilitate logging.
type JobClient interface {
	LogClient
	OriginalVars() (input map[string]interface{}, output map[string]interface{})
}

// LogClient represents a client which is capable of logging to the SHAR infrastructure.
type LogClient interface {
	// Log logs to the underlying SHAR infrastructure.
	Log(ctx context.Context, level slog.Level, message string, attrs map[string]string) error
}

// MessageClient represents a client which supports logging and sending Workflow Messages to the underlying SHAR infrastructure.
type MessageClient interface {
	LogClient
	// SendMessage sends a Workflow Message
	SendMessage(ctx context.Context, name string, key any, vars model.Vars) error
}

// ServiceFn provides the signature for service task functions.
type ServiceFn func(ctx context.Context, client JobClient, vars model.Vars) (model.Vars, error)

// ProcessTerminateFn provides the signature for process terminate functions.
type ProcessTerminateFn func(ctx context.Context, vars model.Vars, wfError *model.Error, endState model.CancellationState)

// SenderFn provides the signature for functions that can act as Workflow Message senders.
type SenderFn func(ctx context.Context, client MessageClient, vars model.Vars) error

// ExecutionType defines the style of execution required for a function e.g. Strongly Typed, or vars Map[string]interface
type ExecutionType int

const (
	// ExecutionTypeVars signals a function is loosely typed.
	ExecutionTypeVars ExecutionType = iota
	// ExecutionTypeTyped signals a function is strongly typed.
	ExecutionTypeTyped
)

// FnDef is a general definition of a function including any mapping needed to call it.
type FnDef struct {
	Type       ExecutionType
	Fn         any
	OutMapping map[string]string
	InMapping  map[string]string
}
