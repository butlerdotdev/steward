// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/webhook/utils"
)

type TenantControlPlaneName struct{}

func (t TenantControlPlaneName) OnCreate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*stewardv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if errs := validation.IsDNS1035Label(tcp.Name); len(errs) > 0 {
			return nil, fmt.Errorf("the provided name is invalid, %s", strings.Join(errs, ","))
		}

		return nil, nil
	}
}

func (t TenantControlPlaneName) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneName) OnUpdate(runtime.Object, runtime.Object) AdmissionResponse {
	return utils.NilOp()
}
