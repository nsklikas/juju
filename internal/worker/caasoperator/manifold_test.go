// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperator_test

import (
	"os"
	"sync"
	"time"

	"github.com/juju/clock/testclock"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/worker/v3"
	"github.com/juju/worker/v3/dependency"
	dt "github.com/juju/worker/v3/dependency/testing"
	"github.com/juju/worker/v3/workertest"
	"go.uber.org/mock/gomock"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api/agent/secretsmanager"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/caas"
	"github.com/juju/juju/caas/kubernetes/provider/exec"
	"github.com/juju/juju/core/machinelock"
	"github.com/juju/juju/internal/worker/caasoperator"
	"github.com/juju/juju/internal/worker/caasoperator/mocks"
	"github.com/juju/juju/internal/worker/uniter"
	"github.com/juju/juju/rpc/params"
	_ "github.com/juju/juju/secrets/provider/all"
	coretesting "github.com/juju/juju/testing"
)

type ManifoldSuite struct {
	testing.IsolationSuite

	manifold        dependency.Manifold
	agent           fakeAgent
	apiCaller       *mocks.MockAPICaller
	charmDownloader fakeDownloader
	client          fakeClient
	clock           *testclock.Clock
	dataDir         string
	stub            testing.Stub
}

var _ = gc.Suite(&ManifoldSuite{})

func (s *ManifoldSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)

	os.Setenv("JUJU_OPERATOR_SERVICE_IP", "127.0.0.1")
	os.Setenv("JUJU_OPERATOR_POD_IP", "127.0.0.2")

	s.dataDir = c.MkDir()
	s.agent = fakeAgent{
		config: fakeAgentConfig{
			tag:     names.NewApplicationTag("gitlab"),
			dataDir: s.dataDir,
		},
	}
	s.clock = testclock.NewClock(time.Time{})
	s.stub.ResetCalls()
}

func (s *ManifoldSuite) TearDownTest(c *gc.C) {
	os.Setenv("JUJU_OPERATOR_SERVICE_IP", "")
	os.Setenv("JUJU_OPERATOR_POD_IP", "")

	s.IsolationSuite.TearDownTest(c)
}

func (s *ManifoldSuite) setupManifold(c *gc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)
	s.apiCaller = mocks.NewMockAPICaller(ctrl)

	s.manifold = caasoperator.Manifold(caasoperator.ManifoldConfig{
		AgentName:             "agent",
		APICallerName:         "api-caller",
		ClockName:             "clock",
		CharmDirName:          "charm-dir",
		HookRetryStrategyName: "hook-retry-strategy",
		ProfileDir:            "profile-dir",
		MachineLock:           &fakemachinelock{},
		NewWorker:             s.newWorker,
		NewClient:             s.newClient,
		NewCharmDownloader:    s.newCharmDownloader,
		LeadershipGuarantee:   30 * time.Second,
		NewExecClient: func(modelName string) (exec.Executor, error) {
			return mocks.NewMockExecutor(ctrl), nil
		},
		LoadOperatorInfo: func(paths caasoperator.Paths) (*caas.OperatorInfo, error) {
			return &caas.OperatorInfo{
				CACert:     coretesting.CACert,
				Cert:       coretesting.ServerCert,
				PrivateKey: coretesting.ServerKey,
			}, nil
		},
		Logger: loggo.GetLogger("test"),
	})
	return ctrl
}

func (s *ManifoldSuite) newContext(overlay map[string]interface{}) dependency.Context {
	resources := map[string]interface{}{
		"agent":               &s.agent,
		"api-caller":          s.apiCaller,
		"clock":               s.clock,
		"charm-dir":           &mockCharmDirGuard{},
		"hook-retry-strategy": params.RetryStrategy{},
	}
	for k, v := range overlay {
		resources[k] = v
	}
	return dt.StubContext(nil, resources)
}

func (s *ManifoldSuite) newWorker(config caasoperator.Config) (worker.Worker, error) {
	s.stub.MethodCall(s, "NewWorker", config)
	if err := s.stub.NextErr(); err != nil {
		return nil, err
	}
	w := worker.NewRunner(worker.RunnerParams{})
	s.AddCleanup(func(c *gc.C) { workertest.DirtyKill(c, w) })
	return w, nil
}

func (s *ManifoldSuite) newClient(caller base.APICaller) caasoperator.Client {
	s.stub.MethodCall(s, "NewClient", caller)
	return &s.client
}

func (s *ManifoldSuite) newCharmDownloader(caller base.APICaller) caasoperator.Downloader {
	s.stub.MethodCall(s, "NewCharmDownloader", caller)
	return &s.charmDownloader
}

var expectedInputs = []string{"agent", "api-caller", "clock", "charm-dir", "hook-retry-strategy"}

func (s *ManifoldSuite) TestInputs(c *gc.C) {
	ctrl := s.setupManifold(c)
	defer ctrl.Finish()

	c.Assert(s.manifold.Inputs, jc.SameContents, expectedInputs)
}

