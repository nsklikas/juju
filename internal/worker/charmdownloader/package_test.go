// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmdownloader

import (
	stdtesting "testing"

	coretesting "github.com/juju/juju/testing"
)

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mocks.go github.com/juju/juju/internal/worker/charmdownloader CharmDownloaderAPI,Logger
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_watcher.go github.com/juju/juju/core/watcher StringsWatcher

func TestAll(t *stdtesting.T) {
	coretesting.MgoTestPackage(t)
}
