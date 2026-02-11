// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// TrustdCredentials contains the OS-level credentials for steward-trustd.
type TrustdCredentials struct {
	// OSCACert is the PEM-encoded Ed25519 CA certificate (10-year validity).
	OSCACert []byte
	// OSCAKey is the PEM-encoded Ed25519 CA private key.
	OSCAKey []byte
	// ServerChain is the PEM-encoded server cert + CA cert concatenated.
	// This chain is required for TLS handshakes — without it, workers get
	// "certificate signed by unknown authority".
	ServerChain []byte
	// ServerKey is the PEM-encoded server private key.
	ServerKey []byte
	// Token is the machine token in the format "butler.<32-hex-chars>".
	Token string
}

// GenerateTrustdCredentials generates a full set of OS credentials for steward-trustd:
// Ed25519 CA, server certificate with IP SANs and DNS SANs (chained with CA), and a token.
func GenerateTrustdCredentials(clusterName string, ipAddresses []net.IP, dnsNames []string) (*TrustdCredentials, error) {
	// Generate CA
	caCert, caKey, caCertPEM, caKeyPEM, err := generateEd25519CA(clusterName)
	if err != nil {
		return nil, fmt.Errorf("generating OS CA: %w", err)
	}

	// Generate server cert with chain
	serverChain, serverKey, err := generateEd25519ServerCert(caCert, caKey, caCertPEM, ipAddresses, dnsNames)
	if err != nil {
		return nil, fmt.Errorf("generating trustd server cert: %w", err)
	}

	// Generate token
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	return &TrustdCredentials{
		OSCACert:    caCertPEM,
		OSCAKey:     caKeyPEM,
		ServerChain: serverChain,
		ServerKey:   serverKey,
		Token:       token,
	}, nil
}

// RegenerateTrustdServerCert regenerates only the server certificate when SANs change.
// The CA and token are preserved. Returns the new server chain and key.
func RegenerateTrustdServerCert(caCertPEM, caKeyPEM []byte, ipAddresses []net.IP, dnsNames []string) (serverChain, serverKey []byte, err error) {
	caCert, err := ParseCertificateBytes(caCertPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA cert: %w", err)
	}

	caKey, err := parseEd25519PrivateKey(caKeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA key: %w", err)
	}

	return generateEd25519ServerCert(caCert, caKey, caCertPEM, ipAddresses, dnsNames)
}

func generateEd25519CA(clusterName string) (*x509.Certificate, ed25519.PrivateKey, []byte, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("generating Ed25519 key: %w", err)
	}

	serialNumber, err := cryptorand.Int(cryptorand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("generating serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("%s OS CA", clusterName),
			Organization: []string{"steward"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(cryptorand.Reader, template, template, pub, priv)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("creating CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	certPEM := encodeCertPEM(certDER)

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshaling CA private key: %w", err)
	}
	keyPEM := encodeKeyPEM(keyDER)

	return caCert, priv, certPEM, keyPEM, nil
}

func generateEd25519ServerCert(caCert *x509.Certificate, caKey ed25519.PrivateKey, caCertPEM []byte, ipAddresses []net.IP, dnsNames []string) ([]byte, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generating Ed25519 key: %w", err)
	}

	serialNumber, err := cryptorand.Int(cryptorand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generating serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "steward-trustd",
			Organization: []string{"steward"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: ipAddresses,
		DNSNames:    dnsNames,
	}

	certDER, err := x509.CreateCertificate(cryptorand.Reader, template, caCert, pub, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("creating server certificate: %w", err)
	}

	serverCertPEM := encodeCertPEM(certDER)

	// Chain: server cert + CA cert (required for full chain TLS verification)
	var chainBuf bytes.Buffer
	chainBuf.Write(serverCertPEM)
	chainBuf.Write(caCertPEM)
	serverChain := chainBuf.Bytes()

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling server private key: %w", err)
	}
	serverKeyPEM := encodeKeyPEM(keyDER)

	return serverChain, serverKeyPEM, nil
}

func generateToken() (string, error) {
	tokenBytes := make([]byte, 16)
	if _, err := cryptorand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return "butler." + hex.EncodeToString(tokenBytes), nil
}

func parseEd25519PrivateKey(keyPEM []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PKCS8 private key: %w", err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected ed25519.PrivateKey, got %T", key)
	}

	return edKey, nil
}

func encodeCertPEM(certDER []byte) []byte {
	var buf bytes.Buffer
	_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return buf.Bytes()
}

func encodeKeyPEM(keyDER []byte) []byte {
	var buf bytes.Buffer
	_ = pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return buf.Bytes()
}

// ParseTrustdServerCertSANs parses a PEM-encoded server certificate (possibly chained)
// and returns its IP addresses and DNS names.
func ParseTrustdServerCertSANs(certPEM []byte) ([]net.IP, []string, error) {
	cert, err := ParseCertificateBytes(certPEM)
	if err != nil {
		return nil, nil, err
	}
	return cert.IPAddresses, cert.DNSNames, nil
}
