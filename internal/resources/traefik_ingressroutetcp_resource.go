// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/utilities"
)

var ingressRouteTCPGVK = schema.GroupVersionKind{
	Group:   "traefik.io",
	Version: "v1alpha1",
	Kind:    "IngressRouteTCP",
}

// TraefikIngressRouteTCPResource manages Traefik IngressRouteTCP resources for TLS passthrough.
// This is used when controllerType=traefik since standard Kubernetes Ingress does not support
// TLS passthrough with Traefik.
type TraefikIngressRouteTCPResource struct {
	resource *unstructured.Unstructured
	Client   client.Client
}

func (r *TraefikIngressRouteTCPResource) GetHistogram() prometheus.Histogram {
	// Reuse the ingress collector since this is functionally similar
	ingressCollector = LazyLoadHistogramFromResource(ingressCollector, r)

	return ingressCollector
}

func (r *TraefikIngressRouteTCPResource) ShouldStatusBeUpdated(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) bool {
	// IngressRouteTCP doesn't have LoadBalancer status like standard Ingress
	// Status updates are handled by the Traefik controller
	return false
}

func (r *TraefikIngressRouteTCPResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	// Cleanup if ingress is not specified or if controllerType is not traefik
	if tcp.Spec.ControlPlane.Ingress == nil {
		return true
	}

	return tcp.Spec.ControlPlane.Ingress.ControllerType != "traefik"
}

func (r *TraefikIngressRouteTCPResource) CleanUp(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(ingressRouteTCPGVK)

	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: tcp.GetNamespace(),
		Name:      tcp.GetName(),
	}, existing); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to get IngressRouteTCP resource before cleanup")

			return false, err
		}

		return false, nil
	}

	// Check if this resource is owned by the TCP
	ownerRefs := existing.GetOwnerReferences()
	isOwned := false
	for _, ref := range ownerRefs {
		if ref.UID == tcp.GetUID() {
			isOwned = true

			break
		}
	}

	if !isOwned {
		logger.Info("skipping cleanup: IngressRouteTCP is not managed by Steward", "name", existing.GetName(), "namespace", existing.GetNamespace())

		return false, nil
	}

	if err := r.Client.Delete(ctx, existing); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot cleanup IngressRouteTCP resource")

			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *TraefikIngressRouteTCPResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) error {
	// IngressRouteTCP status is managed by Traefik, we don't track it in TCP status
	return nil
}

func (r *TraefikIngressRouteTCPResource) Define(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &unstructured.Unstructured{}
	r.resource.SetGroupVersionKind(ingressRouteTCPGVK)
	r.resource.SetName(tenantControlPlane.GetName())
	r.resource.SetNamespace(tenantControlPlane.GetNamespace())

	return nil
}

func (r *TraefikIngressRouteTCPResource) mutate(tenantControlPlane *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if tenantControlPlane.Spec.ControlPlane.Ingress == nil {
			return fmt.Errorf("ingress spec is nil")
		}

		hostname := tenantControlPlane.Spec.ControlPlane.Ingress.Hostname
		if hostname == "" {
			return fmt.Errorf("missing hostname for IngressRouteTCP")
		}

		if tenantControlPlane.Status.Kubernetes.Service.Name == "" ||
			tenantControlPlane.Status.Kubernetes.Service.Port == 0 {
			return fmt.Errorf("IngressRouteTCP cannot be configured yet: service not ready")
		}

		serviceName := tenantControlPlane.Status.Kubernetes.Service.Name
		servicePort := tenantControlPlane.Status.Kubernetes.Service.Port

		// Get the host part without port
		host, _ := utilities.GetControlPlaneAddressAndPortFromHostname(hostname, 0)

		// Generate konnectivity hostname by replacing "k8s" with "konnectivity"
		konnectivityHost := strings.Replace(host, ".k8s.", ".konnectivity.", 1)

		// Set labels
		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tenantControlPlane.GetName(), r.GetName()),
			tenantControlPlane.Spec.ControlPlane.Ingress.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		// Set annotations
		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tenantControlPlane.Spec.ControlPlane.Ingress.AdditionalMetadata.Annotations,
		)
		r.resource.SetAnnotations(annotations)

		// Build routes list - API server route
		// Include multiple SNI patterns to support:
		// 1. External access with proper hostname
		// 2. In-cluster access via kubernetes.default.svc (tcp-proxy forwards these)
		// The tcp-proxy intercepts connections to kubernetes.default.svc and forwards
		// them to this ingress using the proper external hostname as SNI.
		// NOTE: Do NOT use HostSNI(`*`) - it breaks multi-tenant routing on shared Traefik.
		apiServerMatch := fmt.Sprintf(
			"HostSNI(`%s`) || HostSNI(`kubernetes`) || HostSNI(`kubernetes.default`) || HostSNI(`kubernetes.default.svc`) || HostSNI(`kubernetes.default.svc.cluster.local`)",
			host,
		)
		routes := []interface{}{
			map[string]interface{}{
				"match": apiServerMatch,
				"services": []interface{}{
					map[string]interface{}{
						"name": serviceName,
						"port": int64(servicePort),
					},
				},
			},
		}

		// Add konnectivity route if konnectivity addon is enabled
		if tenantControlPlane.Spec.Addons.Konnectivity != nil {
			konnectivityPort := tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityServerSpec.Port
			routes = append(routes, map[string]interface{}{
				"match": fmt.Sprintf("HostSNI(`%s`)", konnectivityHost),
				"services": []interface{}{
					map[string]interface{}{
						"name": serviceName,
						"port": int64(konnectivityPort),
					},
				},
			})
		}

		// Build the IngressRouteTCP spec
		// See: https://doc.traefik.io/traefik/routing/providers/kubernetes-crd/#kind-ingressroutetcp
		spec := map[string]interface{}{
			"entryPoints": []interface{}{"websecure"},
			"routes":      routes,
			"tls": map[string]interface{}{
				"passthrough": true,
			},
		}

		if err := unstructured.SetNestedMap(r.resource.Object, spec, "spec"); err != nil {
			return fmt.Errorf("failed to set IngressRouteTCP spec: %w", err)
		}

		// Set owner reference using known GVK for TenantControlPlane
		// (APIVersion and Kind fields may be empty on retrieved objects)
		ownerRef := metav1.OwnerReference{
			APIVersion:         stewardv1alpha1.GroupVersion.String(),
			Kind:               "TenantControlPlane",
			Name:               tenantControlPlane.Name,
			UID:                tenantControlPlane.UID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		}
		r.resource.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

		return nil
	}
}

func (r *TraefikIngressRouteTCPResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tenantControlPlane))
}

func (r *TraefikIngressRouteTCPResource) GetName() string {
	return "traefik-ingressroutetcp"
}