func (s *ManifoldSuite) TestMissingInputs(c *gc.C) {
	ctrl := s.setupManifold(c)
	defer ctrl.Finish()

	for _, input := range expectedInputs {
		context := s.newContext(map[string]interface{}{
			input: dependency.ErrMissing,
		})
		_, err := s.manifold.Start(context)
		c.Assert(errors.Cause(err), gc.Equals, dependency.ErrMissing)
	}
}

func (s *ManifoldSuite) TestStart(c *gc.C) {
	w := s.startWorkerClean(c)
	workertest.CleanKill(c, w)

	s.stub.CheckCallNames(c, "NewClient", "NewCharmDownloader", "NewWorker")
	s.stub.CheckCall(c, 0, "NewClient", s.apiCaller)
	s.stub.CheckCall(c, 1, "NewCharmDownloader", s.apiCaller)

	args := s.stub.Calls()[2].Args
	c.Assert(args, gc.HasLen, 1)
	c.Assert(args[0], gc.FitsTypeOf, caasoperator.Config{})
	config := args[0].(caasoperator.Config)

	// Don't care about some helper funcs.
	c.Assert(config.UniterParams, gc.NotNil)
	c.Assert(config.LeadershipTrackerFunc, gc.NotNil)
	c.Assert(config.UniterFacadeFunc, gc.NotNil)
	c.Assert(config.StartUniterFunc, gc.NotNil)
	c.Assert(config.RunListenerSocketFunc, gc.NotNil)
	c.Assert(config.UniterParams.UpdateStatusSignal, gc.NotNil)
	c.Assert(config.UniterParams.NewOperationExecutor, gc.NotNil)
	c.Assert(config.UniterParams.NewProcessRunner, gc.NotNil)
	c.Assert(config.UniterParams.NewDeployer, gc.NotNil)
	c.Assert(config.UniterParams.SecretRotateWatcherFunc, gc.NotNil)
	c.Assert(config.UniterParams.SecretsBackendGetter, gc.NotNil)
	c.Assert(config.Logger, gc.NotNil)
	c.Assert(config.ExecClientGetter, gc.NotNil)
	config.LeadershipTrackerFunc = nil
	config.StartUniterFunc = nil
	config.UniterFacadeFunc = nil
	config.ResourcesFacadeFunc = nil
	config.PayloadFacadeFunc = nil
	config.RunListenerSocketFunc = nil
	config.UniterParams.UpdateStatusSignal = nil
	config.UniterParams.NewOperationExecutor = nil
	config.UniterParams.NewDeployer = nil
	config.UniterParams.NewProcessRunner = nil
	config.UniterParams.SecretRotateWatcherFunc = nil
	config.UniterParams.SecretExpiryWatcherFunc = nil
	config.UniterParams.SecretsBackendGetter = nil
	config.Logger = nil
	config.ExecClientGetter = nil

	c.Assert(config.UniterParams.SocketConfig.TLSConfig, gc.NotNil)
	config.UniterParams.SocketConfig.TLSConfig = nil

	jujuSecretsAPI := secretsmanager.NewClient(s.apiCaller)
	c.Assert(config, jc.DeepEquals, caasoperator.Config{
		ModelUUID:             coretesting.ModelTag.Id(),
		ModelName:             "gitlab-model",
		Application:           "gitlab",
		ProfileDir:            "profile-dir",
		DataDir:               s.dataDir,
		CharmGetter:           &s.client,
		Clock:                 s.clock,
		Downloader:            &s.charmDownloader,
		StatusSetter:          &s.client,
		UnitGetter:            &s.client,
		UnitRemover:           &s.client,
		ContainerStartWatcher: &s.client,
		ApplicationWatcher:    &s.client,
		VersionSetter:         &s.client,
		UniterParams: &uniter.UniterParams{
			DataDir:       s.dataDir,
			MachineLock:   &fakemachinelock{},
			SecretsClient: jujuSecretsAPI,
			CharmDirGuard: &mockCharmDirGuard{},
			Clock:         s.clock,
			SocketConfig: &uniter.SocketConfig{
				ServiceAddress:  "127.0.0.1",
				OperatorAddress: "127.0.0.2",
			},
			Logger: loggo.GetLogger("test.uniter"),
		},
		OperatorInfo: caas.OperatorInfo{
			CACert:     coretesting.CACert,
			Cert:       coretesting.ServerCert,
			PrivateKey: coretesting.ServerKey,
		},
	})
}

func (s *ManifoldSuite) startWorkerClean(c *gc.C) worker.Worker {
	ctrl := s.setupManifold(c)
	defer ctrl.Finish()

	s.apiCaller.EXPECT().BestFacadeVersion("SecretsManager").AnyTimes().Return(1)

	w, err := s.manifold.Start(s.newContext(nil))
	c.Assert(err, jc.ErrorIsNil)
	workertest.CheckAlive(c, w)
	return w
}

type fakemachinelock struct {
	mu sync.Mutex
}

func (f *fakemachinelock) Acquire(spec machinelock.Spec) (func(), error) {
	f.mu.Lock()
	return func() {
		f.mu.Unlock()
	}, nil
}

func (f *fakemachinelock) Report(opts ...machinelock.ReportOption) (string, error) {
	return "", nil
}
