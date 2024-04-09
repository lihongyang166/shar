package client_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sharClient "gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
)

func TestGetTaskUIDFromSpec(t *testing.T) {
	spec := &model.TaskSpec{
		Version: "0.0.1",
		Kind:    "ServiceTask",
		Metadata: &model.TaskMetadata{
			Type:    "ServiceTask",
			Version: "0.0.1",
		},
		Behaviour:  nil,
		Parameters: nil,
		Events:     nil,
	}

	client := sharClient.New()
	require.NotNil(t, client)
	uid, err := client.GetTaskUIDFromSpec(spec)
	require.NoError(t, err)
	require.Equal(t, "HsxtXJTAWZTOvs1KYSPkRA2sGakL0ScKNejWmDvPKoz", uid)
}

func TestGetTaskUIDFromSpecChangesWithVersion(t *testing.T) {
	spec := &model.TaskSpec{
		Version: "0.0.1",
		Kind:    "ServiceTask",
		Metadata: &model.TaskMetadata{
			Type:    "ServiceTask",
			Version: "0.0.1",
		},
		Behaviour:  nil,
		Parameters: nil,
		Events:     nil,
	}

	client := sharClient.New()
	require.NotNil(t, client)

	firstUID, err := client.GetTaskUIDFromSpec(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, firstUID)
	assert.Equal(t, "HsxtXJTAWZTOvs1KYSPkRA2sGakL0ScKNejWmDvPKoz", firstUID)

	spec.Version = "0.0.2"

	secondUID, err := client.GetTaskUIDFromSpec(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, secondUID)
	assert.NotEqual(t, firstUID, secondUID)
}
