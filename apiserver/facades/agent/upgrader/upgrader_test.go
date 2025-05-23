// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package upgrader_test

import (
	"path"

	"github.com/juju/errors"
	"github.com/juju/names/v5"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/version/v2"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facades/agent/upgrader"
	apiservertesting "github.com/juju/juju/apiserver/testing"
	jujutesting "github.com/juju/juju/juju/testing"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
	statetesting "github.com/juju/juju/state/testing"
	coretesting "github.com/juju/juju/testing"
	jujuversion "github.com/juju/juju/version"
)

type upgraderSuite struct {
	jujutesting.JujuConnSuite

	// These are raw State objects. Use them for setup and assertions, but
	// should never be touched by the API calls themselves
	rawMachine *state.Machine
	apiMachine *state.Machine
	upgrader   *upgrader.UpgraderAPI
	resources  *common.Resources
	authorizer apiservertesting.FakeAuthorizer
}

var _ = gc.Suite(&upgraderSuite{})

func (s *upgraderSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	s.resources = common.NewResources()
	s.AddCleanup(func(_ *gc.C) { s.resources.StopAll() })

	// Create a machine to work with
	var err error
	// The first machine created is the only one allowed to
	// JobManageModel
	s.apiMachine, err = s.State.AddMachine(state.UbuntuBase("12.10"), state.JobHostUnits,
		state.JobManageModel)
	c.Assert(err, jc.ErrorIsNil)
	s.rawMachine, err = s.State.AddMachine(state.UbuntuBase("12.10"), state.JobHostUnits)
	c.Assert(err, jc.ErrorIsNil)

	// The default auth is as the machine agent
	s.authorizer = apiservertesting.FakeAuthorizer{
		Tag: s.rawMachine.Tag(),
	}
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	s.upgrader, err = upgrader.NewUpgraderAPI(systemState, s.State, s.resources, s.authorizer)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *upgraderSuite) TearDownTest(c *gc.C) {
	if s.resources != nil {
		s.resources.StopAll()
	}
	s.JujuConnSuite.TearDownTest(c)
}

func (s *upgraderSuite) TestWatchAPIVersionNothing(c *gc.C) {
	// Not an error to watch nothing
	results, err := s.upgrader.WatchAPIVersion(params.Entities{})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 0)
}

func (s *upgraderSuite) TestWatchAPIVersion(c *gc.C) {
	args := params.Entities{
		Entities: []params.Entity{{Tag: s.rawMachine.Tag().String()}},
	}
	results, err := s.upgrader.WatchAPIVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Check(results.Results[0].NotifyWatcherId, gc.Not(gc.Equals), "")
	c.Check(results.Results[0].Error, gc.IsNil)
	resource := s.resources.Get(results.Results[0].NotifyWatcherId)
	c.Check(resource, gc.NotNil)

	w := resource.(state.NotifyWatcher)
	wc := statetesting.NewNotifyWatcherC(c, w)
	wc.AssertNoChange()

	err = statetesting.SetAgentVersion(s.State, version.MustParse("3.4.567.8"))
	c.Assert(err, jc.ErrorIsNil)
	wc.AssertOneChange()
	statetesting.AssertStop(c, w)
	wc.AssertClosed()
}

func (s *upgraderSuite) TestWatchAPIVersionApplication(c *gc.C) {
	app := s.Factory.MakeApplication(c, nil)
	authorizer := apiservertesting.FakeAuthorizer{
		Tag: app.Tag(),
	}
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	upgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, authorizer)
	c.Assert(err, jc.ErrorIsNil)
	args := params.Entities{
		Entities: []params.Entity{{Tag: app.Tag().String()}},
	}
	results, err := upgrader.WatchAPIVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Check(results.Results[0].NotifyWatcherId, gc.Not(gc.Equals), "")
	c.Check(results.Results[0].Error, gc.IsNil)
	resource := s.resources.Get(results.Results[0].NotifyWatcherId)
	c.Check(resource, gc.NotNil)

	w := resource.(state.NotifyWatcher)
	wc := statetesting.NewNotifyWatcherC(c, w)
	wc.AssertNoChange()

	err = statetesting.SetAgentVersion(s.State, version.MustParse("3.4.567.8"))
	c.Assert(err, jc.ErrorIsNil)
	wc.AssertOneChange()
	statetesting.AssertStop(c, w)
	wc.AssertClosed()
}

