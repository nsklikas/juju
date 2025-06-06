// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/secretrotate (interfaces: SecretManagerFacade)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/client_mock.go github.com/juju/juju/internal/worker/secretrotate SecretManagerFacade
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	watcher "github.com/juju/juju/core/watcher"
	names "github.com/juju/names/v5"
	gomock "go.uber.org/mock/gomock"
)

// MockSecretManagerFacade is a mock of SecretManagerFacade interface.
type MockSecretManagerFacade struct {
	ctrl     *gomock.Controller
	recorder *MockSecretManagerFacadeMockRecorder
}

// MockSecretManagerFacadeMockRecorder is the mock recorder for MockSecretManagerFacade.
type MockSecretManagerFacadeMockRecorder struct {
	mock *MockSecretManagerFacade
}

// NewMockSecretManagerFacade creates a new mock instance.
func NewMockSecretManagerFacade(ctrl *gomock.Controller) *MockSecretManagerFacade {
	mock := &MockSecretManagerFacade{ctrl: ctrl}
	mock.recorder = &MockSecretManagerFacadeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSecretManagerFacade) EXPECT() *MockSecretManagerFacadeMockRecorder {
	return m.recorder
}

// WatchSecretsRotationChanges mocks base method.
func (m *MockSecretManagerFacade) WatchSecretsRotationChanges(arg0 ...names.Tag) (watcher.SecretTriggerWatcher, error) {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WatchSecretsRotationChanges", varargs...)
	ret0, _ := ret[0].(watcher.SecretTriggerWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchSecretsRotationChanges indicates an expected call of WatchSecretsRotationChanges.
func (mr *MockSecretManagerFacadeMockRecorder) WatchSecretsRotationChanges(arg0 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchSecretsRotationChanges", reflect.TypeOf((*MockSecretManagerFacade)(nil).WatchSecretsRotationChanges), arg0...)
}
