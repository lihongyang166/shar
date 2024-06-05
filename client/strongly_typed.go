package client

import (
	"context"
	"fmt"
	"github.com/goccy/go-yaml"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/model"
	"os"
	"reflect"
)

// ParseTaskSpecFromFile reads a task specification from a file and parses it into a *model.TaskSpec.
// It returns the parsed task specification or an error if the file cannot be read or the task specification cannot be parsed.
func ParseTaskSpecFromFile(path string) (*model.TaskSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("os failed to read file %s: %w", path, err)
	}
	spec, err := ParseTaskSpec(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse task spec: %w", err)
	}
	return spec, nil
}

// ParseTaskSpec parses a yaml task spec string into a task spec proto.
func ParseTaskSpec(taskSpec string) (*model.TaskSpec, error) {
	spec := &model.TaskSpec{}
	err := yaml.Unmarshal([]byte(taskSpec), spec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal spec yaml: %v", err)
	}
	return spec, nil
}

// RegisterTaskWithSpecFile reads a task specification from a file, parses it into a *model.TaskSpec,
// and registers a task with the parsed specification and a function.
// It takes a context, a *Client, a specFile string, and a function as parameters.
// The function fn takes a context, a task.JobClient, and a parameter of type T, and returns a result of type U.
// The function returns an error if there is an error parsing the task specification from the file,
// registering the task, or any other error that occurs.
// This function is a wrapper around the ParseTaskSpecFromFile and RegisterTaskWithSpec functions.
// The specFile parameter is the path to the file containing the task specification.
// The fn parameter is the function to be executed for the registered task.
// The context parameter is used for passing cancellation signals and deadlines between function calls.
func RegisterTaskWithSpecFile[T, U any](ctx context.Context, c *Client, specFile string, fn func(context.Context, task.JobClient, T) (U, error)) error {
	spec, err := ParseTaskSpecFromFile(specFile)
	if err != nil {
		return fmt.Errorf("parse task spec from file: %w", err)
	}
	if err := RegisterTaskWithSpec(ctx, c, spec, fn); err != nil {
		return fmt.Errorf("register task: %w", err)
	}
	return nil
}

// RegisterTaskWithSpec registers a task with the provided task specification and function.
// It takes a context, a *Client, a *model.TaskSpec, and a function as parameters.
// The function fn takes a context, a task.JobClient, and a parameter of type T,
// and returns a result of type U and an error. The function should handle the task execution logic.
// Before registering the task function, it validates the function parameters against the task specification.
// It returns an error if there is any validation error or if the task function registration fails.
func RegisterTaskWithSpec[T, U any](ctx context.Context, c *Client, spec *model.TaskSpec, fn func(context.Context, task.JobClient, T) (U, error)) error {
	inMapping, outMapping, err := validateFnParams(spec, fn)
	if err != nil {
		return fmt.Errorf("validate task function params: %w", err)
	}
	if err := c.registerTaskFunction(ctx, spec, &task.FnDef{Fn: fn, Type: task.ExecutionTypeTyped, InMapping: inMapping, OutMapping: outMapping}); err != nil {
		return fmt.Errorf("register typed task function: %w", err)
	}
	return nil
}

// RegisterProcessComplete registers a process completion function for a specific process name.
// The function takes a context.Context, a generic input parameter T, *model.Error, and model.CancellationState as its arguments.
// It returns an error if the input parameter is not a struct or if there is an error registering the process completion function.
// The function stores the process completion task definition in the Client's proCompleteTasks map.
//
// Example usage:
// err := client.RegisterProcessComplete(ctx, cl, "SimpleProcess", d.processEnd)
func RegisterProcessComplete[T any](ctx context.Context, c *Client, processName string, fn func(context.Context, T, *model.Error, model.CancellationState)) error {
	inParam := reflect.TypeOf(fn).In(1)
	if inParam.Kind() != reflect.Struct {
		return fmt.Errorf("input parameter must be a struct")
	}
	mapping := getMapping(inParam)

	c.proCompleteTasks[processName] = &task.FnDef{Fn: fn, Type: task.ExecutionTypeTyped, InMapping: mapping}
	return nil
}

func validateFnParams[T, U any](spec *model.TaskSpec, fn func(context.Context, task.JobClient, T) (U, error)) (map[string]string, map[string]string, error) {
	inParam := reflect.TypeOf(fn).In(2)
	outParam := reflect.TypeOf(fn).Out(0)

	if inParam.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("input parameter must be a struct")
	}
	if outParam.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("input parameter must be a struct")
	}
	inMapping, err := validateParamsToSpec(inParam, spec.Parameters.Input)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid input parameter: %w", err)
	}
	outMapping, err := validateParamsToSpec(outParam, spec.Parameters.Output)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid output parameter: %w", err)
	}
	return inMapping, outMapping, nil
}

func validateParamsToSpec(s reflect.Type, input []*model.Parameter) (map[string]string, error) {
	mapping := getMapping(s)
	for _, p := range input {
		f, ok := s.FieldByName(mapping[p.Name])
		if !ok {
			return nil, fmt.Errorf("field %s not found", p.Name)
		}
		fmt.Println(f.Type)
	}
	return mapping, nil
}

func getMapping(s reflect.Type) map[string]string {
	numFields := s.NumField()
	mapping := make(map[string]string, numFields)
	for i := 0; i < numFields; i++ {
		f := s.Field(i)
		k := reflect.StructTag.Get(f.Tag, "shar")
		if len(k) == 0 {
			mapping[f.Name] = f.Name
			continue
		}
		mapping[k] = f.Name
	}
	return mapping
}
