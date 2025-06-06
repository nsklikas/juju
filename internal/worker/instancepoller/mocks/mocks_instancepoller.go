// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/instancepoller (interfaces: Environ,Machine)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mocks_instancepoller.go github.com/juju/juju/internal/worker/instancepoller Environ,Machine
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	instance "github.com/juju/juju/core/instance"
	life "github.com/juju/juju/core/life"
	network "github.com/juju/juju/core/network"
	status "github.com/juju/juju/core/status"
	context "github.com/juju/juju/environs/context"
	instances "github.com/juju/juju/environs/instances"
	params "github.com/juju/juju/rpc/params"
	gomock "go.uber.org/mock/gomock"
)

// MockEnviron is a mock of Environ interface.
type MockEnviron struct {
	ctrl     *gomock.Controller
	recorder *MockEnvironMockRecorder
}

// MockEnvironMockRecorder is the mock recorder for MockEnviron.
type MockEnvironMockRecorder struct {
	mock *MockEnviron
}

// NewMockEnviron creates a new mock instance.
func NewMockEnviron(ctrl *gomock.Controller) *MockEnviron {
	mock := &MockEnviron{ctrl: ctrl}
	mock.recorder = &MockEnvironMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEnviron) EXPECT() *MockEnvironMockRecorder {
	return m.recorder
}

// Instances mocks base method.
func (m *MockEnviron) Instances(arg0 context.ProviderCallContext, arg1 []instance.Id) ([]instances.Instance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Instances", arg0, arg1)
	ret0, _ := ret[0].([]instances.Instance)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Instances indicates an expected call of Instances.
func (mr *MockEnvironMockRecorder) Instances(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Instances", reflect.TypeOf((*MockEnviron)(nil).Instances), arg0, arg1)
}

// NetworkInterfaces mocks base method.
func (m *MockEnviron) NetworkInterfaces(arg0 context.ProviderCallContext, arg1 []instance.Id) ([]network.InterfaceInfos, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetworkInterfaces", arg0, arg1)
	ret0, _ := ret[0].([]network.InterfaceInfos)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NetworkInterfaces indicates an expected call of NetworkInterfaces.
func (mr *MockEnvironMockRecorder) NetworkInterfaces(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetworkInterfaces", reflect.TypeOf((*MockEnviron)(nil).NetworkInterfaces), arg0, arg1)
}

// MockMachine is a mock of Machine interface.
type MockMachine struct {
	ctrl     *gomock.Controller
	recorder *MockMachineMockRecorder
}

// MockMachineMockRecorder is the mock recorder for MockMachine.
type MockMachineMockRecorder struct {
	mock *MockMachine
}

// NewMockMachine creates a new mock instance.
func NewMockMachine(ctrl *gomock.Controller) *MockMachine {
	mock := &MockMachine{ctrl: ctrl}
	mock.recorder = &MockMachineMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMachine) EXPECT() *MockMachineMockRecorder {
	return m.recorder
}

// Id mocks base method.
func (m *MockMachine) Id() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Id")
	ret0, _ := ret[0].(string)
	return ret0
}

// Id indicates an expected call of Id.
func (mr *MockMachineMockRecorder) Id() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Id", reflect.TypeOf((*MockMachine)(nil).Id))
}

// InstanceId mocks base method.
func (m *MockMachine) InstanceId() (instance.Id, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InstanceId")
	ret0, _ := ret[0].(instance.Id)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InstanceId indicates an expected call of InstanceId.
func (mr *MockMachineMockRecorder) InstanceId() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstanceId", reflect.TypeOf((*MockMachine)(nil).InstanceId))
}

// InstanceStatus mocks base method.
func (m *MockMachine) InstanceStatus() (params.StatusResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InstanceStatus")
	ret0, _ := ret[0].(params.StatusResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InstanceStatus indicates an expected call of InstanceStatus.
func (mr *MockMachineMockRecorder) InstanceStatus() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstanceStatus", reflect.TypeOf((*MockMachine)(nil).InstanceStatus))
}

// IsManual mocks base method.
func (m *MockMachine) IsManual() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsManual")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsManual indicates an expected call of IsManual.
func (mr *MockMachineMockRecorder) IsManual() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsManual", reflect.TypeOf((*MockMachine)(nil).IsManual))
}

// Life mocks base method.
func (m *MockMachine) Life() life.Value {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Life")
	ret0, _ := ret[0].(life.Value)
	return ret0
}

// Life indicates an expected call of Life.
func (mr *MockMachineMockRecorder) Life() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Life", reflect.TypeOf((*MockMachine)(nil).Life))
}

// Refresh mocks base method.
func (m *MockMachine) Refresh() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Refresh")
	ret0, _ := ret[0].(error)
	return ret0
}

// Refresh indicates an expected call of Refresh.
func (mr *MockMachineMockRecorder) Refresh() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Refresh", reflect.TypeOf((*MockMachine)(nil).Refresh))
}

// SetInstanceStatus mocks base method.
func (m *MockMachine) SetInstanceStatus(arg0 status.Status, arg1 string, arg2 map[string]any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetInstanceStatus", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetInstanceStatus indicates an expected call of SetInstanceStatus.
func (mr *MockMachineMockRecorder) SetInstanceStatus(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetInstanceStatus", reflect.TypeOf((*MockMachine)(nil).SetInstanceStatus), arg0, arg1, arg2)
}

// SetProviderNetworkConfig mocks base method.
func (m *MockMachine) SetProviderNetworkConfig(arg0 network.InterfaceInfos) (network.ProviderAddresses, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetProviderNetworkConfig", arg0)
	ret0, _ := ret[0].(network.ProviderAddresses)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// SetProviderNetworkConfig indicates an expected call of SetProviderNetworkConfig.
func (mr *MockMachineMockRecorder) SetProviderNetworkConfig(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProviderNetworkConfig", reflect.TypeOf((*MockMachine)(nil).SetProviderNetworkConfig), arg0)
}

// Status mocks base method.
func (m *MockMachine) Status() (params.StatusResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(params.StatusResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Status indicates an expected call of Status.
func (mr *MockMachineMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockMachine)(nil).Status))
}

// String mocks base method.
func (m *MockMachine) String() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "String")
	ret0, _ := ret[0].(string)
	return ret0
}

// String indicates an expected call of String.
func (mr *MockMachineMockRecorder) String() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "String", reflect.TypeOf((*MockMachine)(nil).String))
}
