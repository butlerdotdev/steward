# Steward Helm Chart

Steward is the Hosted Control Plane Manager for Kubernetes.

## Installation

Add the Butler Labs Helm repository:
```bash
helm repo add butlerlabs https://butlerdotdev.github.io/charts
helm repo update
```

Install the Chart:
```bash
helm upgrade --install --namespace steward-system --create-namespace steward butlerlabs/steward
```

## Maintainers

| Name | Email | Url |
|------|-------|-----|
| Andrew Bagan | <andrew@butlerlabs.dev> | <https://butlerlabs.dev> |
