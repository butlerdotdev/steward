// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

//+kubebuilder:webhook:path=/mutate-steward-butlerlabs-dev-v1alpha1-tenantcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=steward.butlerlabs.dev,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=mtenantcontrolplane.kb.io,admissionReviewVersions=v1

type TenantControlPlaneDefaults struct{}

func (t TenantControlPlaneDefaults) GetObject() runtime.Object {
	return &stewardv1alpha1.TenantControlPlane{}
}

func (t TenantControlPlaneDefaults) GetPath() string {
	return "/mutate-steward-butlerlabs-dev-v1alpha1-tenantcontrolplane"
}
