// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"context"
	"fmt"
	"net"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/crypto"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

// TalosCredentialsResource manages the OS credential Secret for steward-trustd.
type TalosCredentialsResource struct {
	resource *corev1.Secret
	Client   client.Client
}

func (r *TalosCredentialsResource) GetHistogram() prometheus.Histogram {
	credentialsCollector = resources.LazyLoadHistogramFromResource(credentialsCollector, r)
	return credentialsCollector
}

func (r *TalosCredentialsResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	if !shouldHaveWorkerBootstrap(tcp) {
		return tcp.Status.Addons.WorkerBootstrap.Enabled
	}
	return tcp.Status.Addons.WorkerBootstrap.Credentials.SecretName != r.resource.Name ||
		tcp.Status.Addons.WorkerBootstrap.Endpoint == ""
}

func (r *TalosCredentialsResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return !shouldHaveWorkerBootstrap(tcp) && tcp.Status.Addons.WorkerBootstrap.Enabled
}

func (r *TalosCredentialsResource) CleanUp(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "failed to delete trustd credentials secret")
			return false, err
		}
		return false, nil
	}

	logger.V(1).Info("trustd credentials secret cleaned up")
	return true, nil
}

func (r *TalosCredentialsResource) UpdateTenantControlPlaneStatus(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	if !shouldHaveWorkerBootstrap(tcp) {
		tcp.Status.Addons.WorkerBootstrap = stewardv1alpha1.WorkerBootstrapStatus{}
		return nil
	}

	tcp.Status.Addons.WorkerBootstrap.Enabled = true
	tcp.Status.Addons.WorkerBootstrap.Provider = tcp.Spec.Addons.WorkerBootstrap.Provider
	tcp.Status.Addons.WorkerBootstrap.Credentials.SecretName = r.resource.Name
	tcp.Status.Addons.WorkerBootstrap.Endpoint = deriveEndpoint(tcp)

	return nil
}

func (r *TalosCredentialsResource) Define(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-trustd-creds", tcp.GetName()),
			Namespace: tcp.GetNamespace(),
		},
	}
	return nil
}

func (r *TalosCredentialsResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if !shouldHaveWorkerBootstrap(tcp) {
		return controllerutil.OperationResultNone, nil
	}

	logger := log.FromContext(ctx, "resource", r.GetName())

	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, func() error {
		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
		)
		r.resource.SetLabels(labels)

		ipAddresses, dnsNames := collectSANs(tcp)

		// Read the Service directly from Kubernetes to get the LB/ClusterIP,
		// since the credentials resource runs before the service status resource
		// populates tcp.Status fields.
		if declaredAddr, dErr := tcp.DeclaredControlPlaneAddress(ctx, r.Client); dErr == nil && declaredAddr != "" {
			if ip := net.ParseIP(declaredAddr); ip != nil {
				ipAddresses = append(ipAddresses, ip)
			} else {
				dnsNames = append(dnsNames, declaredAddr)
			}
		}

		if r.resource.Data == nil || len(r.resource.Data["os-ca.crt"]) == 0 {
			logger.V(1).Info("generating new trustd credentials")
			creds, err := crypto.GenerateTrustdCredentials(tcp.GetName(), ipAddresses, dnsNames)
			if err != nil {
				return fmt.Errorf("generating trustd credentials: %w", err)
			}
			r.resource.Data = map[string][]byte{
				"os-ca.crt":  creds.OSCACert,
				"os-ca.key":  creds.OSCAKey,
				"server.crt": creds.ServerChain,
				"server.key": creds.ServerKey,
				"token":      []byte(creds.Token),
			}
		} else {
			// Check if SANs changed and regenerate server cert only
			existingIPs, existingDNS, err := crypto.ParseTrustdServerCertSANs(r.resource.Data["server.crt"])
			if err != nil {
				logger.V(1).Info("failed to parse existing server cert, regenerating", "error", err)
				return r.regenerateServerCert(ipAddresses, dnsNames)
			}

			if !sansEqual(existingIPs, ipAddresses, existingDNS, dnsNames) {
				logger.V(1).Info("SANs changed, regenerating server cert")
				return r.regenerateServerCert(ipAddresses, dnsNames)
			}
		}

		return controllerutil.SetControllerReference(tcp, r.resource, r.Client.Scheme())
	})
}

func (r *TalosCredentialsResource) GetName() string {
	return "talos-trustd-credentials"
}

func (r *TalosCredentialsResource) regenerateServerCert(ipAddresses []net.IP, dnsNames []string) error {
	chain, key, err := crypto.RegenerateTrustdServerCert(
		r.resource.Data["os-ca.crt"],
		r.resource.Data["os-ca.key"],
		ipAddresses,
		dnsNames,
	)
	if err != nil {
		return fmt.Errorf("regenerating server cert: %w", err)
	}
	r.resource.Data["server.crt"] = chain
	r.resource.Data["server.key"] = key
	return nil
}

func shouldHaveWorkerBootstrap(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.WorkerBootstrap != nil && tcp.Spec.Addons.WorkerBootstrap.Provider == stewardv1alpha1.TalosProvider
}

