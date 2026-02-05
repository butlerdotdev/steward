// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package tcpproxy

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	deploymentCollector         prometheus.Histogram
	serviceCollector            prometheus.Histogram
	serviceAccountCollector     prometheus.Histogram
	clusterRoleCollector        prometheus.Histogram
	clusterRoleBindingCollector prometheus.Histogram
)
