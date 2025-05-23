// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/internal/worker/caasfirewallersidecar (interfaces: Client,CAASFirewallerAPI,LifeGetter)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/client_mock.go github.com/juju/juju/internal/worker/caasfirewallersidecar Client,CAASFirewallerAPI,LifeGetter
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	charms "github.com/juju/juju/api/common/charms"
	config "github.com/juju/juju/core/config"
	life "github.com/juju/juju/core/life"
	network "github.com/juju/juju/core/network"
	watcher "github.com/juju/juju/core/watcher"
	gomock "go.uber.org/mock/gomock"
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

// ApplicationCharmInfo mocks base method.
func (m *MockClient) ApplicationCharmInfo(arg0 string) (*charms.CharmInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApplicationCharmInfo", arg0)
	ret0, _ := ret[0].(*charms.CharmInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplicationCharmInfo indicates an expected call of ApplicationCharmInfo.
func (mr *MockClientMockRecorder) ApplicationCharmInfo(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApplicationCharmInfo", reflect.TypeOf((*MockClient)(nil).ApplicationCharmInfo), arg0)
}

// ApplicationConfig mocks base method.
func (m *MockClient) ApplicationConfig(arg0 string) (config.ConfigAttributes, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApplicationConfig", arg0)
	ret0, _ := ret[0].(config.ConfigAttributes)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplicationConfig indicates an expected call of ApplicationConfig.
func (mr *MockClientMockRecorder) ApplicationConfig(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApplicationConfig", reflect.TypeOf((*MockClient)(nil).ApplicationConfig), arg0)
}

// GetOpenedPorts mocks base method.
func (m *MockClient) GetOpenedPorts(arg0 string) (network.GroupedPortRanges, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOpenedPorts", arg0)
	ret0, _ := ret[0].(network.GroupedPortRanges)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOpenedPorts indicates an expected call of GetOpenedPorts.
func (mr *MockClientMockRecorder) GetOpenedPorts(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOpenedPorts", reflect.TypeOf((*MockClient)(nil).GetOpenedPorts), arg0)
}

// IsExposed mocks base method.
func (m *MockClient) IsExposed(arg0 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsExposed", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsExposed indicates an expected call of IsExposed.
func (mr *MockClientMockRecorder) IsExposed(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsExposed", reflect.TypeOf((*MockClient)(nil).IsExposed), arg0)
}

// Life mocks base method.
func (m *MockClient) Life(arg0 string) (life.Value, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Life", arg0)
	ret0, _ := ret[0].(life.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Life indicates an expected call of Life.
func (mr *MockClientMockRecorder) Life(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Life", reflect.TypeOf((*MockClient)(nil).Life), arg0)
}

// WatchApplication mocks base method.
func (m *MockClient) WatchApplication(arg0 string) (watcher.NotifyWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchApplication", arg0)
	ret0, _ := ret[0].(watcher.NotifyWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchApplication indicates an expected call of WatchApplication.
func (mr *MockClientMockRecorder) WatchApplication(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchApplication", reflect.TypeOf((*MockClient)(nil).WatchApplication), arg0)
}

// WatchApplications mocks base method.
func (m *MockClient) WatchApplications() (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchApplications")
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchApplications indicates an expected call of WatchApplications.
func (mr *MockClientMockRecorder) WatchApplications() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchApplications", reflect.TypeOf((*MockClient)(nil).WatchApplications))
}

// WatchOpenedPorts mocks base method.
func (m *MockClient) WatchOpenedPorts() (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchOpenedPorts")
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchOpenedPorts indicates an expected call of WatchOpenedPorts.
func (mr *MockClientMockRecorder) WatchOpenedPorts() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchOpenedPorts", reflect.TypeOf((*MockClient)(nil).WatchOpenedPorts))
}

// MockCAASFirewallerAPI is a mock of CAASFirewallerAPI interface.
type MockCAASFirewallerAPI struct {
	ctrl     *gomock.Controller
	recorder *MockCAASFirewallerAPIMockRecorder
}

// MockCAASFirewallerAPIMockRecorder is the mock recorder for MockCAASFirewallerAPI.
type MockCAASFirewallerAPIMockRecorder struct {
	mock *MockCAASFirewallerAPI
}

// NewMockCAASFirewallerAPI creates a new mock instance.
func NewMockCAASFirewallerAPI(ctrl *gomock.Controller) *MockCAASFirewallerAPI {
	mock := &MockCAASFirewallerAPI{ctrl: ctrl}
	mock.recorder = &MockCAASFirewallerAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCAASFirewallerAPI) EXPECT() *MockCAASFirewallerAPIMockRecorder {
	return m.recorder
}

// ApplicationCharmInfo mocks base method.
func (m *MockCAASFirewallerAPI) ApplicationCharmInfo(arg0 string) (*charms.CharmInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApplicationCharmInfo", arg0)
	ret0, _ := ret[0].(*charms.CharmInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplicationCharmInfo indicates an expected call of ApplicationCharmInfo.
func (mr *MockCAASFirewallerAPIMockRecorder) ApplicationCharmInfo(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApplicationCharmInfo", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).ApplicationCharmInfo), arg0)
}

// ApplicationConfig mocks base method.
func (m *MockCAASFirewallerAPI) ApplicationConfig(arg0 string) (config.ConfigAttributes, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApplicationConfig", arg0)
	ret0, _ := ret[0].(config.ConfigAttributes)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplicationConfig indicates an expected call of ApplicationConfig.
func (mr *MockCAASFirewallerAPIMockRecorder) ApplicationConfig(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApplicationConfig", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).ApplicationConfig), arg0)
}

// GetOpenedPorts mocks base method.
func (m *MockCAASFirewallerAPI) GetOpenedPorts(arg0 string) (network.GroupedPortRanges, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOpenedPorts", arg0)
	ret0, _ := ret[0].(network.GroupedPortRanges)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOpenedPorts indicates an expected call of GetOpenedPorts.
func (mr *MockCAASFirewallerAPIMockRecorder) GetOpenedPorts(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOpenedPorts", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).GetOpenedPorts), arg0)
}

// IsExposed mocks base method.
func (m *MockCAASFirewallerAPI) IsExposed(arg0 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsExposed", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsExposed indicates an expected call of IsExposed.
func (mr *MockCAASFirewallerAPIMockRecorder) IsExposed(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsExposed", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).IsExposed), arg0)
}

// WatchApplication mocks base method.
func (m *MockCAASFirewallerAPI) WatchApplication(arg0 string) (watcher.NotifyWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchApplication", arg0)
	ret0, _ := ret[0].(watcher.NotifyWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchApplication indicates an expected call of WatchApplication.
func (mr *MockCAASFirewallerAPIMockRecorder) WatchApplication(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchApplication", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).WatchApplication), arg0)
}

// WatchApplications mocks base method.
func (m *MockCAASFirewallerAPI) WatchApplications() (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchApplications")
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchApplications indicates an expected call of WatchApplications.
func (mr *MockCAASFirewallerAPIMockRecorder) WatchApplications() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchApplications", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).WatchApplications))
}

// WatchOpenedPorts mocks base method.
func (m *MockCAASFirewallerAPI) WatchOpenedPorts() (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchOpenedPorts")
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchOpenedPorts indicates an expected call of WatchOpenedPorts.
func (mr *MockCAASFirewallerAPIMockRecorder) WatchOpenedPorts() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchOpenedPorts", reflect.TypeOf((*MockCAASFirewallerAPI)(nil).WatchOpenedPorts))
}

// MockLifeGetter is a mock of LifeGetter interface.
type MockLifeGetter struct {
	ctrl     *gomock.Controller
	recorder *MockLifeGetterMockRecorder
}

// MockLifeGetterMockRecorder is the mock recorder for MockLifeGetter.
type MockLifeGetterMockRecorder struct {
	mock *MockLifeGetter
}

// NewMockLifeGetter creates a new mock instance.
func NewMockLifeGetter(ctrl *gomock.Controller) *MockLifeGetter {
	mock := &MockLifeGetter{ctrl: ctrl}
	mock.recorder = &MockLifeGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLifeGetter) EXPECT() *MockLifeGetterMockRecorder {
	return m.recorder
}

// Life mocks base method.
func (m *MockLifeGetter) Life(arg0 string) (life.Value, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Life", arg0)
	ret0, _ := ret[0].(life.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Life indicates an expected call of Life.
func (mr *MockLifeGetterMockRecorder) Life(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Life", reflect.TypeOf((*MockLifeGetter)(nil).Life), arg0)
}
