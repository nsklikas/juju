// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package lifeflag_test

import (
	"github.com/juju/errors"
	"github.com/juju/names/v5"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/worker/v3"
	"github.com/juju/worker/v3/dependency"
	dt "github.com/juju/worker/v3/dependency/testing"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/cmd/jujud/agent/engine"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/internal/worker/lifeflag"
)

type ManifoldSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&ManifoldSuite{})

func (*ManifoldSuite) TestInputs(c *gc.C) {
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{
		APICallerName: "boris",
	})
	c.Check(manifold.Inputs, jc.DeepEquals, []string{"boris"})
}

func (*ManifoldSuite) TestFilter(c *gc.C) {
	expect := errors.New("squish")
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{
		Filter: func(error) error { return expect },
	})
	actual := manifold.Filter(errors.New("blarg"))
	c.Check(actual, gc.Equals, expect)
}

func (*ManifoldSuite) TestOutputBadWorker(c *gc.C) {
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{})
	worker := struct{ worker.Worker }{}
	var flag engine.Flag
	err := manifold.Output(worker, &flag)
	c.Check(err, gc.ErrorMatches, "expected in to implement Flag; got a .*")
}

func (*ManifoldSuite) TestOutputBadTarget(c *gc.C) {
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{})
	worker := &lifeflag.Worker{}
	var flag interface{}
	err := manifold.Output(worker, &flag)
	c.Check(err, gc.ErrorMatches, "expected out to be a \\*Flag; got a .*")
}

func (*ManifoldSuite) TestOutputSuccess(c *gc.C) {
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{})
	worker := &lifeflag.Worker{}
	var flag engine.Flag
	err := manifold.Output(worker, &flag)
	c.Check(err, jc.ErrorIsNil)
	c.Check(flag, gc.Equals, worker)
}

func (*ManifoldSuite) TestMissingAPICaller(c *gc.C) {
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller": dependency.ErrMissing,
	})
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{
		APICallerName: "api-caller",
	})

	worker, err := manifold.Start(context)
	c.Check(worker, gc.IsNil)
	c.Check(errors.Cause(err), gc.Equals, dependency.ErrMissing)
}

func (*ManifoldSuite) TestNewWorkerError(c *gc.C) {
	expectFacade := struct{ lifeflag.Facade }{}
	expectEntity := names.NewMachineTag("33")
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller": struct{ base.APICaller }{},
	})
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{
		APICallerName: "api-caller",
		Entity:        expectEntity,
		Result:        life.IsNotAlive,
		NewFacade: func(_ base.APICaller) (lifeflag.Facade, error) {
			return expectFacade, nil
		},
		NewWorker: func(config lifeflag.Config) (worker.Worker, error) {
			c.Check(config.Facade, gc.Equals, expectFacade)
			c.Check(config.Entity, gc.Equals, expectEntity)
			c.Check(config.Result, gc.NotNil) // uncomparable
			return nil, errors.New("boof")
		},
	})

	worker, err := manifold.Start(context)
	c.Check(worker, gc.IsNil)
	c.Check(err, gc.ErrorMatches, "boof")
}

func (*ManifoldSuite) TestNewWorkerSuccess(c *gc.C) {
	expectWorker := &struct{ worker.Worker }{}
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller": struct{ base.APICaller }{},
	})
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{
		APICallerName: "api-caller",
		NewFacade: func(_ base.APICaller) (lifeflag.Facade, error) {
			return struct{ lifeflag.Facade }{}, nil
		},
		NewWorker: func(_ lifeflag.Config) (worker.Worker, error) {
			return expectWorker, nil
		},
	})

	worker, err := manifold.Start(context)
	c.Check(worker, gc.Equals, expectWorker)
	c.Check(err, jc.ErrorIsNil)
}

func (*ManifoldSuite) TestNewWorkerSuccessWithAgentName(c *gc.C) {
	expectWorker := &struct{ worker.Worker }{}
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller": struct{ base.APICaller }{},
		"agent-name": &fakeAgent{config: fakeConfig{tag: names.NewUnitTag("ubuntu/0")}},
	})
	manifold := lifeflag.Manifold(lifeflag.ManifoldConfig{
		APICallerName: "api-caller",
		AgentName:     "agent-name",
		NewFacade: func(_ base.APICaller) (lifeflag.Facade, error) {
			return struct{ lifeflag.Facade }{}, nil
		},
		NewWorker: func(config lifeflag.Config) (worker.Worker, error) {
			c.Check(config.Entity, gc.Equals, names.NewUnitTag("ubuntu/0"))
			return expectWorker, nil
		},
	})

	worker, err := manifold.Start(context)
	c.Check(worker, gc.Equals, expectWorker)
	c.Check(err, jc.ErrorIsNil)
}

type fakeAgent struct {
	agent.Agent
	config fakeConfig
}

func (f *fakeAgent) CurrentConfig() agent.Config {
	return &f.config
}

type fakeConfig struct {
	agent.Config
	tag names.Tag
}

func (f *fakeConfig) Tag() names.Tag {
	return f.tag
}
