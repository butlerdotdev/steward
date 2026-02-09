// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	builder "github.com/butlerdotdev/steward/internal/builders/controlplane"
	"github.com/butlerdotdev/steward/internal/utilities"
)

type KubernetesDeploymentResource struct {
	resource           *appsv1.Deployment
	Client             client.Client
	DataStore          stewardv1alpha1.DataStore
	DataStoreOverrides []builder.DataStoreOverrides
	KineContainerImage string
}

func (r *KubernetesDeploymentResource) GetHistogram() prometheus.Histogram {
	deploymentCollector = LazyLoadHistogramFromResource(deploymentCollector, r)

	return deploymentCollector
}

func (r *KubernetesDeploymentResource) isStatusEqual(tenantControlPlane *stewardv1alpha1.TenantControlPlane) bool {
	return r.resource.Status.String() == tenantControlPlane.Status.Kubernetes.Deployment.DeploymentStatus.String()
}

func (r *KubernetesDeploymentResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane) ||
		tenantControlPlane.Spec.Kubernetes.Version != tenantControlPlane.Status.Kubernetes.Version.Version ||
		*r.computeStatus(tenantControlPlane) != ptr.Deref(tenantControlPlane.Status.Kubernetes.Version.Status, stewardv1alpha1.VersionUnknown)
}

func (r *KubernetesDeploymentResource) ShouldCleanup(*stewardv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubernetesDeploymentResource) CleanUp(context.Context, *stewardv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubernetesDeploymentResource) Define(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesDeploymentResource) mutate(ctx context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		(builder.Deployment{
			Client:             r.Client,
			DataStore:          r.DataStore,
			KineContainerImage: r.KineContainerImage,
			DataStoreOverrides: r.DataStoreOverrides,
		}).Build(ctx, r.resource, *tenantControlPlane)

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesDeploymentResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesDeploymentResource) GetName() string {
	return "deployment"
}

func (r *KubernetesDeploymentResource) computeStatus(tenantControlPlane *stewardv1alpha1.TenantControlPlane) *stewardv1alpha1.KubernetesVersionStatus {
	switch {
	case ptr.Deref(tenantControlPlane.Spec.ControlPlane.Deployment.Replicas, 2) == 0:
		return &stewardv1alpha1.VersionSleeping
	case r.isNotReady():
		return &stewardv1alpha1.VersionNotReady
	case tenantControlPlane.Spec.WritePermissions.HasAnyLimitation():
		return &stewardv1alpha1.VersionWriteLimited
	case !r.isProgressingUpgrade():
		return &stewardv1alpha1.VersionReady
	case r.isUpgrading(tenantControlPlane):
		return &stewardv1alpha1.VersionUpgrading
	case r.isProvisioning(tenantControlPlane):
		return &stewardv1alpha1.VersionProvisioning
	default:
		return &stewardv1alpha1.VersionUnknown
	}
}

func (r *KubernetesDeploymentResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Kubernetes.Version.Status = r.computeStatus(tenantControlPlane)
	if *tenantControlPlane.Status.Kubernetes.Version.Status == stewardv1alpha1.VersionReady ||
		*tenantControlPlane.Status.Kubernetes.Version.Status == stewardv1alpha1.VersionSleeping {
		tenantControlPlane.Status.Kubernetes.Version.Version = tenantControlPlane.Spec.Kubernetes.Version
	}

	tenantControlPlane.Status.Kubernetes.Deployment = stewardv1alpha1.KubernetesDeploymentStatus{
		DeploymentStatus: r.resource.Status,
		Selector:         metav1.FormatLabelSelector(r.resource.Spec.Selector),
		Name:             r.resource.GetName(),
		Namespace:        r.resource.GetNamespace(),
		LastUpdate:       metav1.Now(),
	}

	return nil
}

func (r *KubernetesDeploymentResource) isProgressingUpgrade() bool {
	if r.resource.ObjectMeta.GetGeneration() != r.resource.Status.ObservedGeneration {
		return true
	}

	if r.resource.Status.UnavailableReplicas > 0 {
		return true
	}

	// An update is complete when new pods are ready and old pods deleted.
	desired := ptr.Deref(r.resource.Spec.Replicas, 2)
	if r.resource.Status.UpdatedReplicas != desired {
		return true
	}

	if r.resource.Status.ReadyReplicas != desired {
		return true
	}

	if r.resource.Status.Replicas != desired {
		return true
	}

	return false
}

func (r *KubernetesDeploymentResource) isUpgrading(tenantControlPlane *stewardv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Kubernetes.Version.Version) > 0 &&
		tenantControlPlane.Spec.Kubernetes.Version != tenantControlPlane.Status.Kubernetes.Version.Version &&
		r.isProgressingUpgrade()
}

func (r *KubernetesDeploymentResource) isProvisioning(tenantControlPlane *stewardv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Kubernetes.Version.Version) == 0
}

func (r *KubernetesDeploymentResource) isNotReady() bool {
	return r.resource.Status.ReadyReplicas == 0
}
