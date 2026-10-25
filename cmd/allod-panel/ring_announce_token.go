package main

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var validVaultNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var validVaultHostnameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func isValidVaultAddress(addr string) bool {
	clean := strings.TrimSpace(addr)
	if clean == "" {
		return false
	}
	if net.ParseIP(clean) != nil {
		return true
	}
	host := strings.ToLower(strings.TrimSuffix(clean, ".local"))
	return validVaultHostnameRegex.MatchString(host)
}

// RingAnnounceTokenStore tracks single-use or time-bounded tokens for registering vault nodes into ring.yaml.
type RingAnnounceTokenStore struct {
	mu     sync.Mutex
	tokens map[string]time.Time // token -> expiry
}

var globalRingAnnounceTokens = &RingAnnounceTokenStore{
	tokens: make(map[string]time.Time),
}

// Generate creates a cryptographically secure token with a specified TTL (e.g. 24 hours).
func (s *RingAnnounceTokenStore) Generate(ttl time.Duration) (string, error) {
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
func (s *RingAnnounceTokenStore) ValidateAndConsume(token string) bool {
	clean := strings.TrimSpace(token)
	if clean == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.tokens[clean]
	if !ok {
		return false
	}
	delete(s.tokens, clean)
	return time.Now().Before(exp)
}

// Peek checks if the token is currently valid without consuming it (useful for pre-flight auth checks).
func (s *RingAnnounceTokenStore) Peek(token string) bool {
	clean := strings.TrimSpace(token)
	if clean == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.tokens[clean]
	if !ok {
		return false
	}
	return time.Now().Before(exp)
}

// extractRingAnnounceToken extracts the announce token from Authorization: Bearer <tok> or ?token=<tok>.
func extractRingAnnounceToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	if tok := strings.TrimSpace(r.URL.Query().Get("token")); tok != "" {
		return tok
	}
	return ""
}
