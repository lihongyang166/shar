package api

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"gitlab.com/shar-workflow/shar/model"
	options2 "gitlab.com/shar-workflow/shar/server/server/option"
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

func TestGetFatalErrorInvalidRequestWhenNoSegmentsAreSpecified(t *testing.T) {
	ctx := context.Background()

	req := &model.GetFatalErrorRequest{
		WfName:            "",
		ExecutionId:       "",
		ProcessInstanceId: "",
	}

	mockOperations := &workflow.MockOps{}
	endpoints, _ := New(mockOperations, nil, &options2.ServerOptions{})

	respChan := make(chan *model.FatalError)
	errChan := make(chan error)

	mockOperations.On("GetFatalErrors", ctx, fatalErrorKeyPrefixBuilder(req), (chan<- *model.FatalError)(respChan), (chan<- error)(errChan))
	//.
	//	Run(func(args mock.Arguments) {
	//		args.Get()
	//	})

	go func() {
		endpoints.getFatalErrors(ctx, req, respChan, errChan)
	}()

	expectedErr := <-errChan
	assert.ErrorContains(t, expectedErr, invalidGetFatalErrorRequest)
}

// TODO can Ops be decomposed into smaller structs? OR is the canonical way to do this in
// go to decompose into separate files but still hang the functions off the same struct???

func TestReturnsMatchingFatalErrors(t *testing.T) {

}
