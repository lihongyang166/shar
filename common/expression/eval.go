package expression

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/common/logx"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
)

// Variable contains metadata about a variable.
type Variable struct {
	Name string
}

// Engine represents an expression engine implementation.
type Engine interface {
	// Eval evaluates an expression given a set of variables and returns a generic type.
	Eval(ctx context.Context, expr string, vars map[string]interface{}) (interface{}, error)
	// GetVariables returns a list of variables mentioned in an expression
	GetVariables(ctx context.Context, expr string) ([]Variable, error)
}

// Eval evaluates an expression given a set of variables and returns a generic type.
func Eval[T any](ctx context.Context, eng Engine, exp string, vars map[string]interface{}) (retval T, reterr error) { //nolint:ireturn
	defer func() {
		if err := recover(); err != nil {
			retval = *new(T)
			reterr = logx.Err(ctx, "panic: evaluate expression", &errors2.ErrWorkflowFatal{Err: err.(error)}, "expression", exp)
		}
	}()
	res, err := eng.Eval(ctx, exp, vars)
	if err != nil {
		return *new(T), fmt.Errorf("evaluate expression: %w", err)
	}
	return res.(T), nil
}

// EvalAny evaluates an expression given a set of variables and returns a 'boxed' interface type.
func EvalAny(ctx context.Context, eng Engine, exp string, vars map[string]interface{}) (retval interface{}, reterr error) { //nolint:ireturn
	defer func() {
		if err := recover(); err != nil {
			reterr = logx.Err(ctx, "panic: evaluate expression", &errors2.ErrWorkflowFatal{Err: err.(error)}, "expression", exp)
		}
	}()
	res, err := eng.Eval(ctx, exp, vars)
	if err != nil {
		return nil, fmt.Errorf("evaluate expression: %w", err)
	}
	return res, nil
}

// GetVariables returns a list of variables mentioned in an expression
func GetVariables(ctx context.Context, eng Engine, exp string) ([]Variable, error) {
	res, err := eng.GetVariables(ctx, exp)
	if err != nil {
		return nil, fmt.Errorf("get expression variables: %w", err)
	}
	return res, nil
}
