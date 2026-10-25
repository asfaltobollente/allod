package main

import (
	"testing"
	"time"
)

func TestRingAnnounceTokenLifecycle(t *testing.T) {
	store := &RingAnnounceTokenStore{
		tokens: make(map[string]time.Time),
	}

	// 1. Generate token
	tok, err := store.Generate(1 * time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if len(tok) < 16 {
		t.Fatalf("token too short: %s", tok)
	}

	// 2. Peek should return true without consuming
	if !store.Peek(tok) {
		t.Fatalf("expected peek to succeed for valid token %s", tok)
	}
	if !store.Peek(tok) {
		t.Fatalf("expected second peek to succeed without consuming token %s", tok)
	}

	// 3. Validate and consume
	if !store.ValidateAndConsume(tok) {
		t.Fatalf("expected token %s to be valid", tok)
	}

	// 4. Second attempt must fail (single-use)
	if store.ValidateAndConsume(tok) {
		t.Fatalf("expected token %s to be consumed already", tok)
	}
	if store.Peek(tok) {
		t.Fatalf("expected peek to fail after consumption")
	}

	// 5. Expired token must fail
	expTok, err := store.Generate(-1 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	if store.Peek(expTok) {
		t.Fatalf("expected expired token to fail peek")
	}
	if store.ValidateAndConsume(expTok) {
		t.Fatalf("expected expired token to fail validation")
	}

	// 6. Unknown or empty token must fail
	if store.ValidateAndConsume("unknown-token-xyz") {
		t.Fatalf("expected unknown token to fail validation")
	}
	if store.ValidateAndConsume("") {
		t.Fatalf("expected empty token to fail validation")
	}
}
