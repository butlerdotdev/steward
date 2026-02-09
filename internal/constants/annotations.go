// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// Checksum is the annotation label that we use to store the checksum for the resource:
	// it allows to check by comparing it if the resource has been changed and must be aligned with the reconciliation.
	Checksum = "steward.butlerlabs.dev/checksum"
)
