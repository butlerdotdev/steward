// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTrustdCredentials(t *testing.T) {
	ips := []net.IP{net.ParseIP("10.40.0.100")}
	dnsNames := []string{"my-cluster.k8s.butlerlabs.dev"}

	creds, err := GenerateTrustdCredentials("my-cluster", ips, dnsNames)
	require.NoError(t, err)
	require.NotNil(t, creds)

	t.Run("CA cert is valid", func(t *testing.T) {
		cert, err := ParseCertificateBytes(creds.OSCACert)
		require.NoError(t, err)
		assert.True(t, cert.IsCA)
		assert.Contains(t, cert.Subject.CommonName, "my-cluster")
		assert.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign, cert.KeyUsage)
	})

	t.Run("CA key is valid Ed25519", func(t *testing.T) {
		key, err := parseEd25519PrivateKey(creds.OSCAKey)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})

	t.Run("server chain contains two certificates", func(t *testing.T) {
		count := 0
		rest := creds.ServerChain
		for len(rest) > 0 {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				count++
			}
		}
		assert.Equal(t, 2, count, "server chain should contain server cert + CA cert")
	})

	t.Run("server cert has correct SANs", func(t *testing.T) {
		cert, err := ParseCertificateBytes(creds.ServerChain)
		require.NoError(t, err)
		assert.False(t, cert.IsCA)

		foundIP := false
		for _, ip := range cert.IPAddresses {
			if ip.Equal(net.ParseIP("10.40.0.100")) {
				foundIP = true
			}
		}
		assert.True(t, foundIP, "server cert should contain IP SAN 10.40.0.100")
		assert.Contains(t, cert.DNSNames, "my-cluster.k8s.butlerlabs.dev")
	})

	t.Run("server cert has ServerAuth usage", func(t *testing.T) {
		cert, err := ParseCertificateBytes(creds.ServerChain)
		require.NoError(t, err)
		assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	})

	t.Run("server cert is signed by CA", func(t *testing.T) {
		caCert, err := ParseCertificateBytes(creds.OSCACert)
		require.NoError(t, err)
		serverCert, err := ParseCertificateBytes(creds.ServerChain)
		require.NoError(t, err)

		roots := x509.NewCertPool()
		roots.AddCert(caCert)
		_, err = serverCert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		})
		assert.NoError(t, err)
	})

	t.Run("server key is valid Ed25519", func(t *testing.T) {
		key, err := parseEd25519PrivateKey(creds.ServerKey)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})

	t.Run("token has correct format", func(t *testing.T) {
		assert.True(t, strings.HasPrefix(creds.Token, "butler."))
		// "butler." (7 chars) + 32 hex chars = 39 total
		assert.Len(t, creds.Token, 39)
	})
}

func TestRegenerateTrustdServerCert(t *testing.T) {
	ips := []net.IP{net.ParseIP("10.40.0.100")}
	dnsNames := []string{"my-cluster.k8s.butlerlabs.dev"}

	creds, err := GenerateTrustdCredentials("my-cluster", ips, dnsNames)
	require.NoError(t, err)

	newIPs := []net.IP{net.ParseIP("10.40.0.200")}
	newDNS := []string{"new-cluster.k8s.butlerlabs.dev"}

	newChain, newKey, err := RegenerateTrustdServerCert(creds.OSCACert, creds.OSCAKey, newIPs, newDNS)
	require.NoError(t, err)

	t.Run("new cert has updated SANs", func(t *testing.T) {
		cert, err := ParseCertificateBytes(newChain)
		require.NoError(t, err)

		foundNewIP := false
		for _, ip := range cert.IPAddresses {
			if ip.Equal(net.ParseIP("10.40.0.200")) {
				foundNewIP = true
			}
		}
		assert.True(t, foundNewIP, "regenerated cert should have new IP SAN")
		assert.Contains(t, cert.DNSNames, "new-cluster.k8s.butlerlabs.dev")
	})

	t.Run("new cert is signed by same CA", func(t *testing.T) {
		caCert, err := ParseCertificateBytes(creds.OSCACert)
		require.NoError(t, err)
		newCert, err := ParseCertificateBytes(newChain)
		require.NoError(t, err)

		roots := x509.NewCertPool()
		roots.AddCert(caCert)
		_, err = newCert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		})
		assert.NoError(t, err)
	})

	t.Run("new key is valid", func(t *testing.T) {
		key, err := parseEd25519PrivateKey(newKey)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})

	t.Run("chain contains two certs", func(t *testing.T) {
		count := 0
		rest := newChain
		for len(rest) > 0 {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				count++
			}
		}
		assert.Equal(t, 2, count)
	})
}

