package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/asfaltobollente/allod/internal/helper"
	"github.com/asfaltobollente/allod/internal/state"
)

func TestPortalPasswordHandlerRequiresToken(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")

	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test state db: %v", err)
	}
	defer st.Close()

	// Seed a family member
	err = st.CreateFamilyMember(&state.FamilyMember{
		Username:  "mamma",
		FirstName: "Mamma",
		LastName:  "Rossi",
		Role:      "member",
	})
	if err != nil {
		t.Fatalf("failed to create family member: %v", err)
	}

	handler := HandlePortalSetPassword(dbPath, func(client *helper.Client, username string) error {
		return nil
	})

	// 1. Attack payload: only username and new_password (no token)
	bodyAttack, _ := json.Marshal(map[string]string{
		"username":     "mamma",
		"new_password": "evilpassword123",
	})
	reqAttack := httptest.NewRequest(http.MethodPost, "/api/portal/set-password", bytes.NewReader(bodyAttack))
	reqAttack.Header.Set("Content-Type", "application/json")
	recAttack := httptest.NewRecorder()

	handler(recAttack, reqAttack)

	if recAttack.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for username-only request without token, got: %d", recAttack.Code)
	}

	// 2. Invalid token
	bodyInvalid, _ := json.Marshal(map[string]string{
		"token":        "invalid-token-12345",
		"new_password": "password123",
	})
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/portal/set-password", bytes.NewReader(bodyInvalid))
	reqInvalid.Header.Set("Content-Type", "application/json")
	recInvalid := httptest.NewRecorder()

	handler(recInvalid, reqInvalid)

	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid token, got: %d", recInvalid.Code)
	}

	// 3. Valid token succeeds and consumes token
	validToken, errTok := st.CreateResetToken("mamma", 15*time.Minute)
	if errTok != nil {
		t.Fatalf("failed to create reset token: %v", errTok)
	}

	bodyValid, _ := json.Marshal(map[string]string{
		"token":        validToken,
		"new_password": "validpassword123",
	})
	reqValid := httptest.NewRequest(http.MethodPost, "/api/portal/set-password", bytes.NewReader(bodyValid))
	reqValid.Header.Set("Content-Type", "application/json")
	recValid := httptest.NewRecorder()

	handler(recValid, reqValid)

	// Verify token cannot be used again (consumed)
	_, errVal := st.ValidateResetToken(validToken)
	if errVal == nil {
		t.Fatalf("expected token to be consumed/invalidated, but ValidateResetToken succeeded")
	}
}
