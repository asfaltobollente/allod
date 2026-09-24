package watch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherOutageAndRecovery(t *testing.T) {
	var isOnline atomic.Bool
	isOnline.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isOnline.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		payload := NodeHealthPayload{
			Status:        "ok",
			NodeName:      "test-node",
			UptimeSeconds: 3600,
			StorageOK:     true,
			StorageUsed:   "50 GB",
			StorageFree:   "450 GB",
			MemoryUsedMB:  2048,
			MemoryTotalMB: 8192,
			ActiveModules: []string{"photos", "shares"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	w := NewWatcher(100*time.Millisecond, 20*time.Millisecond)
	w.failureCountLimit = 1

	var alertReceived atomic.Bool
	var recoveryReceived atomic.Bool

	w.OnAlert = func(nodeID, reason string, downtime time.Duration) {
		alertReceived.Store(true)
	}

	w.OnRecover = func(nodeID string, totalDowntime time.Duration) {
		recoveryReceived.Store(true)
	}

	w.AddNode("test-node", server.URL, "")

	// 1. Initial check: server is online
	w.checkPeers()
	status, ok := w.GetPeerStatus("test-node")
	if !ok || !status.IsOnline {
		t.Fatalf("Nodo dovrebbe essere inizialmente online")
	}

	// 2. Simulate server failure
	isOnline.Store(false)
	// First failed check starts downtime
	w.checkPeers()
	// Wait past alert threshold
	time.Sleep(150 * time.Millisecond)
	// Second failed check triggers alert
	w.checkPeers()

	// Wait for alert callback goroutine
	for i := 0; i < 50 && !alertReceived.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	if !alertReceived.Load() {
		t.Errorf("Allarme non ricevuto dopo che il server è andato offline")
	}

	status, _ = w.GetPeerStatus("test-node")
	if status.IsOnline {
		t.Errorf("Stato del nodo dovrebbe essere offline")
	}

	// 3. Simulate recovery
	isOnline.Store(true)
	w.checkPeers()

	// Wait for recovery callback goroutine
	for i := 0; i < 50 && !recoveryReceived.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	if !recoveryReceived.Load() {
		t.Errorf("Notifica di rientro non ricevuta dopo il ripristino")
	}

	status, _ = w.GetPeerStatus("test-node")
	if !status.IsOnline {
		t.Errorf("Stato del nodo dovrebbe essere tornato online")
	}
}

func TestReceiverPushMode(t *testing.T) {
	w := NewWatcher(100*time.Millisecond, 20*time.Millisecond)
	w.SetSecretToken("secret-token-123")

	var alertReceived atomic.Bool
	var recoveryReceived atomic.Bool
	var warningReceived atomic.Bool

	w.OnAlert = func(nodeID, reason string, downtime time.Duration) {
		alertReceived.Store(true)
	}

	w.OnRecover = func(nodeID string, totalDowntime time.Duration) {
		recoveryReceived.Store(true)
	}

	w.OnWarning = func(nodeID, title, details string) {
		warningReceived.Store(true)
	}

	// 1. Send valid heartbeat
	payload := &NodeHealthPayload{
		NodeName:      "nas-server",
		Status:        "ok",
		UptimeSeconds: 12000,
		StorageOK:     true,
		CPUTemp:       45.0,
		MemoryUsedMB:  1024,
		MemoryTotalMB: 4096,
	}

	// Test bad token
	err := w.HandleHeartbeat(payload, "wrong-token")
	if err == nil {
		t.Fatalf("expected error on invalid token, got nil")
	}

	// Test good token
	err = w.HandleHeartbeat(payload, "secret-token-123")
	if err != nil {
		t.Fatalf("unexpected error on valid heartbeat: %v", err)
	}

	peer, ok := w.GetPeerStatus("nas-server")
	if !ok || !peer.IsOnline || peer.LatestPayload == nil {
		t.Fatalf("peer should be registered and online")
	}

	// 2. Simulate timeout past threshold
	time.Sleep(150 * time.Millisecond)
	w.CheckDeadManTimeout()

	for i := 0; i < 50 && !alertReceived.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	if !alertReceived.Load() {
		t.Errorf("expected dead man's timeout alert, but none received")
	}

	peer, _ = w.GetPeerStatus("nas-server")
	if peer.IsOnline {
		t.Errorf("peer should be marked offline after timeout")
	}

	// 3. Heartbeat recovery
	err = w.HandleHeartbeat(payload, "Bearer secret-token-123")
	if err != nil {
		t.Fatalf("recovery heartbeat failed: %v", err)
	}

	for i := 0; i < 50 && !recoveryReceived.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	if !recoveryReceived.Load() {
		t.Errorf("expected recovery notification, but none received")
	}

	peer, _ = w.GetPeerStatus("nas-server")
	if !peer.IsOnline {
		t.Errorf("peer should be marked online after recovery")
	}

	// 4. Test High Temperature Warning
	hotPayload := &NodeHealthPayload{
		NodeName: "nas-server",
		StorageOK: true,
		CPUTemp: 85.5,
	}
	_ = w.HandleHeartbeat(hotPayload, "secret-token-123")
	for i := 0; i < 50 && !warningReceived.Load(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if !warningReceived.Load() {
		t.Errorf("expected high temp warning, but none received")
	}
}

