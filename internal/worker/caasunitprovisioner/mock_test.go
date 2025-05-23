// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// TODO (manadart 2020-04-16): The hand-rolled mocks here make for brittle
// tests and are hard to reason about.
// Replace them with generated mocks as has been done with
// ProvisioningStatusSetter.

package caasunitprovisioner_test

import (
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/juju/names/v5"
	"github.com/juju/testing"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/common/charms"
	apicaasunitprovisioner "github.com/juju/juju/api/controller/caasunitprovisioner"
	"github.com/juju/juju/caas"
	"github.com/juju/juju/core/config"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/status"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/watchertest"
	"github.com/juju/juju/internal/worker/caasunitprovisioner"
	"github.com/juju/juju/rpc/params"
	coretesting "github.com/juju/juju/testing"
)

type fakeAPICaller struct {
	base.APICaller
}

type fakeBroker struct {
	caas.Broker
}

type fakeClient struct {
	caasunitprovisioner.Client
}

type mockServiceBroker struct {
	testing.Stub
	caas.ContainerEnvironProvider
	ensured        chan<- struct{}
	deleted        chan<- struct{}
	serviceStatus  status.StatusInfo
	serviceWatcher *watchertest.MockNotifyWatcher
}

func (m *mockServiceBroker) Provider() caas.ContainerEnvironProvider {
	return m
}

func (m *mockServiceBroker) EnsureService(appName string, statusCallback caas.StatusCallbackFunc, params *caas.ServiceParams, numUnits int, config config.ConfigAttributes) error {
	m.MethodCall(m, "EnsureService", appName, params, numUnits, config)
	statusCallback(appName, status.Waiting, "ensuring", map[string]interface{}{"foo": "bar"})
	m.ensured <- struct{}{}
	return m.NextErr()
}

func (m *mockServiceBroker) GetService(appName string, mode caas.DeploymentMode, includeClusterIP bool) (*caas.Service, error) {
	m.MethodCall(m, "GetService", appName, mode)
	scale := 4
	return &caas.Service{
		Id:        "id",
		Scale:     &scale,
		Addresses: network.NewMachineAddresses([]string{"10.0.0.1"}).AsProviderAddresses(),
		Status:    m.serviceStatus,
	}, m.NextErr()
}

func (m *mockServiceBroker) WatchService(appName string, mode caas.DeploymentMode) (watcher.NotifyWatcher, error) {
	m.MethodCall(m, "WatchService", appName, mode)
	return m.serviceWatcher, m.NextErr()
}

func (m *mockServiceBroker) DeleteService(appName string) error {
	m.MethodCall(m, "DeleteService", appName)
	m.deleted <- struct{}{}
	return m.NextErr()
}

func (m *mockServiceBroker) ApplyRawK8sSpec(spec string) error {
	m.MethodCall(m, "ApplyRawK8sSpec", spec)
	return m.NextErr()
}

func (m *mockServiceBroker) UnexposeService(appName string) error {
	m.MethodCall(m, "UnexposeService", appName)
	return m.NextErr()
}

type mockContainerBroker struct {
	testing.Stub
	caas.ContainerEnvironProvider
	unitsWatcher           *watchertest.MockNotifyWatcher
	operatorWatcher        *watchertest.MockNotifyWatcher
	reportedUnitStatus     status.Status
	reportedOperatorStatus status.Status
	units                  []caas.Unit
}

func (m *mockContainerBroker) Provider() caas.ContainerEnvironProvider {
	return m
}

func (m *mockContainerBroker) WatchUnits(appName string, mode caas.DeploymentMode) (watcher.NotifyWatcher, error) {
	m.MethodCall(m, "WatchUnits", appName, mode)
	return m.unitsWatcher, m.NextErr()
}

func (m *mockContainerBroker) Units(appName string, mode caas.DeploymentMode) ([]caas.Unit, error) {
	m.MethodCall(m, "Units", appName, mode)
	for i, u := range m.units {
		u.Status = status.StatusInfo{Status: m.reportedUnitStatus}
		m.units[i] = u

	}
	return m.units, m.NextErr()
}

func (m *mockContainerBroker) Operator(appName string) (*caas.Operator, error) {
	m.MethodCall(m, "Operator", appName)
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	return &caas.Operator{
		Dying: false,
		Status: status.StatusInfo{
			Status:  m.reportedOperatorStatus,
			Message: "testing 1. 2. 3.",
			Data:    map[string]interface{}{"zip": "zap"},
		},
	}, nil
}

func (m *mockContainerBroker) WatchOperator(appName string) (watcher.NotifyWatcher, error) {
	m.MethodCall(m, "WatchOperator", appName)
	return m.operatorWatcher, m.NextErr()
}

func (m *mockContainerBroker) AnnotateUnit(appName string, mode caas.DeploymentMode, podName string, unit names.UnitTag) error {
	m.MethodCall(m, "AnnotateUnit", appName, mode, podName, unit)
	return m.NextErr()
}

