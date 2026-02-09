// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

//+kubebuilder:webhook:path=/validate-steward-butlerlabs-dev-v1alpha1-datastore,mutating=false,failurePolicy=fail,sideEffects=None,groups=steward.butlerlabs.dev,resources=datastores,verbs=create;update;delete,versions=v1alpha1,name=vdatastore.kb.io,admissionReviewVersions=v1

type DataStoreValidate struct{}

func (d DataStoreValidate) GetPath() string {
	return "/validate-steward-butlerlabs-dev-v1alpha1-datastore"
}

func (d DataStoreValidate) GetObject() runtime.Object {
	return &stewardv1alpha1.DataStore{}
}
