package panel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asfaltobollente/allod/internal/helper"
)

type mockHelperClient struct {
	executeFn func(action string, args map[string]interface{}, plan bool) (helper.Response, error)
}

func (m *mockHelperClient) Execute(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
	if m.executeFn != nil {
		return m.executeFn(action, args, plan)
	}
	return helper.Response{Ok: true}, nil
}

func TestNetworkStatus(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	// Create netbird secret env
	secDir := filepath.Join(tempDir, "network", "secrets")
	if err := os.MkdirAll(secDir, 0700); err != nil {
		t.Fatalf("failed to create secrets dir: %v", err)
	}
	envContent := "NB_SETUP_KEY=nb-key-12345\nNB_MANAGEMENT_URL=https://api.netbird.io:443\n"
	if err := os.WriteFile(filepath.Join(secDir, "netbird.env"), []byte(envContent), 0600); err != nil {
		t.Fatalf("failed to write netbird.env: %v", err)
	}

	mock := &mockHelperClient{
		executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
			if action == "network.netbird_status" {
				return helper.Response{
					Ok: true,
					Output: `{
						"netbirdIp": "100.64.0.1/16",
						"publicKey": "pubkey-xyz",
						"management": { "connected": true, "url": "https://api.netbird.io:443" },
						"signal": { "connected": true, "url": "https://signal.netbird.io:443" },
						"peers": { "total": 2, "connected": 2, "details": [] }
					}`,
				}, nil
			}
			return helper.Response{Ok: false, Error: "unexpected action"}, nil
		},
	}

	mux := http.NewServeMux()
	RegisterNetworkRoutes(mux, &NetworkHandler{
		Helper: mock,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/network/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp PanelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", resp.Data)
	}
	if dataMap["has_key"] != true {
		t.Errorf("expected has_key=true, got %v", dataMap["has_key"])
	}
	if dataMap["management_url"] != "https://api.netbird.io:443" {
		t.Errorf("expected management_url=https://api.netbird.io:443, got %v", dataMap["management_url"])
	}
	if dataMap["mesh_ip"] != "100.64.0.1" {
		t.Errorf("expected mesh_ip=100.64.0.1, got %v", dataMap["mesh_ip"])
	}
	if dataMap["peers_count"] != float64(2) {
		t.Errorf("expected peers_count=2, got %v", dataMap["peers_count"])
	}
}

func TestNetworkConfigure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	mock := &mockHelperClient{}
	mux := http.NewServeMux()
	RegisterNetworkRoutes(mux, &NetworkHandler{
		Helper: mock,
	})

	body := []byte(`{"mode":"selfhosted","setup_key":"my-netbird-setup-key","management_url":"mesh.example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/network/configure", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var resp PanelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	// Verify secret env file was created
	secEnvPath := filepath.Join(tempDir, "network", "secrets", "netbird.env")
	secBytes, err := os.ReadFile(secEnvPath)
	if err != nil {
		t.Fatalf("expected secrets/netbird.env to exist: %v", err)
	}
	content := string(secBytes)
	if !strings.Contains(content, "NB_SETUP_KEY=my-netbird-setup-key") {
		t.Errorf("expected NB_SETUP_KEY to be saved, got:\n%s", content)
	}
	if !strings.Contains(content, "NB_MANAGEMENT_URL=https://mesh.example.com") {
		t.Errorf("expected NB_MANAGEMENT_URL with https:// prefix, got:\n%s", content)
	}
}

func TestNetworkPairingInfo(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	secDir := filepath.Join(tempDir, "network", "secrets")
	_ = os.MkdirAll(secDir, 0700)
	_ = os.WriteFile(filepath.Join(secDir, "netbird.env"), []byte("NB_SETUP_KEY=pair-key-xyz\nNB_MANAGEMENT_URL=https://mesh.test.lan\n"), 0600)

	mock := &mockHelperClient{}
	mux := http.NewServeMux()
	RegisterNetworkRoutes(mux, &NetworkHandler{
		Helper: mock,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/network/pairing-info", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp PanelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	data := resp.Data.(map[string]interface{})
	if data["key"] != "pair-key-xyz" {
		t.Errorf("expected key pair-key-xyz, got %v", data["key"])
	}
	if data["management_url"] != "https://mesh.test.lan" {
		t.Errorf("expected management_url https://mesh.test.lan, got %v", data["management_url"])
	}
}

func TestNetworkConfigureManaged(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	serverUpCalled := false
	mock := &mockHelperClient{
		executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
			if action == "network.server_up" {
				serverUpCalled = true
				return helper.Response{Ok: true, Applied: true, Output: "server started"}, nil
			}
			return helper.Response{Ok: true}, nil
		},
	}
	mux := http.NewServeMux()
	RegisterNetworkRoutes(mux, &NetworkHandler{
		Helper: mock,
	})

	body := []byte(`{"mode":"selfhosted_managed","setup_key":"local-setup-key","server_domain":"mesh.local","server_port":33073,"server_dash_port":8088}`)
	req := httptest.NewRequest(http.MethodPost, "/api/network/configure", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !serverUpCalled {
		t.Errorf("expected network.server_up to be called for selfhosted_managed mode")
	}
}

func TestNetworkInstallNative(t *testing.T) {
	nativeCalled := false
	mock := &mockHelperClient{
		executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
			if action == "network.install_native" {
				nativeCalled = true
				return helper.Response{Ok: true, Applied: true, Output: "installed netbird 0.79.0"}, nil
			}
			return helper.Response{Ok: false, Error: "unexpected action"}, nil
		},
	}
	mux := http.NewServeMux()
	RegisterNetworkRoutes(mux, &NetworkHandler{
		Helper: mock,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/network/install-native", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !nativeCalled {
		t.Errorf("expected network.install_native to be called")
	}
}

func TestNetworkServerEndpoints(t *testing.T) {
	serverUpCalled := false
	serverDownCalled := false
	mock := &mockHelperClient{
		executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
			if action == "network.server_up" {
				serverUpCalled = true
				return helper.Response{Ok: true, Applied: true, Output: "Server NetBird avviato"}, nil
			}
			if action == "network.server_down" {
				serverDownCalled = true
				return helper.Response{Ok: true, Applied: true, Output: "Server NetBird fermato"}, nil
			}
			return helper.Response{Ok: true}, nil
		},
	}
	mux := http.NewServeMux()
	RegisterNetworkRoutes(mux, &NetworkHandler{
		Helper: mock,
	})

	// Test start
	bodyStart := []byte(`{"domain":"192.168.1.100","port":33073,"dash_port":8088}`)
	reqStart := httptest.NewRequest(http.MethodPost, "/api/network/server/start", bytes.NewReader(bodyStart))
	recStart := httptest.NewRecorder()
	mux.ServeHTTP(recStart, reqStart)
	if recStart.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recStart.Code)
	}
	if !serverUpCalled {
		t.Errorf("expected server_up to be called")
	}

	// Test stop
	reqStop := httptest.NewRequest(http.MethodPost, "/api/network/server/stop", nil)
	recStop := httptest.NewRecorder()
	mux.ServeHTTP(recStop, reqStop)
	if recStop.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recStop.Code)
	}
	if !serverDownCalled {
		t.Errorf("expected server_down to be called")
	}
}

