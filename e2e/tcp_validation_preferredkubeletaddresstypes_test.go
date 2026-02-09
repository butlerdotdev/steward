// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with wrong preferred kubelet address type entries", func() {
	It("should fail when using duplicates", func() {
		Consistently(func() error {
			tcp := &stewardv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicated-kubelet-preferred-address-type",
					Namespace: "default",
				},
				Spec: stewardv1alpha1.TenantControlPlaneSpec{
					DataStore: "default",
					ControlPlane: stewardv1alpha1.ControlPlane{
						Deployment: stewardv1alpha1.DeploymentSpec{
							Replicas: pointer.To(int32(1)),
						},
						Service: stewardv1alpha1.ServiceSpec{
							ServiceType: "ClusterIP",
						},
					},
					Kubernetes: stewardv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: stewardv1alpha1.KubeletSpec{
							PreferredAddressTypes: []stewardv1alpha1.KubeletPreferredAddressType{
								stewardv1alpha1.NodeHostName,
								stewardv1alpha1.NodeInternalIP,
								stewardv1alpha1.NodeExternalIP,
								stewardv1alpha1.NodeHostName,
							},
							CGroupFS: "cgroupfs",
						},
					},
				},
			}

			return k8sClient.Create(context.Background(), tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})

	It("should fail when using non valid entries", func() {
		Consistently(func() error {
			tcp := &stewardv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicated-kubelet-preferred-address-type",
					Namespace: "default",
				},
				Spec: stewardv1alpha1.TenantControlPlaneSpec{
					DataStore: "default",
					ControlPlane: stewardv1alpha1.ControlPlane{
						Deployment: stewardv1alpha1.DeploymentSpec{
							Replicas: pointer.To(int32(1)),
						},
						Service: stewardv1alpha1.ServiceSpec{
							ServiceType: "ClusterIP",
						},
					},
					Kubernetes: stewardv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: stewardv1alpha1.KubeletSpec{
							PreferredAddressTypes: []stewardv1alpha1.KubeletPreferredAddressType{
								stewardv1alpha1.NodeHostName,
								stewardv1alpha1.NodeInternalIP,
								stewardv1alpha1.NodeExternalIP,
								"Foo",
							},
							CGroupFS: "cgroupfs",
						},
					},
				},
			}

			return k8sClient.Create(context.Background(), tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})
})
