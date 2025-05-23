// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc_test

import (
	"github.com/juju/cmd/v3"
	"github.com/juju/cmd/v3/cmdtesting"
	"github.com/juju/errors"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/internal/worker/uniter/runner/jujuc"
	"github.com/juju/juju/testing"
)

type isLeaderSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&isLeaderSuite{})

func (s *isLeaderSuite) TestInitError(c *gc.C) {
	command, err := jujuc.NewIsLeaderCommand(nil)
	c.Assert(err, jc.ErrorIsNil)
	err = command.Init([]string{"blah"})
	c.Assert(err, gc.ErrorMatches, `unrecognized args: \["blah"\]`)
}

func (s *isLeaderSuite) TestInitSuccess(c *gc.C) {
	command, err := jujuc.NewIsLeaderCommand(nil)
	c.Assert(err, jc.ErrorIsNil)
	err = command.Init(nil)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *isLeaderSuite) TestFormatError(c *gc.C) {
	command, err := jujuc.NewIsLeaderCommand(nil)
	c.Assert(err, jc.ErrorIsNil)
	runContext := cmdtesting.Context(c)
	code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(command), runContext, []string{"--format", "bad"})
	c.Check(code, gc.Equals, 2)
	c.Check(bufferString(runContext.Stdout), gc.Equals, "")
	c.Check(bufferString(runContext.Stderr), gc.Equals, `ERROR invalid value "bad" for option --format: unknown format "bad"`+"\n")
}

func (s *isLeaderSuite) TestIsLeaderError(c *gc.C) {
	jujucContext := &isLeaderContext{err: errors.New("pow")}
	command, err := jujuc.NewIsLeaderCommand(jujucContext)
	c.Assert(err, jc.ErrorIsNil)
	runContext := cmdtesting.Context(c)
	code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(command), runContext, nil)
	c.Check(code, gc.Equals, 1)
	c.Check(jujucContext.called, jc.IsTrue)
	c.Check(bufferString(runContext.Stdout), gc.Equals, "")
	c.Check(bufferString(runContext.Stderr), gc.Equals, "ERROR leadership status unknown: pow\n")
}

func (s *isLeaderSuite) TestFormatDefaultYes(c *gc.C) {
	s.testOutput(c, true, nil, "True\n")
}

func (s *isLeaderSuite) TestFormatDefaultNo(c *gc.C) {
	s.testOutput(c, false, nil, "False\n")
}

func (s *isLeaderSuite) TestFormatSmartYes(c *gc.C) {
	s.testOutput(c, true, []string{"--format", "smart"}, "True\n")
}

func (s *isLeaderSuite) TestFormatSmartNo(c *gc.C) {
	s.testOutput(c, false, []string{"--format", "smart"}, "False\n")
}

func (s *isLeaderSuite) TestFormatYamlYes(c *gc.C) {
	s.testParseOutput(c, true, []string{"--format", "yaml"}, jc.YAMLEquals)
}

func (s *isLeaderSuite) TestFormatYamlNo(c *gc.C) {
	s.testParseOutput(c, false, []string{"--format", "yaml"}, jc.YAMLEquals)
}

func (s *isLeaderSuite) TestFormatJsonYes(c *gc.C) {
	s.testParseOutput(c, true, []string{"--format", "json"}, jc.JSONEquals)
}

func (s *isLeaderSuite) TestFormatJsonNo(c *gc.C) {
	s.testParseOutput(c, false, []string{"--format", "json"}, jc.JSONEquals)
}

func (s *isLeaderSuite) testOutput(c *gc.C, leader bool, args []string, expect string) {
	jujucContext := &isLeaderContext{leader: leader}
	command, err := jujuc.NewIsLeaderCommand(jujucContext)
	c.Assert(err, jc.ErrorIsNil)
	runContext := cmdtesting.Context(c)
	code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(command), runContext, args)
	c.Check(code, gc.Equals, 0)
	c.Check(jujucContext.called, jc.IsTrue)
	c.Check(bufferString(runContext.Stdout), gc.Equals, expect)
	c.Check(bufferString(runContext.Stderr), gc.Equals, "")
}

func (s *isLeaderSuite) testParseOutput(c *gc.C, leader bool, args []string, checker gc.Checker) {
	jujucContext := &isLeaderContext{leader: leader}
	command, err := jujuc.NewIsLeaderCommand(jujucContext)
	c.Assert(err, jc.ErrorIsNil)
	runContext := cmdtesting.Context(c)
	code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(command), runContext, args)
	c.Check(code, gc.Equals, 0)
	c.Check(jujucContext.called, jc.IsTrue)
	c.Check(bufferString(runContext.Stdout), checker, leader)
	c.Check(bufferString(runContext.Stderr), gc.Equals, "")
}

type isLeaderContext struct {
	jujuc.Context
	called bool
	leader bool
	err    error
}

func (ctx *isLeaderContext) IsLeader() (bool, error) {
	ctx.called = true
	return ctx.leader, ctx.err
}
