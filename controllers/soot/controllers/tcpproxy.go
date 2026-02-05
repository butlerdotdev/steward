// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
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

	"github.com/butlerdotdev/steward/controllers"
	sooterrors "github.com/butlerdotdev/steward/controllers/soot/controllers/errors"
	"github.com/butlerdotdev/steward/controllers/utils"
	"github.com/butlerdotdev/steward/internal/resources"
	"github.com/butlerdotdev/steward/internal/resources/tcpproxy"
)

// TCPProxy is the soot controller responsible for deploying and managing
// the tcp-proxy addon inside tenant clusters. It follows the same pattern
// as KonnectivityAgent.
type TCPProxy struct {
	Logger                    logr.Logger
	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
	ControllerName            string
}

func (t *TCPProxy) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := t.GetTenantControlPlaneFunc()
	if err != nil {
		if errors.Is(err, sooterrors.ErrPausedReconciliation) {
			t.Logger.Info(err.Error())
			return reconcile.Result{}, nil
		}

		t.Logger.Error(err, "cannot retrieve TenantControlPlane")
		return reconcile.Result{}, err
	}

	// Early return if tcpProxy addon is not configured.
	if tcp.Spec.Addons.TCPProxy == nil {
		return reconcile.Result{}, nil
	}

	for _, resource := range controllers.GetExternalTCPProxyResources(t.AdminClient) {
		t.Logger.Info("start processing", "resource", resource.GetName())

		result, handlingErr := resources.Handle(ctx, resource, tcp)
		if handlingErr != nil {
			t.Logger.Error(handlingErr, "resource process failed", "resource", resource.GetName())
			return reconcile.Result{}, handlingErr
		}

		if result == controllerutil.OperationResultNone {
			t.Logger.Info("resource processed", "resource", resource.GetName())
			continue
		}

		if err = utils.UpdateStatus(ctx, t.AdminClient, tcp, resource); err != nil {
			t.Logger.Error(err, "update status failed", "resource", resource.GetName())
			return reconcile.Result{}, err
		}
	}

	t.Logger.Info("reconciliation completed")
	return reconcile.Result{}, nil
}

func (t *TCPProxy) SetupWithManager(mgr manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(t.ControllerName).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			SkipNameValidation: ptr.To(true),
		}).
		// Primary watch: the tcp-proxy Deployment in kube-system
		For(&appsv1.Deployment{}, builder.WithPredicates(
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				return object.GetName() == tcpproxy.DeploymentName &&
					object.GetNamespace() == tcpproxy.Namespace
			}),
		)).
		// Watch ServiceAccount
		Watches(&corev1.ServiceAccount{},
			handler.EnqueueRequestsFromMapFunc(
				func(_ context.Context, object client.Object) []reconcile.Request {
					if object.GetName() == tcpproxy.ServiceAccountName &&
						object.GetNamespace() == tcpproxy.Namespace {
						return []reconcile.Request{{
							NamespacedName: types.NamespacedName{
								Namespace: object.GetNamespace(),
								Name:      object.GetName(),
							},
						}}
					}
					return nil
				},
			),
		).
		// Watch ClusterRoleBinding
		Watches(&rbacv1.ClusterRoleBinding{},
			handler.EnqueueRequestsFromMapFunc(
				func(_ context.Context, object client.Object) []reconcile.Request {
					if object.GetName() == tcpproxy.ClusterRoleBindingName {
						return []reconcile.Request{{
							NamespacedName: types.NamespacedName{
								Name: tcpproxy.ClusterRoleBindingName,
							},
						}}
					}
					return nil
				},
			),
		).
		// Trigger channel for TCP spec changes
		WatchesRawSource(source.Channel(
			t.TriggerChannel, &handler.EnqueueRequestForObject{},
		)).
		Complete(t)
}
