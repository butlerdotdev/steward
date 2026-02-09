// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package tcpproxy

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/constants"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

// ClusterRoleBindingResource manages the tcp-proxy ClusterRoleBinding inside the tenant cluster.
type ClusterRoleBindingResource struct {
	Client client.Client

	resource     *rbacv1.ClusterRoleBinding
	tenantClient client.Client
}

func (r *ClusterRoleBindingResource) GetHistogram() prometheus.Histogram {
	clusterRoleBindingCollector = resources.LazyLoadHistogramFromResource(clusterRoleBindingCollector, r)

	return clusterRoleBindingCollector
}

func (r *ClusterRoleBindingResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.ClusterRoleBinding.Name != "":
		return true
	case tcp.Spec.Addons.TCPProxy != nil && tcp.Status.Addons.TCPProxy.ClusterRoleBinding.Name != r.resource.GetName():
		return true
	default:
		return false
	}
}

func (r *ClusterRoleBindingResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.Enabled
}

func (r *ClusterRoleBindingResource) CleanUp(ctx context.Context, _ *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.tenantClient.Get(ctx, client.ObjectKeyFromObject(r.resource), r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot retrieve the requested resource for deletion")

		return false, err
	}

	if labels := r.resource.GetLabels(); labels == nil || labels[constants.ProjectNameLabelKey] != constants.ProjectNameLabelValue {
		return false, nil
	}

	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot delete the requested resource")

		return false, err
	}

	return true, nil
}

func (r *ClusterRoleBindingResource) Define(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &rbacv1.ClusterRoleBinding{}
	r.resource.SetName(ClusterRoleBindingName)

	var err error
	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tcp); err != nil {
		logger.Error(err, "unable to retrieve the Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *ClusterRoleBindingResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.TCPProxy == nil {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(tcp))
}

func (r *ClusterRoleBindingResource) GetName() string {
	return "tcp-proxy-cluster-role-binding"
}

func (r *ClusterRoleBindingResource) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.TCPProxy.ClusterRoleBinding = stewardv1alpha1.ExternalKubernetesObjectStatus{}

	if tcp.Spec.Addons.TCPProxy != nil {
		tcp.Status.Addons.TCPProxy.ClusterRoleBinding = stewardv1alpha1.ExternalKubernetesObjectStatus{
			Name:       r.resource.GetName(),
			LastUpdate: metav1.Now(),
		}
	}

	return nil
}

func (r *ClusterRoleBindingResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
		))

		r.resource.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     ClusterRoleName,
		}

		r.resource.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ServiceAccountName,
				Namespace: Namespace,
			},
		}

		return nil
	}
}
