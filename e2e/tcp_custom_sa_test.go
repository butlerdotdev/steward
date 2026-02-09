// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with resource with custom service account", func() {
	// service account object
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	// Fill TenantControlPlane object
	tcp := &stewardv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-clusterip-customsa",
			Namespace: "default",
		},
		Spec: stewardv1alpha1.TenantControlPlaneSpec{
			ControlPlane: stewardv1alpha1.ControlPlane{
				Deployment: stewardv1alpha1.DeploymentSpec{
					Replicas:           pointer.To(int32(1)),
					ServiceAccountName: sa.GetName(),
				},
				Service: stewardv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
				},
			},
			NetworkProfile: stewardv1alpha1.NetworkProfileSpec{
				Address: "172.18.0.2",
			},
			Kubernetes: stewardv1alpha1.KubernetesSpec{
				Version: "v1.23.6",
				Kubelet: stewardv1alpha1.KubeletSpec{
					CGroupFS: "cgroupfs",
				},
				AdmissionControllers: stewardv1alpha1.AdmissionControllers{
					"LimitRanger",
					"ResourceQuota",
				},
			},
			Addons: stewardv1alpha1.AddonsSpec{},
		},
	}

	// Create service account and TenantControlPlane resources into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), sa)).NotTo(HaveOccurred())
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	// Delete the service account and TenantControlPlane resources after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), sa)).NotTo(HaveOccurred())
	})
	// Check if TenantControlPlane resource has been created and if its pods have the right service account
	It("Should be Ready and have correct sa", func() {
		StatusMustEqualTo(tcp, stewardv1alpha1.VersionReady)
		PodsServiceAccountMustEqualTo(tcp, sa)
	})
})
