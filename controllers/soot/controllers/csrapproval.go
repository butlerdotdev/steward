// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	sooterrors "github.com/butlerdotdev/steward/controllers/soot/controllers/errors"
	"github.com/butlerdotdev/steward/controllers/utils"
)

// CSRApproval automatically approves kubelet-serving CertificateSigningRequests
// from worker nodes when workerBootstrap auto-approve is enabled.
type CSRApproval struct {
	Logger                    logr.Logger
	AdminClient               client.Client
	GetTenantControlPlaneFunc utils.TenantControlPlaneRetrievalFn
	TriggerChannel            chan event.GenericEvent
	ControllerName            string
	client                    client.Client
}

func (c *CSRApproval) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	tcp, err := c.GetTenantControlPlaneFunc()
	if err != nil {
		if errors.Is(err, sooterrors.ErrPausedReconciliation) {
			c.Logger.Info(err.Error())
			return reconcile.Result{}, nil
		}
		c.Logger.Error(err, "cannot retrieve TenantControlPlane")
		return reconcile.Result{}, err
	}

	if tcp.Spec.Addons.WorkerBootstrap == nil || !tcp.Spec.Addons.WorkerBootstrap.CSRApproval.AutoApprove {
		return reconcile.Result{}, nil
	}

	var csrList certificatesv1.CertificateSigningRequestList
	if err := c.client.List(ctx, &csrList); err != nil {
		return reconcile.Result{}, err
	}

	for i := range csrList.Items {
		csr := &csrList.Items[i]
		if isApprovedOrDenied(csr) {
			continue
		}
		if csr.Spec.SignerName != certificatesv1.KubeletServingSignerName {
			continue
		}
		if err := c.validateAndApprove(ctx, csr, tcp); err != nil {
			c.Logger.Error(err, "failed to process CSR", "csr", csr.Name)
		}
	}

	return reconcile.Result{}, nil
}

func (c *CSRApproval) validateAndApprove(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, tcp *stewardv1alpha1.TenantControlPlane) error {
	// Validate requestor is a node
	if !strings.HasPrefix(csr.Spec.Username, "system:node:") {
		c.Logger.V(1).Info("skipping CSR: requestor is not a node", "csr", csr.Name, "username", csr.Spec.Username)
		return nil
	}

	hasNodesGroup := false
	for _, group := range csr.Spec.Groups {
		if group == "system:nodes" {
			hasNodesGroup = true
			break
		}
	}
	if !hasNodesGroup {
		c.Logger.V(1).Info("skipping CSR: requestor not in system:nodes group", "csr", csr.Name)
		return nil
	}

	// Validate usages include ServerAuth
	hasServerAuth := false
	for _, usage := range csr.Spec.Usages {
		if usage == certificatesv1.UsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	if !hasServerAuth {
		c.Logger.V(1).Info("skipping CSR: missing server auth usage", "csr", csr.Name)
		return nil
	}

	// Validate IP SANs against allowed subnets if configured
	if len(tcp.Spec.Addons.WorkerBootstrap.AllowedSubnets) > 0 {
		parsedCSR, err := parseCSRRequest(csr.Spec.Request)
		if err != nil {
			c.Logger.V(1).Info("skipping CSR: cannot parse request", "csr", csr.Name, "error", err)
			return nil
		}

		if !ipSANsInAllowedSubnets(parsedCSR.IPAddresses, tcp.Spec.Addons.WorkerBootstrap.AllowedSubnets) {
			c.Logger.V(1).Info("skipping CSR: IP SANs not in allowed subnets", "csr", csr.Name)
			return nil
		}
	}

	// Approve the CSR
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:               certificatesv1.CertificateApproved,
		Status:             corev1.ConditionTrue,
		Reason:             "StewardAutoApproved",
		Message:            "Auto-approved by Steward worker bootstrap CSR approval controller",
		LastUpdateTime:     metav1.Now(),
	})

	if err := c.client.SubResource("approval").Update(ctx, csr); err != nil {
		return fmt.Errorf("approving CSR %s: %w", csr.Name, err)
	}

	c.Logger.Info("approved CSR", "csr", csr.Name, "username", csr.Spec.Username)
	return nil
}

func (c *CSRApproval) SetupWithManager(mgr manager.Manager) error {
	c.client = mgr.GetClient()
	c.Logger = mgr.GetLogger().WithName("csrapproval")

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(c.ControllerName).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: ptr.To(true)}).
		For(&certificatesv1.CertificateSigningRequest{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			csr := object.(*certificatesv1.CertificateSigningRequest) //nolint:forcetypeassert
			return csr.Spec.SignerName == certificatesv1.KubeletServingSignerName && !isApprovedOrDenied(csr)
		}))).
		WatchesRawSource(source.Channel(c.TriggerChannel, &handler.EnqueueRequestForObject{})).
		Complete(c)
}

func isApprovedOrDenied(csr *certificatesv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved || condition.Type == certificatesv1.CertificateDenied {
			return true
		}
	}
	return false
}

func parseCSRRequest(csrPEM []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return x509.ParseCertificateRequest(csrPEM)
	}
	return x509.ParseCertificateRequest(block.Bytes)
}

func ipSANsInAllowedSubnets(ips []net.IP, subnets []string) bool {
	var cidrs []*net.IPNet
	for _, s := range subnets {
		_, cidr, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		cidrs = append(cidrs, cidr)
	}

	for _, ip := range ips {
		allowed := false
		for _, cidr := range cidrs {
			if cidr.Contains(ip) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	return true
}
