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

## Maintainers

| Name | Email | Url |
|------|-------|-----|
| Andrew Bagan | <andrew@butlerlabs.dev> | <https://butlerlabs.dev> |
