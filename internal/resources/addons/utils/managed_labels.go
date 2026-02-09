// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/butlerdotdev/steward/internal/constants"
	"github.com/butlerdotdev/steward/internal/utilities"
)

func SetStewardManagedLabels(obj client.Object) {
	obj.SetLabels(utilities.MergeMaps(obj.GetLabels(), map[string]string{
		constants.ProjectNameLabelKey: constants.ProjectNameLabelValue,
	}))
}