func TestParseTrustdServerCertSANs(t *testing.T) {
	ips := []net.IP{net.ParseIP("10.40.0.100"), net.ParseIP("10.40.0.101")}
	dnsNames := []string{"a.example.com", "b.example.com"}

	creds, err := GenerateTrustdCredentials("test", ips, dnsNames)
	require.NoError(t, err)

	parsedIPs, parsedDNS, err := ParseTrustdServerCertSANs(creds.ServerChain)
	require.NoError(t, err)
	assert.Len(t, parsedIPs, 2)
	assert.Len(t, parsedDNS, 2)
}

func TestGenerateAdminClientCert(t *testing.T) {
	ips := []net.IP{net.ParseIP("10.40.0.100")}
	dnsNames := []string{"my-cluster.k8s.butlerlabs.dev"}

	creds, err := GenerateTrustdCredentials("my-cluster", ips, dnsNames)
	require.NoError(t, err)

	t.Run("admin cert is valid", func(t *testing.T) {
		cert, err := ParseCertificateBytes(creds.AdminCert)
		require.NoError(t, err)
		assert.False(t, cert.IsCA)
		assert.Equal(t, "my-cluster admin", cert.Subject.CommonName)
		assert.Equal(t, []string{"steward"}, cert.Subject.Organization)
	})

	t.Run("admin cert has ClientAuth usage", func(t *testing.T) {
		cert, err := ParseCertificateBytes(creds.AdminCert)
		require.NoError(t, err)
		assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
		assert.NotContains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	})

	t.Run("admin cert has no SANs", func(t *testing.T) {
		cert, err := ParseCertificateBytes(creds.AdminCert)
		require.NoError(t, err)
		assert.Empty(t, cert.IPAddresses)
		assert.Empty(t, cert.DNSNames)
	})

	t.Run("admin cert is signed by CA", func(t *testing.T) {
		caCert, err := ParseCertificateBytes(creds.OSCACert)
		require.NoError(t, err)
		adminCert, err := ParseCertificateBytes(creds.AdminCert)
		require.NoError(t, err)

		roots := x509.NewCertPool()
		roots.AddCert(caCert)
		_, err = adminCert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		assert.NoError(t, err)
	})

	t.Run("admin key is valid Ed25519", func(t *testing.T) {
		key, err := parseEd25519PrivateKey(creds.AdminKey)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})
}

func TestRegenerateAdminClientCert(t *testing.T) {
	ips := []net.IP{net.ParseIP("10.40.0.100")}
	dnsNames := []string{"my-cluster.k8s.butlerlabs.dev"}

	creds, err := GenerateTrustdCredentials("my-cluster", ips, dnsNames)
	require.NoError(t, err)

	// Simulate upgrade: regenerate admin cert from existing CA
	adminCert, adminKey, err := RegenerateAdminClientCert("my-cluster", creds.OSCACert, creds.OSCAKey)
	require.NoError(t, err)

	t.Run("regenerated cert is signed by same CA", func(t *testing.T) {
		caCert, err := ParseCertificateBytes(creds.OSCACert)
		require.NoError(t, err)
		cert, err := ParseCertificateBytes(adminCert)
		require.NoError(t, err)

		roots := x509.NewCertPool()
		roots.AddCert(caCert)
		_, err = cert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		assert.NoError(t, err)
	})

	t.Run("regenerated key is valid", func(t *testing.T) {
		key, err := parseEd25519PrivateKey(adminKey)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})
}

func TestGenerateTrustdCredentials_NoSANs(t *testing.T) {
	creds, err := GenerateTrustdCredentials("empty", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, creds)

	cert, err := ParseCertificateBytes(creds.ServerChain)
	require.NoError(t, err)
	assert.Empty(t, cert.IPAddresses)
	assert.Empty(t, cert.DNSNames)
}
