// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package manual

import (
	"github.com/juju/errors"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/context"
)

var _ environs.NetworkingEnviron = &manualEnviron{}

// SupportsSpaces implements environs.NetworkingEnviron.
func (e *manualEnviron) SupportsSpaces(context.ProviderCallContext) (bool, error) {
	return true, nil
}

// Subnets implements environs.NetworkingEnviron.
func (e *manualEnviron) Subnets(context.ProviderCallContext, instance.Id, []network.Id) ([]network.SubnetInfo, error) {
	return nil, errors.NotSupportedf("subnets")
}

// SuperSubnets implements environs.NetworkingEnviron.
func (e *manualEnviron) SuperSubnets(context.ProviderCallContext) ([]string, error) {
	return nil, errors.NotSupportedf("super subnets")
}

// AreSpacesRoutable implements environs.NetworkingEnviron.
func (*manualEnviron) AreSpacesRoutable(_ context.ProviderCallContext, _, _ *environs.ProviderSpaceInfo) (bool, error) {
	return false, nil
}

// NetworkInterfaces implements environs.NetworkingEnviron.
func (e *manualEnviron) NetworkInterfaces(
	context.ProviderCallContext, []instance.Id,
) ([]network.InterfaceInfos, error) {
	return nil, errors.NotSupportedf("network interfaces")
}
