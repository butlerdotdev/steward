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

// ClusterRoleResource manages the tcp-proxy ClusterRole inside the tenant cluster.
type ClusterRoleResource struct {
	Client client.Client

	resource     *rbacv1.ClusterRole
	tenantClient client.Client
}

func (r *ClusterRoleResource) GetHistogram() prometheus.Histogram {
	clusterRoleCollector = resources.LazyLoadHistogramFromResource(clusterRoleCollector, r)
	return clusterRoleCollector
}

func (r *ClusterRoleResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.ClusterRole.Name != "":
		return true
	case tcp.Spec.Addons.TCPProxy != nil && tcp.Status.Addons.TCPProxy.ClusterRole.Name != r.resource.GetName():
		return true
	default:
		return false
	}
}

func (r *ClusterRoleResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.Enabled
}

func (r *ClusterRoleResource) CleanUp(ctx context.Context, _ *stewardv1alpha1.TenantControlPlane) (bool, error) {
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

func (r *ClusterRoleResource) Define(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &rbacv1.ClusterRole{}
	r.resource.SetName(ClusterRoleName)

	var err error
	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tcp); err != nil {
		logger.Error(err, "unable to retrieve the Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *ClusterRoleResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.TCPProxy == nil {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(tcp))
}

func (r *ClusterRoleResource) GetName() string {
	return "tcp-proxy-cluster-role"
}

func (r *ClusterRoleResource) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.TCPProxy.ClusterRole = stewardv1alpha1.ExternalKubernetesObjectStatus{}

	if tcp.Spec.Addons.TCPProxy != nil {
		tcp.Status.Addons.TCPProxy.ClusterRole = stewardv1alpha1.ExternalKubernetesObjectStatus{
			Name:       r.resource.GetName(),
			LastUpdate: metav1.Now(),
		}
	}

	return nil
}

func (r *ClusterRoleResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
		))

		r.resource.Rules = []rbacv1.PolicyRule{
			{
				// EndpointSlices: tcp-proxy manages the "kubernetes" EndpointSlice
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
			{
				// Services: tcp-proxy reads the "kubernetes" Service to discover ClusterIP
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				// Leases: used for leader election
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update"},
			},
			{
				// Events: for recording reconciliation events
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
		}

		return nil
	}
}
