// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package runcommands_test

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v3/exec"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/internal/worker/uniter/operation"
	"github.com/juju/juju/internal/worker/uniter/remotestate"
	"github.com/juju/juju/internal/worker/uniter/resolver"
	"github.com/juju/juju/internal/worker/uniter/runcommands"
	"github.com/juju/juju/internal/worker/uniter/runner"
	runnercontext "github.com/juju/juju/internal/worker/uniter/runner/context"
)

type runcommandsSuite struct {
	charmURL         string
	remoteState      remotestate.Snapshot
	mockRunner       mockRunner
	callbacks        *mockCallbacks
	opFactory        operation.Factory
	resolver         resolver.Resolver
	commands         runcommands.Commands
	runCommands      func(string, runner.RunLocation) (*exec.ExecResponse, error)
	commandCompleted func(string)
}

var _ = gc.Suite(&runcommandsSuite{})

func (s *runcommandsSuite) SetUpTest(c *gc.C) {
	s.charmURL = "ch:precise/mysql-2"
	s.remoteState = remotestate.Snapshot{
		CharmURL: s.charmURL,
	}
	s.mockRunner = mockRunner{runCommands: func(commands string, runLocation runner.RunLocation) (*exec.ExecResponse, error) {
		return s.runCommands(commands, runLocation)
	}}
	s.callbacks = &mockCallbacks{}
	s.opFactory = operation.NewFactory(operation.FactoryParams{
		Callbacks: s.callbacks,
		RunnerFactory: &mockRunnerFactory{
			newCommandRunner: func(info runnercontext.CommandInfo) (runner.Runner, error) {
				return &s.mockRunner, nil
			},
		},
		Logger: loggo.GetLogger("test"),
	})

	s.commands = runcommands.NewCommands()
	s.commandCompleted = nil
	s.resolver = runcommands.NewCommandsResolver(
		s.commands, func(id string) {
			if s.commandCompleted != nil {
				s.commandCompleted(id)
			}
		},
	)
}

func (s *runcommandsSuite) TestRunCommands(c *gc.C) {
	localState := resolver.LocalState{
		CharmURL: s.charmURL,
		State: operation.State{
			Kind: operation.Continue,
		},
	}
	id := s.commands.AddCommand(operation.CommandArgs{
		Commands:    "echo foxtrot",
		RunLocation: runner.Operator,
	}, func(*exec.ExecResponse, error) bool { return false })
	s.remoteState.Commands = []string{id}
	op, err := s.resolver.NextOp(localState, s.remoteState, s.opFactory)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(op.String(), gc.Equals, "run commands (0)")
}

