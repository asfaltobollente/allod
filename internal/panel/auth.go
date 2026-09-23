package panel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/asfaltobollente/allod/internal/state"
)

const (
	AdminCookieName  = "allod_admin_session"
	FamilyCookieName = "allod_family_session"
	HashIterations   = 10000
	SessionDuration  = 30 * 24 * time.Hour
)

// GenerateSalt creates a 32-byte cryptographically secure random salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate random salt: %w", err)
	}
	return salt, nil
}

// HashPassword computes a multi-round salted HMAC-SHA256 hash.
func HashPassword(password string, salt []byte) string {
	key := []byte(password)
	h := hmac.New(sha256.New, key)
	h.Write(salt)
	res := h.Sum(nil)
	for i := 1; i < HashIterations; i++ {
		h.Reset()
		h.Write(res)
		res = h.Sum(nil)
	}
	return hex.EncodeToString(res)
}

// VerifyPassword verifies a password against a hex-encoded salt and hash in constant time.
func VerifyPassword(password, saltHex, hashHex string) bool {
	salt, err := hex.DecodeString(saltHex)
	if err != nil || len(salt) == 0 {
		return false
	}
	computed := HashPassword(password, salt)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hashHex)) == 1
}

type loginAttempt struct {
	count       int
	firstFailed time.Time
	blockedTill time.Time
}

// RateLimiter tracks and blocks repeated failed authentication attempts.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
	maxFails int
	window   time.Duration
	blockDur time.Duration
}

// NewRateLimiter creates a new thread-safe rate limiter.
func NewRateLimiter(maxFails int, window, blockDur time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]*loginAttempt),
		maxFails: maxFails,
		window:   window,
		blockDur: blockDur,
	}
}

// IsBlocked checks if the specified IP address is currently locked out.
func (rl *RateLimiter) IsBlocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	att, exists := rl.attempts[ip]
	if !exists {
		return false
	}
	now := time.Now()
	if now.Before(att.blockedTill) {
		return true
	}
	if now.Sub(att.firstFailed) > rl.window && now.After(att.blockedTill) {
		delete(rl.attempts, ip)
		return false
	}
	return false
}

// RecordFailure notes a failed authentication attempt.
func (rl *RateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	att, exists := rl.attempts[ip]
	if !exists || now.Sub(att.firstFailed) > rl.window {
		rl.attempts[ip] = &loginAttempt{
			count:       1,
			firstFailed: now,
		}
		return
	}

	att.count++
	if att.count >= rl.maxFails {
		att.blockedTill = now.Add(rl.blockDur)
	}
}

// RecordSuccess clears any failure count for the IP address.
func (rl *RateLimiter) RecordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// SetSessionCookie sets a secure HttpOnly cookie.
func SetSessionCookie(w http.ResponseWriter, cookieName, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie invalidates the given cookie.
func ClearSessionCookie(w http.ResponseWriter, cookieName string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionToken retrieves the session token from cookie or authorization headers.
func GetSessionToken(r *http.Request, cookieName string) string {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if h := r.Header.Get("X-Allod-Admin-Token"); h != "" && cookieName == AdminCookieName {
		return h
	}
	if h := r.Header.Get("X-Allod-Family-Token"); h != "" && cookieName == FamilyCookieName {
		return h
	}
	return ""
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

// AuthHandler handles both admin dashboard authentication and family member portal operations.
type AuthHandler struct {
	DBPath            string
	Helper            HelperClient
	RateLimiter       *RateLimiter
	EnsureSystemUser  func(client HelperClient, username string) error
	SetSambaPassword  func(client HelperClient, username, password string) error
}

// NewAuthHandler creates an AuthHandler with defaults.
func NewAuthHandler(dbPath string, helperClient HelperClient) *AuthHandler {
	return &AuthHandler{
		DBPath:      dbPath,
		Helper:      helperClient,
		RateLimiter: NewRateLimiter(5, 5*time.Minute, 5*time.Minute),
	}
}

// RegisterAuthRoutes registers all authentication and family portal API routes on the mux.
func (h *AuthHandler) RegisterAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/status", h.handleAuthStatus)
	mux.HandleFunc("/api/auth/setup", h.handleAuthSetup)
	mux.HandleFunc("/api/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", h.handleAuthLogout)

	mux.HandleFunc("/api/portal/login", h.handlePortalLogin)
	mux.HandleFunc("/api/portal/me", h.handlePortalMe)
	mux.HandleFunc("/api/portal/change-password", h.handlePortalChangePassword)
	mux.HandleFunc("/api/portal/logout", h.handlePortalLogout)
}

// RequireAdminAuth wraps an HTTP handler and blocks unauthenticated requests.
func (h *AuthHandler) RequireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := state.Open(h.DBPath)
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer st.Close()

		hasAdmin, err := st.HasAdminAuth()
		if err != nil || !hasAdmin {
			if acceptsHTML(r) {
				http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: "Primo setup richiesto: configura la password amministratore su /login",
			})
			return
		}

		token := GetSessionToken(r, AdminCookieName)
		sess, err := st.GetSession(token)
		if err != nil || sess == nil || sess.UserType != "admin" {
			if acceptsHTML(r) {
				http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: "Autenticazione amministratore richiesta",
			})
			return
		}

		next(w, r)
	}
}

