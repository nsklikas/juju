// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package pruner_test

import (
	"sync"
	"time"

	"github.com/juju/clock/testclock"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/worker/v3"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/internal/worker/pruner"
	"github.com/juju/juju/internal/worker/statushistorypruner"
	coretesting "github.com/juju/juju/testing"
)

type PrunerSuite struct {
	coretesting.BaseSuite
}

var _ = gc.Suite(&PrunerSuite{})

type newPrunerFunc func(pruner.Config) (worker.Worker, error)

func (s *PrunerSuite) setupPruner(c *gc.C, newPruner newPrunerFunc) (*fakeFacade, *testclock.Clock) {
	facade := newFakeFacade()
	attrs := coretesting.FakeConfig()
	attrs["max-status-history-age"] = "1s"
	attrs["max-status-history-size"] = "3M"
	cfg, err := config.New(config.UseDefaults, attrs)
	c.Assert(err, jc.ErrorIsNil)
	facade.modelConfig = cfg

	testClock := testclock.NewClock(time.Time{})
	conf := pruner.Config{
		Facade:        facade,
		PruneInterval: coretesting.ShortWait,
		Clock:         testClock,
		Logger:        loggo.GetLogger("test"),
	}

	pruner, err := newPruner(conf)
	c.Check(err, jc.ErrorIsNil)
	s.AddCleanup(func(*gc.C) {
		c.Assert(worker.Stop(pruner), jc.ErrorIsNil)
	})

	facade.modelChangesWatcher.changes <- struct{}{}

	return facade, testClock
}

func (s *PrunerSuite) assertWorkerCallsPrune(c *gc.C, facade *fakeFacade, testClock *testclock.Clock, collectionSize int) {
	// NewTimer/Reset will have been called with the PruneInterval.
	testClock.WaitAdvance(coretesting.ShortWait-time.Nanosecond, coretesting.LongWait, 1)
	select {
	case <-facade.pruned:
		c.Fatal("unexpected call to Prune")
	case <-time.After(coretesting.ShortWait):
	}
	testClock.Advance(time.Nanosecond)
	select {
	case args := <-facade.pruned:
		c.Assert(args.maxHistoryMB, gc.Equals, collectionSize)
	case <-time.After(coretesting.LongWait):
		c.Fatal("timed out waiting for call to Prune")
	}
}

func (s *PrunerSuite) TestWorkerCallsPrune(c *gc.C) {
	facade, clock := s.setupPruner(c, statushistorypruner.New)
	s.assertWorkerCallsPrune(c, facade, clock, 3)
}

func (s *PrunerSuite) TestWorkerWontCallPruneBeforeFiringTimer(c *gc.C) {
	facade, _ := s.setupPruner(c, statushistorypruner.New)

	select {
	case <-facade.pruned:
		c.Fatal("called before firing timer.")
	case <-time.After(coretesting.ShortWait):
	}
}

func (s *PrunerSuite) TestModelConfigChange(c *gc.C) {
	facade, clock := s.setupPruner(c, statushistorypruner.New)
	s.assertWorkerCallsPrune(c, facade, clock, 3)

	var err error
	facade.modelConfig, err = facade.modelConfig.Apply(map[string]interface{}{"max-status-history-size": "4M"})
	c.Assert(err, jc.ErrorIsNil)
	facade.modelChangesWatcher.changes <- struct{}{}

	s.assertWorkerCallsPrune(c, facade, clock, 4)
}

type fakeFacade struct {
	pruned              chan pruneParams
	modelChangesWatcher *mockNotifyWatcher
	modelConfig         *config.Config
}

type pruneParams struct {
	maxAge       time.Duration
	maxHistoryMB int
}

func newFakeFacade() *fakeFacade {
	return &fakeFacade{
		pruned:              make(chan pruneParams, 1),
		modelChangesWatcher: newMockNotifyWatcher(),
	}
}

// Prune implements Facade
func (f *fakeFacade) Prune(maxAge time.Duration, maxHistoryMB int) error {
	select {
	case f.pruned <- pruneParams{maxAge, maxHistoryMB}:
	case <-time.After(coretesting.LongWait):
		return errors.New("timed out waiting for facade call Prune to run")
	}
	return nil
}

// WatchForModelConfigChanges implements Facade
func (f *fakeFacade) WatchForModelConfigChanges() (watcher.NotifyWatcher, error) {
	return f.modelChangesWatcher, nil
}

// ModelConfig implements Facade
func (f *fakeFacade) ModelConfig() (*config.Config, error) {
	return f.modelConfig, nil
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{
		stopped: make(chan struct{}),
	}
}

type mockWatcher struct {
	mu      sync.Mutex
	stopped chan struct{}
}

func (w *mockWatcher) Kill() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.Stopped() {
		close(w.stopped)
	}
}

func (w *mockWatcher) Wait() error {
	<-w.stopped
	return nil
}

func (w *mockWatcher) Stopped() bool {
	select {
	case <-w.stopped:
		return true
	default:
		return false
	}
}

func newMockNotifyWatcher() *mockNotifyWatcher {
	return &mockNotifyWatcher{
		mockWatcher: newMockWatcher(),
		changes:     make(chan struct{}, 1),
	}
}

type mockNotifyWatcher struct {
	*mockWatcher
	changes chan struct{}
}

func (w *mockNotifyWatcher) Changes() watcher.NotifyChannel {
	return w.changes
}
