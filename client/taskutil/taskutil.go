package taskutil

import (
	"context"
	"fmt"
	task2 "gitlab.com/shar-workflow/shar/common/task"
	"os"

	"github.com/goccy/go-yaml"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/task"
	"gitlab.com/shar-workflow/shar/model"
)

// LoadSpecFromBytes loads and parses a task.yaml file to a *model.TaskSpec type.
func LoadSpecFromBytes(c *client.Client, buf []byte) (*model.TaskSpec, error) {
	spec := &model.TaskSpec{}
	err := yaml.Unmarshal(buf, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task spec: %w", err)
	}
	return spec, nil
}

// LoadTaskYaml registers a service task with a task spec.
// It returns the unique identifier of the registered task.
func LoadTaskYaml(ctx context.Context, c *client.Client, taskYaml []byte) (uid string, err error) {
	yml, err := LoadSpecFromBytes(c, taskYaml)
	if err != nil {
		return uid, fmt.Errorf("RegisterTaskFromYaml: %w", err)
	}
	if err := c.StoreTask(ctx, yml); err != nil {
		return uid, fmt.Errorf("RegisterTaskYaml: %w", err)
	}
	if uid, err = task2.CreateUID(yml); err != nil {
		return uid, fmt.Errorf("RegisterTaskYaml: %w", err)
	}
	return uid, nil
}

// RegisterTaskFunctionFromYaml registers a service task with a task spec from YAML.
// It first loads and parses the task YAML,
// and then calls the RegisterTaskFunction method on the client object to register the task.
func RegisterTaskFunctionFromYaml(ctx context.Context, c *client.Client, taskYaml []byte, fn task.ServiceFn) (string, error) {
	yml, err := LoadSpecFromBytes(c, taskYaml)
	if err != nil {
		return "", fmt.Errorf("RegisterTaskFromYaml: %w", err)
	}
	if err := c.RegisterTaskFunction(ctx, yml, fn); err != nil {
		return "", fmt.Errorf("RegisterTaskYaml: %w", err)
	}
	uid, err := task2.CreateUID(yml)
	if err != nil {
		return "", fmt.Errorf("RegisterTaskYaml: %w", err)
	}
	return uid, nil
}

// LoadTaskFromYamlFile registers a service task with a task spec.
func LoadTaskFromYamlFile(ctx context.Context, cl *client.Client, yamlFile string) (string, error) {
	sb, err := os.ReadFile(yamlFile)
	if err != nil {
		return "", fmt.Errorf("register task yaml file: %w", err)
	}
	uid, err := LoadTaskYaml(ctx, cl, sb)
	if err != nil {
		return "", fmt.Errorf("RegisterTaskYamlFile: %w", err)
	}
	return uid, nil
}

// RegisterTaskFunctionFromYamlFile registers a service task function from a task spec YAML file.
// It first loads and parses the task YAML,
// and then calls the RegisterTaskFunction method on the client object to register the task function.
func RegisterTaskFunctionFromYamlFile(ctx context.Context, c *client.Client, yamlFile string, fn task.ServiceFn) (string, error) {
	sb, err := os.ReadFile(yamlFile)
	if err != nil {
		return "", fmt.Errorf("register task yaml file: %w", err)
	}
	uid, err := RegisterTaskFunctionFromYaml(ctx, c, sb, fn)
	if err != nil {
		return "", fmt.Errorf("RegisterTaskFunctionFromYaml: %w", err)
	}
	return uid, nil
}