func (s *upgraderSuite) TestWatchAPIVersionUnit(c *gc.C) {
	app := s.Factory.MakeApplication(c, nil)
	providerId := "provider-id1"
	unit, err := app.AddUnit(state.AddUnitParams{ProviderId: &providerId})
	c.Assert(err, jc.ErrorIsNil)
	authorizer := apiservertesting.FakeAuthorizer{
		Tag: unit.Tag(),
	}
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	upgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, authorizer)
	c.Assert(err, jc.ErrorIsNil)
	args := params.Entities{
		Entities: []params.Entity{{Tag: unit.Tag().String()}},
	}
	results, err := upgrader.WatchAPIVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Check(results.Results[0].NotifyWatcherId, gc.Not(gc.Equals), "")
	c.Check(results.Results[0].Error, gc.IsNil)
	resource := s.resources.Get(results.Results[0].NotifyWatcherId)
	c.Check(resource, gc.NotNil)

	w := resource.(state.NotifyWatcher)
	wc := statetesting.NewNotifyWatcherC(c, w)
	wc.AssertNoChange()

	err = statetesting.SetAgentVersion(s.State, version.MustParse("3.4.567.8"))
	c.Assert(err, jc.ErrorIsNil)
	wc.AssertOneChange()
	statetesting.AssertStop(c, w)
	wc.AssertClosed()
}

func (s *upgraderSuite) TestWatchAPIVersionControllerAgent(c *gc.C) {
	node, err := s.State.ControllerNode("0")
	c.Assert(err, jc.ErrorIsNil)
	authorizer := apiservertesting.FakeAuthorizer{
		Tag: node.Tag(),
	}
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	upgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, authorizer)
	c.Assert(err, jc.ErrorIsNil)

	args := params.Entities{
		Entities: []params.Entity{{Tag: node.Tag().String()}},
	}
	results, err := upgrader.WatchAPIVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Check(results.Results[0].NotifyWatcherId, gc.Not(gc.Equals), "")
	c.Check(results.Results[0].Error, gc.IsNil)
	resource := s.resources.Get(results.Results[0].NotifyWatcherId)
	c.Check(resource, gc.NotNil)

	w := resource.(state.NotifyWatcher)
	wc := statetesting.NewNotifyWatcherC(c, w)
	wc.AssertNoChange()

	err = statetesting.SetAgentVersion(s.State, version.MustParse("3.4.567.8"))
	c.Assert(err, jc.ErrorIsNil)
	wc.AssertOneChange()
	statetesting.AssertStop(c, w)
	wc.AssertClosed()
}

func (s *upgraderSuite) TestWatchAPIVersionRefusesWrongAgent(c *gc.C) {
	// We are a machine agent, but not the one we are trying to track
	anAuthorizer := s.authorizer
	anAuthorizer.Tag = names.NewMachineTag("12354")
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	anUpgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, anAuthorizer)
	c.Check(err, jc.ErrorIsNil)
	args := params.Entities{
		Entities: []params.Entity{{Tag: s.rawMachine.Tag().String()}},
	}
	results, err := anUpgrader.WatchAPIVersion(args)
	// It is not an error to make the request, but the specific item is rejected
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Check(results.Results[0].NotifyWatcherId, gc.Equals, "")
	c.Assert(results.Results[0].Error, gc.DeepEquals, apiservertesting.ErrUnauthorized)
}

func (s *upgraderSuite) TestToolsNothing(c *gc.C) {
	// Not an error to watch nothing
	results, err := s.upgrader.Tools(params.Entities{})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 0)
}

func (s *upgraderSuite) TestToolsRefusesWrongAgent(c *gc.C) {
	anAuthorizer := s.authorizer
	anAuthorizer.Tag = names.NewMachineTag("12354")
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	anUpgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, anAuthorizer)
	c.Check(err, jc.ErrorIsNil)
	args := params.Entities{
		Entities: []params.Entity{{Tag: s.rawMachine.Tag().String()}},
	}
	results, err := anUpgrader.Tools(args)
	// It is not an error to make the request, but the specific item is rejected
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	toolResult := results.Results[0]
	c.Assert(toolResult.Error, gc.DeepEquals, apiservertesting.ErrUnauthorized)
}

