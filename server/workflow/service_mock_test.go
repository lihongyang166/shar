// Code generated by mockery v2.14.0. DO NOT EDIT.

package workflow

import (
	context "context"

	common "gitlab.com/shar-workflow/shar/common"

	mock "github.com/stretchr/testify/mock"

	model "gitlab.com/shar-workflow/shar/model"

	services "gitlab.com/shar-workflow/shar/server/services"
)

// MockNatsService is an autogenerated mock type for the NatsService type
type MockNatsService struct {
	mock.Mock
}

// AwaitMsg provides a mock function with given fields: ctx, state
func (_m *MockNatsService) AwaitMsg(ctx context.Context, state *model.WorkflowState) error {
	ret := _m.Called(ctx, state)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *model.WorkflowState) error); ok {
		r0 = rf(ctx, state)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CloseUserTask provides a mock function with given fields: ctx, trackingID
func (_m *MockNatsService) CloseUserTask(ctx context.Context, trackingID string) error {
	ret := _m.Called(ctx, trackingID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, trackingID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Conn provides a mock function with given fields:
func (_m *MockNatsService) Conn() common.NatsConn {
	ret := _m.Called()

	var r0 common.NatsConn
	if rf, ok := ret.Get(0).(func() common.NatsConn); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(common.NatsConn)
		}
	}

	return r0
}

// CreateJob provides a mock function with given fields: ctx, job
func (_m *MockNatsService) CreateJob(ctx context.Context, job *model.WorkflowState) (string, error) {
	ret := _m.Called(ctx, job)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, *model.WorkflowState) string); ok {
		r0 = rf(ctx, job)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *model.WorkflowState) error); ok {
		r1 = rf(ctx, job)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateWorkflowInstance provides a mock function with given fields: ctx, wfInstance
func (_m *MockNatsService) CreateWorkflowInstance(ctx context.Context, wfInstance *model.WorkflowInstance) (*model.WorkflowInstance, error) {
	ret := _m.Called(ctx, wfInstance)

	var r0 *model.WorkflowInstance
	if rf, ok := ret.Get(0).(func(context.Context, *model.WorkflowInstance) *model.WorkflowInstance); ok {
		r0 = rf(ctx, wfInstance)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.WorkflowInstance)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *model.WorkflowInstance) error); ok {
		r1 = rf(ctx, wfInstance)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteJob provides a mock function with given fields: ctx, trackingID
func (_m *MockNatsService) DeleteJob(ctx context.Context, trackingID string) error {
	ret := _m.Called(ctx, trackingID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, trackingID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DestroyWorkflowInstance provides a mock function with given fields: ctx, workflowInstanceID, state, wfError
func (_m *MockNatsService) DestroyWorkflowInstance(ctx context.Context, workflowInstanceID string, state model.CancellationState, wfError *model.Error) error {
	ret := _m.Called(ctx, workflowInstanceID, state, wfError)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, model.CancellationState, *model.Error) error); ok {
		r0 = rf(ctx, workflowInstanceID, state, wfError)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetElement provides a mock function with given fields: ctx, state
func (_m *MockNatsService) GetElement(ctx context.Context, state *model.WorkflowState) (*model.Element, error) {
	ret := _m.Called(ctx, state)

	var r0 *model.Element
	if rf, ok := ret.Get(0).(func(context.Context, *model.WorkflowState) *model.Element); ok {
		r0 = rf(ctx, state)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Element)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *model.WorkflowState) error); ok {
		r1 = rf(ctx, state)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetJob provides a mock function with given fields: ctx, id
func (_m *MockNatsService) GetJob(ctx context.Context, id string) (*model.WorkflowState, error) {
	ret := _m.Called(ctx, id)

	var r0 *model.WorkflowState
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.WorkflowState); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.WorkflowState)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestVersion provides a mock function with given fields: ctx, workflowName
func (_m *MockNatsService) GetLatestVersion(ctx context.Context, workflowName string) (string, error) {
	ret := _m.Called(ctx, workflowName)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, string) string); ok {
		r0 = rf(ctx, workflowName)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, workflowName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMessageSenderRoutingKey provides a mock function with given fields: ctx, workflowName, messageName
func (_m *MockNatsService) GetMessageSenderRoutingKey(ctx context.Context, workflowName string, messageName string) (string, error) {
	ret := _m.Called(ctx, workflowName, messageName)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, string, string) string); ok {
		r0 = rf(ctx, workflowName, messageName)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, workflowName, messageName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetOldState provides a mock function with given fields: ctx, id
func (_m *MockNatsService) GetOldState(ctx context.Context, id string) (*model.WorkflowState, error) {
	ret := _m.Called(ctx, id)

	var r0 *model.WorkflowState
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.WorkflowState); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.WorkflowState)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetServiceTaskRoutingKey provides a mock function with given fields: ctx, taskName
func (_m *MockNatsService) GetServiceTaskRoutingKey(ctx context.Context, taskName string) (string, error) {
	ret := _m.Called(ctx, taskName)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, string) string); ok {
		r0 = rf(ctx, taskName)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, taskName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkflow provides a mock function with given fields: ctx, workflowID
func (_m *MockNatsService) GetWorkflow(ctx context.Context, workflowID string) (*model.Workflow, error) {
	ret := _m.Called(ctx, workflowID)

	var r0 *model.Workflow
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.Workflow); ok {
		r0 = rf(ctx, workflowID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Workflow)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, workflowID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkflowInstance provides a mock function with given fields: ctx, workflowInstanceID
func (_m *MockNatsService) GetWorkflowInstance(ctx context.Context, workflowInstanceID string) (*model.WorkflowInstance, error) {
	ret := _m.Called(ctx, workflowInstanceID)

	var r0 *model.WorkflowInstance
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.WorkflowInstance); ok {
		r0 = rf(ctx, workflowInstanceID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.WorkflowInstance)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, workflowInstanceID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkflowInstanceStatus provides a mock function with given fields: ctx, id
func (_m *MockNatsService) GetWorkflowInstanceStatus(ctx context.Context, id string) (*model.WorkflowInstanceStatus, error) {
	ret := _m.Called(ctx, id)

	var r0 *model.WorkflowInstanceStatus
	if rf, ok := ret.Get(0).(func(context.Context, string) *model.WorkflowInstanceStatus); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.WorkflowInstanceStatus)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListWorkflowInstance provides a mock function with given fields: ctx, workflowName
func (_m *MockNatsService) ListWorkflowInstance(ctx context.Context, workflowName string) (chan *model.ListWorkflowInstanceResult, chan error) {
	ret := _m.Called(ctx, workflowName)

	var r0 chan *model.ListWorkflowInstanceResult
	if rf, ok := ret.Get(0).(func(context.Context, string) chan *model.ListWorkflowInstanceResult); ok {
		r0 = rf(ctx, workflowName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *model.ListWorkflowInstanceResult)
		}
	}

	var r1 chan error
	if rf, ok := ret.Get(1).(func(context.Context, string) chan error); ok {
		r1 = rf(ctx, workflowName)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(chan error)
		}
	}

	return r0, r1
}

// ListWorkflows provides a mock function with given fields: ctx
func (_m *MockNatsService) ListWorkflows(ctx context.Context) (chan *model.ListWorkflowResult, chan error) {
	ret := _m.Called(ctx)

	var r0 chan *model.ListWorkflowResult
	if rf, ok := ret.Get(0).(func(context.Context) chan *model.ListWorkflowResult); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *model.ListWorkflowResult)
		}
	}

	var r1 chan error
	if rf, ok := ret.Get(1).(func(context.Context) chan error); ok {
		r1 = rf(ctx)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(chan error)
		}
	}

	return r0, r1
}

