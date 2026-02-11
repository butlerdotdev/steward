// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

// TalosGatewayResource manages a TLSRoute for trustd on the steward-trustd Gateway listener.
type TalosGatewayResource struct {
	resource *gatewayv1alpha2.TLSRoute
	Client   client.Client
}

func (r *TalosGatewayResource) GetHistogram() prometheus.Histogram {
	gatewayCollector = resources.LazyLoadHistogramFromResource(gatewayCollector, r)
	return gatewayCollector
}

func (r *TalosGatewayResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	if !r.shouldHaveGateway(tcp) {
		return false
	}
	return false
}

func (r *TalosGatewayResource) shouldHaveGateway(tcp *stewardv1alpha1.TenantControlPlane) bool {
	if !shouldHaveWorkerBootstrap(tcp) {
		return false
	}
	return tcp.Spec.ControlPlane.Gateway != nil
}

func (r *TalosGatewayResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	return !r.shouldHaveGateway(tcp)
}

func (r *TalosGatewayResource) CleanUp(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	cleaned, err := resources.CleanupTLSRoute(ctx, r.Client, r.resource.GetName(), r.resource.GetNamespace(), tcp)
	if err != nil {
		logger.Error(err, "failed to cleanup trustd TLSRoute")
		return false, err
	}

	if cleaned {
		logger.V(1).Info("trustd TLSRoute cleaned up")
	}

	return cleaned, nil
}

func (r *TalosGatewayResource) UpdateTenantControlPlaneStatus(_ context.Context, _ *stewardv1alpha1.TenantControlPlane) error {
	return nil
}

func (r *TalosGatewayResource) Define(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &gatewayv1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-trustd", tcp.GetName()),
			Namespace: tcp.GetNamespace(),
		},
	}
	return nil
}

func (r *TalosGatewayResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if !r.shouldHaveGateway(tcp) {
		return controllerutil.OperationResultNone, nil
	}

	logger := log.FromContext(ctx, "resource", r.GetName())
	logger.V(1).Info("creating or updating trustd TLSRoute")

	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tcp))
}

func (r *TalosGatewayResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if tcp.Spec.ControlPlane.Gateway == nil {
			return fmt.Errorf("control plane gateway is not configured")
		}

		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
			tcp.Spec.ControlPlane.Gateway.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tcp.Spec.ControlPlane.Gateway.AdditionalMetadata.Annotations,
		)
		r.resource.SetAnnotations(annotations)

		if len(tcp.Spec.ControlPlane.Gateway.Hostname) == 0 {
			return fmt.Errorf("control plane gateway hostname is not set")
		}

		port := gatewayv1.PortNumber(tcp.Spec.Addons.WorkerBootstrap.Talos.Port)
		serviceName := gatewayv1alpha2.ObjectName(tcp.Status.Kubernetes.Service.Name)

		if serviceName == "" || port == 0 {
			return fmt.Errorf("service not ready, cannot create trustd TLSRoute")
		}

		// ParentRefs with port and sectionName for the steward-trustd listener
		if tcp.Spec.ControlPlane.Gateway.GatewayParentRefs == nil {
			return fmt.Errorf("control plane gateway parentRefs are not specified")
		}
		r.resource.Spec.ParentRefs = resources.NewParentRefsSpecWithPortAndSection(tcp.Spec.ControlPlane.Gateway.GatewayParentRefs, int32(port), "steward-trustd")

		rule := gatewayv1alpha2.TLSRouteRule{
			BackendRefs: []gatewayv1alpha2.BackendRef{
				{
					BackendObjectReference: gatewayv1alpha2.BackendObjectReference{
						Name: serviceName,
						Port: &port,
					},
				},
			},
		}

		// Use the CP hostname directly (NOT .trustd. derivation)
		hostname := tcp.Spec.ControlPlane.Gateway.Hostname
		r.resource.Spec.Hostnames = []gatewayv1.Hostname{hostname}
		r.resource.Spec.Rules = []gatewayv1alpha2.TLSRouteRule{rule}

		return controllerutil.SetControllerReference(tcp, r.resource, r.Client.Scheme())
	}
}

func (r *TalosGatewayResource) GetName() string {
	return "talos-trustd-gateway"
}

// Ensure interface satisfaction at compile time.
var _ resources.Resource = &TalosGatewayResource{}
var _ resources.Resource = &TalosCredentialsResource{}
var _ resources.Resource = &TalosDeploymentResource{}
var _ resources.Resource = &TalosServiceResource{}
var _ resources.Resource = &TalosTraefikIngressRouteTCPResource{}

// Suppress unused import warning for v1.
var _ = v1.LocalObjectReference{}
