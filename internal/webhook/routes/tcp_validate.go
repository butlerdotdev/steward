// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

//+kubebuilder:webhook:path=/validate-steward-butlerlabs-dev-v1alpha1-tenantcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=steward.butlerlabs.dev,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=vtenantcontrolplane.kb.io,admissionReviewVersions=v1

type TenantControlPlaneValidate struct{}

func (t TenantControlPlaneValidate) GetPath() string {
	return "/validate-steward-butlerlabs-dev-v1alpha1-tenantcontrolplane"
}

func (t TenantControlPlaneValidate) GetObject() runtime.Object {
	return &stewardv1alpha1.TenantControlPlane{}
}
