// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package tcpproxy

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/constants"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

// Agent manages the tcp-proxy Deployment inside the tenant cluster.
type Agent struct {
	Client client.Client

	resource     *appsv1.Deployment
	tenantClient client.Client
}

func (r *Agent) GetHistogram() prometheus.Histogram {
	deploymentCollector = resources.LazyLoadHistogramFromResource(deploymentCollector, r)
	return deploymentCollector
}

func (r *Agent) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.Enabled:
		return true
	case tcp.Spec.Addons.TCPProxy != nil && !tcp.Status.Addons.TCPProxy.Enabled:
		return true
	case tcp.Spec.Addons.TCPProxy != nil &&
		tcp.Status.Addons.TCPProxy.Deployment.Name != r.resource.GetName():
		return true
	default:
		return false
	}
}

func (r *Agent) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.TCPProxy == nil && tcp.Status.Addons.TCPProxy.Enabled
}

func (r *Agent) CleanUp(ctx context.Context, _ *stewardv1alpha1.TenantControlPlane) (bool, error) {
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

func (r *Agent) Define(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &appsv1.Deployment{}
	r.resource.SetNamespace(Namespace)
	r.resource.SetName(DeploymentName)

	var err error
	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tcp); err != nil {
		logger.Error(err, "unable to retrieve the Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *Agent) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.TCPProxy == nil {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(ctx, tcp))
}

func (r *Agent) GetName() string {
	return "tcp-proxy-agent"
}

func (r *Agent) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.TCPProxy.Deployment = stewardv1alpha1.ExternalKubernetesObjectStatus{}
	tcp.Status.Addons.TCPProxy.Enabled = false

	if tcp.Spec.Addons.TCPProxy != nil {
		tcp.Status.Addons.TCPProxy.Enabled = true
		tcp.Status.Addons.TCPProxy.Deployment = stewardv1alpha1.ExternalKubernetesObjectStatus{
			Name:       r.resource.GetName(),
			Namespace:  r.resource.GetNamespace(),
			LastUpdate: metav1.Now(),
		}
	}

	return nil
}

// resolveExternalEndpoint derives the external API server endpoint that
// tcp-proxy should forward traffic to. This is the hostname:port of the
// Ingress or Gateway that fronts the tenant control plane.
func resolveExternalEndpoint(tcp *stewardv1alpha1.TenantControlPlane) (string, error) {
	// Gateway mode: use the hostname from the gateway spec.
	if tcp.Spec.ControlPlane.Gateway != nil &&
		len(tcp.Spec.ControlPlane.Gateway.Hostname) > 0 &&
		(tcp.Spec.ControlPlane.Ingress == nil || tcp.Spec.ControlPlane.Ingress.IngressClassName == "") {
		hostname, _ := utilities.GetControlPlaneAddressAndPortFromHostname(
			string(tcp.Spec.ControlPlane.Gateway.Hostname),
			tcp.Spec.NetworkProfile.Port,
		)
		return fmt.Sprintf("%s:%d", hostname, tcp.Spec.NetworkProfile.Port), nil
	}

	// Ingress mode or ClusterIP/LoadBalancer: use the assigned address.
	address, port, err := tcp.AssignedControlPlaneAddress()
	if err != nil {
		return "", err
	}

	if port == 0 {
		port = 443
	}

	return fmt.Sprintf("%s:%d", address, port), nil
}

func (r *Agent) mutate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		externalEndpoint, err := resolveExternalEndpoint(tcp)
		if err != nil {
			logger.Error(err, "unable to resolve external endpoint for tcp-proxy")

			return err
		}

		image := DefaultImage
		if tcp.Spec.Addons.TCPProxy.Image != "" {
			image = tcp.Spec.Addons.TCPProxy.Image
		}

		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
		))

		specSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": AppLabel,
			},
		}

		r.resource.Spec.Replicas = ptr.To(int32(2))
		r.resource.Spec.Selector = specSelector
		r.resource.Spec.Template.SetLabels(utilities.MergeMaps(
			r.resource.Spec.Template.GetLabels(),
			specSelector.MatchLabels,
		))

		r.resource.Spec.Template.Spec.PriorityClassName = "system-cluster-critical"
		r.resource.Spec.Template.Spec.ServiceAccountName = ServiceAccountName
		r.resource.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(true)
		r.resource.Spec.Template.Spec.NodeSelector = map[string]string{
			"kubernetes.io/os": "linux",
		}

		if len(r.resource.Spec.Template.Spec.Containers) != 1 {
			r.resource.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
		}

		container := &r.resource.Spec.Template.Spec.Containers[0]
		container.Name = "tcp-proxy"
		container.Image = image
		container.Args = []string{
			fmt.Sprintf("--external-endpoint=%s", externalEndpoint),
			fmt.Sprintf("--listen-port=%d", ProxyPort),
			fmt.Sprintf("--health-port=%d", HealthPort),
			fmt.Sprintf("--metrics-port=%d", MetricsPort),
			"--manage-endpoint-slice=true",
		}
		container.Ports = []corev1.ContainerPort{
			{
				Name:          "proxy",
				ContainerPort: ProxyPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "health",
				ContainerPort: HealthPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "metrics",
				ContainerPort: MetricsPort,
				Protocol:      corev1.ProtocolTCP,
			},
		}
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(HealthPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			TimeoutSeconds:      5,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/readyz",
					Port:   intstr.FromInt32(HealthPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 3,
			TimeoutSeconds:      3,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}
		container.SecurityContext = &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			ReadOnlyRootFilesystem:   ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
		}

		// Apply user-specified resources or sensible defaults.
		if tcp.Spec.Addons.TCPProxy.Resources != nil {
			container.Resources = *tcp.Spec.Addons.TCPProxy.Resources
		} else {
			container.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}
		}

		return nil
	}
}
