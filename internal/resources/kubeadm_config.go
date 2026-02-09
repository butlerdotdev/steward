// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"net"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/kubeadm"
	"github.com/butlerdotdev/steward/internal/utilities"
)

type KubeadmConfigResource struct {
	resource     *corev1.ConfigMap
	Client       client.Client
	ETCDs        []string
	TmpDirectory string
}

func (r *KubeadmConfigResource) GetHistogram() prometheus.Histogram {
	kubeadmconfigCollector = LazyLoadHistogramFromResource(kubeadmconfigCollector, r)

	return kubeadmconfigCollector
}

func (r *KubeadmConfigResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.KubeadmConfig.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *KubeadmConfigResource) ShouldCleanup(*stewardv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeadmConfigResource) CleanUp(context.Context, *stewardv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubeadmConfigResource) Define(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubeadmConfigResource) getPrefixedName(tenantControlPlane *stewardv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *KubeadmConfigResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubeadmConfigResource) GetName() string {
	return "kubeadmconfig"
}

func (r *KubeadmConfigResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.KubeadmConfig.LastUpdate = metav1.Now()
	tenantControlPlane.Status.KubeadmConfig.Checksum = utilities.GetObjectChecksum(r.resource)
	tenantControlPlane.Status.KubeadmConfig.ConfigmapName = r.resource.GetName()

	return nil
}

func (r *KubeadmConfigResource) mutate(ctx context.Context, tenantControlPlane *stewardv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		// Use DeclaredControlPlaneAddress for kubeadm config - it needs an IP, not hostname
		// For Ingress/Gateway modes, Status.ControlPlaneEndpoint contains the hostname,
		// but kubeadm requires an internal IP address for advertiseAddress
		address, err := tenantControlPlane.DeclaredControlPlaneAddress(ctx, r.Client)
		if err != nil {
			logger.Error(err, "cannot retrieve Tenant Control Plane address")

			return err
		}
		port := tenantControlPlane.Spec.NetworkProfile.Port

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.StewardLabels(tenantControlPlane.GetName(), r.GetName())))

		endpoint := net.JoinHostPort(address, strconv.FormatInt(int64(port), 10))
		spec := tenantControlPlane.Spec.ControlPlane
		if spec.Gateway != nil {
			if len(spec.Gateway.Hostname) > 0 {
				// For Gateway mode, default to port 443 (standard HTTPS port)
				// since Gateway/Ingress controllers expose on 443, not 6443
				gaddr, gport := utilities.GetControlPlaneAddressAndPortFromHostname(string(spec.Gateway.Hostname), 443)
				endpoint = net.JoinHostPort(gaddr, strconv.FormatInt(int64(gport), 10))
			}
		}
		if spec.Ingress != nil {
			if len(spec.Ingress.Hostname) > 0 {
				// For Ingress mode, default to port 443 (standard HTTPS port)
				// since Ingress controllers expose on 443, not 6443
				iaddr, iport := utilities.GetControlPlaneAddressAndPortFromHostname(spec.Ingress.Hostname, 443)
				endpoint = net.JoinHostPort(iaddr, strconv.FormatInt(int64(iport), 10))
			}
		}

		params := kubeadm.Parameters{
			TenantControlPlaneAddress:       address,
			TenantControlPlanePort:          port,
			TenantControlPlaneName:          tenantControlPlane.GetName(),
			TenantControlPlaneNamespace:     tenantControlPlane.GetNamespace(),
			TenantControlPlaneEndpoint:      endpoint,
			TenantControlPlaneCertSANs:      tenantControlPlane.Spec.NetworkProfile.CertSANs,
			TenantControlPlaneClusterDomain: tenantControlPlane.Spec.NetworkProfile.ClusterDomain,
			TenantControlPlanePodCIDR:       tenantControlPlane.Spec.NetworkProfile.PodCIDR,
			TenantControlPlaneServiceCIDR:   tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
			TenantControlPlaneVersion:       tenantControlPlane.Spec.Kubernetes.Version,
			ETCDs:                           r.ETCDs,
			CertificatesDir:                 r.TmpDirectory,
		}

		config, err := kubeadm.CreateKubeadmInitConfiguration(params)
		if err != nil {
			return err
		}
		if r.resource.Data, err = kubeadm.GetKubeadmInitConfigurationMap(*config); err != nil {
			logger.Error(err, "cannot retrieve kubeadm init configuration")

			return err
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
