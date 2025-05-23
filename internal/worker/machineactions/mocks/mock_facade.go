// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/machineactions (interfaces: Facade)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_facade.go github.com/juju/juju/internal/worker/machineactions Facade
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	machineactions "github.com/juju/juju/api/agent/machineactions"
	watcher "github.com/juju/juju/core/watcher"
	params "github.com/juju/juju/rpc/params"
	names "github.com/juju/names/v5"
	gomock "go.uber.org/mock/gomock"
)

// MockFacade is a mock of Facade interface.
type MockFacade struct {
	ctrl     *gomock.Controller
	recorder *MockFacadeMockRecorder
}

// MockFacadeMockRecorder is the mock recorder for MockFacade.
type MockFacadeMockRecorder struct {
	mock *MockFacade
}

// NewMockFacade creates a new mock instance.
func NewMockFacade(ctrl *gomock.Controller) *MockFacade {
	mock := &MockFacade{ctrl: ctrl}
	mock.recorder = &MockFacadeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFacade) EXPECT() *MockFacadeMockRecorder {
	return m.recorder
}

// Action mocks base method.
func (m *MockFacade) Action(arg0 names.ActionTag) (*machineactions.Action, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Action", arg0)
	ret0, _ := ret[0].(*machineactions.Action)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Action indicates an expected call of Action.
func (mr *MockFacadeMockRecorder) Action(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Action", reflect.TypeOf((*MockFacade)(nil).Action), arg0)
}

// ActionBegin mocks base method.
func (m *MockFacade) ActionBegin(arg0 names.ActionTag) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActionBegin", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ActionBegin indicates an expected call of ActionBegin.
func (mr *MockFacadeMockRecorder) ActionBegin(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActionBegin", reflect.TypeOf((*MockFacade)(nil).ActionBegin), arg0)
}

// ActionFinish mocks base method.
func (m *MockFacade) ActionFinish(arg0 names.ActionTag, arg1 string, arg2 map[string]any, arg3 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActionFinish", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// ActionFinish indicates an expected call of ActionFinish.
func (mr *MockFacadeMockRecorder) ActionFinish(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActionFinish", reflect.TypeOf((*MockFacade)(nil).ActionFinish), arg0, arg1, arg2, arg3)
}

// RunningActions mocks base method.
func (m *MockFacade) RunningActions(arg0 names.MachineTag) ([]params.ActionResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunningActions", arg0)
	ret0, _ := ret[0].([]params.ActionResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunningActions indicates an expected call of RunningActions.
func (mr *MockFacadeMockRecorder) RunningActions(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunningActions", reflect.TypeOf((*MockFacade)(nil).RunningActions), arg0)
}

// WatchActionNotifications mocks base method.
func (m *MockFacade) WatchActionNotifications(arg0 names.MachineTag) (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchActionNotifications", arg0)
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchActionNotifications indicates an expected call of WatchActionNotifications.
func (mr *MockFacadeMockRecorder) WatchActionNotifications(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchActionNotifications", reflect.TypeOf((*MockFacade)(nil).WatchActionNotifications), arg0)
}
