package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type WatchInstallTokenStore struct {
	mu     sync.Mutex
	tokens map[string]time.Time // token -> expiry
}

var globalWatchInstallTokens = &WatchInstallTokenStore{
	tokens: make(map[string]time.Time),
}

// Generate creates a cryptographically secure single-use token with a specified TTL.
func (s *WatchInstallTokenStore) Generate(ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean expired tokens
	now := time.Now()
	for tok, exp := range s.tokens {
		if now.After(exp) {
			delete(s.tokens, tok)
		}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	s.tokens[token] = now.Add(ttl)
	return token, nil
}

// ValidateAndConsume checks if the token is valid, not expired, and consumes it (single-use).
func (s *WatchInstallTokenStore) ValidateAndConsume(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.tokens[token]
	if !ok {
		return false
	}
	delete(s.tokens, token)
	return time.Now().Before(exp)
}

// HandleWatchInstallToken handles POST /api/watch/install-token (generates a 15-minute single-use token).
func HandleWatchInstallToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	tok, err := globalWatchInstallTokens.Generate(15 * time.Minute)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore generazione token: " + err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"token":  tok,
	})
}
