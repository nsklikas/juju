// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provisioner

import "github.com/juju/juju/environs"

// This is needed to test provisioner.processProfileChanges
//
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/lxdprofileinstancebroker_mock.go github.com/juju/juju/internal/worker/provisioner LXDProfileInstanceBroker
type LXDProfileInstanceBroker interface {
	environs.InstanceBroker
	environs.LXDProfiler
}
