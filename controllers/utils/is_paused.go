// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/butlerdotdev/steward/api/v1alpha1"
)

func IsPaused(obj client.Object) bool {
	if obj.GetAnnotations() == nil {
		return false
	}
	_, paused := obj.GetAnnotations()[v1alpha1.PausedReconciliationAnnotation]

	return paused
}
