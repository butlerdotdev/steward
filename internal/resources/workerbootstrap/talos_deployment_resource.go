// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	builder "github.com/butlerdotdev/steward/internal/builders/controlplane"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

// TalosDeploymentResource patches the TCP Deployment to add/remove the steward-trustd sidecar.
// Follows the konnectivity KubernetesDeploymentResource pattern.
type TalosDeploymentResource struct {
	resource *appsv1.Deployment
	Builder  builder.Trustd
	Client   client.Client
}

func (r *TalosDeploymentResource) GetHistogram() prometheus.Histogram {
	deploymentCollector = resources.LazyLoadHistogramFromResource(deploymentCollector, r)

	return deploymentCollector
}

func (r *TalosDeploymentResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	switch {
	case !shouldHaveWorkerBootstrap(tcp) && tcp.Status.Addons.WorkerBootstrap.Enabled:
		return true
	case shouldHaveWorkerBootstrap(tcp) && !tcp.Status.Addons.WorkerBootstrap.Enabled:
		return true
	default:
		return false
	}
}

func (r *TalosDeploymentResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return !shouldHaveWorkerBootstrap(tcp) && tcp.Status.Addons.WorkerBootstrap.Enabled
}

func (r *TalosDeploymentResource) CleanUp(ctx context.Context, _ *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	res, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, func() error {
		logger.V(1).Info("removing steward-trustd container")
		r.Builder.RemoveContainer(&r.resource.Spec.Template.Spec)

		logger.V(1).Info("removing steward-trustd volumes")
		r.Builder.RemoveVolumes(&r.resource.Spec.Template.Spec)

		return nil
	})

	return res == controllerutil.OperationResultUpdated, err
}

func (r *TalosDeploymentResource) Define(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcp.GetName(),
			Namespace: tcp.GetNamespace(),
		},
	}

	return nil
}

func (r *TalosDeploymentResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if !shouldHaveWorkerBootstrap(tcp) {
		return controllerutil.OperationResultNone, nil
	}

	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tcp))
}

func (r *TalosDeploymentResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if !shouldHaveWorkerBootstrap(tcp) {
			return nil
		}

		if len(r.resource.Spec.Template.Spec.Containers) == 0 {
			return fmt.Errorf("the Deployment resource is not ready for steward-trustd sidecar")
		}

		r.Builder.Build(r.resource, *tcp)

		return nil
	}
}

func (r *TalosDeploymentResource) GetName() string {
	return "talos-trustd-deployment"
}

func (r *TalosDeploymentResource) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.WorkerBootstrap.Enabled = shouldHaveWorkerBootstrap(tcp)

	return nil
}
