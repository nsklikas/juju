// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperator

import (
	"crypto/tls"
	"encoding/pem"
	"os"
	"path"
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/http/v2"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"
	"github.com/juju/worker/v3"
	"github.com/juju/worker/v3/dependency"

	"github.com/juju/juju/agent"
	apileadership "github.com/juju/juju/api/agent/leadership"
	"github.com/juju/juju/api/agent/secretsmanager"
	apiuniter "github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/caas"
	caasconstants "github.com/juju/juju/caas/kubernetes/provider/constants"
	"github.com/juju/juju/caas/kubernetes/provider/exec"
	coreleadership "github.com/juju/juju/core/leadership"
	"github.com/juju/juju/core/machinelock"
	"github.com/juju/juju/internal/worker/fortress"
	"github.com/juju/juju/internal/worker/leadership"
	"github.com/juju/juju/internal/worker/secretexpire"
	"github.com/juju/juju/internal/worker/secretrotate"
	"github.com/juju/juju/internal/worker/uniter"
	"github.com/juju/juju/internal/worker/uniter/charm"
	"github.com/juju/juju/internal/worker/uniter/operation"
	"github.com/juju/juju/internal/worker/uniter/runner"
	"github.com/juju/juju/juju/sockets"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/secrets"
)

type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
	Warningf(string, ...interface{})

	Child(string) loggo.Logger
}

// ManifoldConfig defines the names of the manifolds on which a
// Manifold will depend.
type ManifoldConfig struct {
	Logger Logger

	AgentName     string
	APICallerName string
	ClockName     string

	MachineLock           machinelock.Lock
	LeadershipGuarantee   time.Duration
	CharmDirName          string
	ProfileDir            string
	HookRetryStrategyName string
	TranslateResolverErr  func(error) error

	NewWorker          func(Config) (worker.Worker, error)
	NewClient          func(base.APICaller) Client
	NewCharmDownloader func(base.APICaller) Downloader

	NewExecClient     func(namespace string) (exec.Executor, error)
	RunListenerSocket func(*uniter.SocketConfig) (*sockets.Socket, error)

	LoadOperatorInfo func(paths Paths) (*caas.OperatorInfo, error)

	NewContainerStartWatcherClient func(Client) ContainerStartWatcher
}

func (config ManifoldConfig) Validate() error {
	if config.Logger == nil {
		return errors.NotValidf("missing Logger")
	}
	if config.AgentName == "" {
		return errors.NotValidf("empty AgentName")
	}
	if config.APICallerName == "" {
		return errors.NotValidf("empty APICallerName")
	}
	if config.ClockName == "" {
		return errors.NotValidf("empty ClockName")
	}
	if config.NewWorker == nil {
		return errors.NotValidf("missing NewWorker")
	}
	if config.NewClient == nil {
		return errors.NotValidf("missing NewClient")
	}
	if config.NewCharmDownloader == nil {
		return errors.NotValidf("missing NewCharmDownloader")
	}
	if config.CharmDirName == "" {
		return errors.NotValidf("missing CharmDirName")
	}
	if config.ProfileDir == "" {
		return errors.NotValidf("missing ProfileDir")
	}
	if config.MachineLock == nil {
		return errors.NotValidf("missing MachineLock")
	}
	if config.HookRetryStrategyName == "" {
		return errors.NotValidf("missing HookRetryStrategyName")
	}
	if config.LeadershipGuarantee == 0 {
		return errors.NotValidf("missing LeadershipGuarantee")
	}
	if config.NewExecClient == nil {
		return errors.NotValidf("missing NewExecClient")
	}
	return nil
}

