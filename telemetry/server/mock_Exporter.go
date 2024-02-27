// Code generated by mockery v2.41.0. DO NOT EDIT.

package server

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	trace "go.opentelemetry.io/otel/sdk/trace"
)

// MockExporter is an autogenerated mock type for the Exporter type
type MockExporter struct {
	mock.Mock
}

type MockExporter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockExporter) EXPECT() *MockExporter_Expecter {
	return &MockExporter_Expecter{mock: &_m.Mock}
}

// ExportSpans provides a mock function with given fields: ctx, spans
func (_m *MockExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	ret := _m.Called(ctx, spans)

	if len(ret) == 0 {
		panic("no return value specified for ExportSpans")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []trace.ReadOnlySpan) error); ok {
		r0 = rf(ctx, spans)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockExporter_ExportSpans_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ExportSpans'
type MockExporter_ExportSpans_Call struct {
	*mock.Call
}

// ExportSpans is a helper method to define mock.On call
//   - ctx context.Context
//   - spans []trace.ReadOnlySpan
func (_e *MockExporter_Expecter) ExportSpans(ctx interface{}, spans interface{}) *MockExporter_ExportSpans_Call {
	return &MockExporter_ExportSpans_Call{Call: _e.mock.On("ExportSpans", ctx, spans)}
}

func (_c *MockExporter_ExportSpans_Call) Run(run func(ctx context.Context, spans []trace.ReadOnlySpan)) *MockExporter_ExportSpans_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]trace.ReadOnlySpan))
	})
	return _c
}

func (_c *MockExporter_ExportSpans_Call) Return(_a0 error) *MockExporter_ExportSpans_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockExporter_ExportSpans_Call) RunAndReturn(run func(context.Context, []trace.ReadOnlySpan) error) *MockExporter_ExportSpans_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockExporter creates a new instance of MockExporter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockExporter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockExporter {
	mock := &MockExporter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}