// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasfirewaller_test

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/worker/v3"
	"github.com/juju/worker/v3/dependency"
	dt "github.com/juju/worker/v3/dependency/testing"
	"github.com/juju/worker/v3/workertest"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/internal/worker/caasfirewaller"
	coretesting "github.com/juju/juju/testing"
)

type ManifoldSuite struct {
	testing.IsolationSuite
	testing.Stub
	manifold dependency.Manifold
	context  dependency.Context

	apiCaller fakeAPICaller
	broker    fakeBroker
	client    fakeClient
}

var _ = gc.Suite(&ManifoldSuite{})

func (s *ManifoldSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)
	s.ResetCalls()

	s.context = s.newContext(nil)
	s.manifold = caasfirewaller.Manifold(s.validConfig())
}

func (s *ManifoldSuite) validConfig() caasfirewaller.ManifoldConfig {
	return caasfirewaller.ManifoldConfig{
		APICallerName:  "api-caller",
		BrokerName:     "broker",
		ControllerUUID: coretesting.ControllerTag.Id(),
		ModelUUID:      coretesting.ModelTag.Id(),
		NewClient:      s.newClient,
		NewWorker:      s.newWorker,
		Logger:         loggo.GetLogger("test"),
	}
}

func (s *ManifoldSuite) newClient(apiCaller base.APICaller) caasfirewaller.Client {
	s.MethodCall(s, "NewClient", apiCaller)
	return &s.client
}

func (s *ManifoldSuite) newWorker(config caasfirewaller.Config) (worker.Worker, error) {
	s.MethodCall(s, "NewWorker", config)
	if err := s.NextErr(); err != nil {
		return nil, err
	}
	w := worker.NewRunner(worker.RunnerParams{})
	s.AddCleanup(func(c *gc.C) { workertest.DirtyKill(c, w) })
	return w, nil
}

func (s *ManifoldSuite) newContext(overlay map[string]interface{}) dependency.Context {
	resources := map[string]interface{}{
		"api-caller": &s.apiCaller,
		"broker":     &s.broker,
	}
	for k, v := range overlay {
		resources[k] = v
	}
	return dt.StubContext(nil, resources)
}

func (s *ManifoldSuite) TestMissingControllerUUID(c *gc.C) {
	config := s.validConfig()
	config.ControllerUUID = ""
	s.checkConfigInvalid(c, config, "empty ControllerUUID not valid")
}

func (s *ManifoldSuite) TestMissingModelUUID(c *gc.C) {
	config := s.validConfig()
	config.ModelUUID = ""
	s.checkConfigInvalid(c, config, "empty ModelUUID not valid")
}

func (s *ManifoldSuite) TestMissingAPICallerName(c *gc.C) {
	config := s.validConfig()
	config.APICallerName = ""
	s.checkConfigInvalid(c, config, "empty APICallerName not valid")
}

func (s *ManifoldSuite) TestMissingBrokerName(c *gc.C) {
	config := s.validConfig()
	config.BrokerName = ""
	s.checkConfigInvalid(c, config, "empty BrokerName not valid")
}

func (s *ManifoldSuite) TestMissingNewWorker(c *gc.C) {
	config := s.validConfig()
	config.NewWorker = nil
	s.checkConfigInvalid(c, config, "nil NewWorker not valid")
}

func (s *ManifoldSuite) TestMissingLogger(c *gc.C) {
	config := s.validConfig()
	config.Logger = nil
	s.checkConfigInvalid(c, config, "nil Logger not valid")
}

func (s *ManifoldSuite) checkConfigInvalid(c *gc.C, config caasfirewaller.ManifoldConfig, expect string) {
	err := config.Validate()
	c.Check(err, gc.ErrorMatches, expect)
	c.Check(err, jc.Satisfies, errors.IsNotValid)
}

var expectedInputs = []string{"api-caller", "broker"}

func (s *ManifoldSuite) TestInputs(c *gc.C) {
	c.Assert(s.manifold.Inputs, jc.SameContents, expectedInputs)
}

func (s *ManifoldSuite) TestMissingInputs(c *gc.C) {
	for _, input := range expectedInputs {
		context := s.newContext(map[string]interface{}{
			input: dependency.ErrMissing,
		})
		_, err := s.manifold.Start(context)
		c.Assert(errors.Cause(err), gc.Equals, dependency.ErrMissing)
	}
}

func (s *ManifoldSuite) TestStart(c *gc.C) {
	w, err := s.manifold.Start(s.context)
	c.Assert(err, jc.ErrorIsNil)
	workertest.CleanKill(c, w)

	s.CheckCallNames(c, "NewClient", "NewWorker")
	s.CheckCall(c, 0, "NewClient", &s.apiCaller)

	args := s.Calls()[1].Args
	c.Assert(args, gc.HasLen, 1)
	c.Assert(args[0], gc.FitsTypeOf, caasfirewaller.Config{})
	config := args[0].(caasfirewaller.Config)

	c.Assert(config, jc.DeepEquals, caasfirewaller.Config{
		ControllerUUID:    coretesting.ControllerTag.Id(),
		ModelUUID:         coretesting.ModelTag.Id(),
		ApplicationGetter: &s.client,
		ServiceExposer:    &s.broker,
		LifeGetter:        &s.client,
		CharmGetter:       &s.client,
		Logger:            loggo.GetLogger("test"),
	})
}
