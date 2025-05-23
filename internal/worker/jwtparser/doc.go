// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// Package jwtparser provides a singleton JWTParser
// that can be used as a dependency to any workers
// that need to parse JWTs (JSON Web Token).
//
// This worker uses state directly rather than
// making calls to the API server because it is
// used by the API server itself.
package jwtparser