// OwnerID provides a mock function with given fields: name
func (_m *MockNatsService) OwnerID(name string) (string, error) {
	ret := _m.Called(name)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(name)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// OwnerName provides a mock function with given fields: id
func (_m *MockNatsService) OwnerName(id string) (string, error) {
	ret := _m.Called(id)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(id)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PublishMessage provides a mock function with given fields: ctx, workflowInstanceID, name, key, vars
func (_m *MockNatsService) PublishMessage(ctx context.Context, workflowInstanceID string, name string, key string, vars []byte) error {
	ret := _m.Called(ctx, workflowInstanceID, name, key, vars)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, []byte) error); ok {
		r0 = rf(ctx, workflowInstanceID, name, key, vars)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PublishWorkflowState provides a mock function with given fields: ctx, stateName, state, ops
func (_m *MockNatsService) PublishWorkflowState(ctx context.Context, stateName string, state *model.WorkflowState, ops ...services.PublishOpt) error {
	_va := make([]interface{}, len(ops))
	for _i := range ops {
		_va[_i] = ops[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, stateName, state)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *model.WorkflowState, ...services.PublishOpt) error); ok {
		r0 = rf(ctx, stateName, state, ops...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetAbort provides a mock function with given fields: processor
func (_m *MockNatsService) SetAbort(processor services.AbortFunc) {
	_m.Called(processor)
}

// SetCompleteActivity provides a mock function with given fields: processor
func (_m *MockNatsService) SetCompleteActivity(processor services.CompleteActivityFunc) {
	_m.Called(processor)
}

// SetCompleteActivityProcessor provides a mock function with given fields: processor
func (_m *MockNatsService) SetCompleteActivityProcessor(processor services.CompleteActivityProcessorFunc) {
	_m.Called(processor)
}

// SetCompleteJobProcessor provides a mock function with given fields: processor
func (_m *MockNatsService) SetCompleteJobProcessor(processor services.CompleteJobProcessorFunc) {
	_m.Called(processor)
}

// SetEventProcessor provides a mock function with given fields: processor
func (_m *MockNatsService) SetEventProcessor(processor services.EventProcessorFunc) {
	_m.Called(processor)
}

// SetLaunchFunc provides a mock function with given fields: processor
func (_m *MockNatsService) SetLaunchFunc(processor services.LaunchFunc) {
	_m.Called(processor)
}

// SetMessageCompleteProcessor provides a mock function with given fields: processor
func (_m *MockNatsService) SetMessageCompleteProcessor(processor services.MessageCompleteProcessorFunc) {
	_m.Called(processor)
}

// SetMessageProcessor provides a mock function with given fields: processor
func (_m *MockNatsService) SetMessageProcessor(processor services.MessageProcessorFunc) {
	_m.Called(processor)
}

// SetTraversalProvider provides a mock function with given fields: provider
func (_m *MockNatsService) SetTraversalProvider(provider services.TraversalFunc) {
	_m.Called(provider)
}

// Shutdown provides a mock function with given fields:
func (_m *MockNatsService) Shutdown() {
	_m.Called()
}

// StartProcessing provides a mock function with given fields: ctx
func (_m *MockNatsService) StartProcessing(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StoreWorkflow provides a mock function with given fields: ctx, wf
func (_m *MockNatsService) StoreWorkflow(ctx context.Context, wf *model.Workflow) (string, error) {
	ret := _m.Called(ctx, wf)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, *model.Workflow) string); ok {
		r0 = rf(ctx, wf)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *model.Workflow) error); ok {
		r1 = rf(ctx, wf)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockNatsService interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockNatsService creates a new instance of MockNatsService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockNatsService(t mockConstructorTestingTNewMockNatsService) *MockNatsService {
	mock := &MockNatsService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
