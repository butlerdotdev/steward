// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

type KubeconfigGeneratorWatcher struct {
	Client        client.Client
	GeneratorChan chan event.GenericEvent
}

func (r *KubeconfigGeneratorWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling resource")

	var tcp stewardv1alpha1.TenantControlPlane
	if err := r.Client.Get(ctx, req.NamespacedName, &tcp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("resource may have been deleted, skipping")

			return ctrl.Result{}, nil
		}

		logger.Error(err, "cannot retrieve the required resource")

		return ctrl.Result{}, err
	}

	var generators stewardv1alpha1.KubeconfigGeneratorList
	if err := r.Client.List(ctx, &generators); err != nil {
		logger.Error(err, "cannot list generators")

		return ctrl.Result{}, err
	}

	for _, generator := range generators.Items {
		sel, err := metav1.LabelSelectorAsSelector(&generator.Spec.TenantControlPlaneSelector)
		if err != nil {
			logger.Error(err, "cannot validate Selector", "generator", generator.Name)

			return ctrl.Result{}, err
		}

		if sel.Matches(labels.Set(tcp.Labels)) {
			logger.Info("pushing Generator", "generator", generator.Name)

			r.GeneratorChan <- event.GenericEvent{
				Object: &generator,
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *KubeconfigGeneratorWatcher) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stewardv1alpha1.TenantControlPlane{}).
		Complete(r)
}