func (s *upgraderSuite) TestToolsForAgent(c *gc.C) {
	current := coretesting.CurrentVersion()
	agent := params.Entity{Tag: s.rawMachine.Tag().String()}

	// The machine must have its existing tools set before we query for the
	// next tools. This is so that we can grab Arch and OSType without
	// having to pass it in again
	err := s.rawMachine.SetAgentVersion(current)
	c.Assert(err, jc.ErrorIsNil)

	args := params.Entities{Entities: []params.Entity{agent}}
	results, err := s.upgrader.Tools(args)
	c.Assert(err, jc.ErrorIsNil)
	assertTools := func() {
		c.Check(results.Results, gc.HasLen, 1)
		c.Assert(results.Results[0].Error, gc.IsNil)
		agentTools := results.Results[0].ToolsList[0]
		url := s.APIState.Addr()
		url.Scheme = "https"
		url.Path = path.Join(url.Path, "model", coretesting.ModelTag.Id(), "tools", current.String())
		c.Check(agentTools.URL, gc.Equals, url.String())
		c.Check(agentTools.Version, gc.DeepEquals, current)
	}
	assertTools()
}

func (s *upgraderSuite) TestSetToolsNothing(c *gc.C) {
	// Not an error to watch nothing
	results, err := s.upgrader.SetTools(params.EntitiesVersion{})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 0)
}

func (s *upgraderSuite) TestSetToolsRefusesWrongAgent(c *gc.C) {
	anAuthorizer := s.authorizer
	anAuthorizer.Tag = names.NewMachineTag("12354")
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	anUpgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, anAuthorizer)
	c.Check(err, jc.ErrorIsNil)
	args := params.EntitiesVersion{
		AgentTools: []params.EntityVersion{{
			Tag: s.rawMachine.Tag().String(),
			Tools: &params.Version{
				Version: coretesting.CurrentVersion(),
			},
		}},
	}

	results, err := anUpgrader.SetTools(args)
	c.Check(err, jc.ErrorIsNil)
	c.Assert(results.Results, gc.HasLen, 1)
	c.Assert(results.Results[0].Error, gc.DeepEquals, apiservertesting.ErrUnauthorized)
}

func (s *upgraderSuite) TestSetTools(c *gc.C) {
	current := coretesting.CurrentVersion()
	_, err := s.rawMachine.AgentTools()
	c.Assert(err, jc.Satisfies, errors.IsNotFound)
	args := params.EntitiesVersion{
		AgentTools: []params.EntityVersion{{
			Tag: s.rawMachine.Tag().String(),
			Tools: &params.Version{
				Version: current,
			}},
		},
	}
	results, err := s.upgrader.SetTools(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(results.Results, gc.HasLen, 1)
	c.Assert(results.Results[0].Error, gc.IsNil)
	// Check that the new value actually got set, we must Refresh because
	// it was set on a different Machine object
	err = s.rawMachine.Refresh()
	c.Assert(err, jc.ErrorIsNil)
	realTools, err := s.rawMachine.AgentTools()
	c.Assert(err, jc.ErrorIsNil)
	c.Check(realTools.Version, gc.Equals, current)
	c.Check(realTools.URL, gc.Equals, "")
}

func (s *upgraderSuite) TestDesiredVersionNothing(c *gc.C) {
	// Not an error to watch nothing
	results, err := s.upgrader.DesiredVersion(params.Entities{})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 0)
}

func (s *upgraderSuite) TestDesiredVersionRefusesWrongAgent(c *gc.C) {
	anAuthorizer := s.authorizer
	anAuthorizer.Tag = names.NewMachineTag("12354")
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	anUpgrader, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, anAuthorizer)
	c.Check(err, jc.ErrorIsNil)
	args := params.Entities{
		Entities: []params.Entity{{Tag: s.rawMachine.Tag().String()}},
	}
	results, err := anUpgrader.DesiredVersion(args)
	// It is not an error to make the request, but the specific item is rejected
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	toolResult := results.Results[0]
	c.Assert(toolResult.Error, gc.DeepEquals, apiservertesting.ErrUnauthorized)
}

