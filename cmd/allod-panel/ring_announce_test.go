package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asfaltobollente/allod/internal/panel"
	"github.com/asfaltobollente/allod/internal/state"
)

func TestRingAnnounceEndpointSecurity(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")
	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create state.db: %v", err)
	}
	defer st.Close()

	// Switch working dir for ring.yaml creation
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	mux := http.NewServeMux()

	// Register /api/ring/announce handler
	mux.HandleFunc("/api/ring/announce", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Address string `json:"address"`
			QuotaGB int    `json:"quota_gb"`
			Token   string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		authorized := false
		if admToken := panel.GetSessionToken(r, panel.AdminCookieName); admToken != "" {
			if sess, errGet := st.GetSession(admToken); errGet == nil && sess != nil && sess.UserType == "admin" {
				authorized = true
			}
		}

		if !authorized {
			tokenCandidate := extractRingAnnounceToken(r)
			if tokenCandidate == "" {
				tokenCandidate = req.Token
			}
			if tokenCandidate != "" && globalRingAnnounceTokens.ValidateAndConsume(tokenCandidate) {
				authorized = true
			}
		}

		if !authorized {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: "Accesso negato: richiesta autenticazione admin o token di annuncio monouso valido",
			})
			return
		}

		if !validVaultNameRegex.MatchString(req.Name) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.QuotaGB <= 0 || req.QuotaGB > 100000 {
			req.QuotaGB = 500
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Registrato"})
	})

	// 1. Unauthenticated request without token must return 403 Forbidden
	unauthBody := []byte(`{"name":"test-vault","address":"100.64.0.5","quota_gb":500}`)
	req1 := httptest.NewRequest(http.MethodPost, "/api/ring/announce", bytes.NewReader(unauthBody))
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unauthenticated request, got %d", rec1.Code)
	}

	// 2. Request with valid announce token in payload must return 200 OK
	validTok, err := globalRingAnnounceTokens.Generate(15 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate announce token: %v", err)
	}
	validBody, _ := json.Marshal(map[string]interface{}{
		"name":     "test-vault-01",
		"address":  "100.64.0.10",
		"quota_gb": 500,
		"token":    validTok,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/ring/announce", bytes.NewReader(validBody))
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid announce token, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 3. Second request with same token must fail (single-use token consumed)
	req3 := httptest.NewRequest(http.MethodPost, "/api/ring/announce", bytes.NewReader(validBody))
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for consumed token, got %d", rec3.Code)
	}

	// 4. Request with Bearer authorization header must succeed
	bearerTok, _ := globalRingAnnounceTokens.Generate(15 * time.Minute)
	req4Body := []byte(`{"name":"test-vault-02","address":"100.64.0.11","quota_gb":500}`)
	req4 := httptest.NewRequest(http.MethodPost, "/api/ring/announce", bytes.NewReader(req4Body))
	req4.Header.Set("Authorization", "Bearer "+bearerTok)
	rec4 := httptest.NewRecorder()
	mux.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with Bearer token, got %d", rec4.Code)
	}

	// 5. Invalid name must return 400 Bad Request
	tok5, _ := globalRingAnnounceTokens.Generate(15 * time.Minute)
	badNameBody, _ := json.Marshal(map[string]interface{}{
		"name":     "invalid name with spaces!",
		"address":  "100.64.0.12",
		"quota_gb": 500,
		"token":    tok5,
	})
	req5 := httptest.NewRequest(http.MethodPost, "/api/ring/announce", bytes.NewReader(badNameBody))
	rec5 := httptest.NewRecorder()
	mux.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid name, got %d", rec5.Code)
	}
}
