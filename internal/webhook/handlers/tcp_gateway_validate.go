// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/utilities"
	"github.com/butlerdotdev/steward/internal/webhook/utils"
)

type TenantControlPlaneGatewayValidation struct {
	Client          client.Client
	DiscoveryClient discovery.DiscoveryInterface
}

func (t TenantControlPlaneGatewayValidation) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*stewardv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		if tcp.Spec.ControlPlane.Gateway != nil {
			// NOTE: Do we actually want to deny here if Gateway API is not available or a warning?
			// Seems sensible to deny to avoid anything.
			if err := t.validateGatewayAPIAvailability(ctx); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}
}

func (t TenantControlPlaneGatewayValidation) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*stewardv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		if tcp.Spec.ControlPlane.Gateway != nil {
			if err := t.validateGatewayAPIAvailability(ctx); err != nil {
				return nil, err
			}
		}

		return nil, nil
	}
}

func (t TenantControlPlaneGatewayValidation) OnDelete(object runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneGatewayValidation) validateGatewayAPIAvailability(ctx context.Context) error {
	if !utilities.AreGatewayResourcesAvailable(ctx, t.Client, t.DiscoveryClient) {
		return fmt.Errorf("the Gateway API is not available in this cluster, cannot use gatewayRoute configuration")
	}

	// Additional check for TLSRoute specifically
	if !utilities.IsTLSRouteAvailable(ctx, t.Client, t.DiscoveryClient) {
		return fmt.Errorf("TLSRoute resource is not available in this cluster")
	}

	return nil
}
