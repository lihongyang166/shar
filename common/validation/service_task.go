package validation

import (
	"fmt"
	"github.com/antonmedv/expr"
	"gitlab.com/shar-workflow/shar/model"
)

func ValidateTaskSpec(td *model.TaskSpec) error {
	if td.Version != "1.0" {
		return fmt.Errorf("spec version %s not found: %w", td.Version, ErrTaskSpecVersion)
	}

	// Metadata
	if td.Metadata == nil {
		return fmt.Errorf("task metadata section not found: %w", ErrServiceTaskNoMetadata)
	}
	if err := validName(td.Metadata.Type); err != nil {
		return fmt.Errorf("task type name is not valid: %w", err)
	}
	if err := validVersion(td.Metadata.Version); err != nil {
		return fmt.Errorf("task version is not valid: %w", err)
	}
	if td.Metadata.EstimatedMaxDuration == 0 {
		return fmt.Errorf("task estimated duration not provided: %w", ErrServiceTaskDuration)
	}

	// Behaviour
	if td.Behaviour == nil {
		return fmt.Errorf("task behaviour section not found: %w", ErrServiceTaskNoMetadata)
	}
	if td.Behaviour.DefaultRetry == nil {
		return fmt.Errorf("no default retry given: %w", ErrNoDefaultRetry)
	}

	// Parameters
	if td.Behaviour == nil {
		return fmt.Errorf("task parameters section not found: %w", ErrServiceTaskNoParameters)
	}
	for _, v := range td.Parameters.Input {
		if v.ValidateExpr != "" {
			if _, err := expr.Compile(v.ValidateExpr); err != nil {
				return fmt.Errorf("%s has a bad validation expression: %w", v.Name, err)
			}
		}
		if err := validExpName(v.Name); err != nil {
			return fmt.Errorf("input name '%s'is not valid: %w", v.Name, err)
		}
		if v.Example == "" {
			if td.Behaviour.Mock {
				return fmt.Errorf("task is placeholder, but no example was given", ErrServiceMockValue)
			}
		} else {
			if _, err := expr.Compile(v.Example); err != nil {
				return fmt.Errorf("example value for '%s'is not valid: %w", v.Name, err)
			}
		}
	}

	return nil
}

func validVersion(version string) interface{} {
	return nil
}

func validName(name string) error {
	return nil
}

func validExpName(name string) error {
	return nil
}
