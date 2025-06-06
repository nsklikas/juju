// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package instancemutater_test

import (
	"testing"

	gc "gopkg.in/check.v1"
)

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/instancebroker_mock.go github.com/juju/juju/internal/worker/instancemutater InstanceMutaterAPI
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/logger_mock.go github.com/juju/juju/internal/worker/instancemutater Logger
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/namestag_mock.go github.com/juju/names/v5 Tag
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/machinemutater_mock.go github.com/juju/juju/api/agent/instancemutater MutaterMachine

func TestPackage(t *testing.T) {
	gc.TestingT(t)
}
