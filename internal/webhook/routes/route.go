// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type Route interface {
	GetPath() string
	GetObject() runtime.Object
}
