// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storageprovisioner_test

import (
	stdcontext "context"
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"
	"github.com/juju/retry"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/worker/v3/workertest"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/internal/worker/storageprovisioner"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/storage"
	coretesting "github.com/juju/juju/testing"
)

type WorkerSuite struct {
	testing.IsolationSuite

	config              storageprovisioner.Config
	applicationsWatcher *mockApplicationsWatcher
	lifeGetter          *mockLifecycleManager

	applicationChanges chan []string
}

var _ = gc.Suite(&WorkerSuite{})

func (s *WorkerSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)

	s.applicationChanges = make(chan []string)
	s.applicationsWatcher = newMockApplicationsWatcher(s.applicationChanges)
	s.lifeGetter = &mockLifecycleManager{}

	s.config = storageprovisioner.Config{
		Model:                coretesting.ModelTag,
		Scope:                coretesting.ModelTag,
		Applications:         s.applicationsWatcher,
		Volumes:              newMockVolumeAccessor(),
		Filesystems:          newMockFilesystemAccessor(),
		Life:                 s.lifeGetter,
		Status:               &mockStatusSetter{},
		Clock:                &mockClock{},
		Logger:               loggo.GetLogger("test"),
		Registry:             storage.StaticProviderRegistry{},
		CloudCallContextFunc: func(_ stdcontext.Context) context.ProviderCallContext { return context.NewEmptyCloudCallContext() },
	}
}

func (s *WorkerSuite) TestValidateConfig(c *gc.C) {
	s.testValidateConfig(c, func(config *storageprovisioner.Config) {
		config.Scope = names.NewApplicationTag("mariadb")
		config.Applications = nil
	}, `nil Applications not valid`)
}

func (s *WorkerSuite) testValidateConfig(c *gc.C, f func(*storageprovisioner.Config), expect string) {
	config := s.config
	f(&config)
	w, err := storageprovisioner.NewCaasWorker(config)
	if err == nil {
		workertest.DirtyKill(c, w)
	}
	c.Check(err, gc.ErrorMatches, expect)
}

func (s *WorkerSuite) TestStartStop(c *gc.C) {
	w, err := storageprovisioner.NewCaasWorker(s.config)
	c.Assert(err, jc.ErrorIsNil)
	workertest.CheckAlive(c, w)
	workertest.CleanKill(c, w)
}

func (s *WorkerSuite) TestWatchApplicationDead(c *gc.C) {
	w, err := storageprovisioner.NewCaasWorker(s.config)
	c.Assert(err, jc.ErrorIsNil)
	defer workertest.CleanKill(c, w)

	select {
	case s.applicationChanges <- []string{"postgresql"}:
	case <-time.After(coretesting.LongWait):
		c.Fatal("timed out sending applications change")
	}

	// Given the worker time to startup.
	retryCallArgs := retry.CallArgs{
		Clock:       clock.WallClock,
		MaxDuration: coretesting.LongWait,
		Delay:       10 * time.Millisecond,
		Func: func() error {
			if len(s.config.Filesystems.(*mockFilesystemAccessor).Calls()) > 0 {
				return nil
			}
			return errors.NotYetAvailablef("Worker not up")
		},
	}
	err = retry.Call(retryCallArgs)
	c.Assert(err, jc.ErrorIsNil)

	workertest.CleanKill(c, w)
	// Only call is to watch model.
	s.config.Filesystems.(*mockFilesystemAccessor).CheckCallNames(c, "WatchFilesystems")
	s.config.Filesystems.(*mockFilesystemAccessor).CheckCall(c, 0, "WatchFilesystems", coretesting.ModelTag)
}

func (s *WorkerSuite) TestStopsWatchingApplicationBecauseApplicationRemoved(c *gc.C) {
	s.assertStopsWatchingApplication(c, func() {
		s.lifeGetter.err = &params.Error{Code: params.CodeNotFound}
	})
}

func (s *WorkerSuite) assertStopsWatchingApplication(c *gc.C, lifeGetterInjecter func()) {
	w, err := storageprovisioner.NewCaasWorker(s.config)
	c.Assert(err, jc.ErrorIsNil)
	defer workertest.CleanKill(c, w)

	select {
	case s.applicationChanges <- []string{"mariadb"}:
	case <-time.After(coretesting.LongWait):
		c.Fatal("timed out sending applications change")
	}

	// Check that the worker is running or not;
	// given it time to startup.
	startingRetryCallArgs := retry.CallArgs{
		Clock:       clock.WallClock,
		MaxDuration: coretesting.LongWait,
		Delay:       10 * time.Millisecond,
		Func: func() error {
			_, running := storageprovisioner.StorageWorker(w, "mariadb")
			if running {
				return nil
			}
			return errors.NotYetAvailablef("Worker not up")
		},
	}
	err = retry.Call(startingRetryCallArgs)
	c.Assert(err, jc.ErrorIsNil)

	// Add an additional app worker so we can check that the correct one is accessed.
	storageprovisioner.NewStorageWorker(c, w, "postgresql")

	if lifeGetterInjecter != nil {
		lifeGetterInjecter()
	}
	select {
	case s.applicationChanges <- []string{"postgresql"}:
	case <-time.After(coretesting.LongWait):
		c.Fatal("timed out sending applications change")
	}

	// The mariadb worker should still be running.
	_, ok := storageprovisioner.StorageWorker(w, "mariadb")
	c.Assert(ok, jc.IsTrue)

	// Check that the postgresql worker is running or not;
	// given it time to shutdown.
	stoppingRetryCallArgs := retry.CallArgs{
		Clock:       clock.WallClock,
		MaxDuration: coretesting.LongWait,
		Delay:       10 * time.Millisecond,
		Func: func() error {
			_, running := storageprovisioner.StorageWorker(w, "postgresql")
			if !running {
				return nil
			}
			return errors.NotYetAvailablef("Worker not down")
		},
	}
	err = retry.Call(stoppingRetryCallArgs)
	c.Assert(err, jc.ErrorIsNil)

	workertest.CleanKill(c, w)
	workertest.CheckKilled(c, s.applicationsWatcher.watcher)
}

func (s *WorkerSuite) TestStopsWatchingApplicationBecauseApplicationDead(c *gc.C) {
	s.assertStopsWatchingApplication(c, nil)
}

func (s *WorkerSuite) TestWatcherErrorStopsWorker(c *gc.C) {
	w, err := storageprovisioner.NewCaasWorker(s.config)
	c.Assert(err, jc.ErrorIsNil)
	defer workertest.DirtyKill(c, w)

	select {
	case s.applicationChanges <- []string{"mariadb"}:
	case <-time.After(coretesting.LongWait):
		c.Fatal("timed out sending applications change")
	}

	s.applicationsWatcher.watcher.KillErr(errors.New("splat"))
	workertest.CheckKilled(c, s.applicationsWatcher.watcher)
	err = workertest.CheckKilled(c, w)
	c.Assert(err, gc.ErrorMatches, "splat")
}