type mockApplicationGetter struct {
	testing.Stub
	watcher        *watchertest.MockStringsWatcher
	appWatcher     *watchertest.MockNotifyWatcher
	scaleWatcher   *watchertest.MockNotifyWatcher
	deploymentMode caas.DeploymentMode
	scale          int
}

func (m *mockApplicationGetter) WatchApplications() (watcher.StringsWatcher, error) {
	m.MethodCall(m, "WatchApplications")
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	return m.watcher, nil
}

func (m *mockApplicationGetter) WatchApplication(appName string) (watcher.NotifyWatcher, error) {
	m.MethodCall(m, "WatchApplication")
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	return m.appWatcher, nil
}

func (a *mockApplicationGetter) ApplicationConfig(appName string) (config.ConfigAttributes, error) {
	a.MethodCall(a, "ApplicationConfig", appName)
	return config.ConfigAttributes{
		"juju-external-hostname": "exthost",
	}, a.NextErr()
}

func (a *mockApplicationGetter) DeploymentMode(appName string) (caas.DeploymentMode, error) {
	a.MethodCall(a, "DeploymentMode", appName)
	return a.deploymentMode, a.NextErr()
}

func (a *mockApplicationGetter) WatchApplicationScale(application string) (watcher.NotifyWatcher, error) {
	a.MethodCall(a, "WatchApplicationScale", application)
	if err := a.NextErr(); err != nil {
		return nil, err
	}
	return a.scaleWatcher, nil
}

func (a *mockApplicationGetter) ApplicationScale(application string) (int, error) {
	a.MethodCall(a, "ApplicationScale", application)
	if err := a.NextErr(); err != nil {
		return 0, err
	}
	return a.scale, nil
}

type mockApplicationUpdater struct {
	testing.Stub
	updated chan<- struct{}
	cleared chan<- struct{}
}

func (m *mockApplicationUpdater) UpdateApplicationService(arg params.UpdateApplicationServiceArg) error {
	m.MethodCall(m, "UpdateApplicationService", arg)
	m.updated <- struct{}{}
	return m.NextErr()
}

func (m *mockApplicationUpdater) ClearApplicationResources(appName string) error {
	m.MethodCall(m, "ClearApplicationResources", appName)
	m.cleared <- struct{}{}
	return m.NextErr()
}

type mockProvisioningInfoGetterGetter struct {
	testing.Stub
	provisioningInfo apicaasunitprovisioner.ProvisioningInfo
	watcher          *watchertest.MockNotifyWatcher
	specRetrieved    chan struct{}
}

func (m *mockProvisioningInfoGetterGetter) setProvisioningInfo(provisioningInfo apicaasunitprovisioner.ProvisioningInfo) {
	m.provisioningInfo = provisioningInfo
	m.specRetrieved = make(chan struct{}, 2)
}

func (m *mockProvisioningInfoGetterGetter) assertSpecRetrieved(c *gc.C) {
	select {
	case <-m.specRetrieved:
	case <-time.After(coretesting.LongWait):
		c.Fatal("timed out waiting for pod spec to be retrieved")
	}
}

func (m *mockProvisioningInfoGetterGetter) ProvisioningInfo(appName string) (*apicaasunitprovisioner.ProvisioningInfo, error) {
	m.MethodCall(m, "ProvisioningInfo", appName)
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	provisioningInfo := m.provisioningInfo
	select {
	case m.specRetrieved <- struct{}{}:
	default:
	}
	return &provisioningInfo, nil
}

func (m *mockProvisioningInfoGetterGetter) WatchPodSpec(appName string) (watcher.NotifyWatcher, error) {
	m.MethodCall(m, "WatchPodSpec", appName)
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	return m.watcher, nil
}

type mockLifeGetter struct {
	testing.Stub
	mu            sync.Mutex
	life          life.Value
	lifeRetrieved chan struct{}
}

func (m *mockLifeGetter) setLife(life life.Value) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.life = life
	m.lifeRetrieved = make(chan struct{}, 1)
}

func (m *mockLifeGetter) Life(entityName string) (life.Value, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MethodCall(m, "Life", entityName)
	if err := m.NextErr(); err != nil {
		return "", err
	}
	life := m.life
	select {
	case m.lifeRetrieved <- struct{}{}:
	default:
	}
	return life, nil
}

type mockUnitUpdater struct {
	testing.Stub
	unitsInfo *params.UpdateApplicationUnitsInfo
}

func (m *mockUnitUpdater) UpdateUnits(arg params.UpdateApplicationUnits) (*params.UpdateApplicationUnitsInfo, error) {
	m.MethodCall(m, "UpdateUnits", arg)
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	return m.unitsInfo, nil
}

type mockCharmGetter struct {
	testing.Stub
	charmInfo *charms.CharmInfo
}

func (m *mockCharmGetter) ApplicationCharmInfo(appName string) (*charms.CharmInfo, error) {
	m.MethodCall(m, "ApplicationCharmInfo", appName)
	if err := m.NextErr(); err != nil {
		return nil, err
	}
	if m.charmInfo == nil {
		return nil, errors.NotFoundf("application %q", appName)
	}
	return m.charmInfo, nil
}
