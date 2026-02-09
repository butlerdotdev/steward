// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/resources/tcpproxy"
)

// GetExternalTCPProxyResources returns the ordered list of tcp-proxy resources
// to be reconciled inside the tenant cluster by the soot controller.
// Order: RBAC first (SA → ClusterRole → CRB), then Service, then Deployment.
func GetExternalTCPProxyResources(c client.Client) []resources.Resource {
	return []resources.Resource{
		&tcpproxy.ServiceAccountResource{Client: c},
		&tcpproxy.ClusterRoleResource{Client: c},
		&tcpproxy.ClusterRoleBindingResource{Client: c},
		&tcpproxy.ServiceResource{Client: c},
		&tcpproxy.Agent{Client: c},
	}
}
