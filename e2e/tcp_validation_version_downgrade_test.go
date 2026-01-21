// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

var _ = Describe("downgrade of a TenantControlPlane Kubernetes version", func() {
	// Fill TenantControlPlane object
	tcp := stewardv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "downgrade",
			Namespace: "default",
		},
		Spec: stewardv1alpha1.TenantControlPlaneSpec{
			ControlPlane: stewardv1alpha1.ControlPlane{
				Deployment: stewardv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
				},
				Service: stewardv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
				},
			},
			Kubernetes: stewardv1alpha1.KubernetesSpec{
				Version: "v1.23.0",
				Kubelet: stewardv1alpha1.KubeletSpec{
					CGroupFS: "cgroupfs",
				},
			},
		},
	}
	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), &tcp)).NotTo(HaveOccurred())
	})
	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), &tcp)).Should(Succeed())
	})

	It("should be blocked", func() {
		Consistently(func() error {
			tcp := tcp.DeepCopy()

			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, tcp)
			if err != nil {
				return nil
			}

			tcp.Spec.Kubernetes.Version = "v1.22.0"

			return k8sClient.Update(context.Background(), tcp)
		}, 10*time.Second, time.Second).ShouldNot(Succeed())
	})
})
