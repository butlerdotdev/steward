// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	stewarderrors "github.com/butlerdotdev/steward/internal/errors"
)

// AssignedControlPlaneAddress returns the announced address and port of a Tenant Control Plane.
// In case of non-well formed values, or missing announcement, an error is returned.
func (in *TenantControlPlane) AssignedControlPlaneAddress() (string, int32, error) {
	if len(in.Status.ControlPlaneEndpoint) == 0 {
		return "", 0, fmt.Errorf("the Tenant Control Plane is not yet exposed")
	}

	address, portString, err := net.SplitHostPort(in.Status.ControlPlaneEndpoint)
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot split host port from Tenant Control Plane endpoint")
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, errors.Wrap(err, "cannot convert Tenant Control Plane port from endpoint")
	}

	return address, int32(port), nil
}

// DeclaredControlPlaneAddress returns the desired Tenant Control Plane address.
// In case of dynamic allocation, e.g. using a Load Balancer, it queries the API Server looking for the allocated IP.
// When an IP has not been yet assigned, or it is expected, an error is returned.
// Note: For internal kubeadm configuration, this always returns an IP address.
// For the external endpoint (Ingress/Gateway), use ExternalControlPlaneAddress instead.
func (in *TenantControlPlane) DeclaredControlPlaneAddress(ctx context.Context, client client.Client) (string, error) {
	var loadBalancerStatus corev1.LoadBalancerStatus

	svc := &corev1.Service{}
	err := client.Get(ctx, types.NamespacedName{Namespace: in.GetNamespace(), Name: in.GetName()}, svc)
	if err != nil {
		return "", errors.Wrap(err, "cannot retrieve Service for the TenantControlPlane")
	}

	switch {
	case len(in.Spec.NetworkProfile.Address) > 0:
		// Returning the hard-coded value in the specification in case of non LoadBalanced resources
		return in.Spec.NetworkProfile.Address, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP:
		return svc.Spec.ClusterIP, nil
	case svc.Spec.Type == corev1.ServiceTypeNodePort:
		// NodePort services (used in Ingress/Gateway mode) still have a ClusterIP
		// which is valid for internal cluster communication and kubeadm config
		return svc.Spec.ClusterIP, nil
	case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
		loadBalancerStatus = svc.Status.LoadBalancer
		if len(loadBalancerStatus.Ingress) == 0 {
			return "", stewarderrors.NonExposedLoadBalancerError{}
		}

		return getLoadBalancerAddress(loadBalancerStatus.Ingress)
	}

	return "", stewarderrors.MissingValidIPError{}
}

// ExternalControlPlaneAddress returns the external address for the control plane.
// For Ingress/Gateway modes, this returns the configured hostname.
// For LoadBalancer mode, this returns the LoadBalancer IP.
// This is used for Status.ControlPlaneEndpoint and konnectivity-agent configuration.
func (in *TenantControlPlane) ExternalControlPlaneAddress(ctx context.Context, client client.Client) (address string, port int32, err error) {
	// For Ingress mode, return the Ingress hostname and port 443
	if in.Spec.ControlPlane.Ingress != nil && in.Spec.ControlPlane.Ingress.Hostname != "" {
		hostname := in.Spec.ControlPlane.Ingress.Hostname
		// Extract just the hostname without port if present
		if idx := strings.Index(hostname, ":"); idx != -1 {
			hostname = hostname[:idx]
		}

		return hostname, 443, nil
	}

	// For Gateway mode, return the Gateway hostname and port 443
	if in.Spec.ControlPlane.Gateway != nil && in.Spec.ControlPlane.Gateway.Hostname != "" {
		return string(in.Spec.ControlPlane.Gateway.Hostname), 443, nil
	}

	// For other modes, use the declared address with the configured port
	addr, err := in.DeclaredControlPlaneAddress(ctx, client)
	if err != nil {
		return "", 0, err
	}

	return addr, in.Spec.NetworkProfile.Port, nil
}

// getLoadBalancerAddress extracts the IP address from LoadBalancer ingress.
// It also checks and rejects hostname usage for LoadBalancer ingress.
//
// Reasons for not supporting hostnames:
// - DNS resolution can differ across environments, leading to inconsistent behavior.
// - It may cause connectivity problems between Kubernetes components.
// - The DNS resolution could change over time, potentially breaking cluster-to-API-server connections.
//
// Recommended solutions:
// - Use a static IP address to ensure stable and predictable communication within the cluster.
// - If a hostname is necessary, consider setting up a Virtual IP (VIP) for the given hostname.
// - Alternatively, use an external load balancer that can provide a stable IP address.
//
// Note: Implementing L7 routing with the API Server requires a deep understanding of the implications.
// Users should be aware of the complexities involved, including potential issues with TLS passthrough
// for client-based certificate authentication in Ingress expositions.
func getLoadBalancerAddress(ingress []corev1.LoadBalancerIngress) (string, error) {
	for _, lb := range ingress {
		if ip := lb.IP; len(ip) > 0 {
			return ip, nil
		}
		if hostname := lb.Hostname; len(hostname) > 0 {
			return "", fmt.Errorf("hostname not supported for LoadBalancer ingress: use static IP instead")
		}
	}

	return "", stewarderrors.MissingValidIPError{}
}

func (in *TenantControlPlane) normalizeNamespaceName() string {
	// The dash character (-) must be replaced with an underscore, PostgreSQL is complaining about it:
	// https://github.com/butlerdotdev/steward/issues/328
	return strings.ReplaceAll(fmt.Sprintf("%s_%s", in.GetNamespace(), in.GetName()), "-", "_")
}

func (in *TenantControlPlane) GetDefaultDatastoreUsername() string {
	return in.normalizeNamespaceName()
}

func (in *TenantControlPlane) GetDefaultDatastoreSchema() string {
	return in.normalizeNamespaceName()
}
