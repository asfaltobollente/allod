package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWatchInstallTokenLifecycle(t *testing.T) {
	store := &WatchInstallTokenStore{
		tokens: make(map[string]time.Time),
	}

	// 1. Generate token
	tok, err := store.Generate(15 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if len(tok) < 16 {
		t.Fatalf("token too short: %s", tok)
	}

	// 2. Validate and consume
	if !store.ValidateAndConsume(tok) {
		t.Fatalf("expected token %s to be valid", tok)
	}

	// 3. Second attempt must fail (single-use)
	if store.ValidateAndConsume(tok) {
		t.Fatalf("expected token %s to be consumed already", tok)
	}

	// 4. Expired token must fail
	expTok, err := store.Generate(-1 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	if store.ValidateAndConsume(expTok) {
		t.Fatalf("expected expired token to fail validation")
	}

	// 5. Unknown token must fail
	if store.ValidateAndConsume("unknown-token-xyz") {
		t.Fatalf("expected unknown token to fail validation")
	}
}

func TestHandleWatchInstallTokenEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/watch/install-token", nil)
	rec := httptest.NewRecorder()

	HandleWatchInstallToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", rec.Code)
	}

	var resp struct {
		Status string `json:"status"`
		Token  string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" || resp.Token == "" {
		t.Fatalf("invalid response payload: %+v", resp)
	}

	// Token must be consumable
	if !globalWatchInstallTokens.ValidateAndConsume(resp.Token) {
		t.Fatalf("generated token from endpoint could not be consumed")
	}
}
