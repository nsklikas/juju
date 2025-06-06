// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package secrets_test

import (
	"testing"

	gc "gopkg.in/check.v1"
)

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/tracker_mock.go github.com/juju/juju/internal/worker/uniter/secrets SecretStateTracker
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/client_mock.go github.com/juju/juju/internal/worker/uniter/secrets SecretsClient

func TestPackage(t *testing.T) {
	gc.TestingT(t)
}
