// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package engine_test

import (
	"github.com/juju/errors"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	worker "github.com/juju/worker/v3"
	"github.com/juju/worker/v3/dependency"
	dt "github.com/juju/worker/v3/dependency/testing"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/cmd/jujud/agent/engine"
)

type APIManifoldSuite struct {
	testing.IsolationSuite
	testing.Stub
	manifold dependency.Manifold
	worker   worker.Worker
}

var _ = gc.Suite(&APIManifoldSuite{})

func (s *APIManifoldSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)
	s.Stub = testing.Stub{}
	s.worker = &dummyWorker{}
	s.manifold = engine.APIManifold(engine.APIManifoldConfig{
		APICallerName: "api-caller-name",
	}, s.newWorker)
}

func (s *APIManifoldSuite) newWorker(apiCaller base.APICaller) (worker.Worker, error) {
	s.AddCall("newWorker", apiCaller)
	if err := s.NextErr(); err != nil {
		return nil, err
	}
	return s.worker, nil
}

func (s *APIManifoldSuite) TestInputs(c *gc.C) {
	c.Check(s.manifold.Inputs, jc.DeepEquals, []string{"api-caller-name"})
}

func (s *APIManifoldSuite) TestOutput(c *gc.C) {
	c.Check(s.manifold.Output, gc.IsNil)
}

func (s *APIManifoldSuite) TestStartAPIMissing(c *gc.C) {
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller-name": dependency.ErrMissing,
	})

	worker, err := s.manifold.Start(context)
	c.Check(worker, gc.IsNil)
	c.Check(err, gc.Equals, dependency.ErrMissing)
}

func (s *APIManifoldSuite) TestStartFailure(c *gc.C) {
	expectAPICaller := &dummyAPICaller{}
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller-name": expectAPICaller,
	})
	s.SetErrors(errors.New("some error"))

	worker, err := s.manifold.Start(context)
	c.Check(worker, gc.IsNil)
	c.Check(err, gc.ErrorMatches, "some error")
	s.CheckCalls(c, []testing.StubCall{{
		FuncName: "newWorker",
		Args:     []interface{}{expectAPICaller},
	}})
}

func (s *APIManifoldSuite) TestStartSuccess(c *gc.C) {
	expectAPICaller := &dummyAPICaller{}
	context := dt.StubContext(nil, map[string]interface{}{
		"api-caller-name": expectAPICaller,
	})

	worker, err := s.manifold.Start(context)
	c.Check(err, jc.ErrorIsNil)
	c.Check(worker, gc.Equals, s.worker)
	s.CheckCalls(c, []testing.StubCall{{
		FuncName: "newWorker",
		Args:     []interface{}{expectAPICaller},
	}})
}
