// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package tcpproxy

const (
	// Namespace is where tcp-proxy resources are deployed inside the tenant cluster.
	Namespace = "kube-system"

	// DeploymentName is the name of the tcp-proxy Deployment.
	DeploymentName = "steward-tcp-proxy"

	// ServiceName is the name of the tcp-proxy Service.
	ServiceName = "steward-tcp-proxy"

	// ServiceAccountName is the name of the tcp-proxy ServiceAccount.
	ServiceAccountName = "steward-tcp-proxy"

	// ClusterRoleName is the name of the tcp-proxy ClusterRole.
	ClusterRoleName = "steward:tcp-proxy"

	// ClusterRoleBindingName is the name of the tcp-proxy ClusterRoleBinding.
	ClusterRoleBindingName = "steward:tcp-proxy"

	// DefaultImage is the default container image for tcp-proxy.
	// TLS termination mode for Ingress/Gateway, passthrough for LoadBalancer/NodePort.
	DefaultImage = "ghcr.io/butlerdotdev/steward-tcp-proxy:tls-termination-20260208151640"

	// ProxyPort is the port tcp-proxy listens on for proxied connections.
	ProxyPort = 6443

	// HealthPort is the port for health check endpoints.
	HealthPort = 8080

	// MetricsPort is the port for Prometheus metrics.
	MetricsPort = 9090

	// AppLabel is the app label value used for pod selection.
	AppLabel = "steward-tcp-proxy"
)