func (s *upgraderSuite) TestDesiredVersionNoticesMixedAgents(c *gc.C) {
	args := params.Entities{Entities: []params.Entity{
		{Tag: s.rawMachine.Tag().String()},
		{Tag: "machine-12345"},
	}}
	results, err := s.upgrader.DesiredVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 2)
	c.Assert(results.Results[0].Error, gc.IsNil)
	agentVersion := results.Results[0].Version
	c.Assert(agentVersion, gc.NotNil)
	c.Check(*agentVersion, gc.DeepEquals, jujuversion.Current)

	c.Assert(results.Results[1].Error, gc.DeepEquals, apiservertesting.ErrUnauthorized)
	c.Assert(results.Results[1].Version, gc.IsNil)

}

func (s *upgraderSuite) TestDesiredVersionForAgent(c *gc.C) {
	args := params.Entities{Entities: []params.Entity{{Tag: s.rawMachine.Tag().String()}}}
	results, err := s.upgrader.DesiredVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Assert(results.Results[0].Error, gc.IsNil)
	agentVersion := results.Results[0].Version
	c.Assert(agentVersion, gc.NotNil)
	c.Check(*agentVersion, gc.DeepEquals, jujuversion.Current)
}

func (s *upgraderSuite) bumpDesiredAgentVersion(c *gc.C) version.Number {
	// In order to call SetModelAgentVersion we have to first SetTools on
	// all the existing machines
	current := coretesting.CurrentVersion()
	err := s.apiMachine.SetAgentVersion(current)
	c.Assert(err, jc.ErrorIsNil)
	err = s.rawMachine.SetAgentVersion(current)
	c.Assert(err, jc.ErrorIsNil)
	newer := current
	newer.Patch++
	err = s.State.SetModelAgentVersion(newer.Number, nil, false)
	c.Assert(err, jc.ErrorIsNil)
	cfg, err := s.Model.ModelConfig()
	c.Assert(err, jc.ErrorIsNil)
	vers, ok := cfg.AgentVersion()
	c.Assert(ok, jc.IsTrue)
	c.Check(vers, gc.Equals, newer.Number)
	return newer.Number
}

func (s *upgraderSuite) TestDesiredVersionUnrestrictedForAPIAgents(c *gc.C) {
	newVersion := s.bumpDesiredAgentVersion(c)
	// Grab a different Upgrader for the apiMachine
	authorizer := apiservertesting.FakeAuthorizer{
		Tag: s.apiMachine.Tag(),
	}
	systemState, err := s.StatePool.SystemState()
	c.Assert(err, jc.ErrorIsNil)
	upgraderAPI, err := upgrader.NewUpgraderAPI(systemState, s.State, s.resources, authorizer)
	c.Assert(err, jc.ErrorIsNil)
	args := params.Entities{Entities: []params.Entity{{Tag: s.apiMachine.Tag().String()}}}
	results, err := upgraderAPI.DesiredVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Assert(results.Results[0].Error, gc.IsNil)
	agentVersion := results.Results[0].Version
	c.Assert(agentVersion, gc.NotNil)
	c.Check(*agentVersion, gc.DeepEquals, newVersion)
}

func (s *upgraderSuite) TestDesiredVersionRestrictedForNonAPIAgents(c *gc.C) {
	newVersion := s.bumpDesiredAgentVersion(c)
	c.Assert(newVersion, gc.Not(gc.Equals), jujuversion.Current)
	args := params.Entities{Entities: []params.Entity{{Tag: s.rawMachine.Tag().String()}}}
	results, err := s.upgrader.DesiredVersion(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(results.Results, gc.HasLen, 1)
	c.Assert(results.Results[0].Error, gc.IsNil)
	agentVersion := results.Results[0].Version
	c.Assert(agentVersion, gc.NotNil)
	c.Check(*agentVersion, gc.DeepEquals, jujuversion.Current)
}
