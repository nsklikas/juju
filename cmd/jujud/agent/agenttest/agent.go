// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package agenttest

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/juju/clock"
	"github.com/juju/cmd/v3"
	"github.com/juju/cmd/v3/cmdtesting"
	"github.com/juju/loggo"
	"github.com/juju/mgo/v3"
	mgotesting "github.com/juju/mgo/v3/testing"
	"github.com/juju/names/v5"
	"github.com/juju/replicaset/v3"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/version/v2"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/agent"
	agenttools "github.com/juju/juju/agent/tools"
	cmdutil "github.com/juju/juju/cmd/jujud/util"
	"github.com/juju/juju/controller"
	coredatabase "github.com/juju/juju/core/database"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/database"
	"github.com/juju/juju/environs/filestorage"
	"github.com/juju/juju/environs/simplestreams"
	sstesting "github.com/juju/juju/environs/simplestreams/testing"
	envtesting "github.com/juju/juju/environs/testing"
	envtools "github.com/juju/juju/environs/tools"
	"github.com/juju/juju/internal/worker/peergrouper"
	"github.com/juju/juju/juju/testing"
	"github.com/juju/juju/mongo"
	"github.com/juju/juju/mongo/mongotest"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/stateenvirons"
	coretesting "github.com/juju/juju/testing"
	coretools "github.com/juju/juju/tools"
)

// TODO (stickupkid): Remove this once we have a better way of using a logger
// in tests.
var logger = loggo.GetLogger("juju.agenttest.agent")

type patchingSuite interface {
	PatchValue(interface{}, interface{})
}

// InstallFakeEnsureMongo creates a new FakeEnsureMongo, patching
// out replicaset.CurrentConfig and cmdutil.EnsureMongoServerInstalled/Started.
func InstallFakeEnsureMongo(suite patchingSuite, dataDir string) *FakeEnsureMongo {
	f := &FakeEnsureMongo{}
	suite.PatchValue(&mongo.CurrentReplicasetConfig, f.CurrentConfig)
	suite.PatchValue(&cmdutil.EnsureMongoServerInstalled, f.EnsureMongo)
	ensureParams := cmdutil.NewEnsureMongoParams
	suite.PatchValue(&cmdutil.NewEnsureMongoParams, func(agentConfig agent.Config) (mongo.EnsureServerParams, error) {
		params, err := ensureParams(agentConfig)
		if err == nil {
			params.MongoDataDir = dataDir
		}
		return params, err
	})
	return f
}

// FakeEnsureMongo provides test fakes for the functions used to
// initialise MongoDB.
type FakeEnsureMongo struct {
	EnsureCount    int
	InitiateCount  int
	MongoDataDir   string
	OplogSize      int
	Info           controller.StateServingInfo
	InitiateParams peergrouper.InitiateMongoParams
	Err            error
}

func (f *FakeEnsureMongo) CurrentConfig(*mgo.Session) (*replicaset.Config, error) {
	// Return a dummy replicaset config that's good enough to
	// indicate that the replicaset is initiated.
	return &replicaset.Config{
		Members: []replicaset.Member{{}},
	}, nil
}

func (f *FakeEnsureMongo) EnsureMongo(ctx context.Context, args mongo.EnsureServerParams) error {
	f.EnsureCount++
	f.MongoDataDir, f.OplogSize = args.MongoDataDir, args.OplogSize
	f.Info = controller.StateServingInfo{
		APIPort:        args.APIPort,
		StatePort:      args.StatePort,
		Cert:           args.Cert,
		PrivateKey:     args.PrivateKey,
		CAPrivateKey:   args.CAPrivateKey,
		SharedSecret:   args.SharedSecret,
		SystemIdentity: args.SystemIdentity,
	}
	return f.Err
}

func (f *FakeEnsureMongo) InitiateMongo(p peergrouper.InitiateMongoParams) error {
	f.InitiateCount++
	f.InitiateParams = p
	return nil
}

// AgentSuite is a fixture to be used by agent test suites.
type AgentSuite struct {
	testing.JujuConnSuite

	// InitialDBOps can be set prior to calling PrimeStateAgentVersion,
	// ensuring that the functions are executed against the controller database
	// immediately after Dqlite is set up.
	InitialDBOps []func(db *sql.DB) error
}

func (s *AgentSuite) SetUpSuite(c *gc.C) {
	s.JujuConnSuite.SetUpSuite(c)

	s.InitialDBOps = make([]func(db *sql.DB) error, 0)
}

