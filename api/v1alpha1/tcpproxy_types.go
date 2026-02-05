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
