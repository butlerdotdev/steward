// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// This file contains the NEW types to be added to the Steward API.
// These types must be merged into the existing api/v1alpha1/ package.
//
// Integration points:
//   - TCPProxySpec → AddonsSpec.TCPProxy
//   - TCPProxyStatus → AddonsStatus.TCPProxy

import (
	corev1 "k8s.io/api/core/v1"
)

// TCPProxyHostAlias defines a hostname-to-IP mapping for /etc/hosts injection.
// Used to resolve hostnames before DNS is available (bootstrap phase).
type TCPProxyHostAlias struct {
	// IP address of the host entry.
	IP string `json:"ip"`
	// Hostnames for the IP address.
	Hostnames []string `json:"hostnames"`
}

// TCPProxySpec defines the configuration for the TCP proxy addon.
// When enabled, Steward deploys a tcp-proxy into the tenant cluster that handles
// kubernetes.default.svc routing and manages the kubernetes EndpointSlice.
// Required when using Ingress or Gateway API to expose the tenant API server.
type TCPProxySpec struct {
	// Image is the container image for the tcp-proxy.
	// Defaults to ghcr.io/butlerdotdev/steward-tcp-proxy:<steward-version>
	// +optional
	Image string `json:"image,omitempty"`

	// Resources defines the compute resources for the tcp-proxy container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// HostAliases provides hostname-to-IP mappings for /etc/hosts injection.
	// Required for Ingress/Gateway modes where the API server hostname must be
	// resolved before CoreDNS is available. The tcp-proxy uses hostNetwork,
	// so it needs these entries to connect to the upstream API server.
	// +optional
	HostAliases []TCPProxyHostAlias `json:"hostAliases,omitempty"`

	// InternalEndpoint is the direct endpoint for tcp-proxy to reach the API server.
	// For Ingress/Gateway modes, this should be a management cluster node IP that
	// is reachable from tenant worker nodes (e.g., "10.40.0.201"). The NodePort
	// is automatically appended by Steward based on the service configuration.
	// If not specified, Steward attempts to use the service's LoadBalancer IP.
	// +optional
	InternalEndpoint string `json:"internalEndpoint,omitempty"`
}

// TCPProxyStatus defines the observed state of the TCP proxy addon.
type TCPProxyStatus struct {
	// Enabled indicates whether the tcp-proxy addon is currently active.
	Enabled bool `json:"enabled"`

	// Deployment contains the status of the tcp-proxy Deployment in the tenant cluster.
	Deployment ExternalKubernetesObjectStatus `json:"deployment,omitempty"`

	// Service contains the status of the tcp-proxy Service in the tenant cluster.
	Service ExternalKubernetesObjectStatus `json:"service,omitempty"`

	// ServiceAccount contains the status of the tcp-proxy ServiceAccount.
	ServiceAccount ExternalKubernetesObjectStatus `json:"serviceAccount,omitempty"`

	// ClusterRole contains the status of the tcp-proxy ClusterRole.
	ClusterRole ExternalKubernetesObjectStatus `json:"clusterRole,omitempty"`

	// ClusterRoleBinding contains the status of the tcp-proxy ClusterRoleBinding.
	ClusterRoleBinding ExternalKubernetesObjectStatus `json:"clusterRoleBinding,omitempty"`
}
