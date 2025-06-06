// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/uniter/relation (interfaces: RelationStateTracker)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_statetracker.go github.com/juju/juju/internal/worker/uniter/relation RelationStateTracker
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	life "github.com/juju/juju/core/life"
	hook "github.com/juju/juju/internal/worker/uniter/hook"
	relation "github.com/juju/juju/internal/worker/uniter/relation"
	remotestate "github.com/juju/juju/internal/worker/uniter/remotestate"
	context "github.com/juju/juju/internal/worker/uniter/runner/context"
	gomock "go.uber.org/mock/gomock"
)

// MockRelationStateTracker is a mock of RelationStateTracker interface.
type MockRelationStateTracker struct {
	ctrl     *gomock.Controller
	recorder *MockRelationStateTrackerMockRecorder
}

// MockRelationStateTrackerMockRecorder is the mock recorder for MockRelationStateTracker.
type MockRelationStateTrackerMockRecorder struct {
	mock *MockRelationStateTracker
}

// NewMockRelationStateTracker creates a new mock instance.
func NewMockRelationStateTracker(ctrl *gomock.Controller) *MockRelationStateTracker {
	mock := &MockRelationStateTracker{ctrl: ctrl}
	mock.recorder = &MockRelationStateTrackerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRelationStateTracker) EXPECT() *MockRelationStateTrackerMockRecorder {
	return m.recorder
}

// CommitHook mocks base method.
func (m *MockRelationStateTracker) CommitHook(arg0 hook.Info) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitHook", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CommitHook indicates an expected call of CommitHook.
func (mr *MockRelationStateTrackerMockRecorder) CommitHook(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitHook", reflect.TypeOf((*MockRelationStateTracker)(nil).CommitHook), arg0)
}

// GetInfo mocks base method.
func (m *MockRelationStateTracker) GetInfo() map[int]*context.RelationInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInfo")
	ret0, _ := ret[0].(map[int]*context.RelationInfo)
	return ret0
}

// GetInfo indicates an expected call of GetInfo.
func (mr *MockRelationStateTrackerMockRecorder) GetInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInfo", reflect.TypeOf((*MockRelationStateTracker)(nil).GetInfo))
}

// HasContainerScope mocks base method.
func (m *MockRelationStateTracker) HasContainerScope(arg0 int) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasContainerScope", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HasContainerScope indicates an expected call of HasContainerScope.
func (mr *MockRelationStateTrackerMockRecorder) HasContainerScope(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasContainerScope", reflect.TypeOf((*MockRelationStateTracker)(nil).HasContainerScope), arg0)
}

// IsImplicit mocks base method.
func (m *MockRelationStateTracker) IsImplicit(arg0 int) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsImplicit", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsImplicit indicates an expected call of IsImplicit.
func (mr *MockRelationStateTrackerMockRecorder) IsImplicit(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsImplicit", reflect.TypeOf((*MockRelationStateTracker)(nil).IsImplicit), arg0)
}

// IsKnown mocks base method.
func (m *MockRelationStateTracker) IsKnown(arg0 int) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsKnown", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsKnown indicates an expected call of IsKnown.
func (mr *MockRelationStateTrackerMockRecorder) IsKnown(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsKnown", reflect.TypeOf((*MockRelationStateTracker)(nil).IsKnown), arg0)
}

// IsPeerRelation mocks base method.
func (m *MockRelationStateTracker) IsPeerRelation(arg0 int) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsPeerRelation", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsPeerRelation indicates an expected call of IsPeerRelation.
func (mr *MockRelationStateTrackerMockRecorder) IsPeerRelation(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsPeerRelation", reflect.TypeOf((*MockRelationStateTracker)(nil).IsPeerRelation), arg0)
}

// LocalUnitAndApplicationLife mocks base method.
func (m *MockRelationStateTracker) LocalUnitAndApplicationLife() (life.Value, life.Value, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalUnitAndApplicationLife")
	ret0, _ := ret[0].(life.Value)
	ret1, _ := ret[1].(life.Value)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// LocalUnitAndApplicationLife indicates an expected call of LocalUnitAndApplicationLife.
func (mr *MockRelationStateTrackerMockRecorder) LocalUnitAndApplicationLife() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalUnitAndApplicationLife", reflect.TypeOf((*MockRelationStateTracker)(nil).LocalUnitAndApplicationLife))
}

// LocalUnitName mocks base method.
func (m *MockRelationStateTracker) LocalUnitName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalUnitName")
	ret0, _ := ret[0].(string)
	return ret0
}

// LocalUnitName indicates an expected call of LocalUnitName.
func (mr *MockRelationStateTrackerMockRecorder) LocalUnitName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalUnitName", reflect.TypeOf((*MockRelationStateTracker)(nil).LocalUnitName))
}

// Name mocks base method.
func (m *MockRelationStateTracker) Name(arg0 int) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Name indicates an expected call of Name.
func (mr *MockRelationStateTrackerMockRecorder) Name(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockRelationStateTracker)(nil).Name), arg0)
}

// PrepareHook mocks base method.
func (m *MockRelationStateTracker) PrepareHook(arg0 hook.Info) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareHook", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PrepareHook indicates an expected call of PrepareHook.
func (mr *MockRelationStateTrackerMockRecorder) PrepareHook(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareHook", reflect.TypeOf((*MockRelationStateTracker)(nil).PrepareHook), arg0)
}

// RelationCreated mocks base method.
func (m *MockRelationStateTracker) RelationCreated(arg0 int) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RelationCreated", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// RelationCreated indicates an expected call of RelationCreated.
func (mr *MockRelationStateTrackerMockRecorder) RelationCreated(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RelationCreated", reflect.TypeOf((*MockRelationStateTracker)(nil).RelationCreated), arg0)
}

// RemoteApplication mocks base method.
func (m *MockRelationStateTracker) RemoteApplication(arg0 int) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoteApplication", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// RemoteApplication indicates an expected call of RemoteApplication.
func (mr *MockRelationStateTrackerMockRecorder) RemoteApplication(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoteApplication", reflect.TypeOf((*MockRelationStateTracker)(nil).RemoteApplication), arg0)
}

// Report mocks base method.
func (m *MockRelationStateTracker) Report() map[string]any {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Report")
	ret0, _ := ret[0].(map[string]any)
	return ret0
}

// Report indicates an expected call of Report.
func (mr *MockRelationStateTrackerMockRecorder) Report() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Report", reflect.TypeOf((*MockRelationStateTracker)(nil).Report))
}

// State mocks base method.
func (m *MockRelationStateTracker) State(arg0 int) (*relation.State, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "State", arg0)
	ret0, _ := ret[0].(*relation.State)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// State indicates an expected call of State.
func (mr *MockRelationStateTrackerMockRecorder) State(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "State", reflect.TypeOf((*MockRelationStateTracker)(nil).State), arg0)
}

// StateFound mocks base method.
func (m *MockRelationStateTracker) StateFound(arg0 int) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateFound", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// StateFound indicates an expected call of StateFound.
func (mr *MockRelationStateTrackerMockRecorder) StateFound(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateFound", reflect.TypeOf((*MockRelationStateTracker)(nil).StateFound), arg0)
}

// SynchronizeScopes mocks base method.
func (m *MockRelationStateTracker) SynchronizeScopes(arg0 remotestate.Snapshot) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SynchronizeScopes", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SynchronizeScopes indicates an expected call of SynchronizeScopes.
func (mr *MockRelationStateTrackerMockRecorder) SynchronizeScopes(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SynchronizeScopes", reflect.TypeOf((*MockRelationStateTracker)(nil).SynchronizeScopes), arg0)
}
