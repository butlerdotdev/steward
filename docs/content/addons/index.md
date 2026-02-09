# Addons

Steward provides optional addons that extend the functionality of Tenant Control Planes.

## What is a Steward Addon?

A Steward Addon is a component that Steward can automatically deploy and manage either in the management cluster or within tenant clusters. Addons provide additional functionality beyond the core hosted control plane.

## Available Addons

### Core Addons (Built-in)

These addons are part of Steward and configured directly in the TenantControlPlane spec:

- **[Ingress Mode](ingress.md)**: Expose multiple Tenant Control Planes behind a single Ingress Controller IP, eliminating the need for individual LoadBalancer services per tenant
- **Konnectivity**: Secure tunnel for API server to node communication when nodes are in separate networks
- **CoreDNS**: Automatic CoreDNS deployment in tenant clusters

### Enabling Addons

Addons are enabled through the `TenantControlPlane` spec. For example, to enable Ingress mode with tcp-proxy:

```yaml
apiVersion: steward.butlerlabs.dev/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-tenant
  namespace: tenants
spec:
  controlPlane:
    service:
      serviceType: ClusterIP
    ingress:
      hostname: my-tenant.k8s.example.com
      ingressClassName: traefik
      controllerType: traefik
  addons:
    tcpProxy: {}
```

See the individual addon documentation for detailed configuration options.