// Manifold returns a dependency manifold that runs a caasoperator worker,
// using the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.AgentName,
			config.APICallerName,
			config.ClockName,
			config.CharmDirName,
			config.HookRetryStrategyName,
		},
		Start: func(context dependency.Context) (worker.Worker, error) {
			if err := config.Validate(); err != nil {
				return nil, errors.Trace(err)
			}

			var agent agent.Agent
			if err := context.Get(config.AgentName, &agent); err != nil {
				return nil, errors.Trace(err)
			}

			var apiCaller base.APICaller
			if err := context.Get(config.APICallerName, &apiCaller); err != nil {
				return nil, errors.Trace(err)
			}
			client := config.NewClient(apiCaller)
			downloader := config.NewCharmDownloader(apiCaller)

			var clock clock.Clock
			if err := context.Get(config.ClockName, &clock); err != nil {
				return nil, errors.Trace(err)
			}

			model, err := client.Model()
			if err != nil {
				return nil, errors.Trace(err)
			}

			var charmDirGuard fortress.Guard
			if err := context.Get(config.CharmDirName, &charmDirGuard); err != nil {
				return nil, err
			}

			var hookRetryStrategy params.RetryStrategy
			if err := context.Get(config.HookRetryStrategyName, &hookRetryStrategy); err != nil {
				return nil, err
			}

			// Configure and start the caasoperator worker.
			agentConfig := agent.CurrentConfig()
			tag := agentConfig.Tag()
			applicationTag, ok := tag.(names.ApplicationTag)
			if !ok {
				return nil, errors.Errorf("expected an application tag, got %v", tag)
			}
			newUniterFunc := func(unitTag names.UnitTag) *apiuniter.State {
				return apiuniter.NewState(apiCaller, unitTag)
			}
			newResourcesFacadeFunc := func(unitTag names.UnitTag) (*apiuniter.ResourcesFacadeClient, error) {
				return apiuniter.NewResourcesFacadeClient(apiCaller, unitTag)
			}
			newPayloadFacadeFunc := func() *apiuniter.PayloadFacadeClient {
				return apiuniter.NewPayloadFacadeClient(apiCaller)
			}
			leadershipTrackerFunc := func(unitTag names.UnitTag) coreleadership.Tracker {
				claimer := apileadership.NewClient(apiCaller)
				return leadership.NewTracker(unitTag, claimer, clock, config.LeadershipGuarantee)
			}

			runListenerSocketFunc := config.RunListenerSocket
			if runListenerSocketFunc == nil {
				runListenerSocketFunc = runListenerSocket
			}
			containerStartWatcherClient := config.NewContainerStartWatcherClient
			if containerStartWatcherClient == nil {
				containerStartWatcherClient = func(c Client) ContainerStartWatcher {
					return c
				}
			}
			jujuSecretsAPI := secretsmanager.NewClient(apiCaller)
			secretsBackendGetter := func() (secrets.BackendsClient, error) {
				return secrets.NewClient(jujuSecretsAPI)
			}
			secretRotateWatcherFunc := func(unitTag names.UnitTag, isLeader bool, rotateSecrets chan []string) (worker.Worker, error) {
				owners := []names.Tag{unitTag}
				if isLeader {
					appName, _ := names.UnitApplication(unitTag.Id())
					owners = append(owners, names.NewApplicationTag(appName))
				}
				return secretrotate.New(secretrotate.Config{
					SecretManagerFacade: jujuSecretsAPI,
					Clock:               clock,
					Logger:              config.Logger.Child("secretsrotate"),
					SecretOwners:        owners,
					RotateSecrets:       rotateSecrets,
				})
			}
			secretExpiryWatcherFunc := func(unitTag names.UnitTag, isLeader bool, expireRevisions chan []string) (worker.Worker, error) {
				owners := []names.Tag{unitTag}
				if isLeader {
					appName, _ := names.UnitApplication(unitTag.Id())
					owners = append(owners, names.NewApplicationTag(appName))
				}
				return secretexpire.New(secretexpire.Config{
					SecretManagerFacade: jujuSecretsAPI,
					Clock:               clock,
					Logger:              config.Logger.Child("secretsexpire"),
					SecretOwners:        owners,
					ExpireRevisions:     expireRevisions,
				})
			}

			wCfg := Config{
				Logger:                config.Logger,
				ModelUUID:             agentConfig.Model().Id(),
				ModelName:             model.Name,
				Application:           applicationTag.Id(),
				CharmGetter:           client,
				Clock:                 clock,
				DataDir:               agentConfig.DataDir(),
				ProfileDir:            config.ProfileDir,
				Downloader:            downloader,
				StatusSetter:          client,
				UnitGetter:            client,
				UnitRemover:           client,
				ApplicationWatcher:    client,
				ContainerStartWatcher: containerStartWatcherClient(client),
				VersionSetter:         client,
				StartUniterFunc:       uniter.StartUniter,
				RunListenerSocketFunc: runListenerSocketFunc,
				LeadershipTrackerFunc: leadershipTrackerFunc,
				UniterFacadeFunc:      newUniterFunc,
				ResourcesFacadeFunc:   newResourcesFacadeFunc,
				PayloadFacadeFunc:     newPayloadFacadeFunc,
				ExecClientGetter: func() (exec.Executor, error) {
					return config.NewExecClient(os.Getenv(caasconstants.OperatorNamespaceEnvName))
				},
			}

			loadOperatorInfoFunc := config.LoadOperatorInfo
			if loadOperatorInfoFunc == nil {
				loadOperatorInfoFunc = LoadOperatorInfo
			}
			operatorInfo, err := loadOperatorInfoFunc(wCfg.getPaths())
			if err != nil {
				return nil, errors.Trace(err)
			}
			wCfg.OperatorInfo = *operatorInfo
			wCfg.UniterParams = &uniter.UniterParams{
				NewOperationExecutor:    operation.NewExecutor,
				NewDeployer:             charm.NewDeployer,
				NewProcessRunner:        runner.NewRunner,
				DataDir:                 agentConfig.DataDir(),
				Clock:                   clock,
				MachineLock:             config.MachineLock,
				CharmDirGuard:           charmDirGuard,
				UpdateStatusSignal:      uniter.NewUpdateStatusTimer(),
				HookRetryStrategy:       hookRetryStrategy,
				TranslateResolverErr:    config.TranslateResolverErr,
				SecretsClient:           jujuSecretsAPI,
				SecretsBackendGetter:    secretsBackendGetter,
				SecretRotateWatcherFunc: secretRotateWatcherFunc,
				SecretExpiryWatcherFunc: secretExpiryWatcherFunc,
				Logger:                  wCfg.Logger.Child("uniter"),
			}
			wCfg.UniterParams.SocketConfig, err = socketConfig(operatorInfo)
			if err != nil {
				return nil, errors.Trace(err)
			}

			w, err := config.NewWorker(wCfg)
			if err != nil {
				return nil, errors.Trace(err)
			}
			return w, nil
		},
	}
}

