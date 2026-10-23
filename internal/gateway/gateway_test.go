package gateway

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/asfaltobollente/allod/internal/pki"
)

func TestGatewayLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "allod-gw-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pkiMgr := pki.NewManager(tmpDir)
	if err := pkiMgr.EnsureRootCA(); err != nil {
		t.Fatalf("failed to ensure root ca: %v", err)
	}
	if err := pkiMgr.EnsureServerCert(); err != nil {
		t.Fatalf("failed to ensure server cert: %v", err)
	}

	gw := New(pkiMgr)
	if gw == nil {
		t.Fatalf("New gateway returned nil")
	}

	// Mock internal backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proto := r.Header.Get("X-Forwarded-Proto")
		w.Header().Set("X-Received-Proto", proto)
		_, _ = w.Write([]byte("backend-ok"))
	}))
	defer backend.Close()

	// Test Offline / Starting page rendering
	rec := httptest.NewRecorder()
	renderModuleOfflinePage(rec, Route{
		ModuleID:    "passwords",
		Title:       "Allod Passwords",
		Description: "Vaultwarden Password Manager",
	})

	body := rec.Body.String()
	if !strings.Contains(body, "Allod Passwords") || !strings.Contains(body, "Crittografia TLS 1.3 Attiva") {
		t.Fatalf("Offline page missing expected content: %s", body)
	}
}

func TestGatewaySniffRedirect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "allod-gw-test2-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pkiMgr := pki.NewManager(tmpDir)
	_ = pkiMgr.EnsureRootCA()
	_ = pkiMgr.EnsureServerCert()

	// Choose random high port
	extPort := 29876
	internalPort := 29877

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock-internal"))
	}))
	defer backend.Close()

	gw := New(pkiMgr)
	gw.routes = []Route{
		{
			ModuleID:     "testmod",
			ExternalPort: extPort,
			InternalPort: internalPort,
			Title:        "Test Mod",
		},
	}

	if err := gw.Start(); err != nil {
		t.Fatalf("failed to start gateway: %v", err)
	}
	defer gw.Stop()

	// 1. Plain HTTP request to external port should receive 307 redirect
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirect
		},
	}

	httpResp, err := noRedirectClient.Get(fmt.Sprintf("http://127.0.0.1:%d/test-path", extPort))
	if err != nil {
		t.Fatalf("Plain HTTP request failed: %v", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("Expected 307 redirect, got: %d", httpResp.StatusCode)
	}

	loc := httpResp.Header.Get("Location")
	expectedPrefix := fmt.Sprintf("https://127.0.0.1:%d/test-path", extPort)
	if loc != expectedPrefix {
		t.Fatalf("Expected redirect to %s, got: %s", expectedPrefix, loc)
	}

	// 2. HTTPS request with TLS client skipping verification (since test CA is not in system pool)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpsClient := &http.Client{Transport: tr}

	httpsResp, err := httpsClient.Get(fmt.Sprintf("https://127.0.0.1:%d/test-path", extPort))
	if err != nil {
		t.Fatalf("HTTPS request failed: %v", err)
	}
	defer httpsResp.Body.Close()

	// Backend is not listening on internalPort, so should return 502 Bad Gateway with offline page
	if httpsResp.StatusCode != http.StatusBadGateway {
		t.Fatalf("Expected 502 Bad Gateway for offline backend, got: %d", httpsResp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(httpsResp.Body)
	if !strings.Contains(string(bodyBytes), "Test Mod") {
		t.Fatalf("Expected offline page body, got: %s", string(bodyBytes))
	}
}
