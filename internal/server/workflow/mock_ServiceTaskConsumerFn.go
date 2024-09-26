// Code generated by mockery v2.43.1. DO NOT EDIT.

package workflow

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockServiceTaskConsumerFn is an autogenerated mock type for the ServiceTaskConsumerFn type
type MockServiceTaskConsumerFn struct {
	mock.Mock
}

type MockServiceTaskConsumerFn_Expecter struct {
	mock *mock.Mock
}

func (_m *MockServiceTaskConsumerFn) EXPECT() *MockServiceTaskConsumerFn_Expecter {
	return &MockServiceTaskConsumerFn_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: ctx, id
func (_m *MockServiceTaskConsumerFn) Execute(ctx context.Context, id string) error {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockServiceTaskConsumerFn_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type MockServiceTaskConsumerFn_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - ctx context.Context
//   - id string
func (_e *MockServiceTaskConsumerFn_Expecter) Execute(ctx interface{}, id interface{}) *MockServiceTaskConsumerFn_Execute_Call {
	return &MockServiceTaskConsumerFn_Execute_Call{Call: _e.mock.On("Execute", ctx, id)}
}

func (_c *MockServiceTaskConsumerFn_Execute_Call) Run(run func(ctx context.Context, id string)) *MockServiceTaskConsumerFn_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockServiceTaskConsumerFn_Execute_Call) Return(_a0 error) *MockServiceTaskConsumerFn_Execute_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServiceTaskConsumerFn_Execute_Call) RunAndReturn(run func(context.Context, string) error) *MockServiceTaskConsumerFn_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockServiceTaskConsumerFn creates a new instance of MockServiceTaskConsumerFn. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockServiceTaskConsumerFn(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockServiceTaskConsumerFn {
	mock := &MockServiceTaskConsumerFn{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
