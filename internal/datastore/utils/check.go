// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

// CheckExists ensures that the default Datastore exists before starting the manager.
func CheckExists(ctx context.Context, scheme *runtime.Scheme, datastoreName string) error {
	if datastoreName == "" {
		return nil
	}

	ctrlClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("unable to create controlerruntime.Client: %w", err)
	}

	if err = ctrlClient.Get(ctx, types.NamespacedName{Name: datastoreName}, &stewardv1alpha1.DataStore{}); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("the default Datastore %s doesn't exist", datastoreName)
		}

		return fmt.Errorf("an error occurred during datastore retrieval: %w", err)
	}

	return nil
}
