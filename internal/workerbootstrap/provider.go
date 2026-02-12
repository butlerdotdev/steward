// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	builder "github.com/butlerdotdev/steward/internal/builders/controlplane"
	"github.com/butlerdotdev/steward/internal/resources"
	wb "github.com/butlerdotdev/steward/internal/resources/workerbootstrap"
)

// GetPreDeploymentResources returns resources that MUST be created before the
// KubernetesDeploymentResource. The credentials Secret is mounted as a volume
// in the trustd sidecar container.
func GetPreDeploymentResources(spec *stewardv1alpha1.WorkerBootstrapSpec, c client.Client) []resources.Resource {
	if spec == nil {
		return nil
	}

	switch spec.Provider {
	case stewardv1alpha1.TalosProvider:
		return []resources.Resource{
			&wb.TalosCredentialsResource{Client: c},
		}
	default:
		return nil
	}
}

// GetPostDeploymentResources returns resources created after the Deployment.
// Includes the deployment patch (sidecar), service port, and Traefik IngressRouteTCP
// (if applicable).
func GetPostDeploymentResources(spec *stewardv1alpha1.WorkerBootstrapSpec, c client.Client, tcp *stewardv1alpha1.TenantControlPlane) []resources.Resource {
	if spec == nil {
		return nil
	}

	switch spec.Provider {
	case stewardv1alpha1.TalosProvider:
		res := []resources.Resource{
			&wb.TalosDeploymentResource{
				Builder: builder.Trustd{Scheme: *c.Scheme()},
				Client:  c,
			},
			&wb.TalosServiceResource{Client: c},
		}
		if tcp.Spec.ControlPlane.Ingress != nil && tcp.Spec.ControlPlane.Ingress.ControllerType == "traefik" {
			res = append(res, &wb.TalosTraefikIngressRouteTCPResource{Client: c})
		}

		return res
	default:
		return nil
	}
}

// GetProviderGatewayResources returns provider-specific Gateway API resources.
// Only called when Gateway API CRDs are available on the cluster.
func GetProviderGatewayResources(spec *stewardv1alpha1.WorkerBootstrapSpec, c client.Client) []resources.Resource {
	if spec == nil {
		return nil
	}

	switch spec.Provider {
	case stewardv1alpha1.TalosProvider:
		return []resources.Resource{&wb.TalosGatewayResource{Client: c}}
	default:
		return nil
	}
}