// PrimeAgent writes the configuration file and tools for an agent
// with the given entity name. It returns the agent's configuration and the
// current tools.
func (s *AgentSuite) PrimeAgent(c *gc.C, tag names.Tag, password string) (agent.ConfigSetterWriter, *coretools.Tools) {
	vers := coretesting.CurrentVersion()
	return s.PrimeAgentVersion(c, tag, password, vers)
}

// PrimeAgentVersion writes the configuration file and tools with version
// vers for an agent with the given entity name. It returns the agent's
// configuration and the current tools.
func (s *AgentSuite) PrimeAgentVersion(c *gc.C, tag names.Tag, password string, vers version.Binary) (agent.ConfigSetterWriter, *coretools.Tools) {
	c.Logf("priming agent %s", tag.String())

	store, err := filestorage.NewFileStorageWriter(c.MkDir())
	c.Assert(err, jc.ErrorIsNil)

	agentTools := envtesting.PrimeTools(c, store, s.DataDir(), "released", vers)
	ss := simplestreams.NewSimpleStreams(sstesting.TestDataSourceFactory())
	err = envtools.MergeAndWriteMetadata(ss, store, "released", "released", coretools.List{agentTools}, envtools.DoNotWriteMirrors)
	c.Assert(err, jc.ErrorIsNil)

	tools1, err := agenttools.ChangeAgentTools(s.DataDir(), tag.String(), vers)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tools1, gc.DeepEquals, agentTools)

	stateInfo := s.MongoInfo()
	apiInfo := s.APIInfo(c)

	paths := agent.DefaultPaths
	paths.DataDir = s.DataDir()
	paths.TransientDataDir = s.TransientDataDir()
	paths.LogDir = s.LogDir
	paths.MetricsSpoolDir = c.MkDir()

	dqlitePort := mgotesting.FindTCPPort()

	conf, err := agent.NewAgentConfig(
		agent.AgentConfigParams{
			Paths:             paths,
			Tag:               tag,
			UpgradedToVersion: vers.Number,
			Password:          password,
			Nonce:             agent.BootstrapNonce,
			APIAddresses:      apiInfo.Addrs,
			CACert:            stateInfo.CACert,
			Controller:        coretesting.ControllerTag,
			Model:             apiInfo.ModelTag,

			QueryTracingEnabled:   controller.DefaultQueryTracingEnabled,
			QueryTracingThreshold: controller.DefaultQueryTracingThreshold,

			DqlitePort: dqlitePort,
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	conf.SetPassword(password)
	c.Assert(conf.Write(), gc.IsNil)

	s.primeAPIHostPorts(c)
	return conf, agentTools
}

// PrimeStateAgent writes the configuration file and tools for
// a state agent with the given entity name. It returns the agent's
// configuration and the current tools.
func (s *AgentSuite) PrimeStateAgent(c *gc.C, tag names.Tag, password string) (agent.ConfigSetterWriter, *coretools.Tools) {
	vers := coretesting.CurrentVersion()
	return s.PrimeStateAgentVersion(c, tag, password, vers)
}

// PrimeStateAgentVersion writes the configuration file and tools with
// version vers for a state agent with the given entity name. It
// returns the agent's configuration and the current tools.
func (s *AgentSuite) PrimeStateAgentVersion(c *gc.C, tag names.Tag, password string, vers version.Binary) (
	agent.ConfigSetterWriter, *coretools.Tools,
) {
	stor, err := filestorage.NewFileStorageWriter(c.MkDir())
	c.Assert(err, jc.ErrorIsNil)

	agentTools := envtesting.PrimeTools(c, stor, s.DataDir(), "released", vers)
	tools1, err := agenttools.ChangeAgentTools(s.DataDir(), tag.String(), vers)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tools1, gc.DeepEquals, agentTools)

	model, err := s.State.Model()
	c.Assert(err, jc.ErrorIsNil)

	conf := s.WriteStateAgentConfig(c, tag, password, vers, model.ModelTag())
	s.primeAPIHostPorts(c)

	err = database.BootstrapDqlite(
		context.Background(),
		database.NewNodeManager(conf, true, logger, coredatabase.NoopSlowQueryLogger{}),
		logger,
		s.InitialDBOps...,
	)
	c.Assert(err, jc.ErrorIsNil)

	return conf, agentTools
}

