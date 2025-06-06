// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package runner_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/juju/clock/testclock"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v3"
	"github.com/juju/utils/v3/fs"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/internal/worker/uniter/runner"
	"github.com/juju/juju/internal/worker/uniter/runner/context"
	runnertesting "github.com/juju/juju/internal/worker/uniter/runner/testing"
	"github.com/juju/juju/juju/testing"
	"github.com/juju/juju/state"
	"github.com/juju/juju/testcharms"
)

var (
	hookName      = "something-happened"
	echoPidScript = "echo $$ > pid"
)

type ContextSuite struct {
	testing.JujuConnSuite

	paths          runnertesting.RealPaths
	factory        runner.Factory
	contextFactory context.ContextFactory
	membership     map[int][]string

	st          api.Connection
	model       *state.Model
	application *state.Application
	machine     *state.Machine
	unit        *state.Unit
	uniter      *uniter.State
	apiUnit     *uniter.Unit
	payloads    *uniter.PayloadFacadeClient
	secrets     *runnertesting.SecretsContextAccessor

	apiRelunits map[int]*uniter.RelationUnit
	relch       *state.Charm
	relunits    map[int]*state.RelationUnit
}

func (s *ContextSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)

	s.machine = nil

	ch := s.AddTestingCharm(c, "wordpress-nolimit")
	s.application = s.AddTestingApplication(c, "u", ch)
	s.unit = s.AddUnit(c, s.application)

	s.secrets = &runnertesting.SecretsContextAccessor{}

	password, err := utils.RandomPassword()
	c.Assert(err, jc.ErrorIsNil)
	err = s.unit.SetPassword(password)
	c.Assert(err, jc.ErrorIsNil)
	s.st = s.OpenAPIAs(c, s.unit.Tag(), password)
	s.uniter, err = uniter.NewFromConnection(s.st)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(s.uniter, gc.NotNil)
	s.apiUnit, err = s.uniter.Unit(s.unit.Tag().(names.UnitTag))
	c.Assert(err, jc.ErrorIsNil)
	s.model, err = s.State.Model()
	c.Assert(err, jc.ErrorIsNil)
	s.payloads = uniter.NewPayloadFacadeClient(s.st)

	s.paths = runnertesting.NewRealPaths(c)
	s.membership = map[int][]string{}

	// Note: The unit must always have a charm URL set, because this
	// happens as part of the installation process (that happens
	// before the initial install hook).
	err = s.unit.SetCharmURL(ch.URL())
	c.Assert(err, jc.ErrorIsNil)
	s.relch = s.AddTestingCharm(c, "mysql")
	s.relunits = map[int]*state.RelationUnit{}
	s.apiRelunits = map[int]*uniter.RelationUnit{}
	s.AddContextRelation(c, "db0")
	s.AddContextRelation(c, "db1")

	s.contextFactory, err = context.NewContextFactory(context.FactoryConfig{
		State:            s.uniter,
		Unit:             s.apiUnit,
		Payloads:         s.payloads,
		Tracker:          &runnertesting.FakeTracker{},
		GetRelationInfos: s.getRelationInfos,
		SecretsClient:    s.secrets,
		Paths:            s.paths,
		Clock:            testclock.NewClock(time.Time{}),
		Logger:           loggo.GetLogger("test"),
	})
	c.Assert(err, jc.ErrorIsNil)

	factory, err := runner.NewFactory(
		s.paths,
		s.contextFactory,
		runner.NewRunner,
		nil,
	)
	c.Assert(err, jc.ErrorIsNil)
	s.factory = factory
}

func (s *ContextSuite) AddContextRelation(c *gc.C, name string) {
	s.AddTestingApplication(c, name, s.relch)
	eps, err := s.State.InferEndpoints("u", name)
	c.Assert(err, jc.ErrorIsNil)
	rel, err := s.State.AddRelation(eps...)
	c.Assert(err, jc.ErrorIsNil)
	ru, err := rel.Unit(s.unit)
	c.Assert(err, jc.ErrorIsNil)
	err = ru.EnterScope(map[string]interface{}{"relation-name": name})
	c.Assert(err, jc.ErrorIsNil)
	s.relunits[rel.Id()] = ru
	apiRel, err := s.uniter.Relation(rel.Tag().(names.RelationTag))
	c.Assert(err, jc.ErrorIsNil)
	apiRelUnit, err := apiRel.Unit(s.apiUnit.Tag())
	c.Assert(err, jc.ErrorIsNil)
	s.apiRelunits[rel.Id()] = apiRelUnit
}

