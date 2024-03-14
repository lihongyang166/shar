package taskutil

import (
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/task"
	"gitlab.com/shar-workflow/shar/model"
)

// RegisterTaskYamlFile registers a service task with a task spec.
func RegisterTaskYamlFile(ctx context.Context, cl *client.Client, yamlFile string, fn client.ServiceFn) (uid string, err error) {
	sb, err := os.ReadFile(yamlFile)
	if err != nil {
		return uid, fmt.Errorf("register task yaml file: %w", err)
	}
	if uid, err = RegisterTaskYaml(ctx, cl, sb, fn); err != nil {
		return uid, fmt.Errorf("RegisterTaskYamlFile: %w", err)
	}
	return uid, nil
}

// LoadSpecFromBytes loads and parses a task.yaml file to a *model.TaskSpec type.
func LoadSpecFromBytes(c *client.Client, buf []byte) (*model.TaskSpec, error) {
	spec := &model.TaskSpec{}
	err := yaml.Unmarshal(buf, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task spec: %w", err)
	}
	return spec, nil
}

// RegisterTaskYaml registers a service task with a task spec.
func RegisterTaskYaml(ctx context.Context, c *client.Client, taskYaml []byte, fn client.ServiceFn) (uid string, err error) {
	yml, err := LoadSpecFromBytes(c, taskYaml)
	if err != nil {
		return uid, fmt.Errorf("RegisterTaskFromYaml: %w", err)
	}
	if err := c.RegisterTask(ctx, yml, fn); err != nil {
		return uid, fmt.Errorf("RegisterTaskYaml: %w", err)
	}
	if uid, err = task.CreateUID(yml); err != nil {
		return uid, fmt.Errorf("RegisterTaskYaml: %w", err)
	}
	return uid, nil
}
