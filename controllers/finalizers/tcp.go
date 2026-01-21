// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package finalizers

const (
	// DatastoreFinalizer is using a wrong name, since it's related to the underlying datastore.
	DatastoreFinalizer       = "finalizer.steward.butlerlabs.dev"
	DatastoreSecretFinalizer = "finalizer.steward.butlerlabs.dev/datastore-secret"
	SootFinalizer            = "finalizer.steward.butlerlabs.dev/soot"
)
