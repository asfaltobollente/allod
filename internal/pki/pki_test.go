package pki

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPKIManagerLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "allod-pki-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// 1. Ensure Root CA
	if err := mgr.EnsureRootCA(); err != nil {
		t.Fatalf("EnsureRootCA failed: %v", err)
	}

	caCertFile := filepath.Join(tmpDir, "allod-root-ca.crt")
	if _, err := os.Stat(caCertFile); err != nil {
		t.Fatalf("Root CA cert file was not created: %v", err)
	}

	rootPEM, err := mgr.GetRootCACertPEM()
	if err != nil {
		t.Fatalf("GetRootCACertPEM failed: %v", err)
	}
	if !strings.Contains(string(rootPEM), "BEGIN CERTIFICATE") {
		t.Fatalf("Root CA PEM invalid: %s", string(rootPEM))
	}

	// 2. Fingerprint SHA256
	fp, err := mgr.GetRootCAFingerprintSHA256()
	if err != nil {
		t.Fatalf("GetRootCAFingerprintSHA256 failed: %v", err)
	}
	if len(fp) < 32 || !strings.Contains(fp, ":") {
		t.Fatalf("invalid fingerprint: %s", fp)
	}

	// 3. Ensure Server Cert
	if err := mgr.EnsureServerCert(); err != nil {
		t.Fatalf("EnsureServerCert failed: %v", err)
	}

	srvCertPath, srvKeyPath := mgr.GetServerCertPaths()
	srvPEM, err := os.ReadFile(srvCertPath)
	if err != nil {
		t.Fatalf("failed to read server cert: %v", err)
	}
	block, _ := pem.Decode(srvPEM)
	if block == nil {
		t.Fatalf("failed to decode server cert pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server cert: %v", err)
	}

	// Verify signed by Root CA
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(rootPEM)
	opts := x509.VerifyOptions{
		Roots: roots,
	}
	if _, err := cert.Verify(opts); err != nil {
		t.Fatalf("Server certificate verification against Root CA failed: %v", err)
	}

	// 4. Apple mobileconfig
	mobileconfig, err := mgr.GenerateAppleMobileConfig()
	if err != nil {
		t.Fatalf("GenerateAppleMobileConfig failed: %v", err)
	}
	mcStr := string(mobileconfig)
	if !strings.Contains(mcStr, "<plist version=\"1.0\">") || !strings.Contains(mcStr, "Allod Sovereign Family Root CA") {
		t.Fatalf("Apple mobileconfig missing required fields: %s", mcStr)
	}

	// 5. Status
	status := mgr.GetStatus()
	if status["ca_common_name"] != "Allod Sovereign Family Root CA" {
		t.Fatalf("status ca_common_name mismatch: %v", status)
	}
	if status["days_remaining"].(int) < 80 {
		t.Fatalf("unexpected days_remaining: %v", status["days_remaining"])
	}

	// 6. TLS config check
	tlsCfg := mgr.GetTLSConfig()
	if tlsCfg == nil || tlsCfg.GetCertificate == nil {
		t.Fatalf("invalid tls config")
	}
	tlsCert, err := tlsCfg.GetCertificate(nil)
	if err != nil || tlsCert == nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}
	_ = srvKeyPath
}
