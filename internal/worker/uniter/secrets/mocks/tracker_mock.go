// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/uniter/secrets (interfaces: SecretStateTracker)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/tracker_mock.go github.com/juju/juju/internal/worker/uniter/secrets SecretStateTracker
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	hook "github.com/juju/juju/internal/worker/uniter/hook"
	gomock "go.uber.org/mock/gomock"
)

// MockSecretStateTracker is a mock of SecretStateTracker interface.
type MockSecretStateTracker struct {
	ctrl     *gomock.Controller
	recorder *MockSecretStateTrackerMockRecorder
}

// MockSecretStateTrackerMockRecorder is the mock recorder for MockSecretStateTracker.
type MockSecretStateTrackerMockRecorder struct {
	mock *MockSecretStateTracker
}

// NewMockSecretStateTracker creates a new mock instance.
func NewMockSecretStateTracker(ctrl *gomock.Controller) *MockSecretStateTracker {
	mock := &MockSecretStateTracker{ctrl: ctrl}
	mock.recorder = &MockSecretStateTrackerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSecretStateTracker) EXPECT() *MockSecretStateTrackerMockRecorder {
	return m.recorder
}

// CommitHook mocks base method.
func (m *MockSecretStateTracker) CommitHook(arg0 hook.Info) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitHook", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CommitHook indicates an expected call of CommitHook.
func (mr *MockSecretStateTrackerMockRecorder) CommitHook(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitHook", reflect.TypeOf((*MockSecretStateTracker)(nil).CommitHook), arg0)
}

// ConsumedSecretRevision mocks base method.
func (m *MockSecretStateTracker) ConsumedSecretRevision(arg0 string) int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConsumedSecretRevision", arg0)
	ret0, _ := ret[0].(int)
	return ret0
}

// ConsumedSecretRevision indicates an expected call of ConsumedSecretRevision.
func (mr *MockSecretStateTrackerMockRecorder) ConsumedSecretRevision(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConsumedSecretRevision", reflect.TypeOf((*MockSecretStateTracker)(nil).ConsumedSecretRevision), arg0)
}

// PrepareHook mocks base method.
func (m *MockSecretStateTracker) PrepareHook(arg0 hook.Info) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareHook", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// PrepareHook indicates an expected call of PrepareHook.
func (mr *MockSecretStateTrackerMockRecorder) PrepareHook(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareHook", reflect.TypeOf((*MockSecretStateTracker)(nil).PrepareHook), arg0)
}

// Report mocks base method.
func (m *MockSecretStateTracker) Report() map[string]any {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Report")
	ret0, _ := ret[0].(map[string]any)
	return ret0
}

// Report indicates an expected call of Report.
func (mr *MockSecretStateTrackerMockRecorder) Report() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Report", reflect.TypeOf((*MockSecretStateTracker)(nil).Report))
}

// SecretObsoleteRevisions mocks base method.
func (m *MockSecretStateTracker) SecretObsoleteRevisions(arg0 string) []int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SecretObsoleteRevisions", arg0)
	ret0, _ := ret[0].([]int)
	return ret0
}

// SecretObsoleteRevisions indicates an expected call of SecretObsoleteRevisions.
func (mr *MockSecretStateTrackerMockRecorder) SecretObsoleteRevisions(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SecretObsoleteRevisions", reflect.TypeOf((*MockSecretStateTracker)(nil).SecretObsoleteRevisions), arg0)
}

// SecretsRemoved mocks base method.
func (m *MockSecretStateTracker) SecretsRemoved(arg0 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SecretsRemoved", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SecretsRemoved indicates an expected call of SecretsRemoved.
func (mr *MockSecretStateTrackerMockRecorder) SecretsRemoved(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SecretsRemoved", reflect.TypeOf((*MockSecretStateTracker)(nil).SecretsRemoved), arg0)
}
