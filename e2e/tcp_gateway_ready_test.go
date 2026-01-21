// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane with Gateway API", func() {
	var tcp *stewardv1alpha1.TenantControlPlane

	JustBeforeEach(func() {
		tcp = &stewardv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp-gateway",
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
					Gateway: &stewardv1alpha1.GatewaySpec{
						Hostname: gatewayv1.Hostname("tcp-gateway.example.com"),
						AdditionalMetadata: stewardv1alpha1.AdditionalMetadata{
							Labels: map[string]string{
								"test.steward.io/gateway": "true",
							},
							Annotations: map[string]string{
								"test.steward.io/created-by": "e2e-test",
							},
						},
						GatewayParentRefs: []gatewayv1.ParentReference{
							{
								Name: "test-gateway",
							},
						},
					},
				},
				NetworkProfile: stewardv1alpha1.NetworkProfileSpec{
					Address: "172.18.0.3",
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
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())

		// Wait for the object to be completely deleted
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, &stewardv1alpha1.TenantControlPlane{})

			return err != nil // Returns true when object is not found (deleted)
		}).WithTimeout(time.Minute).Should(BeTrue())
	})

	It("Should be Ready", func() {
		StatusMustEqualTo(tcp, stewardv1alpha1.VersionReady)
	})

	It("Should create control plane TLSRoute with correct sectionName", func() {
		Eventually(func() error {
			route := &gatewayv1alpha2.TLSRoute{}
			// TODO: Check ownership.
			if err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name,
				Namespace: tcp.Namespace,
			}, route); err != nil {
				return err
			}
			if len(route.Spec.ParentRefs) == 0 {
				return fmt.Errorf("parentRefs is empty")
			}
			if route.Spec.ParentRefs[0].SectionName == nil {
				return fmt.Errorf("sectionName is nil")
			}
			if *route.Spec.ParentRefs[0].SectionName != gatewayv1.SectionName("kube-apiserver") {
				return fmt.Errorf("expected sectionName 'kube-apiserver', got '%s'", *route.Spec.ParentRefs[0].SectionName)
			}

			return nil
		}).WithTimeout(time.Minute).Should(Succeed())
	})

	It("Should not create Konnectivity TLSRoute", func() {
		// Verify Konnectivity route is not created
		Consistently(func() error {
			route := &gatewayv1alpha2.TLSRoute{}

			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.Name + "-konnectivity",
				Namespace: tcp.Namespace,
			}, route)
		}, 10*time.Second, time.Second).Should(HaveOccurred())
	})
})
