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
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/constants"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

const (
	// tlsCertSecretName is the name of the Secret containing the API server cert
	// that tcp-proxy uses for TLS termination.
	tlsCertSecretName = "steward-tcp-proxy-tls"

	// tlsCertMountPath is where the TLS cert is mounted in the container.
	tlsCertMountPath = "/etc/tcp-proxy/tls"
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

	// For Ingress/Gateway mode, ensure the TLS cert Secret exists in tenant cluster
	if isIngressOrGatewayMode(tcp) {
		if err := r.ensureTLSSecret(ctx, tcp); err != nil {
			return controllerutil.OperationResultNone, err
		}
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

// isIngressOrGatewayMode returns true if the TCP is exposed via Ingress or Gateway.
func isIngressOrGatewayMode(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return (tcp.Spec.ControlPlane.Ingress != nil && len(tcp.Spec.ControlPlane.Ingress.Hostname) > 0) ||
		(tcp.Spec.ControlPlane.Gateway != nil && len(tcp.Spec.ControlPlane.Gateway.Hostname) > 0)
}

// getIngressHostname returns the Ingress/Gateway hostname for the TCP.
func getIngressHostname(tcp *stewardv1alpha1.TenantControlPlane) string {
	if tcp.Spec.ControlPlane.Ingress != nil && len(tcp.Spec.ControlPlane.Ingress.Hostname) > 0 {
		return tcp.Spec.ControlPlane.Ingress.Hostname
	}
	if tcp.Spec.ControlPlane.Gateway != nil && len(tcp.Spec.ControlPlane.Gateway.Hostname) > 0 {
		return string(tcp.Spec.ControlPlane.Gateway.Hostname)
	}
	return ""
}

// ensureTLSSecret copies the API server certificate to the tenant cluster
// for tcp-proxy to use in TLS termination mode.
func (r *Agent) ensureTLSSecret(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	// Get the API server certificate from the management cluster
	apiServerSecretName := tcp.Status.Certificates.APIServer.SecretName
	if apiServerSecretName == "" {
		return fmt.Errorf("API server certificate secret not yet created")
	}

	mgmtSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      apiServerSecretName,
		Namespace: tcp.GetNamespace(),
	}, mgmtSecret); err != nil {
		return fmt.Errorf("failed to get API server certificate: %w", err)
	}

	// Create/update the TLS secret in the tenant cluster
	tenantSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsCertSecretName,
			Namespace: Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.tenantClient, tenantSecret, func() error {
		tenantSecret.SetLabels(utilities.MergeMaps(
			tenantSecret.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
		))
		tenantSecret.Type = corev1.SecretTypeOpaque
		tenantSecret.Data = map[string][]byte{
			"tls.crt": mgmtSecret.Data[kubeadmconstants.APIServerCertName],
			"tls.key": mgmtSecret.Data[kubeadmconstants.APIServerKeyName],
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to create/update TLS secret in tenant cluster")
		return err
	}

	return nil
}

// resolveUpstreamEndpoint determines the upstream address for tcp-proxy.
// For Ingress/Gateway modes, uses the Ingress hostname (TLS termination mode).
// For LoadBalancer/NodePort, uses the assigned address (passthrough mode).
func resolveUpstreamEndpoint(ctx context.Context, c client.Client, tcp *stewardv1alpha1.TenantControlPlane) (string, error) {
	if isIngressOrGatewayMode(tcp) {
		// Use the Ingress hostname - tcp-proxy will connect with proper SNI
		hostname := getIngressHostname(tcp)
		if hostname == "" {
			return "", fmt.Errorf("Ingress/Gateway mode but no hostname configured")
		}
		// The Ingress uses port 443
		return fmt.Sprintf("%s:443", hostname), nil
	}

	// LoadBalancer/NodePort mode: use the assigned address directly (passthrough mode)
	address, port, err := tcp.AssignedControlPlaneAddress()
	if err != nil {
		return "", err
	}

	if port == 0 {
		port = 6443
	}

	return fmt.Sprintf("%s:%d", address, port), nil
}

func (r *Agent) mutate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		upstreamEndpoint, err := resolveUpstreamEndpoint(ctx, r.Client, tcp)
		if err != nil {
			logger.Error(err, "unable to resolve upstream endpoint for tcp-proxy")
			return err
		}

		tlsMode := isIngressOrGatewayMode(tcp)

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
		// tcp-proxy must be able to run on NotReady nodes to bootstrap the cluster.
		// Use hostNetwork so it can run before CNI is ready.
		r.resource.Spec.Template.Spec.HostNetwork = true
		r.resource.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
		r.resource.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Key:      "node.kubernetes.io/not-ready",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node.kubernetes.io/not-ready",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
			{
				Key:      "node.cilium.io/agent-not-ready",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}

		if len(r.resource.Spec.Template.Spec.Containers) != 1 {
			r.resource.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
		}

		container := &r.resource.Spec.Template.Spec.Containers[0]
		container.Name = "tcp-proxy"
		container.Image = image

		// Build container args
		args := []string{
			fmt.Sprintf("--upstream-addr=%s", upstreamEndpoint),
			fmt.Sprintf("--listen-addr=:%d", ProxyPort),
		}

		if tlsMode {
			// TLS termination mode - add cert/key paths
			args = append(args,
				fmt.Sprintf("--tls-cert-file=%s/tls.crt", tlsCertMountPath),
				fmt.Sprintf("--tls-key-file=%s/tls.key", tlsCertMountPath),
				// Use insecure for now since we're connecting to our own Ingress
				// TODO: Add proper CA verification
				"--upstream-insecure=true",
			)
		}
		container.Args = args

		// Build environment variables
		env := []corev1.EnvVar{
			{
				// POD_IP is used by tcp-proxy to configure the EndpointSlice.
				// With hostNetwork, this equals the node IP.
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		}

		// For bootstrap, override KUBERNETES_SERVICE_HOST/PORT
		// In TLS mode, we need a reachable address before the proxy is running
		if !tlsMode {
			// Passthrough mode - use the upstream directly for bootstrap
			env = append(env,
				corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_HOST",
					Value: utilities.ExtractHost(upstreamEndpoint),
				},
				corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_PORT",
					Value: utilities.ExtractPort(upstreamEndpoint),
				},
			)
		} else {
			// TLS mode - tcp-proxy needs to reach the Ingress for bootstrap
			// The hostAliases provide DNS resolution for the Ingress hostname
			env = append(env,
				corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_HOST",
					Value: utilities.ExtractHost(upstreamEndpoint),
				},
				corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_PORT",
					Value: "443",
				},
			)
		}
		container.Env = env

		container.Ports = []corev1.ContainerPort{
			{
				Name:          "proxy",
				ContainerPort: ProxyPort,
				Protocol:      corev1.ProtocolTCP,
			},
		}
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(ProxyPort),
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
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(ProxyPort),
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

		// Mount TLS certificate for TLS termination mode
		if tlsMode {
			container.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "tls-certs",
					MountPath: tlsCertMountPath,
					ReadOnly:  true,
				},
			}
			r.resource.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "tls-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: tlsCertSecretName,
						},
					},
				},
			}
		} else {
			// Clear volumes for passthrough mode
			container.VolumeMounts = nil
			r.resource.Spec.Template.Spec.Volumes = nil
		}

		// Apply hostAliases for Ingress/Gateway mode.
		// tcp-proxy uses hostNetwork, so it needs /etc/hosts entries to resolve
		// the API server hostname before CoreDNS is available.
		if len(tcp.Spec.Addons.TCPProxy.HostAliases) > 0 {
			hostAliases := make([]corev1.HostAlias, 0, len(tcp.Spec.Addons.TCPProxy.HostAliases))
			for _, ha := range tcp.Spec.Addons.TCPProxy.HostAliases {
				hostAliases = append(hostAliases, corev1.HostAlias{
					IP:        ha.IP,
					Hostnames: ha.Hostnames,
				})
			}
			r.resource.Spec.Template.Spec.HostAliases = hostAliases
		}

		return nil
	}
}
