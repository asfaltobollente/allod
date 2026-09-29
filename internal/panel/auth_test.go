package panel

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asfaltobollente/allod/internal/helper"
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

func TestPortalLoginAutoSyncWithSamba(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")

	store, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	// Create user 'davide' without password (as imported from Samba share)
	err = store.CreateFamilyMember(&state.FamilyMember{
		Username:     "davide",
		FirstName:    "Davide",
		LastName:     "Arrus",
		Role:         "member",
		SmbActive:    true,
		PasswordHash: "",
		PasswordSalt: "",
	})
	if err != nil {
		t.Fatalf("failed to create member: %v", err)
	}
	store.Close()

	// Mock helper that verifies Samba credentials
	mockHelper := &mockHelperClient{
		executeFn: func(action string, args map[string]interface{}, plan bool) (helper.Response, error) {
			if action == "shares.verify_password" {
				user, _ := args["username"].(string)
				pass, _ := args["password"].(string)
				if user == "davide" && pass == "DavideSambaPass2026!" {
					return helper.Response{Ok: true, Applied: true, Output: "valid"}, nil
				}
				return helper.Response{Ok: true, Applied: true, Output: "invalid"}, nil
			}
			return helper.Response{Ok: true}, nil
		},
	}

	authHandler := NewAuthHandler(dbPath, mockHelper)
	mux := http.NewServeMux()
	authHandler.RegisterAuthRoutes(mux)

	// 1. Try login with wrong password -> 400 with "Nessuna password ancora impostata"
	badLogin := bytes.NewBufferString(`{"username":"davide","password":"WrongPassword"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/portal/login", badLogin)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unverified password, got %d", rec.Code)
	}

	// 2. Try login with valid Samba password -> 200 OK & auto-activates!
	goodLogin := bytes.NewBufferString(`{"username":"davide","password":"DavideSambaPass2026!"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/portal/login", goodLogin)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for auto-sync login, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Verify in state.db that password_hash is now populated
	stCheck, _ := state.Open(dbPath)
	mem, _ := stCheck.GetFamilyMember("davide")
	stCheck.Close()
	if mem.PasswordHash == "" || mem.PasswordSalt == "" {
		t.Errorf("expected password hash and salt to be populated in state.db after auto-sync")
	}

	// 4. Subsequent login works directly via PBKDF2
	rec2 := httptest.NewRecorder()
	goodLogin2 := bytes.NewBufferString(`{"username":"davide","password":"DavideSambaPass2026!"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/portal/login", goodLogin2)
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 on second login, got %d", rec2.Code)
	}
}

func TestGetClientIPRemoteAddrVersusForwarded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	req.Header.Set("X-Real-IP", "10.0.0.99")

	// Default: untrusted proxy, must ignore headers and use RemoteAddr
	ip := getClientIP(req)
	if ip != "192.168.1.100" {
		t.Errorf("expected getClientIP to return %q without trusted proxy, got %q", "192.168.1.100", ip)
	}

	// With ALLOD_TRUSTED_PROXY=true, honors X-Forwarded-For
	t.Setenv("ALLOD_TRUSTED_PROXY", "true")
	trustedIP := getClientIP(req)
	if trustedIP != "10.0.0.1" {
		t.Errorf("expected getClientIP to return %q with trusted proxy, got %q", "10.0.0.1", trustedIP)
	}
}

func TestArgon2idFormatAndVerification(t *testing.T) {
	password := "CorrectHorseBatteryStaple123!"

	hash, err := HashPasswordArgon2(password)
	if err != nil {
		t.Fatalf("HashPasswordArgon2 failed: %v", err)
	}

	if !strings.HasPrefix(hash, "argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}

	// Verify correct password with empty saltHex (salt extracted from hash)
	if !VerifyPassword(password, "", hash) {
		t.Errorf("expected verification with embedded salt to succeed")
	}

	// Verify wrong password fails
	if VerifyPassword("WrongPassword123!", "", hash) {
		t.Errorf("expected wrong password verification to fail")
	}

	// Verify corrupted hash fails
	if VerifyPassword(password, "", "argon2id$invalid$format") {
		t.Errorf("expected malformed hash to fail")
	}
}

func TestLegacyPasswordFallbackAndTransparentRehash(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-rehash.db")

	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open state db: %v", err)
	}

	// 1. Seed admin with legacy HMAC-SHA256 hash
	adminPass := "LegacyAdminPass123"
	saltBytes := []byte("1234567890123456")
	legacyHash := legacyHashPassword(adminPass, saltBytes)
	saltHex := hex.EncodeToString(saltBytes)

	if err := st.SetAdminAuth(legacyHash, saltHex); err != nil {
		t.Fatalf("failed to set admin auth: %v", err)
	}

	// 2. Seed family member with legacy hash
	familyPass := "LegacyFamilyPass123"
	famSaltBytes := []byte("abcdefabcdefabcd")
	famLegacyHash := legacyHashPassword(familyPass, famSaltBytes)
	famSaltHex := hex.EncodeToString(famSaltBytes)

	if err := st.CreateFamilyMember(&state.FamilyMember{
		Username:     "giulia",
		FirstName:    "Giulia",
		LastName:     "Verdi",
		Role:         "member",
		SmbActive:    true,
		PasswordHash: famLegacyHash,
		PasswordSalt: famSaltHex,
	}); err != nil {
		t.Fatalf("failed to create family member: %v", err)
	}
	st.Close()

	handler := NewAuthHandler(dbPath, nil)
	mux := http.NewServeMux()
	handler.RegisterAuthRoutes(mux)

	// 3. Admin login with legacy hash should succeed
	adminLoginBody := bytes.NewBufferString(fmt.Sprintf(`{"username":"admin","password":%q}`, adminPass))
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", adminLoginBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on admin login with legacy hash, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Verify admin hash in state.db has been transparently rehashed to argon2id
	stCheck, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	newAdminHash, _, err := stCheck.GetAdminAuth()
	stCheck.Close()
	if err != nil || !strings.HasPrefix(newAdminHash, "argon2id$") {
		t.Errorf("expected admin hash to be rehashed to argon2id, got: %s", newAdminHash)
	}

	// 5. Subsequent admin login works with rehashed password
	adminLoginBody2 := bytes.NewBufferString(fmt.Sprintf(`{"username":"admin","password":%q}`, adminPass))
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", adminLoginBody2)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on second admin login with rehashed argon2id, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 6. Family login with legacy hash should succeed
	famLoginBody := bytes.NewBufferString(fmt.Sprintf(`{"username":"giulia","password":%q}`, familyPass))
	reqFam := httptest.NewRequest(http.MethodPost, "/api/portal/login", famLoginBody)
	recFam := httptest.NewRecorder()
	mux.ServeHTTP(recFam, reqFam)
	if recFam.Code != http.StatusOK {
		t.Fatalf("expected 200 on family login with legacy hash, got %d: %s", recFam.Code, recFam.Body.String())
	}

	// 7. Verify family member hash has been transparently rehashed to argon2id
	stCheckFam, _ := state.Open(dbPath)
	memGiulia, _ := stCheckFam.GetFamilyMember("giulia")
	stCheckFam.Close()
	if memGiulia == nil || !strings.HasPrefix(memGiulia.PasswordHash, "argon2id$") {
		t.Errorf("expected giulia's hash to be rehashed to argon2id, got: %v", memGiulia)
	}
}

func TestPasswordMinimumLengths(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-minpass.db")

	handler := NewAuthHandler(dbPath, nil)
	mux := http.NewServeMux()
	handler.RegisterAuthRoutes(mux)

	// Admin setup requires >= 12 chars: test with 11 chars -> 400
	shortAdmin := bytes.NewBufferString(`{"password":"shortAdmin1"}`) // 11 chars
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", shortAdmin)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for admin password < 12 chars, got %d", rec.Code)
	}

	// Admin setup with 13 chars -> 200
	validAdmin := bytes.NewBufferString(`{"password":"validAdmin123"}`) // 13 chars
	reqValid := httptest.NewRequest(http.MethodPost, "/api/auth/setup", validAdmin)
	recValid := httptest.NewRecorder()
	mux.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Errorf("expected 200 for valid admin password, got %d", recValid.Code)
	}

	// Family change password requires >= 8 chars: test with 7 chars -> 400
	st, _ := state.Open(dbPath)
	_ = st.CreateFamilyMember(&state.FamilyMember{
		Username:     "testuser",
		FirstName:    "Test",
		Role:         "member",
		PasswordHash: HashPassword("InitialSecretPass1", nil),
	})
	token, _ := st.CreateSession("family", "testuser", SessionDuration)
	st.Close()

	shortFam := bytes.NewBufferString(`{"current_password":"InitialSecretPass1","new_password":"short12"}`) // 7 chars
	reqFam := httptest.NewRequest(http.MethodPost, "/api/portal/change-password", shortFam)
	reqFam.AddCookie(&http.Cookie{Name: FamilyCookieName, Value: token})
	recFam := httptest.NewRecorder()
	mux.ServeHTTP(recFam, reqFam)
	if recFam.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for family password < 8 chars, got %d", recFam.Code)
	}
}

func TestAuthResponsesDoNotExposeTokenInJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	h := NewAuthHandler(dbPath, nil)
	mux := http.NewServeMux()
	h.RegisterAuthRoutes(mux)

	// 1. Setup admin
	bodySetup := bytes.NewBufferString(`{"password":"validAdmin123456"}`)
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bodySetup)
	recSetup := httptest.NewRecorder()
	mux.ServeHTTP(recSetup, reqSetup)

	if recSetup.Code != http.StatusOK {
		t.Fatalf("expected 200 from setup, got %d", recSetup.Code)
	}
	if strings.Contains(recSetup.Body.String(), `"token"`) {
		t.Errorf("SECURITY: /api/auth/setup leaked token in JSON: %s", recSetup.Body.String())
	}
	if !strings.Contains(recSetup.Header().Get("Set-Cookie"), AdminCookieName) {
		t.Errorf("expected Set-Cookie with %s", AdminCookieName)
	}

	// 2. Login admin
	bodyLogin := bytes.NewBufferString(`{"password":"validAdmin123456"}`)
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bodyLogin)
	recLogin := httptest.NewRecorder()
	mux.ServeHTTP(recLogin, reqLogin)

	if recLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 from login, got %d", recLogin.Code)
	}
	if strings.Contains(recLogin.Body.String(), `"token"`) {
		t.Errorf("SECURITY: /api/auth/login leaked token in JSON: %s", recLogin.Body.String())
	}
	if !strings.Contains(recLogin.Header().Get("Set-Cookie"), AdminCookieName) {
		t.Errorf("expected Set-Cookie with %s", AdminCookieName)
	}

	// 3. Portal login for family member
	salt, _ := GenerateSalt()
	saltHex := hex.EncodeToString(salt)
	st, _ := state.Open(dbPath)
	_ = st.CreateFamilyMember(&state.FamilyMember{
		Username:     "familymember",
		FirstName:    "Family",
		LastName:     "Member",
		Role:         "member",
		PasswordHash: HashPassword("SuperFamilySecretPass123", salt),
		PasswordSalt: saltHex,
	})
	st.Close()

	bodyPortal := bytes.NewBufferString(`{"username":"familymember","password":"SuperFamilySecretPass123"}`)
	reqPortal := httptest.NewRequest(http.MethodPost, "/api/portal/login", bodyPortal)
	recPortal := httptest.NewRecorder()
	mux.ServeHTTP(recPortal, reqPortal)

	if recPortal.Code != http.StatusOK {
		t.Fatalf("expected 200 from portal login, got %d", recPortal.Code)
	}
	if strings.Contains(recPortal.Body.String(), `"token"`) {
		t.Errorf("SECURITY: /api/portal/login leaked token in JSON: %s", recPortal.Body.String())
	}
	if !strings.Contains(recPortal.Header().Get("Set-Cookie"), FamilyCookieName) {
		t.Errorf("expected Set-Cookie with %s", FamilyCookieName)
	}
}