func collectSANs(tcp *stewardv1alpha1.TenantControlPlane) ([]net.IP, []string) {
	var ipAddresses []net.IP
	var dnsNames []string

	// LB IP from main service status
	for _, ingress := range tcp.Status.Kubernetes.Service.LoadBalancer.Ingress {
		if ingress.IP != "" {
			if ip := net.ParseIP(ingress.IP); ip != nil {
				ipAddresses = append(ipAddresses, ip)
			}
		}
		if ingress.Hostname != "" {
			dnsNames = append(dnsNames, ingress.Hostname)
		}
	}

	// LB IP from workerBootstrap service status (trustd shares the TCP Service)
	for _, ingress := range tcp.Status.Addons.WorkerBootstrap.Service.LoadBalancer.Ingress {
		if ingress.IP != "" {
			if ip := net.ParseIP(ingress.IP); ip != nil {
				ipAddresses = append(ipAddresses, ip)
			}
		}
		if ingress.Hostname != "" {
			dnsNames = append(dnsNames, ingress.Hostname)
		}
	}

	// Ingress hostname — add both the DNS name and resolved IPs.
	// Resolved IPs are needed because Talos validates the trustd cert against
	// the IP the hostname resolves to (the Ingress controller's external IP),
	// not the hostname itself.
	if tcp.Spec.ControlPlane.Ingress != nil && tcp.Spec.ControlPlane.Ingress.Hostname != "" {
		host, _ := utilities.GetControlPlaneAddressAndPortFromHostname(tcp.Spec.ControlPlane.Ingress.Hostname, 0)
		if ip := net.ParseIP(host); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else if host != "" {
			dnsNames = append(dnsNames, host)
			ipAddresses = append(ipAddresses, resolveHostIPs(host)...)
		}
	}

	// Gateway hostname — same DNS resolution treatment as Ingress.
	if tcp.Spec.ControlPlane.Gateway != nil && len(tcp.Spec.ControlPlane.Gateway.Hostname) > 0 {
		host, _ := utilities.GetControlPlaneAddressAndPortFromHostname(string(tcp.Spec.ControlPlane.Gateway.Hostname), 0)
		if ip := net.ParseIP(host); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else if host != "" {
			dnsNames = append(dnsNames, host)
			ipAddresses = append(ipAddresses, resolveHostIPs(host)...)
		}
	}

	// Extra SANs from spec
	if tcp.Spec.Addons.WorkerBootstrap != nil && tcp.Spec.Addons.WorkerBootstrap.Talos != nil {
		for _, san := range tcp.Spec.Addons.WorkerBootstrap.Talos.CertSANs {
			if ip := net.ParseIP(san); ip != nil {
				ipAddresses = append(ipAddresses, ip)
			} else {
				dnsNames = append(dnsNames, san)
			}
		}
	}

	return ipAddresses, dnsNames
}

func deriveEndpoint(tcp *stewardv1alpha1.TenantControlPlane) string {
	if tcp.Spec.Addons.WorkerBootstrap == nil || tcp.Spec.Addons.WorkerBootstrap.Talos == nil {
		return ""
	}
	port := tcp.Spec.Addons.WorkerBootstrap.Talos.Port

	switch {
	case tcp.Spec.ControlPlane.Gateway != nil:
		host, _ := utilities.GetControlPlaneAddressAndPortFromHostname(string(tcp.Spec.ControlPlane.Gateway.Hostname), 0)
		if host != "" {
			return fmt.Sprintf("%s:%d", host, port)
		}
	case tcp.Spec.ControlPlane.Ingress != nil:
		host, _ := utilities.GetControlPlaneAddressAndPortFromHostname(tcp.Spec.ControlPlane.Ingress.Hostname, 0)
		if host != "" {
			return fmt.Sprintf("%s:%d", host, port)
		}
	default:
		for _, ingress := range tcp.Status.Kubernetes.Service.LoadBalancer.Ingress {
			if ingress.IP != "" {
				return fmt.Sprintf("%s:%d", ingress.IP, port)
			}
			if ingress.Hostname != "" {
				return fmt.Sprintf("%s:%d", ingress.Hostname, port)
			}
		}
	}

	return ""
}

// resolveHostIPs resolves a hostname to its IP addresses so they can be added
// as certificate SANs. This is necessary for TLS passthrough proxies (Ingress,
// Gateway) where the client connects to the proxy IP but validates the backend
// cert against the resolved IP rather than the hostname.
// Returns nil on lookup failure — the cert will still have the DNS SAN which
// is sufficient for hostname-based validation. IP SANs are best-effort.
func resolveHostIPs(host string) []net.IP {
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil
	}
	var ips []net.IP
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

func sansEqual(existingIPs, newIPs []net.IP, existingDNS, newDNS []string) bool {
	if len(existingIPs) != len(newIPs) || len(existingDNS) != len(newDNS) {
		return false
	}

	ipSet := make(map[string]struct{}, len(existingIPs))
	for _, ip := range existingIPs {
		ipSet[ip.String()] = struct{}{}
	}
	for _, ip := range newIPs {
		if _, ok := ipSet[ip.String()]; !ok {
			return false
		}
	}

	dnsSet := make(map[string]struct{}, len(existingDNS))
	for _, dns := range existingDNS {
		dnsSet[dns] = struct{}{}
	}
	for _, dns := range newDNS {
		if _, ok := dnsSet[dns]; !ok {
			return false
		}
	}

	return true
}
