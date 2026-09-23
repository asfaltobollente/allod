package panel

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/asfaltobollente/allod/internal/state"
)

func TestPasswordHashingAndVerification(t *testing.T) {
	password := "SuperSecret123!"

	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	if len(salt) != 32 {
		t.Fatalf("expected 32 byte salt, got %d", len(salt))
	}

	hash := HashPassword(password, salt)
	saltHex := hex.EncodeToString(salt)

	if !VerifyPassword(password, saltHex, hash) {
		t.Errorf("expected password verification to succeed")
	}

	if VerifyPassword("WrongPassword!", saltHex, hash) {
		t.Errorf("expected wrong password to fail verification")
	}

	if VerifyPassword(password, "invalid_salt", hash) {
		t.Errorf("expected invalid salt to fail verification")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, 100*time.Millisecond, 200*time.Millisecond)
	ip := "192.168.1.50"

	if rl.IsBlocked(ip) {
		t.Errorf("new IP should not be blocked")
	}

	rl.RecordFailure(ip)
	rl.RecordFailure(ip)
	if rl.IsBlocked(ip) {
		t.Errorf("should not be blocked after 2 failures (max 3)")
	}

	rl.RecordFailure(ip)
	if !rl.IsBlocked(ip) {
		t.Errorf("should be blocked after 3 failures")
	}

	// Wait for block duration to expire
	time.Sleep(210 * time.Millisecond)
	if rl.IsBlocked(ip) {
		t.Errorf("should unblock after block duration")
	}

	// Test success resets
	rl.RecordFailure(ip)
	rl.RecordSuccess(ip)
	rl.RecordFailure(ip)
	rl.RecordFailure(ip)
	if rl.IsBlocked(ip) {
		t.Errorf("counter should have been reset by success")
	}
}

func TestAdminAuthFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-auth.db")

	handler := NewAuthHandler(dbPath, nil)
	mux := http.NewServeMux()
	handler.RegisterAuthRoutes(mux)

	// 1. Initial status: unconfigured
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp PanelResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if data["configured"].(bool) != false || data["authenticated"].(bool) != false {
		t.Errorf("expected configured=false, authenticated=false, got: %+v", data)
	}

	// 2. Setup password
	setupBody := bytes.NewBufferString(`{"password":"adminPassword456"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/setup", setupBody)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("setup failed with code %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	var adminCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == AdminCookieName {
			adminCookie = c
			break
		}
	}
	if adminCookie == nil || adminCookie.Value == "" {
		t.Fatalf("expected session cookie set after setup")
	}

	// 3. Second setup should fail
	setupBody2 := bytes.NewBufferString(`{"password":"anotherPassword"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/setup", setupBody2)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on duplicate setup, got %d", rec.Code)
	}

	// 4. Status with cookie should be authenticated
	req = httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&resp)
	data = resp.Data.(map[string]interface{})
	if data["configured"].(bool) != true || data["authenticated"].(bool) != true {
		t.Errorf("expected configured=true, authenticated=true, got: %+v", data)
	}

	// 5. Protected route test with RequireAdminAuth
	protectedCalled := false
	protectedHandler := handler.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		protectedCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Without cookie -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	rec = httptest.NewRecorder()
	protectedHandler(rec, req)
	if rec.Code != http.StatusUnauthorized || protectedCalled {
		t.Errorf("expected 401 unauthorized, got %d", rec.Code)
	}

	// With cookie -> 200
	protectedCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	protectedHandler(rec, req)
	if rec.Code != http.StatusOK || !protectedCalled {
		t.Errorf("expected 200 ok, got %d", rec.Code)
	}

	// 6. Logout
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("logout failed: %d", rec.Code)
	}

	// 7. Protected route after logout -> 401
	protectedCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	protectedHandler(rec, req)
	if rec.Code != http.StatusUnauthorized || protectedCalled {
		t.Errorf("expected 401 after logout, got %d", rec.Code)
	}

	// 8. Login with wrong password -> 401
	loginBad := bytes.NewBufferString(`{"username":"admin","password":"WrongPassword"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBad)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on bad password, got %d", rec.Code)
	}

	// 9. Login with correct password -> 200 & new cookie
	loginGood := bytes.NewBufferString(`{"username":"admin","password":"adminPassword456"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", loginGood)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFamilyPortalAuthFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-portal-auth.db")

	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open state db: %v", err)
	}
	defer st.Close()

	// Seed family member
	m := &state.FamilyMember{
		Username:     "marco",
		FirstName:    "Marco",
		LastName:     "Bianchi",
		Role:         "member",
		SmbActive:    true,
		PhotosLinked: true,
	}
	if err := st.CreateFamilyMember(m); err != nil {
		t.Fatalf("failed to create marco: %v", err)
	}

	// Set initial password for Marco
	salt, _ := GenerateSalt()
	hash := HashPassword("MarcoInitialPass1", salt)
	if err := st.SetFamilyMemberPassword("marco", hash, hex.EncodeToString(salt)); err != nil {
		t.Fatalf("failed to set marco password: %v", err)
	}

	handler := NewAuthHandler(dbPath, nil)
	mux := http.NewServeMux()
	handler.RegisterAuthRoutes(mux)

	// 1. Login Marco with wrong password -> 401
	badLogin := bytes.NewBufferString(`{"username":"marco","password":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/portal/login", badLogin)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on bad login, got %d", rec.Code)
	}

	// 2. Login Marco with correct password -> 200 & family cookie
	goodLogin := bytes.NewBufferString(`{"username":"marco","password":"MarcoInitialPass1"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/portal/login", goodLogin)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var famCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == FamilyCookieName {
			famCookie = c
			break
		}
	}
	if famCookie == nil || famCookie.Value == "" {
		t.Fatalf("expected family session cookie set")
	}

	// 3. GET /api/portal/me
	req = httptest.NewRequest(http.MethodGet, "/api/portal/me", nil)
	req.AddCookie(famCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/portal/me, got %d", rec.Code)
	}
	var meResp PanelResponse
	json.NewDecoder(rec.Body).Decode(&meResp)
	meData := meResp.Data.(map[string]interface{})
	if meData["username"] != "marco" || meData["first_name"] != "Marco" {
		t.Errorf("unexpected meData: %+v", meData)
	}

	// 4. Change password autonomously
	changeBody := bytes.NewBufferString(`{"current_password":"MarcoInitialPass1","new_password":"MarcoNewSecretPassword2026"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/portal/change-password", changeBody)
	req.AddCookie(famCookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on change-password, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Old password should now fail
	oldLogin := bytes.NewBufferString(`{"username":"marco","password":"MarcoInitialPass1"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/portal/login", oldLogin)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when logging in with old password, got %d", rec.Code)
	}

	// 6. New password should succeed
	newGoodLogin := bytes.NewBufferString(`{"username":"marco","password":"MarcoNewSecretPassword2026"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/portal/login", newGoodLogin)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with new password, got %d", rec.Code)
	}
}
