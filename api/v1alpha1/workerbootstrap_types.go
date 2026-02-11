// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// WorkerBootstrapProvider is the OS-specific bootstrap provider type.
// +kubebuilder:validation:Enum=talos
type WorkerBootstrapProvider string

const (
	TalosProvider WorkerBootstrapProvider = "talos"
)

// WorkerBootstrapSpec configures immutable OS worker node bootstrap.
// The provider field selects the OS-specific implementation.
type WorkerBootstrapSpec struct {
	// Provider specifies the immutable OS bootstrap provider.
	// +kubebuilder:validation:Enum=talos
	Provider WorkerBootstrapProvider `json:"provider"`

	// Talos-specific configuration. Required when provider is "talos".
	// +optional
	Talos *TalosBootstrapSpec `json:"talos,omitempty"`

	// CSRApproval configures automatic CSR approval for worker kubelet-serving certs.
	// +kubebuilder:default={autoApprove: true}
	CSRApproval CSRApprovalSpec `json:"csrApproval,omitempty"`

	// AllowedSubnets restricts which worker IP ranges are valid for CSR approval.
	// CIDR format (e.g., "10.40.0.0/22"). If empty, all IPs are allowed.
	// +optional
	AllowedSubnets []string `json:"allowedSubnets,omitempty"`
}

// TalosBootstrapSpec configures steward-trustd for Talos worker nodes.
type TalosBootstrapSpec struct {
	// Image is the container image for steward-trustd.
	// +kubebuilder:default="ghcr.io/butlerdotdev/steward-trustd"
	Image string `json:"image,omitempty"`

	// ImageTag is the image tag for steward-trustd.
	// +optional
	ImageTag string `json:"imageTag,omitempty"`

	// Port for the trustd gRPC service.
	// +kubebuilder:default=50001
	Port int32 `json:"port,omitempty"`

	// Resources for the trustd container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// CertSANs adds extra Subject Alternative Names to the trustd server certificate.
	// +optional
	CertSANs []string `json:"certSANs,omitempty"`
}

// CSRApprovalSpec configures automatic CSR approval for worker kubelet-serving certs.
type CSRApprovalSpec struct {
	// AutoApprove enables automatic approval of kubelet-serving CSRs from workers.
	// +kubebuilder:default=true
	AutoApprove bool `json:"autoApprove"`
}

// WorkerBootstrapStatus defines the observed state of worker bootstrap.
type WorkerBootstrapStatus struct {
	// Enabled indicates whether worker bootstrap is currently active.
	Enabled bool `json:"enabled"`

	// Provider is the active bootstrap provider.
	Provider WorkerBootstrapProvider `json:"provider,omitempty"`

	// Credentials tracks the OS credential Secret.
	Credentials CertificatePrivateKeyPairStatus `json:"credentials,omitempty"`

	// Endpoint is the trustd endpoint for worker nodes (ip:port or hostname:port).
	Endpoint string `json:"endpoint,omitempty"`

	// Service tracks the trustd port on the TCP Service.
	Service KubernetesServiceStatus `json:"service,omitempty"`
}
