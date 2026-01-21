# Steward CRDs Helm Chart

This chart installs the Steward Custom Resource Definitions.

## Installation

Add the Butler Labs Helm repository:
```bash
helm repo add butlerlabs https://butlerdotdev.github.io/charts
helm repo update
```

Install the Chart:
```bash
helm upgrade --install --namespace steward-system --create-namespace steward-crds butlerlabs/steward-crds
```

## Upgrade
```bash
helm upgrade steward-crds -n steward-system butlerlabs/steward-crds
```

## Uninstall
```bash
helm uninstall steward-crds -n steward-system
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| fullnameOverride | string | `""` | Overrides the full name of the resources created by the chart. |
| nameOverride | string | `""` | Overrides the name of the chart for resource naming purposes. |
| stewardCertificateName | string | `"steward-serving-cert"` | The cert-manager Certificate resource name, holding the Certificate Authority for webhooks. |
| stewardNamespace | string | `"steward-system"` | The namespace where Steward has been installed: required to inject the Certificate Authority for cert-manager. |
| stewardService | string | `"steward-webhook-service"` | The Steward webhook Service name. |
