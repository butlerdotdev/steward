// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	sooterrors "github.com/butlerdotdev/steward/controllers/soot/controllers/errors"
	"github.com/butlerdotdev/steward/controllers/utils"
)

const (
	endpointSliceReaderClusterRoleName        = "steward:node-endpointslice-reader"
	endpointSliceReaderClusterRoleBindingName = "steward:node-endpointslice-reader"
	kubeletServingAutoApproveBindingName      = "kubeadm:node-autoapprove-kubelet-serving"
)

// WorkerBootstrapRBAC creates RBAC resources in the tenant cluster that worker
// nodes need for proper operation: EndpointSlice read access and kubelet-serving
// CSR auto-approve binding for certificate renewal.
type WorkerBootstrapRBAC struct {
	Logger                    logr.Logger
	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
	ControllerName            string
	client                    client.Client
}

func (w *WorkerBootstrapRBAC) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := w.GetTenantControlPlaneFunc()
	if err != nil {
		if errors.Is(err, sooterrors.ErrPausedReconciliation) {
			w.Logger.Info(err.Error())

			return reconcile.Result{}, nil
		}
		w.Logger.Error(err, "cannot retrieve TenantControlPlane")

		return reconcile.Result{}, err
	}

	if tcp.Spec.Addons.WorkerBootstrap == nil {
		return reconcile.Result{}, nil
	}

	w.Logger.Info("start processing")

	if err := w.ensureEndpointSliceReaderRole(ctx); err != nil {
		w.Logger.Error(err, "failed to ensure EndpointSlice reader ClusterRole")

		return reconcile.Result{}, err
	}

	if err := w.ensureEndpointSliceReaderBinding(ctx); err != nil {
		w.Logger.Error(err, "failed to ensure EndpointSlice reader ClusterRoleBinding")

		return reconcile.Result{}, err
	}

	if err := w.ensureKubeletServingAutoApproveBinding(ctx); err != nil {
		w.Logger.Error(err, "failed to ensure kubelet-serving auto-approve ClusterRoleBinding")

		return reconcile.Result{}, err
	}

	w.Logger.Info("reconciliation completed")

	return reconcile.Result{}, nil
}

func (w *WorkerBootstrapRBAC) ensureEndpointSliceReaderRole(ctx context.Context) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: endpointSliceReaderClusterRoleName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, w.client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}

		return nil
	})

	return err
}

func (w *WorkerBootstrapRBAC) ensureEndpointSliceReaderBinding(ctx context.Context) error {
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: endpointSliceReaderClusterRoleBindingName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, w.client, binding, func() error {
		binding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     endpointSliceReaderClusterRoleName,
		}
		binding.Subjects = []rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: "system:nodes",
			},
		}

		return nil
	})

	return err
}

func (w *WorkerBootstrapRBAC) ensureKubeletServingAutoApproveBinding(ctx context.Context) error {
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeletServingAutoApproveBindingName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, w.client, binding, func() error {
		binding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
		}
		binding.Subjects = []rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: "system:nodes",
			},
		}

		return nil
	})

	return err
}

func (w *WorkerBootstrapRBAC) SetupWithManager(mgr manager.Manager) error {
	w.client = mgr.GetClient()
	w.Logger = mgr.GetLogger().WithName("workerbootstrap_rbac")

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(w.ControllerName).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: ptr.To(true)}).
		For(&rbacv1.ClusterRoleBinding{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			name := object.GetName()

			return name == endpointSliceReaderClusterRoleBindingName || name == kubeletServingAutoApproveBindingName
		}))).
		WatchesRawSource(source.Channel(w.TriggerChannel, &handler.EnqueueRequestForObject{})).
		Complete(w)
}
