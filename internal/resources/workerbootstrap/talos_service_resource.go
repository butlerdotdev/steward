// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

const trustdServicePortName = "steward-trustd"

// TalosServiceResource appends the trustd port to the existing TCP Service.
// Follows the konnectivity ServiceResource pattern.
type TalosServiceResource struct {
	resource *corev1.Service
	Client   client.Client
}

func (r *TalosServiceResource) GetHistogram() prometheus.Histogram {
	serviceCollector = resources.LazyLoadHistogramFromResource(serviceCollector, r)

	return serviceCollector
}

func (r *TalosServiceResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	if !shouldHaveWorkerBootstrap(tcp) {
		return tcp.Status.Addons.WorkerBootstrap.Service.Name != ""
	}

	if tcp.Status.Addons.WorkerBootstrap.Service.Name != r.resource.GetName() ||
		tcp.Status.Addons.WorkerBootstrap.Service.Namespace != r.resource.GetNamespace() {
		return true
	}

	// Check if trustd port exists and matches
	port := findTrustdPort(r.resource)
	if port == 0 {
		return true
	}

	return tcp.Status.Addons.WorkerBootstrap.Service.Port != port
}

func (r *TalosServiceResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return !shouldHaveWorkerBootstrap(tcp) && tcp.Status.Addons.WorkerBootstrap.Service.Name != ""
}

func (r *TalosServiceResource) CleanUp(ctx context.Context, _ *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	res, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, func() error {
		for i, port := range r.resource.Spec.Ports {
			if port.Name == trustdServicePortName {
				r.resource.Spec.Ports = append(r.resource.Spec.Ports[:i], r.resource.Spec.Ports[i+1:]...)

				break
			}
		}

		return nil
	})
	if err != nil {
		logger.Error(err, "unable to cleanup trustd service port")

		return false, err
	}

	return res == controllerutil.OperationResultUpdated, nil
}

func (r *TalosServiceResource) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	if !shouldHaveWorkerBootstrap(tcp) {
		tcp.Status.Addons.WorkerBootstrap.Service = stewardv1alpha1.KubernetesServiceStatus{}

		return nil
	}

	tcp.Status.Addons.WorkerBootstrap.Service.Name = r.resource.GetName()
	tcp.Status.Addons.WorkerBootstrap.Service.Namespace = r.resource.GetNamespace()
	tcp.Status.Addons.WorkerBootstrap.Service.Port = findTrustdPort(r.resource)
	tcp.Status.Addons.WorkerBootstrap.Service.ServiceStatus = r.resource.Status

	return nil
}

func (r *TalosServiceResource) Define(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcp.GetName(),
			Namespace: tcp.GetNamespace(),
		},
	}

	return nil
}

func (r *TalosServiceResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if !shouldHaveWorkerBootstrap(tcp) {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(tcp))
}

func (r *TalosServiceResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) func() error {
	return func() error {
		if len(r.resource.Spec.Ports) == 0 {
			return nil // Service not ready yet
		}

		port := tcp.Spec.Addons.WorkerBootstrap.Talos.Port

		// Find or append the trustd port
		found := false
		for i, p := range r.resource.Spec.Ports {
			if p.Name == trustdServicePortName {
				r.resource.Spec.Ports[i].Port = port
				r.resource.Spec.Ports[i].TargetPort = intstr.FromInt32(port)
				r.resource.Spec.Ports[i].Protocol = corev1.ProtocolTCP
				if tcp.Spec.ControlPlane.Service.ServiceType == stewardv1alpha1.ServiceTypeNodePort {
					r.resource.Spec.Ports[i].NodePort = port
				}
				found = true

				break
			}
		}

		if !found {
			sp := corev1.ServicePort{
				Name:       trustdServicePortName,
				Port:       port,
				TargetPort: intstr.FromInt32(port),
				Protocol:   corev1.ProtocolTCP,
			}
			if tcp.Spec.ControlPlane.Service.ServiceType == stewardv1alpha1.ServiceTypeNodePort {
				sp.NodePort = port
			}
			r.resource.Spec.Ports = append(r.resource.Spec.Ports, sp)
		}

		return controllerutil.SetControllerReference(tcp, r.resource, r.Client.Scheme())
	}
}

func (r *TalosServiceResource) GetName() string {
	return "talos-trustd-service"
}

func findTrustdPort(svc *corev1.Service) int32 {
	for _, p := range svc.Spec.Ports {
		if p.Name == trustdServicePortName {
			return p.Port
		}
	}

	return 0
}
