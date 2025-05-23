// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package secretbackendrotate_test

import (
	stdtesting "testing"

	gc "gopkg.in/check.v1"
)

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/client_mock.go github.com/juju/juju/internal/worker/secretbackendrotate SecretBackendManagerFacade
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/watcher_mock.go github.com/juju/juju/core/watcher SecretBackendRotateWatcher

func TestPackage(t *stdtesting.T) {
	gc.TestingT(t)
}
