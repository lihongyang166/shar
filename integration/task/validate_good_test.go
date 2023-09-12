package main

import (
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/common/validation"
	"gitlab.com/shar-workflow/shar/model"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestValidateGood(t *testing.T) {
	specYaml, err := os.ReadFile("../../testdata/servicetask/good.yaml")
	require.NoError(t, err)

	spec := &model.TaskSpec{}

	err = yaml.Unmarshal(specYaml, spec)
	require.NoError(t, err)
	err = validation.ValidateTaskSpec(spec)
	require.NoError(t, err)
}
