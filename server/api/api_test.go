package api

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/model"
	"testing"
)

func TestFatalErrorKeyPrefixBuilderWithAllSegments(t *testing.T) {
	wfName := "wfName"
	executionId := "executionId"
	processInstanceId := "processInstanceId"

	req := &model.GetFatalErrorRequest{
		WfName:            wfName,
		ExecutionId:       executionId,
		ProcessInstanceId: processInstanceId,
	}

	keyPrefix := fatalErrorKeyPrefixBuilder(req)

	assert.Equal(t, fmt.Sprintf("%s.%s.%s", wfName, executionId, processInstanceId), keyPrefix)
}

func TestFatalErrorKeyPrefixBuilderWithPartialSegments(t *testing.T) {
	wfName := "wfName"
	executionId := ""
	processInstanceId := "processInstanceId"

	req := &model.GetFatalErrorRequest{
		WfName:            wfName,
		ExecutionId:       executionId,
		ProcessInstanceId: processInstanceId,
	}

	keyPrefix := fatalErrorKeyPrefixBuilder(req)

	assert.Equal(t, fmt.Sprintf("%s.%s.%s", wfName, "*", processInstanceId), keyPrefix)
}
