// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package engine_test

import (
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	worker "github.com/juju/worker/v3"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/cmd/jujud/agent/engine"
)

type ValueWorkerSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&ValueWorkerSuite{})

func (s *ValueWorkerSuite) TestNewValueWorker_Success(c *gc.C) {
	w, err := engine.NewValueWorker("cheese")
	c.Assert(err, jc.ErrorIsNil)

	err = worker.Stop(w)
	c.Check(err, jc.ErrorIsNil)
}

func (s *ValueWorkerSuite) TestNewValueWorker_NilValue(c *gc.C) {
	w, err := engine.NewValueWorker(nil)
	c.Check(err, gc.ErrorMatches, "NewValueWorker expects a value")
	c.Check(w, gc.IsNil)
}

func (s *ValueWorkerSuite) TestValueWorkerOutput_Success(c *gc.C) {
	value := &testType{}
	w, err := engine.NewValueWorker(value)
	c.Assert(err, jc.ErrorIsNil)

	var outVal testInterface
	err = engine.ValueWorkerOutput(w, &outVal)
	c.Check(err, jc.ErrorIsNil)
	c.Check(outVal, gc.DeepEquals, value)
}

func (s *ValueWorkerSuite) TestValueWorkerOutput_BadInput(c *gc.C) {
	var outVal testInterface
	err := engine.ValueWorkerOutput(&testType{}, &outVal)
	c.Check(err, gc.ErrorMatches, "in should be a \\*valueWorker; is .*")
	c.Check(outVal, gc.IsNil)
}

func (s *ValueWorkerSuite) TestValueWorkerOutput_BadOutputIndirection(c *gc.C) {
	value := &testType{}
	w, err := engine.NewValueWorker(value)
	c.Assert(err, jc.ErrorIsNil)

	var outVal string
	err = engine.ValueWorkerOutput(w, outVal)
	c.Check(err, gc.ErrorMatches, "out should be a pointer; is .*")
	c.Check(outVal, gc.Equals, "")
}

func (s *ValueWorkerSuite) TestValueWorkerOutput_BadOutputType(c *gc.C) {
	value := &testType{}
	w, err := engine.NewValueWorker(value)
	c.Assert(err, jc.ErrorIsNil)

	var outVal string
	err = engine.ValueWorkerOutput(w, &outVal)
	c.Check(err, gc.ErrorMatches, "cannot output into \\*string")
	c.Check(outVal, gc.Equals, "")
}

type testInterface interface {
	worker.Worker
	Foobar()
}

type testType struct {
	testInterface
}
