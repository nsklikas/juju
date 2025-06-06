// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"github.com/juju/errors"
	"github.com/juju/worker/v3/dependency"

	"github.com/juju/juju/internal/worker/lifeflag"
)

// ErrRemoved may be returned by some worker started from Manifolds to
// indicate that the model under management no longer exists.
const ErrRemoved = errors.ConstError("model removed")

// LifeFilter is used with the lifeflag manifolds -- which do not depend
// on runFlag -- to return appropriate errors for consumption by the
// enclosing dependency.Engine (and/or its IsFatal check).
func LifeFilter(err error) error {
	switch {
	case errors.Is(err, lifeflag.ErrNotFound):
		return ErrRemoved
	case errors.Is(err, lifeflag.ErrValueChanged):
		return dependency.ErrBounce
	}
	return err
}

// IsFatal will probably be helpful when configuring a dependency.Engine
// to run the result of Manifolds.
func IsFatal(err error) bool {
	return errors.Is(err, ErrRemoved)
}

// WorstError will probably be helpful when configuring a dependency.Engine
// to run the result of Manifolds.
func WorstError(err, _ error) error {
	// Doesn't matter if there's only one fatal error.
	return err
}

// IgnoreErrRemoved returns nil if passed an error caused by ErrRemoved,
// and otherwise returns the original error.
func IgnoreErrRemoved(err error) error {
	if errors.Is(err, ErrRemoved) {
		return nil
	}
	return err
}
