// Code generated by MockGen. DO NOT EDIT.
// Source: client.go

// Package mock_computemanagement is a generated GoMock package.
package mock_computemanagement

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	core "github.com/oracle/oci-go-sdk/v65/core"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// CreateInstanceConfiguration mocks base method.
func (m *MockClient) CreateInstanceConfiguration(ctx context.Context, request core.CreateInstanceConfigurationRequest) (core.CreateInstanceConfigurationResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateInstanceConfiguration", ctx, request)
	ret0, _ := ret[0].(core.CreateInstanceConfigurationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateInstanceConfiguration indicates an expected call of CreateInstanceConfiguration.
func (mr *MockClientMockRecorder) CreateInstanceConfiguration(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateInstanceConfiguration", reflect.TypeOf((*MockClient)(nil).CreateInstanceConfiguration), ctx, request)
}

// CreateInstancePool mocks base method.
func (m *MockClient) CreateInstancePool(ctx context.Context, request core.CreateInstancePoolRequest) (core.CreateInstancePoolResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateInstancePool", ctx, request)
	ret0, _ := ret[0].(core.CreateInstancePoolResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateInstancePool indicates an expected call of CreateInstancePool.
func (mr *MockClientMockRecorder) CreateInstancePool(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateInstancePool", reflect.TypeOf((*MockClient)(nil).CreateInstancePool), ctx, request)
}

// DeleteInstanceConfiguration mocks base method.
func (m *MockClient) DeleteInstanceConfiguration(ctx context.Context, request core.DeleteInstanceConfigurationRequest) (core.DeleteInstanceConfigurationResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteInstanceConfiguration", ctx, request)
	ret0, _ := ret[0].(core.DeleteInstanceConfigurationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteInstanceConfiguration indicates an expected call of DeleteInstanceConfiguration.
func (mr *MockClientMockRecorder) DeleteInstanceConfiguration(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteInstanceConfiguration", reflect.TypeOf((*MockClient)(nil).DeleteInstanceConfiguration), ctx, request)
}

// GetInstanceConfiguration mocks base method.
func (m *MockClient) GetInstanceConfiguration(ctx context.Context, request core.GetInstanceConfigurationRequest) (core.GetInstanceConfigurationResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstanceConfiguration", ctx, request)
	ret0, _ := ret[0].(core.GetInstanceConfigurationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInstanceConfiguration indicates an expected call of GetInstanceConfiguration.
func (mr *MockClientMockRecorder) GetInstanceConfiguration(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstanceConfiguration", reflect.TypeOf((*MockClient)(nil).GetInstanceConfiguration), ctx, request)
}

// GetInstancePool mocks base method.
func (m *MockClient) GetInstancePool(ctx context.Context, request core.GetInstancePoolRequest) (core.GetInstancePoolResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstancePool", ctx, request)
	ret0, _ := ret[0].(core.GetInstancePoolResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetInstancePool indicates an expected call of GetInstancePool.
func (mr *MockClientMockRecorder) GetInstancePool(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstancePool", reflect.TypeOf((*MockClient)(nil).GetInstancePool), ctx, request)
}

// ListInstanceConfigurations mocks base method.
func (m *MockClient) ListInstanceConfigurations(ctx context.Context, request core.ListInstanceConfigurationsRequest) (core.ListInstanceConfigurationsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListInstanceConfigurations", ctx, request)
	ret0, _ := ret[0].(core.ListInstanceConfigurationsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListInstanceConfigurations indicates an expected call of ListInstanceConfigurations.
func (mr *MockClientMockRecorder) ListInstanceConfigurations(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListInstanceConfigurations", reflect.TypeOf((*MockClient)(nil).ListInstanceConfigurations), ctx, request)
}

// ListInstancePoolInstances mocks base method.
func (m *MockClient) ListInstancePoolInstances(ctx context.Context, request core.ListInstancePoolInstancesRequest) (core.ListInstancePoolInstancesResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListInstancePoolInstances", ctx, request)
	ret0, _ := ret[0].(core.ListInstancePoolInstancesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListInstancePoolInstances indicates an expected call of ListInstancePoolInstances.
func (mr *MockClientMockRecorder) ListInstancePoolInstances(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListInstancePoolInstances", reflect.TypeOf((*MockClient)(nil).ListInstancePoolInstances), ctx, request)
}

// ListInstancePools mocks base method.
func (m *MockClient) ListInstancePools(ctx context.Context, request core.ListInstancePoolsRequest) (core.ListInstancePoolsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListInstancePools", ctx, request)
	ret0, _ := ret[0].(core.ListInstancePoolsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListInstancePools indicates an expected call of ListInstancePools.
func (mr *MockClientMockRecorder) ListInstancePools(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListInstancePools", reflect.TypeOf((*MockClient)(nil).ListInstancePools), ctx, request)
}

// TerminateInstancePool mocks base method.
func (m *MockClient) TerminateInstancePool(ctx context.Context, request core.TerminateInstancePoolRequest) (core.TerminateInstancePoolResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TerminateInstancePool", ctx, request)
	ret0, _ := ret[0].(core.TerminateInstancePoolResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TerminateInstancePool indicates an expected call of TerminateInstancePool.
func (mr *MockClientMockRecorder) TerminateInstancePool(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TerminateInstancePool", reflect.TypeOf((*MockClient)(nil).TerminateInstancePool), ctx, request)
}

// UpdateInstancePool mocks base method.
func (m *MockClient) UpdateInstancePool(ctx context.Context, request core.UpdateInstancePoolRequest) (core.UpdateInstancePoolResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateInstancePool", ctx, request)
	ret0, _ := ret[0].(core.UpdateInstancePoolResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateInstancePool indicates an expected call of UpdateInstancePool.
func (mr *MockClientMockRecorder) UpdateInstancePool(ctx, request interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateInstancePool", reflect.TypeOf((*MockClient)(nil).UpdateInstancePool), ctx, request)
}
