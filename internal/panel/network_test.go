package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allod-project/allod/internal/helper"
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

	// Create headscale config
	hsCfgDir := filepath.Join(tempDir, "network", "headscale", "config")
	if err := os.MkdirAll(hsCfgDir, 0755); err != nil {
		t.Fatalf("failed to create hs cfg dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hsCfgDir, "config.yaml"), []byte("server_url: https://vpn.test.example.com\n"), 0644); err != nil {
		t.Fatalf("failed to write hs cfg: %v", err)
	}

	// Create cloudflared secret env
	secDir := filepath.Join(tempDir, "network", "secrets")
	if err := os.MkdirAll(secDir, 0700); err != nil {
		t.Fatalf("failed to create secrets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secDir, "cloudflared.env"), []byte("TUNNEL_TOKEN=fake-token-123\n"), 0600); err != nil {
		t.Fatalf("failed to write cloudflared.env: %v", err)
	}

	mock := &mockHelperClient{
		executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
			if action == "network.headscale_cli" && args["command"] == "nodes_list" {
				return helper.Response{
					Ok:     true,
					Output: `[{"id": 1, "name": "test-phone"}]`,
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
	if dataMap["has_token"] != true {
		t.Errorf("expected has_token=true, got %v", dataMap["has_token"])
	}
	if dataMap["server_url"] != "https://vpn.test.example.com" {
		t.Errorf("expected server_url=https://vpn.test.example.com, got %v", dataMap["server_url"])
	}
	if dataMap["nodes_count"] != float64(1) {
		t.Errorf("expected nodes_count=1, got %v", dataMap["nodes_count"])
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

	body := []byte(`{"domain":"mesh.example.com","tunnel_token":"my-secure-cf-tunnel-token"}`)
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
	secEnvPath := filepath.Join(tempDir, "network", "secrets", "cloudflared.env")
	secBytes, err := os.ReadFile(secEnvPath)
	if err != nil {
		t.Fatalf("expected secrets/cloudflared.env to exist: %v", err)
	}
	if !strings.Contains(string(secBytes), "TUNNEL_TOKEN=my-secure-cf-tunnel-token") {
		t.Errorf("unexpected secret content: %s", string(secBytes))
	}
}

func TestNetworkPreauthKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	hsCfgDir := filepath.Join(tempDir, "network", "headscale", "config")
	_ = os.MkdirAll(hsCfgDir, 0755)
	_ = os.WriteFile(filepath.Join(hsCfgDir, "config.yaml"), []byte("server_url: https://vpn.test.example.com\n"), 0644)

	t.Run("success", func(t *testing.T) {
		mock := &mockHelperClient{
			executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
				if action == "network.headscale_cli" && args["command"] == "preauthkey_create" {
					return helper.Response{
						Ok:     true,
						Output: "preauth-key-xyz-987654321",
					}, nil
				}
				return helper.Response{Ok: false, Error: "not found"}, nil
			},
		}

		mux := http.NewServeMux()
		RegisterNetworkRoutes(mux, &NetworkHandler{
			Helper: mock,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/network/preauth-key", nil)
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
		if data["key"] != "preauth-key-xyz-987654321" {
			t.Errorf("expected key preauth-key-xyz-987654321, got %v", data["key"])
		}
		if data["server_url"] != "https://vpn.test.example.com" {
			t.Errorf("expected server_url https://vpn.test.example.com, got %v", data["server_url"])
		}
	})

	t.Run("helper error", func(t *testing.T) {
		mock := &mockHelperClient{
			executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
				return helper.Response{
					Ok:    false,
					Error: "headscale daemon not running",
				}, fmt.Errorf("command failed")
			},
		}

		mux := http.NewServeMux()
		RegisterNetworkRoutes(mux, &NetworkHandler{
			Helper: mock,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/network/preauth-key", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		var resp PanelResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if resp.Status != "error" {
			t.Fatalf("expected status error, got %s", resp.Status)
		}
		if !strings.Contains(resp.Message, "headscale daemon not running") {
			t.Errorf("expected error message, got: %s", resp.Message)
		}
	})
}
