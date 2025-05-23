// Copyright 2012-2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package context_test

import (
	"time"

	"github.com/juju/names/v5"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v3"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/agent/uniter"
	apiuniter "github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/core/relation"
	"github.com/juju/juju/core/status"
	"github.com/juju/juju/internal/worker/uniter/runner/context"
	"github.com/juju/juju/juju/testing"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
	jujutesting "github.com/juju/juju/testing"
)

type ContextRelationSuite struct {
	testing.JujuConnSuite
	app *state.Application
	rel *state.Relation
	ru  *state.RelationUnit

	st      api.Connection
	uniter  *apiuniter.State
	relUnit context.RelationUnit
}

var _ = gc.Suite(&ContextRelationSuite{})

func (s *ContextRelationSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	machine, err := s.State.AddMachine(state.UbuntuBase("12.10"), state.JobHostUnits)
	c.Assert(err, jc.ErrorIsNil)
	password, err := utils.RandomPassword()
	c.Assert(err, jc.ErrorIsNil)
	err = machine.SetPassword(password)
	c.Assert(err, jc.ErrorIsNil)
	err = machine.SetProvisioned("foo", "", "fake_nonce", nil)
	c.Assert(err, jc.ErrorIsNil)

	ch := s.AddTestingCharm(c, "riak")
	s.app = s.AddTestingApplication(c, "u", ch)
	rels, err := s.app.Relations()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(rels, gc.HasLen, 1)
	s.rel = rels[0]
	unit, err := s.app.AddUnit(state.AddUnitParams{})
	c.Assert(err, jc.ErrorIsNil)
	err = unit.AssignToMachine(machine)
	c.Assert(err, jc.ErrorIsNil)
	s.ru, err = s.rel.Unit(unit)
	c.Assert(err, jc.ErrorIsNil)
	err = s.ru.EnterScope(nil)
	c.Assert(err, jc.ErrorIsNil)

	password, err = utils.RandomPassword()
	c.Assert(err, jc.ErrorIsNil)
	err = unit.SetPassword(password)
	c.Assert(err, jc.ErrorIsNil)
	s.st = s.OpenAPIAs(c, unit.Tag(), password)
	s.uniter, err = uniter.NewFromConnection(s.st)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(s.uniter, gc.NotNil)

	apiRel, err := s.uniter.Relation(s.rel.Tag().(names.RelationTag))
	c.Assert(err, jc.ErrorIsNil)
	apiUnit, err := s.uniter.Unit(unit.Tag().(names.UnitTag))
	c.Assert(err, jc.ErrorIsNil)
	relUnit, err := apiRel.Unit(apiUnit.Tag())
	c.Assert(err, jc.ErrorIsNil)
	s.relUnit = &relUnitShim{relUnit}
}

func (s *ContextRelationSuite) TestMemberCaching(c *gc.C) {
	unit, err := s.app.AddUnit(state.AddUnitParams{})
	c.Assert(err, jc.ErrorIsNil)
	ru, err := s.rel.Unit(unit)
	c.Assert(err, jc.ErrorIsNil)
	err = ru.EnterScope(map[string]interface{}{"blib": "blob"})
	c.Assert(err, jc.ErrorIsNil)
	settings, err := ru.Settings()
	c.Assert(err, jc.ErrorIsNil)
	settings.Set("ping", "pong")
	_, err = settings.Write()
	c.Assert(err, jc.ErrorIsNil)

	cache := context.NewRelationCache(s.relUnit.ReadSettings, []string{"u/1"})
	ctx := context.NewContextRelation(s.relUnit, cache, false)

	// Check that uncached settings are read from state.
	m, err := ctx.ReadSettings("u/1")
	c.Assert(err, jc.ErrorIsNil)
	expectMap := settings.Map()
	expectSettings := convertMap(expectMap)
	c.Assert(m, gc.DeepEquals, expectSettings)

	// Check that changes to state do not affect the cached settings.
	settings.Set("ping", "pow")
	_, err = settings.Write()
	c.Assert(err, jc.ErrorIsNil)
	m, err = ctx.ReadSettings("u/1")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(m, gc.DeepEquals, expectSettings)
}

func (s *ContextRelationSuite) TestNonMemberCaching(c *gc.C) {
	unit, err := s.app.AddUnit(state.AddUnitParams{})
	c.Assert(err, jc.ErrorIsNil)
	ru, err := s.rel.Unit(unit)
	c.Assert(err, jc.ErrorIsNil)
	err = ru.EnterScope(map[string]interface{}{"blib": "blob"})
	c.Assert(err, jc.ErrorIsNil)
	settings, err := ru.Settings()
	c.Assert(err, jc.ErrorIsNil)
	settings.Set("ping", "pong")
	_, err = settings.Write()
	c.Assert(err, jc.ErrorIsNil)

	cache := context.NewRelationCache(s.relUnit.ReadSettings, nil)
	ctx := context.NewContextRelation(s.relUnit, cache, false)

	// Check that settings are read from state.
	m, err := ctx.ReadSettings("u/1")
	c.Assert(err, jc.ErrorIsNil)
	expectMap := settings.Map()
	expectSettings := convertMap(expectMap)
	c.Assert(m, gc.DeepEquals, expectSettings)

	// Check that changes to state do not affect the obtained settings.
	settings.Set("ping", "pow")
	_, err = settings.Write()
	c.Assert(err, jc.ErrorIsNil)
	m, err = ctx.ReadSettings("u/1")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(m, gc.DeepEquals, expectSettings)
}

func convertMap(settingsMap map[string]interface{}) params.Settings {
	result := make(params.Settings)
	for k, v := range settingsMap {
		result[k] = v.(string)
	}
	return result
}

func (s *ContextRelationSuite) TestSuspended(c *gc.C) {
	_, err := s.app.AddUnit(state.AddUnitParams{})
	c.Assert(err, jc.ErrorIsNil)
	err = s.rel.SetSuspended(true, "")
	c.Assert(err, jc.ErrorIsNil)

	ctx := context.NewContextRelation(s.relUnit, nil, false)
	err = s.relUnit.Relation().Refresh()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(ctx.Suspended(), jc.IsTrue)
}

func (s *ContextRelationSuite) TestSetStatus(c *gc.C) {
	_, err := s.app.AddUnit(state.AddUnitParams{})
	c.Assert(err, jc.ErrorIsNil)
	claimer, err := s.LeaseManager.Claimer("application-leadership", s.State.ModelUUID())
	c.Assert(err, jc.ErrorIsNil)
	err = claimer.Claim("u", "u/0", time.Minute)
	c.Assert(err, jc.ErrorIsNil)

	ctx := context.NewContextRelation(s.relUnit, nil, false)
	err = ctx.SetStatus(relation.Suspended)
	c.Assert(err, jc.ErrorIsNil)
	relStatus, err := s.rel.Status()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(relStatus.Status, gc.Equals, status.Suspended)
}

func (s *ContextRelationSuite) TestRemoteApplicationName(c *gc.C) {
	ctx := context.NewContextRelation(s.relUnit, nil, false)
	c.Assert(ctx.RemoteApplicationName(), gc.Equals, "u")
}

func (s *ContextRelationSuite) TestRemoteModelUUID(c *gc.C) {
	ctx := context.NewContextRelation(s.relUnit, nil, false)
	c.Assert(ctx.RemoteModelUUID(), gc.Equals, jujutesting.ModelTag.Id())
}

type relUnitShim struct {
	*apiuniter.RelationUnit
}

func (r *relUnitShim) Relation() context.Relation {
	return r.RelationUnit.Relation()
}
