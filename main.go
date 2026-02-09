// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/butlerdotdev/steward/cmd"
	kubeconfig_generator "github.com/butlerdotdev/steward/cmd/kubeconfig-generator"
	"github.com/butlerdotdev/steward/cmd/manager"
	"github.com/butlerdotdev/steward/cmd/migrate"
)

func main() {
	scheme := runtime.NewScheme()

	root, mgr, migrator, kubeconfigGenerator := cmd.NewCmd(scheme), manager.NewCmd(scheme), migrate.NewCmd(scheme), kubeconfig_generator.NewCmd(scheme)
	root.AddCommand(mgr)
	root.AddCommand(migrator)
	root.AddCommand(kubeconfigGenerator)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
