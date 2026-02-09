// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"strconv"
	"strings"
)

func GetControlPlaneAddressAndPortFromHostname(hostname string, defaultPort int32) (address string, port int32) {
	parts := strings.Split(hostname, ":")

	address, port = parts[0], defaultPort

	if len(parts) == 2 {
		intPort, _ := strconv.Atoi(parts[1])

		if intPort > 0 {
			port = int32(intPort)
		}
	}

	return address, port
}

// ExtractHost extracts the host part from a host:port string.
func ExtractHost(hostPort string) string {
	parts := strings.Split(hostPort, ":")

	return parts[0]
}

// ExtractPort extracts the port part from a host:port string.
// Returns "6443" as default if no port is specified.
func ExtractPort(hostPort string) string {
	parts := strings.Split(hostPort, ":")
	if len(parts) == 2 {
		return parts[1]
	}

	return "6443"
}