// handleAuthStatus returns whether admin password is configured and whether caller has active admin session.
func (h *AuthHandler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	st, err := state.Open(h.DBPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}
	defer st.Close()

	hasAdmin, err := st.HasAdminAuth()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}

	token := GetSessionToken(r, AdminCookieName)
	sess, _ := st.GetSession(token)
	isAdmin := sess != nil && sess.UserType == "admin"

	// Also check if family member is logged in
	famToken := GetSessionToken(r, FamilyCookieName)
	famSess, _ := st.GetSession(famToken)
	var famUsername string
	if famSess != nil && famSess.UserType == "family" {
		famUsername = famSess.Username
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status: "ok",
		Data: map[string]interface{}{
			"configured":     hasAdmin,
			"authenticated":  isAdmin,
			"admin_user":     "admin",
			"family_user":    famUsername,
			"is_family_auth": famUsername != "",
		},
	})
}

// handleAuthSetup performs first-time admin password creation.
func (h *AuthHandler) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	st, err := state.Open(h.DBPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}
	defer st.Close()

	hasAdmin, err := st.HasAdminAuth()
	if err == nil && hasAdmin {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Password amministratore già configurata. Esegui il login o usa 'allod admin-password reset' da CLI.",
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
		return
	}

	trimmedPass := strings.TrimSpace(req.Password)
	if len(trimmedPass) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La password deve contenere almeno 6 caratteri"})
		return
	}

	salt, err := GenerateSalt()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore generazione salt crittografico"})
		return
	}
	hash := HashPassword(trimmedPass, salt)
	saltHex := hex.EncodeToString(salt)

	if err := st.SetAdminAuth(hash, saltHex); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore salvataggio credenziali: " + err.Error()})
		return
	}

	token, err := st.CreateSession("admin", "admin", SessionDuration)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore generazione sessione"})
		return
	}

	SetSessionCookie(w, AdminCookieName, token, int(SessionDuration.Seconds()))

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Password amministratore configurata con successo!",
		Data: map[string]interface{}{
			"authenticated": true,
			"token":         token,
		},
	})
}

// handleAuthLogin logs in the administrator.
func (h *AuthHandler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ip := getClientIP(r)
	if h.RateLimiter != nil && h.RateLimiter.IsBlocked(ip) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Troppi tentativi errati consecutivi. Accesso temporaneamente bloccato per 5 minuti per sicurezza.",
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
		return
	}

	st, err := state.Open(h.DBPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}
	defer st.Close()

	hasAdmin, err := st.HasAdminAuth()
	if err != nil || !hasAdmin {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Sistema non ancora configurato. Imposta prima la password iniziale su /login",
		})
		return
	}

	storedHash, saltHex, err := st.GetAdminAuth()
	if err != nil || storedHash == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore recupero configurazione admin"})
		return
	}

	if !VerifyPassword(req.Password, saltHex, storedHash) {
		if h.RateLimiter != nil {
			h.RateLimiter.RecordFailure(ip)
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Password amministratore non corretta",
		})
		return
	}

	if h.RateLimiter != nil {
		h.RateLimiter.RecordSuccess(ip)
	}

	token, err := st.CreateSession("admin", "admin", SessionDuration)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore creazione sessione"})
		return
	}

	SetSessionCookie(w, AdminCookieName, token, int(SessionDuration.Seconds()))

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Autenticazione riuscita",
		Data: map[string]interface{}{
			"authenticated": true,
			"token":         token,
		},
	})
}

// handleAuthLogout terminates the admin session.
func (h *AuthHandler) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token := GetSessionToken(r, AdminCookieName)
	if token != "" {
		if st, err := state.Open(h.DBPath); err == nil {
			_ = st.DeleteSession(token)
			st.Close()
		}
	}

	ClearSessionCookie(w, AdminCookieName)

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Sessione terminata con successo",
	})
}