func socketConfig(info *caas.OperatorInfo) (*uniter.SocketConfig, error) {
	tlsCert, err := tls.X509KeyPair([]byte(info.Cert), []byte(info.PrivateKey))
	if err != nil {
		return nil, errors.Annotatef(err, "cannot parse operator TLS certificate")
	}

	block, _ := pem.Decode([]byte(info.CACert))
	tlsCert.Certificate = append(tlsCert.Certificate, block.Bytes)
	tlsConfig := http.SecureTLSConfig()
	tlsConfig.Certificates = []tls.Certificate{tlsCert}

	serviceAddress := os.Getenv(caasconstants.OperatorServiceIPEnvName)
	if serviceAddress == "" {
		return nil, errors.Errorf("env %s missing", caasconstants.OperatorServiceIPEnvName)
	}

	operatorAddress := os.Getenv(caasconstants.OperatorPodIPEnvName)
	if operatorAddress == "" {
		return nil, errors.Errorf("env %s missing", caasconstants.OperatorPodIPEnvName)
	}

	sc := &uniter.SocketConfig{
		ServiceAddress:  serviceAddress,
		OperatorAddress: operatorAddress,
		TLSConfig:       tlsConfig,
	}
	return sc, nil
}

// LoadOperatorInfo loads the operator info file from the state dir.
func LoadOperatorInfo(paths Paths) (*caas.OperatorInfo, error) {
	filepath := path.Join(paths.State.BaseDir, caas.OperatorInfoFile)
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, errors.Annotatef(err, "reading operator info file %s", filepath)
	}
	return caas.UnmarshalOperatorInfo(data)
}
