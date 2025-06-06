// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/uniter/relation (interfaces: UnitGetter)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_unit_getter.go github.com/juju/juju/internal/worker/uniter/relation UnitGetter
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	relation "github.com/juju/juju/internal/worker/uniter/relation"
	names "github.com/juju/names/v5"
	gomock "go.uber.org/mock/gomock"
)

// MockUnitGetter is a mock of UnitGetter interface.
type MockUnitGetter struct {
	ctrl     *gomock.Controller
	recorder *MockUnitGetterMockRecorder
}

// MockUnitGetterMockRecorder is the mock recorder for MockUnitGetter.
type MockUnitGetterMockRecorder struct {
	mock *MockUnitGetter
}

// NewMockUnitGetter creates a new mock instance.
func NewMockUnitGetter(ctrl *gomock.Controller) *MockUnitGetter {
	mock := &MockUnitGetter{ctrl: ctrl}
	mock.recorder = &MockUnitGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUnitGetter) EXPECT() *MockUnitGetterMockRecorder {
	return m.recorder
}

// Unit mocks base method.
func (m *MockUnitGetter) Unit(arg0 names.UnitTag) (relation.Unit, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unit", arg0)
	ret0, _ := ret[0].(relation.Unit)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Unit indicates an expected call of Unit.
func (mr *MockUnitGetterMockRecorder) Unit(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unit", reflect.TypeOf((*MockUnitGetter)(nil).Unit), arg0)
}
