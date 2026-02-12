// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	credentialsCollector prometheus.Histogram
	deploymentCollector  prometheus.Histogram
	serviceCollector     prometheus.Histogram
	traefikCollector     prometheus.Histogram
	gatewayCollector     prometheus.Histogram
)
