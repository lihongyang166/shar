package workflow

import (
	"context"
	"gitlab.com/shar-workflow/shar/model"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestLaunchWorkflow(t *testing.T) {
	ctx := context.Background()

	_, eng, svc, wf := setupTestWorkflow(t, "simple-workflow.bpmn")

	process := wf.Process["WorkflowDemo"]
	els := make(map[string]*model.Element)
	indexProcessElements(process.Elements, els)

	svc.On("GetLatestVersion", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("string")).
		Once().
		Return("test-workflow-id", nil)

	svc.On("GetWorkflow", mock.AnythingOfType("*context.emptyCtx"), "test-workflow-id").
		Once().
		Return(wf, nil)

	svc.On("CreateWorkflowInstance", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*model.WorkflowInstance")).
		Once().
		Return(&model.WorkflowInstance{
			WorkflowInstanceId:       "test-workflow-instance-id",
			ParentWorkflowInstanceId: nil,
			ParentElementId:          nil,
			WorkflowId:               "test-workflow-id",
		}, nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Workflow.Execute", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "StartEvent")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "startEvent")
		}).
		Return(nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Traversal.Execute", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowId, "test-workflow-id")
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowInstanceId, "test-workflow-instance-id")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "serviceTask")
			assert.NotEmpty(t, args[2].(*model.WorkflowState).TrackingId)
		}).
		Return(nil)

	wfiid, err := eng.Launch(ctx, "TestWorkflow", []byte{})
	assert.NoError(t, err)
	assert.Equal(t, "test-workflow-instance-id", wfiid)
	svc.AssertExpectations(t)

}

func TestTraversal(t *testing.T) {
	ctx := context.Background()

	_, eng, svc, wf := setupTestWorkflow(t, "simple-workflow.bpmn")

	process := wf.Process["WorkflowDemo"]
	els := make(map[string]*model.Element)
	indexProcessElements(process.Elements, els)

	wfi := &model.WorkflowInstance{
		WorkflowInstanceId:       "test-workflow-instance-id",
		ParentWorkflowInstanceId: nil,
		ParentElementId:          nil,
		WorkflowId:               "test-workflow-id",
	}

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Traversal.Execute", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowId, "test-workflow-id")
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowInstanceId, "test-workflow-instance-id")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "serviceTask")
			assert.NotEmpty(t, args[2].(*model.WorkflowState).TrackingId)
		}).
		Return(nil)

	err := eng.traverse(ctx, wfi, els["StartEvent"].Outbound, els, []byte{})
	assert.NoError(t, err)
	svc.AssertExpectations(t)

}

func TestActivityProcessorServiceTask(t *testing.T) {
	ctx := context.Background()

	_, eng, svc, wf := setupTestWorkflow(t, "simple-workflow.bpmn")

	process := wf.Process["WorkflowDemo"]
	els := make(map[string]*model.Element)
	indexProcessElements(process.Elements, els)

	svc.On("GetWorkflowInstance", mock.AnythingOfType("*context.emptyCtx"), "test-workflow-instance-id").
		Once().
		Return(&model.WorkflowInstance{
			WorkflowInstanceId:       "test-workflow-instance-id",
			ParentWorkflowInstanceId: nil,
			ParentElementId:          nil,
			WorkflowId:               "test-workflow-id",
		}, nil)

	svc.On("GetWorkflow", mock.AnythingOfType("*context.emptyCtx"), "test-workflow-id").
		Once().
		Return(wf, nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Activity.Execute", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "serviceTask")
		}).
		Return(nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Traversal.Complete", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowId, "test-workflow-id")
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowInstanceId, "test-workflow-instance-id")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "serviceTask")
			assert.NotEmpty(t, args[2].(*model.WorkflowState).TrackingId)
		}).
		Return(nil)

	svc.On("CreateJob", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[1].(*model.WorkflowState).WorkflowId, "test-workflow-id")
			assert.Equal(t, args[1].(*model.WorkflowState).WorkflowInstanceId, "test-workflow-instance-id")
			assert.Equal(t, args[1].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[1].(*model.WorkflowState).ElementType, "serviceTask")
		}).
		Return("test-job-id", nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Activity.Complete", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowId, "test-workflow-id")
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowInstanceId, "test-workflow-instance-id")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "serviceTask")
			//assert.NotEmpty(t, args[2].(*model.WorkflowState).TrackingId)
		}).
		Return(nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Job.Execute.ServiceTask", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowId, "test-workflow-id")
			assert.Equal(t, args[2].(*model.WorkflowState).WorkflowInstanceId, "test-workflow-instance-id")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementId, "Step1")
			assert.Equal(t, args[2].(*model.WorkflowState).ElementType, "serviceTask")
			//assert.NotEmpty(t, args[2].(*model.WorkflowState).TrackingId)
		}).
		Return(nil)
	trackingID := ksuid.New().String()
	err := eng.activityProcessor(ctx, "test-workflow-instance-id", els["Step1"].Id, trackingID, []byte{})
	assert.NoError(t, err)
	svc.AssertExpectations(t)

}

func TestCompleteJobProcessor(t *testing.T) {
	ctx := context.Background()

	_, eng, svc, wf := setupTestWorkflow(t, "simple-workflow.bpmn")

	process := wf.Process["WorkflowDemo"]
	els := make(map[string]*model.Element)
	indexProcessElements(process.Elements, els)

	trackingID := ksuid.New().String()
	svc.On("GetJob", mock.AnythingOfType("*context.emptyCtx"), "test-job-id").
		Once().
		Return(&model.WorkflowState{
			WorkflowId:         "test-workflow-id",
			WorkflowInstanceId: "test-workflow-instance-id",
			ElementId:          "Step1",
			ElementType:        "serviceTask",
			TrackingId:         trackingID,
			Execute:            nil,
			State:              "",
			Condition:          nil,
			UnixTimeNano:       0,
			Vars:               []byte{},
		}, nil)

	svc.On("GetWorkflowInstance", mock.AnythingOfType("*context.emptyCtx"), "test-workflow-instance-id").
		Once().
		Return(&model.WorkflowInstance{
			WorkflowInstanceId:       "test-workflow-instance-id",
			ParentWorkflowInstanceId: nil,
			ParentElementId:          nil,
			WorkflowId:               "test-workflow-id",
		}, nil)

	svc.On("GetWorkflow", mock.AnythingOfType("*context.emptyCtx"), "test-workflow-id").
		Once().
		Return(wf, nil)

	svc.On("PublishWorkflowState", mock.AnythingOfType("*context.emptyCtx"), "WORKFLOW.State.Traversal.Execute", mock.AnythingOfType("*model.WorkflowState")).
		Once().
		Run(func(args mock.Arguments) {
			assert.Equal(t, "test-workflow-id", args[2].(*model.WorkflowState).WorkflowId)
			assert.Equal(t, "test-workflow-instance-id", args[2].(*model.WorkflowState).WorkflowInstanceId)
			assert.Equal(t, "EndEvent", args[2].(*model.WorkflowState).ElementId)
			assert.Equal(t, "endEvent", args[2].(*model.WorkflowState).ElementType)
			assert.NotEmpty(t, args[2].(*model.WorkflowState).TrackingId)
		}).
		Return(nil)

	err := eng.completeJobProcessor(ctx, "test-job-id", []byte{})
	assert.NoError(t, err)
	svc.AssertExpectations(t)

}
