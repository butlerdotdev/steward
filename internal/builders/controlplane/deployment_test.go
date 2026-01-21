// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

var _ = Describe("Controlplane Deployment", func() {
	var d Deployment
	BeforeEach(func() {
		d = Deployment{
			DataStoreOverrides: []DataStoreOverrides{{
				Resource: "/events",
				DataStore: stewardv1alpha1.DataStore{
					Spec: stewardv1alpha1.DataStoreSpec{
						Endpoints: stewardv1alpha1.Endpoints{"etcd-0", "etcd-1", "etcd-2"},
					},
				},
			}},
		}
	})

	Describe("DataStoreOverrides flag generation", func() {
		It("should generate valid --etcd-servers-overrides value", func() {
			etcdSerVersOverrides := d.etcdServersOverrides()
			Expect(etcdSerVersOverrides).To(Equal("/events#https://etcd-0;https://etcd-1;https://etcd-2"))
		})
		It("should generate valid --etcd-servers-overrides value with 2 DataStoreOverrides", func() {
			d.DataStoreOverrides = append(d.DataStoreOverrides, DataStoreOverrides{
				Resource: "/pods",
				DataStore: stewardv1alpha1.DataStore{
					Spec: stewardv1alpha1.DataStoreSpec{
						Endpoints: stewardv1alpha1.Endpoints{"etcd-3", "etcd-4", "etcd-5"},
					},
				},
			})
			etcdSerVersOverrides := d.etcdServersOverrides()
			Expect(etcdSerVersOverrides).To(Equal("/events#https://etcd-0;https://etcd-1;https://etcd-2,/pods#https://etcd-3;https://etcd-4;https://etcd-5"))
		})
	})
})
