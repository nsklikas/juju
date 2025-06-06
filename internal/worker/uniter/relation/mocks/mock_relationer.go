// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/uniter/relation (interfaces: Relationer)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_relationer.go github.com/juju/juju/internal/worker/uniter/relation Relationer
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	hook "github.com/juju/juju/internal/worker/uniter/hook"
	relation "github.com/juju/juju/internal/worker/uniter/relation"
	context "github.com/juju/juju/internal/worker/uniter/runner/context"
	gomock "go.uber.org/mock/gomock"
)

// MockRelationer is a mock of Relationer interface.
type MockRelationer struct {
	ctrl     *gomock.Controller
	recorder *MockRelationerMockRecorder
}

// MockRelationerMockRecorder is the mock recorder for MockRelationer.
type MockRelationerMockRecorder struct {
	mock *MockRelationer
}

// NewMockRelationer creates a new mock instance.
func NewMockRelationer(ctrl *gomock.Controller) *MockRelationer {
	mock := &MockRelationer{ctrl: ctrl}
	mock.recorder = &MockRelationerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRelationer) EXPECT() *MockRelationerMockRecorder {
	return m.recorder
}

// CommitHook mocks base method.
func (m *MockRelationer) CommitHook(arg0 hook.Info) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitHook", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CommitHook indicates an expected call of CommitHook.
func (mr *MockRelationerMockRecorder) CommitHook(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitHook", reflect.TypeOf((*MockRelationer)(nil).CommitHook), arg0)
}

// ContextInfo mocks base method.
func (m *MockRelationer) ContextInfo() *context.RelationInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContextInfo")
	ret0, _ := ret[0].(*context.RelationInfo)
	return ret0
}

// ContextInfo indicates an expected call of ContextInfo.
func (mr *MockRelationerMockRecorder) ContextInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContextInfo", reflect.TypeOf((*MockRelationer)(nil).ContextInfo))
}

// IsDying mocks base method.
func (m *MockRelationer) IsDying() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsDying")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsDying indicates an expected call of IsDying.
func (mr *MockRelationerMockRecorder) IsDying() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDying", reflect.TypeOf((*MockRelationer)(nil).IsDying))
}

// IsImplicit mocks base method.
func (m *MockRelationer) IsImplicit() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsImplicit")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsImplicit indicates an expected call of IsImplicit.
func (mr *MockRelationerMockRecorder) IsImplicit() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsImplicit", reflect.TypeOf((*MockRelationer)(nil).IsImplicit))
}

// Join mocks base method.
func (m *MockRelationer) Join() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join")
	ret0, _ := ret[0].(error)
	return ret0
}

// Join indicates an expected call of Join.
func (mr *MockRelationerMockRecorder) Join() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockRelationer)(nil).Join))
}

// PrepareHook mocks base method.
func (m *MockRelationer) PrepareHook(arg0 hook.Info) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareHook", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PrepareHook indicates an expected call of PrepareHook.
func (mr *MockRelationerMockRecorder) PrepareHook(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareHook", reflect.TypeOf((*MockRelationer)(nil).PrepareHook), arg0)
}

// RelationUnit mocks base method.
func (m *MockRelationer) RelationUnit() relation.RelationUnit {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RelationUnit")
	ret0, _ := ret[0].(relation.RelationUnit)
	return ret0
}

// RelationUnit indicates an expected call of RelationUnit.
func (mr *MockRelationerMockRecorder) RelationUnit() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RelationUnit", reflect.TypeOf((*MockRelationer)(nil).RelationUnit))
}

// SetDying mocks base method.
func (m *MockRelationer) SetDying() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetDying")
	ret0, _ := ret[0].(error)
	return ret0
}

// SetDying indicates an expected call of SetDying.
func (mr *MockRelationerMockRecorder) SetDying() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetDying", reflect.TypeOf((*MockRelationer)(nil).SetDying))
}
