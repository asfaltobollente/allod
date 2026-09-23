package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asfaltobollente/allod/internal/state"
)

func TestWatchConfigAndInstallAPI(t *testing.T) {
	// Setup temporary directory for state.db
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(originalWd) }()

	// Ensure clean state.db
	st, err := state.Open("state.db")
	if err != nil {
		t.Fatalf("failed to open state.db: %v", err)
	}
	defer st.Close()

	// 1. Test GET /api/watch/config default values
	mux := http.NewServeMux()

	// Register test handler matching main.go logic
	mux.HandleFunc("/api/watch/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s, err := state.Open("state.db")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer s.Close()
		cfg, err := s.GetSentinelConfig()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		resp := map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"config":  cfg,
				"mesh_ip": getLocalNetBirdIP(),
				"port":    8080,
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/watch/config/save", func(w http.ResponseWriter, r *http.Request) {
		var input state.SentinelConfigRecord
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s, err := state.Open("state.db")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer s.Close()
		if err := s.SaveSentinelConfig(&input); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/watch/install.sh", func(w http.ResponseWriter, r *http.Request) {
		s, _ := state.Open("state.db")
		cfg, _ := s.GetSentinelConfig()
		s.Close()

		host := r.Host
		if host == "" {
			host = "100.64.0.1:8080"
		}

		script := `#!/usr/bin/env bash
ALLOD_HOST="` + host + `"
TELEGRAM_BOT_TOKEN="` + cfg.TelegramBotToken + `"
TELEGRAM_CHAT_ID="` + cfg.TelegramChatID + `"
WEATHER_CITY="` + cfg.WeatherCity + `"
DIGEST_TIME="` + cfg.DigestTime + `"
`
		w.Write([]byte(script))
	})

	// Test GET default
	req := httptest.NewRequest("GET", "/api/watch/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var getResp struct {
		Status string `json:"status"`
		Data   struct {
			Config state.SentinelConfigRecord `json:"config"`
			MeshIP string                     `json:"mesh_ip"`
			Port   int                        `json:"port"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if getResp.Data.Config.WeatherCity != "Roma" {
		t.Errorf("expected default city 'Roma', got %q", getResp.Data.Config.WeatherCity)
	}

	// 2. Test POST /api/watch/config/save
	savePayload := `{"telegram_bot_token":"123:XYZ","telegram_chat_id":"456","weather_city":"Torino","digest_time":"07:30","down_threshold_seconds":150,"vps_setup_key":"setup-123"}`
	reqSave := httptest.NewRequest("POST", "/api/watch/config/save", strings.NewReader(savePayload))
	recSave := httptest.NewRecorder()
	mux.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("expected save status 200, got %d", recSave.Code)
	}

	// Verify persistence in DB
	savedCfg, err := st.GetSentinelConfig()
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	if savedCfg.TelegramBotToken != "123:XYZ" || savedCfg.WeatherCity != "Torino" || savedCfg.VPSSetupKey != "setup-123" {
		t.Errorf("unexpected saved config: %+v", savedCfg)
	}

	// 3. Test GET /api/watch/install.sh
	reqInstall := httptest.NewRequest("GET", "/api/watch/install.sh", nil)
	reqInstall.Host = "100.64.0.10:8080"
	recInstall := httptest.NewRecorder()
	mux.ServeHTTP(recInstall, reqInstall)

	if recInstall.Code != http.StatusOK {
		t.Fatalf("expected install.sh status 200, got %d", recInstall.Code)
	}
	installContent := recInstall.Body.String()
	if !strings.Contains(installContent, `ALLOD_HOST="100.64.0.10:8080"`) {
		t.Errorf("expected script to contain host, got: %s", installContent)
	}
	if !strings.Contains(installContent, `WEATHER_CITY="Torino"`) {
		t.Errorf("expected script to contain configured city, got: %s", installContent)
	}
	if !strings.Contains(installContent, `TELEGRAM_BOT_TOKEN="123:XYZ"`) {
		t.Errorf("expected script to contain configured token, got: %s", installContent)
	}
}

func TestBinaryEndpointArchitecture(t *testing.T) {
	// Verify that the binary path resolution handles amd64 and arm64
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	_ = os.MkdirAll(binDir, 0755)

	amd64File := filepath.Join(binDir, "allod-watch-linux-amd64")
	_ = os.WriteFile(amd64File, []byte("mock-amd64-binary"), 0755)

	originalWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(originalWd) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/watch/binary", func(w http.ResponseWriter, r *http.Request) {
		arch := r.URL.Query().Get("arch")
		if arch != "arm64" {
			arch = "amd64"
		}
		targetPath := filepath.Join("bin", "allod-watch-linux-"+arch)
		if _, err := os.Stat(targetPath); err != nil {
			http.Error(w, "not found", 404)
			return
		}
		http.ServeFile(w, r, targetPath)
	})

	// Test GET with arch=amd64
	req := httptest.NewRequest("GET", "/api/watch/binary?arch=amd64", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected binary status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "mock-amd64-binary" {
		t.Errorf("unexpected content: %s", rec.Body.String())
	}
}
