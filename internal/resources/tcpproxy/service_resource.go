// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package tcpproxy

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/constants"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

// ServiceResource manages the tcp-proxy ClusterIP Service inside the tenant cluster.
type ServiceResource struct {
	Client client.Client

	resource     *corev1.Service
	tenantClient client.Client
}

func (r *ServiceResource) GetHistogram() prometheus.Histogram {
	serviceCollector = resources.LazyLoadHistogramFromResource(serviceCollector, r)

	return serviceCollector
}

func (r *ServiceResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.Service.Name != "":
		return true
	case tcp.Spec.Addons.TCPProxy != nil && tcp.Status.Addons.TCPProxy.Service.Name != r.resource.GetName():
		return true
	default:
		return false
	}
}

func (r *ServiceResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.Enabled
}

func (r *ServiceResource) CleanUp(ctx context.Context, _ *stewardv1alpha1.TenantControlPlane) (bool, error) {
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

func (r *ServiceResource) Define(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &corev1.Service{}
	r.resource.SetNamespace(Namespace)
	r.resource.SetName(ServiceName)

	var err error
	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tcp); err != nil {
		logger.Error(err, "unable to retrieve the Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *ServiceResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.TCPProxy == nil {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(tcp))
}

func (r *ServiceResource) GetName() string {
	return "tcp-proxy-service"
}

func (r *ServiceResource) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.TCPProxy.Service = stewardv1alpha1.ExternalKubernetesObjectStatus{}

	if tcp.Spec.Addons.TCPProxy != nil {
		tcp.Status.Addons.TCPProxy.Service = stewardv1alpha1.ExternalKubernetesObjectStatus{
			Name:       r.resource.GetName(),
			Namespace:  r.resource.GetNamespace(),
			LastUpdate: metav1.Now(),
		}
	}

	return nil
}

func (r *ServiceResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
		))

		r.resource.Spec.Type = corev1.ServiceTypeClusterIP
		r.resource.Spec.Selector = map[string]string{
			"app": AppLabel,
		}
		r.resource.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "proxy",
				Port:       ProxyPort,
				TargetPort: intstr.FromString("proxy"),
				Protocol:   corev1.ProtocolTCP,
			},
		}

		return nil
	}
}
