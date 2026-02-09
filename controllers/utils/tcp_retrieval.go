// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

type TenantControlPlaneRetrievalFn func() (*stewardv1alpha1.TenantControlPlane, error)
