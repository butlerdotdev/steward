// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"net"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/webhook/utils"
)

type TenantControlPlaneWorkerBootstrapValidation struct{}

func (t TenantControlPlaneWorkerBootstrapValidation) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*stewardv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		return nil, validateWorkerBootstrap(tcp)
	}
}

func (t TenantControlPlaneWorkerBootstrapValidation) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, ok := object.(*stewardv1alpha1.TenantControlPlane)
		if !ok {
			return nil, fmt.Errorf("cannot cast object to TenantControlPlane")
		}

		return nil, validateWorkerBootstrap(tcp)
	}
}

func (t TenantControlPlaneWorkerBootstrapValidation) OnDelete(object runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func validateWorkerBootstrap(tcp *stewardv1alpha1.TenantControlPlane) error {
	wb := tcp.Spec.Addons.WorkerBootstrap
	if wb == nil {
		return nil
	}

	switch wb.Provider {
	case stewardv1alpha1.TalosProvider:
		if wb.Talos == nil {
			return fmt.Errorf("talos configuration is required when provider is %q", stewardv1alpha1.TalosProvider)
		}
		if wb.Talos.Port != 0 && (wb.Talos.Port < 1 || wb.Talos.Port > 65535) {
			return fmt.Errorf("talos port must be between 1 and 65535, got %d", wb.Talos.Port)
		}
	default:
		return fmt.Errorf("unsupported worker bootstrap provider: %q", wb.Provider)
	}

	for _, cidr := range wb.AllowedSubnets {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid CIDR in allowedSubnets: %q: %w", cidr, err)
		}
	}

	return nil
}