// handlePortalLogin logs in a family member.
func (h *AuthHandler) handlePortalLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ip := getClientIP(r)
	if h.RateLimiter != nil && h.RateLimiter.IsBlocked(ip) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Troppi tentativi errati consecutivi. Accesso temporaneamente bloccato per 5 minuti per sicurezza.",
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Username e password sono obbligatori"})
		return
	}

	st, err := state.Open(h.DBPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}
	defer st.Close()

	member, err := st.GetFamilyMember(username)
	if err != nil || member == nil {
		if h.RateLimiter != nil {
			h.RateLimiter.RecordFailure(ip)
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Nome utente o password non validi",
		})
		return
	}

	if member.PasswordHash == "" || member.PasswordSalt == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Nessuna password ancora impostata per questo account. Chiedi all'amministratore un link o QR di benvenuto per attivarla.",
		})
		return
	}

	if !VerifyPassword(req.Password, member.PasswordSalt, member.PasswordHash) {
		if h.RateLimiter != nil {
			h.RateLimiter.RecordFailure(ip)
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Nome utente o password non validi",
		})
		return
	}

	if h.RateLimiter != nil {
		h.RateLimiter.RecordSuccess(ip)
	}

	token, err := st.CreateSession("family", member.Username, SessionDuration)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore generazione sessione"})
		return
	}

	SetSessionCookie(w, FamilyCookieName, token, int(SessionDuration.Seconds()))

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: fmt.Sprintf("Bentornato %s!", member.FirstName),
		Data: map[string]interface{}{
			"username":   member.Username,
			"first_name": member.FirstName,
			"last_name":  member.LastName,
			"role":       member.Role,
			"token":      token,
		},
	})
}

// handlePortalMe returns current family member details and resources.
func (h *AuthHandler) handlePortalMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	token := GetSessionToken(r, FamilyCookieName)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Sessione non valida o non presente"})
		return
	}

	st, err := state.Open(h.DBPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}
	defer st.Close()

	sess, err := st.GetSession(token)
	if err != nil || sess == nil || sess.UserType != "family" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Sessione scaduta o non valida"})
		return
	}

	member, err := st.GetFamilyMember(sess.Username)
	if err != nil || member == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Utente non trovato"})
		return
	}

	hostName, _ := os.Hostname()
	if hostName == "" {
		hostName = "allod"
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status: "ok",
		Data: map[string]interface{}{
			"username":      member.Username,
			"first_name":    member.FirstName,
			"last_name":     member.LastName,
			"role":          member.Role,
			"email":         member.Email,
			"avatar_color":  member.AvatarColor,
			"smb_active":    member.SmbActive,
			"photos_linked": member.PhotosLinked,
			"hostname":      hostName,
			"smb_path_win":  fmt.Sprintf(`\\%s\%s`, hostName, member.Username),
			"smb_path_mac":  fmt.Sprintf("smb://%s/%s", hostName, member.Username),
		},
	})
}

// handlePortalChangePassword allows a logged-in family member to change their password autonomously.
func (h *AuthHandler) handlePortalChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
		return
	}

	if len(req.NewPassword) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La nuova password deve contenere almeno 4 caratteri"})
		return
	}

	token := GetSessionToken(r, FamilyCookieName)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Accesso non autorizzato. Effettua prima il login."})
		return
	}

	st, err := state.Open(h.DBPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
		return
	}
	defer st.Close()

	sess, err := st.GetSession(token)
	if err != nil || sess == nil || sess.UserType != "family" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Sessione scaduta. Effettua nuovamente il login."})
		return
	}

	member, err := st.GetFamilyMember(sess.Username)
	if err != nil || member == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Utente non trovato"})
		return
	}

	// Verify current password if user already had one set
	if member.PasswordHash != "" {
		if !VerifyPassword(req.CurrentPassword, member.PasswordSalt, member.PasswordHash) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: "La password attuale inserita non è corretta",
			})
			return
		}
	}

	// Generate salt and hash for new password
	salt, err := GenerateSalt()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore crittografico"})
		return
	}
	hash := HashPassword(req.NewPassword, salt)
	saltHex := hex.EncodeToString(salt)

	if err := st.SetFamilyMemberPassword(member.Username, hash, saltHex); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore salvataggio nuova password: " + err.Error()})
		return
	}

	// Synchronously update Samba & Linux credentials
	if h.EnsureSystemUser != nil && h.Helper != nil {
		_ = h.EnsureSystemUser(h.Helper, member.Username)
	}
	if h.SetSambaPassword != nil && h.Helper != nil {
		if errSmb := h.SetSambaPassword(h.Helper, member.Username, req.NewPassword); errSmb != nil {
			// Retry once
			if h.EnsureSystemUser != nil {
				_ = h.EnsureSystemUser(h.Helper, member.Username)
			}
			_ = h.SetSambaPassword(h.Helper, member.Username, req.NewPassword)
		}
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Password aggiornata con successo! È ora attiva per l'accesso a Samba e a tutti i tuoi servizi.",
		Data: map[string]interface{}{
			"username": member.Username,
		},
	})
}

// handlePortalLogout clears family session.
func (h *AuthHandler) handlePortalLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token := GetSessionToken(r, FamilyCookieName)
	if token != "" {
		if st, err := state.Open(h.DBPath); err == nil {
			_ = st.DeleteSession(token)
			st.Close()
		}
	}

	ClearSessionCookie(w, FamilyCookieName)

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Disconnesso dal Portale Famiglia",
	})
}
