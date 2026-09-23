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