func (s *runcommandsSuite) TestRunCommandsCallbacks(c *gc.C) {
	var completed []string
	s.commandCompleted = func(id string) {
		completed = append(completed, id)
	}

	var run []string
	s.runCommands = func(commands string, runLocation runner.RunLocation) (*exec.ExecResponse, error) {
		c.Assert(runLocation, gc.Equals, runner.Operator)
		run = append(run, commands)
		return &exec.ExecResponse{}, nil
	}
	localState := resolver.LocalState{
		CharmURL: s.charmURL,
		State: operation.State{
			Kind: operation.Continue,
		},
	}

	id := s.commands.AddCommand(operation.CommandArgs{
		Commands:    "echo foxtrot",
		RunLocation: runner.Operator,
	}, func(*exec.ExecResponse, error) bool { return false })
	s.remoteState.Commands = []string{id}

	op, err := s.resolver.NextOp(localState, s.remoteState, s.opFactory)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(op.String(), gc.Equals, "run commands (0)")

	_, err = op.Prepare(operation.State{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(run, gc.HasLen, 0)
	c.Assert(completed, gc.HasLen, 0)

	_, err = op.Execute(operation.State{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(run, jc.DeepEquals, []string{"echo foxtrot"})
	c.Assert(completed, gc.HasLen, 0)

	_, err = op.Commit(operation.State{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(completed, jc.DeepEquals, []string{id})
}

func (s *runcommandsSuite) TestRunCommandsCommitErrorNoCompletedCallback(c *gc.C) {
	// Override opFactory with one that creates run command
	// operations with failing Commit methods.
	s.opFactory = commitErrorOpFactory{s.opFactory}

	var completed []string
	s.commandCompleted = func(id string) {
		completed = append(completed, id)
	}

	var run []string
	s.runCommands = func(commands string, runLocation runner.RunLocation) (*exec.ExecResponse, error) {
		c.Assert(runLocation, gc.Equals, runner.Operator)
		run = append(run, commands)
		return &exec.ExecResponse{}, nil
	}
	localState := resolver.LocalState{
		CharmURL: s.charmURL,
		State: operation.State{
			Kind: operation.Continue,
		},
	}

	id := s.commands.AddCommand(operation.CommandArgs{
		Commands:    "echo foxtrot",
		RunLocation: runner.Operator,
	}, func(*exec.ExecResponse, error) bool { return false })
	s.remoteState.Commands = []string{id}

	op, err := s.resolver.NextOp(localState, s.remoteState, s.opFactory)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(op.String(), gc.Equals, "run commands (0)")

	_, err = op.Prepare(operation.State{})
	c.Assert(err, jc.ErrorIsNil)

	_, err = op.Execute(operation.State{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(run, jc.DeepEquals, []string{"echo foxtrot"})
	c.Assert(completed, gc.HasLen, 0)

	_, err = op.Commit(operation.State{})
	c.Assert(err, gc.ErrorMatches, "Commit failed")
	// commandCompleted is not called if Commit fails
	c.Assert(completed, gc.HasLen, 0)
}

func (s *runcommandsSuite) TestRunCommandsError(c *gc.C) {
	localState := resolver.LocalState{
		CharmURL: s.charmURL,
		State: operation.State{
			Kind: operation.Continue,
		},
	}
	s.runCommands = func(commands string, runLocation runner.RunLocation) (*exec.ExecResponse, error) {
		c.Assert(runLocation, gc.Equals, runner.Operator)
		return nil, errors.Errorf("executing commands: %s", commands)
	}

	var execErr error
	id := s.commands.AddCommand(operation.CommandArgs{
		Commands:    "echo foxtrot",
		RunLocation: runner.Operator,
	}, func(_ *exec.ExecResponse, err error) bool {
		execErr = err
		return false
	})
	s.remoteState.Commands = []string{id}

	op, err := s.resolver.NextOp(localState, s.remoteState, s.opFactory)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(op.String(), gc.Equals, "run commands (0)")

	_, err = op.Prepare(operation.State{})
	c.Assert(err, jc.ErrorIsNil)

	_, err = op.Execute(operation.State{})
	c.Assert(err, gc.NotNil)
	c.Assert(execErr, gc.ErrorMatches, "executing commands: echo foxtrot")
}

func (s *runcommandsSuite) TestRunCommandsErrorConsumed(c *gc.C) {
	localState := resolver.LocalState{
		CharmURL: s.charmURL,
		State: operation.State{
			Kind: operation.Continue,
		},
	}
	s.runCommands = func(commands string, runLocation runner.RunLocation) (*exec.ExecResponse, error) {
		c.Assert(runLocation, gc.Equals, runner.Operator)
		return nil, errors.Errorf("executing commands: %s", commands)
	}

	var execErr error
	id := s.commands.AddCommand(operation.CommandArgs{
		Commands:    "echo foxtrot",
		RunLocation: runner.Operator,
	}, func(_ *exec.ExecResponse, err error) bool {
		execErr = err
		return true
	})
	s.remoteState.Commands = []string{id}

	op, err := s.resolver.NextOp(localState, s.remoteState, s.opFactory)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(op.String(), gc.Equals, "run commands (0)")

	_, err = op.Prepare(operation.State{})
	c.Assert(err, jc.ErrorIsNil)

	_, err = op.Execute(operation.State{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(execErr, gc.ErrorMatches, "executing commands: echo foxtrot")
}

func (s *runcommandsSuite) TestRunCommandsStatus(c *gc.C) {
	localState := resolver.LocalState{
		CharmURL: s.charmURL,
		State: operation.State{
			Kind: operation.Continue,
		},
	}

	id := s.commands.AddCommand(operation.CommandArgs{
		Commands:    "echo foxtrot",
		RunLocation: runner.Operator,
	}, func(*exec.ExecResponse, error) bool { return false })
	s.remoteState.Commands = []string{id}

	op, err := s.resolver.NextOp(localState, s.remoteState, s.opFactory)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(op.String(), gc.Equals, "run commands (0)")
	s.callbacks.CheckCalls(c, nil /* no calls */)

	_, err = op.Prepare(operation.State{})
	c.Assert(err, jc.ErrorIsNil)
	s.callbacks.CheckCalls(c, nil /* no calls */)

	s.callbacks.SetErrors(errors.New("cannot set status"))
	_, err = op.Execute(operation.State{})
	c.Assert(err, gc.ErrorMatches, "cannot set status")
	s.callbacks.CheckCallNames(c, "SetExecutingStatus")
	s.callbacks.CheckCall(c, 0, "SetExecutingStatus", "running commands")
}

type commitErrorOpFactory struct {
	operation.Factory
}

func (f commitErrorOpFactory) NewCommands(args operation.CommandArgs, sendResponse operation.CommandResponseFunc) (operation.Operation, error) {
	op, err := f.Factory.NewCommands(args, sendResponse)
	if err == nil {
		op = commitErrorOperation{op}
	}
	return op, err
}

type commitErrorOperation struct {
	operation.Operation
}

func (commitErrorOperation) Commit(operation.State) (*operation.State, error) {
	return nil, errors.New("Commit failed")
}