func (s *ContextSuite) AddUnit(c *gc.C, svc *state.Application) *state.Unit {
	unit, err := svc.AddUnit(state.AddUnitParams{})
	c.Assert(err, jc.ErrorIsNil)
	if s.machine != nil {
		err = unit.AssignToMachine(s.machine)
		c.Assert(err, jc.ErrorIsNil)
		return unit
	}

	err = s.State.AssignUnit(unit, state.AssignCleanEmpty)
	c.Assert(err, jc.ErrorIsNil)
	machineId, err := unit.AssignedMachineId()
	c.Assert(err, jc.ErrorIsNil)
	s.machine, err = s.State.Machine(machineId)
	c.Assert(err, jc.ErrorIsNil)
	zone := "a-zone"
	hwc := instance.HardwareCharacteristics{
		AvailabilityZone: &zone,
	}
	err = s.machine.SetProvisioned("i-exist", "", "fake_nonce", &hwc)
	c.Assert(err, jc.ErrorIsNil)

	name := strings.Replace(unit.Name(), "/", "-", 1)
	privateAddr := network.NewSpaceAddress(name+".testing.invalid", network.WithScope(network.ScopeCloudLocal))
	err = s.machine.SetProviderAddresses(privateAddr)
	c.Assert(err, jc.ErrorIsNil)
	return unit
}

func (s *ContextSuite) SetCharm(c *gc.C, name string) {
	err := os.RemoveAll(s.paths.GetCharmDir())
	c.Assert(err, jc.ErrorIsNil)
	err = fs.Copy(testcharms.Repo.CharmDirPath(name), s.paths.GetCharmDir())
	c.Assert(err, jc.ErrorIsNil)
}

func (s *ContextSuite) getRelationInfos() map[int]*context.RelationInfo {
	info := map[int]*context.RelationInfo{}
	for relId, relUnit := range s.apiRelunits {
		info[relId] = &context.RelationInfo{
			RelationUnit: &relUnitShim{relUnit},
			MemberNames:  s.membership[relId],
		}
	}
	return info
}

// hookSpec supports makeCharm.
type hookSpec struct {
	// dir is the directory to create the hook in.
	dir string
	// name is the name of the hook.
	name string
	// perm is the file permissions of the hook.
	perm os.FileMode
	// code is the exit status of the hook.
	code int
	// stdout holds a string to print to stdout
	stdout string
	// stderr holds a string to print to stderr
	stderr string
	// background holds a string to print in the background after 0.2s.
	background string
	// missingShebang will omit the '#!/bin/bash' line
	missingShebang bool
	// charmMissing will remove the charm before running the hook
	charmMissing bool
}

// makeCharm constructs a fake charm dir containing a single named hook
// with permissions perm and exit code code. If output is non-empty,
// the charm will write it to stdout and stderr, with each one prefixed
// by name of the stream.
func makeCharm(c *gc.C, spec hookSpec, charmDir string) {
	dir := charmDir
	if spec.dir != "" {
		dir = filepath.Join(dir, spec.dir)
		err := os.Mkdir(dir, 0755)
		c.Assert(err, jc.ErrorIsNil)
	}
	if !spec.charmMissing {
		makeCharmMetadata(c, charmDir)
	}
	c.Logf("openfile perm %v", spec.perm)
	hook, err := os.OpenFile(
		filepath.Join(dir, spec.name), os.O_CREATE|os.O_WRONLY, spec.perm,
	)
	c.Assert(err, jc.ErrorIsNil)
	defer func() {
		c.Assert(hook.Close(), gc.IsNil)
	}()

	printf := func(f string, a ...interface{}) {
		_, err := fmt.Fprintf(hook, f+"\n", a...)
		c.Assert(err, jc.ErrorIsNil)
	}
	if !spec.missingShebang {
		printf("#!/bin/bash")
	}
	printf(echoPidScript)
	if spec.stdout != "" {
		printf("echo %s", spec.stdout)
	}
	if spec.stderr != "" {
		printf("echo %s >&2", spec.stderr)
	}
	if spec.background != "" {
		// Print something fairly quickly, then sleep for
		// quite a long time - if the hook execution is
		// blocking because of the background process,
		// the hook execution will take much longer than
		// expected.
		printf("(sleep 0.2; echo %s; sleep 10) &", spec.background)
	}
	printf("exit %d", spec.code)
}

func makeCharmMetadata(c *gc.C, charmDir string) {
	err := os.MkdirAll(charmDir, 0755)
	c.Assert(err, jc.ErrorIsNil)
	err = os.WriteFile(path.Join(charmDir, "metadata.yaml"), nil, 0664)
	c.Assert(err, jc.ErrorIsNil)
}

type relUnitShim struct {
	*uniter.RelationUnit
}

func (r *relUnitShim) Relation() context.Relation {
	return r.RelationUnit.Relation()
}
