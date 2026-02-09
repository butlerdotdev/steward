// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane resource with additional pod metadata", func() {
	// Fill TenantControlPlane object
	tcp := &stewardv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-clusterip-additional-metadata",
			Namespace: "default",
		},
		Spec: stewardv1alpha1.TenantControlPlaneSpec{
			ControlPlane: stewardv1alpha1.ControlPlane{
				Deployment: stewardv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
					PodAdditionalMetadata: stewardv1alpha1.AdditionalMetadata{
						Labels: map[string]string{
							"hello-label": "world",
							"foo-label":   "bar",
						},
						Annotations: map[string]string{
							"hello-ann": "world",
							"foo-ann":   "bar",
						},
					},
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

	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	// Check if TenantControlPlane resource has been created
	It("Should be Ready with expected additional metadata", func() {
		StatusMustEqualTo(tcp, stewardv1alpha1.VersionReady)
		AllPodsLabelMustEqualTo(tcp, "hello-label", "world")
		AllPodsLabelMustEqualTo(tcp, "foo-label", "bar")
		AllPodsAnnotationMustEqualTo(tcp, "hello-ann", "world")
		AllPodsAnnotationMustEqualTo(tcp, "foo-ann", "bar")
	})
})