// WriteStateAgentConfig creates and writes a state agent config.
func (s *AgentSuite) WriteStateAgentConfig(
	c *gc.C,
	tag names.Tag,
	password string,
	vers version.Binary,
	modelTag names.ModelTag,
) agent.ConfigSetterWriter {
	stateInfo := s.MongoInfo()
	apiPort := mgotesting.FindTCPPort()
	s.SetControllerConfigAPIPort(c, apiPort)
	apiAddr := []string{fmt.Sprintf("localhost:%d", apiPort)}
	dqlitePort := mgotesting.FindTCPPort()
	conf, err := agent.NewStateMachineConfig(
		agent.AgentConfigParams{
			Paths: agent.NewPathsWithDefaults(agent.Paths{
				DataDir: s.DataDir(),
				LogDir:  s.LogDir,
			}),
			Tag:                   tag,
			UpgradedToVersion:     vers.Number,
			Password:              password,
			Nonce:                 agent.BootstrapNonce,
			APIAddresses:          apiAddr,
			CACert:                stateInfo.CACert,
			Controller:            s.State.ControllerTag(),
			Model:                 modelTag,
			MongoMemoryProfile:    controller.DefaultMongoMemoryProfile,
			QueryTracingEnabled:   controller.DefaultQueryTracingEnabled,
			QueryTracingThreshold: controller.DefaultQueryTracingThreshold,
			DqlitePort:            dqlitePort,
		},
		controller.StateServingInfo{
			Cert:         coretesting.ServerCert,
			PrivateKey:   coretesting.ServerKey,
			CAPrivateKey: coretesting.CAKey,
			StatePort:    mgotesting.MgoServer.Port(),
			APIPort:      apiPort,
		})
	c.Assert(err, jc.ErrorIsNil)

	conf.SetPassword(password)
	c.Assert(conf.Write(), gc.IsNil)

	return conf
}

// SetControllerConfigAPIPort resets the API port in controller config
// to the value provided - this is useful in tests that create
// multiple agents and only start one, so that the API port the http
// server listens on matches the one the agent tries to connect to.
func (s *AgentSuite) SetControllerConfigAPIPort(c *gc.C, apiPort int) {
	// Need to update the controller config with this new API port as
	// well - this is a nasty hack but... oh well!
	controller.AllowedUpdateConfigAttributes.Add("api-port")
	defer func() {
		controller.AllowedUpdateConfigAttributes.Remove("api-port")
	}()
	err := s.State.UpdateControllerConfig(map[string]interface{}{
		"api-port": apiPort,
	}, nil)
	c.Assert(err, jc.ErrorIsNil)
	// Ensure that the local controller config is also up to date.
	s.ControllerConfig["api-port"] = apiPort
}

func (s *AgentSuite) primeAPIHostPorts(c *gc.C) {
	apiInfo := s.APIInfo(c)

	c.Assert(apiInfo.Addrs, gc.HasLen, 1)
	mHP, err := network.ParseMachineHostPort(apiInfo.Addrs[0])
	c.Assert(err, jc.ErrorIsNil)

	hostPorts := network.SpaceHostPorts{
		{SpaceAddress: network.SpaceAddress{MachineAddress: mHP.MachineAddress}, NetPort: mHP.NetPort}}

	err = s.State.SetAPIHostPorts([]network.SpaceHostPorts{hostPorts})
	c.Assert(err, jc.ErrorIsNil)

	c.Logf("api host ports primed %#v", hostPorts)
}

// InitAgent initialises the given agent command with additional
// arguments as provided.
func (s *AgentSuite) InitAgent(c *gc.C, a cmd.Command, args ...string) {
	args = append([]string{"--data-dir", s.DataDir()}, args...)
	err := cmdtesting.InitCommand(a, args)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *AgentSuite) AssertCanOpenState(c *gc.C, tag names.Tag, dataDir string) {
	config, err := agent.ReadConfig(agent.ConfigPath(dataDir, tag))
	c.Assert(err, jc.ErrorIsNil)

	info, ok := config.MongoInfo()
	c.Assert(ok, jc.IsTrue)

	session, err := mongo.DialWithInfo(*info, mongotest.DialOpts())
	c.Assert(err, jc.ErrorIsNil)
	defer session.Close()

	pool, err := state.OpenStatePool(state.OpenParams{
		Clock:              clock.WallClock,
		ControllerTag:      config.Controller(),
		ControllerModelTag: config.Model(),
		MongoSession:       session,
		NewPolicy:          stateenvirons.GetNewPolicyFunc(),
	})
	c.Assert(err, jc.ErrorIsNil)
	pool.Close()
}

func (s *AgentSuite) AssertCannotOpenState(c *gc.C, tag names.Tag, dataDir string) {
	config, err := agent.ReadConfig(agent.ConfigPath(dataDir, tag))
	c.Assert(err, jc.ErrorIsNil)

	_, ok := config.MongoInfo()
	c.Assert(ok, jc.IsFalse)
}
