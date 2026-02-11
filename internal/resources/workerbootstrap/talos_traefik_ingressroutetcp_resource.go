// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package workerbootstrap

import (
	"context"
	"fmt"

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
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/utilities"
)

var trustdIngressRouteTCPGVK = schema.GroupVersionKind{
	Group:   "traefik.io",
	Version: "v1alpha1",
	Kind:    "IngressRouteTCP",
}

// TalosTraefikIngressRouteTCPResource manages a Traefik IngressRouteTCP for trustd
// on the "trustd" entryPoint (port 50001) with HostSNI routing.
type TalosTraefikIngressRouteTCPResource struct {
	resource *unstructured.Unstructured
	Client   client.Client
}

func (r *TalosTraefikIngressRouteTCPResource) GetHistogram() prometheus.Histogram {
	traefikCollector = resources.LazyLoadHistogramFromResource(traefikCollector, r)
	return traefikCollector
}

func (r *TalosTraefikIngressRouteTCPResource) ShouldStatusBeUpdated(_ context.Context, _ *stewardv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *TalosTraefikIngressRouteTCPResource) ShouldCleanup(tcp *stewardv1alpha1.TenantControlPlane) bool {
	if !shouldHaveWorkerBootstrap(tcp) {
		return true
	}
	if tcp.Spec.ControlPlane.Ingress == nil {
		return true
	}
	return tcp.Spec.ControlPlane.Ingress.ControllerType != "traefik"
}

func (r *TalosTraefikIngressRouteTCPResource) CleanUp(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(trustdIngressRouteTCPGVK)

	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: tcp.GetNamespace(),
		Name:      fmt.Sprintf("%s-trustd", tcp.GetName()),
	}, existing); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to get trustd IngressRouteTCP before cleanup")
			return false, err
		}
		return false, nil
	}

	// Check ownership
	ownerRefs := existing.GetOwnerReferences()
	isOwned := false
	for _, ref := range ownerRefs {
		if ref.UID == tcp.GetUID() {
			isOwned = true
			break
		}
	}
	if !isOwned {
		return false, nil
	}

	if err := r.Client.Delete(ctx, existing); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot cleanup trustd IngressRouteTCP")
			return false, err
		}
		return false, nil
	}

	logger.V(1).Info("trustd IngressRouteTCP cleaned up")
	return true, nil
}

func (r *TalosTraefikIngressRouteTCPResource) UpdateTenantControlPlaneStatus(_ context.Context, _ *stewardv1alpha1.TenantControlPlane) error {
	return nil
}

func (r *TalosTraefikIngressRouteTCPResource) Define(_ context.Context, tcp *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &unstructured.Unstructured{}
	r.resource.SetGroupVersionKind(trustdIngressRouteTCPGVK)
	r.resource.SetName(fmt.Sprintf("%s-trustd", tcp.GetName()))
	r.resource.SetNamespace(tcp.GetNamespace())
	return nil
}

func (r *TalosTraefikIngressRouteTCPResource) CreateOrUpdate(ctx context.Context, tcp *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tcp))
}

func (r *TalosTraefikIngressRouteTCPResource) mutate(tcp *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		if tcp.Spec.ControlPlane.Ingress == nil {
			return fmt.Errorf("ingress spec is nil")
		}

		hostname := tcp.Spec.ControlPlane.Ingress.Hostname
		if hostname == "" {
			return fmt.Errorf("missing hostname for trustd IngressRouteTCP")
		}

		port := tcp.Spec.Addons.WorkerBootstrap.Talos.Port
		serviceName := tcp.Status.Kubernetes.Service.Name

		if serviceName == "" {
			return fmt.Errorf("service not ready, cannot create trustd IngressRouteTCP")
		}

		host, _ := utilities.GetControlPlaneAddressAndPortFromHostname(hostname, 0)

		// Set labels
		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.StewardLabels(tcp.GetName(), r.GetName()),
			tcp.Spec.ControlPlane.Ingress.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		// Set annotations
		annotations := utilities.MergeMaps(
			r.resource.GetAnnotations(),
			tcp.Spec.ControlPlane.Ingress.AdditionalMetadata.Annotations,
		)
		r.resource.SetAnnotations(annotations)

		// Single route: HostSNI(<cp-hostname>) → TCP Service on trustd port
		routes := []interface{}{
			map[string]interface{}{
				"match": fmt.Sprintf("HostSNI(`%s`)", host),
				"services": []interface{}{
					map[string]interface{}{
						"name": serviceName,
						"port": int64(port),
					},
				},
			},
		}

		spec := map[string]interface{}{
			"entryPoints": []interface{}{"trustd"},
			"routes":      routes,
			"tls": map[string]interface{}{
				"passthrough": true,
			},
		}

		if err := unstructured.SetNestedMap(r.resource.Object, spec, "spec"); err != nil {
			return fmt.Errorf("failed to set trustd IngressRouteTCP spec: %w", err)
		}

		// Set owner reference
		ownerRef := metav1.OwnerReference{
			APIVersion:         stewardv1alpha1.GroupVersion.String(),
			Kind:               "TenantControlPlane",
			Name:               tcp.Name,
			UID:                tcp.UID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		}
		r.resource.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

		return nil
	}
}

func (r *TalosTraefikIngressRouteTCPResource) GetName() string {
	return "talos-trustd-traefik-ingressroutetcp"
}
