package watch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NodeHealthPayload defines the structure returned by /api/health on the Allod server.
type NodeHealthPayload struct {
	Status         string   `json:"status"`
	NodeName       string   `json:"node_name"`
	UptimeSeconds  int64    `json:"uptime_seconds"`
	StorageOK      bool     `json:"storage_ok"`
	StorageUsed    string   `json:"storage_used,omitempty"`
	StorageFree    string   `json:"storage_free,omitempty"`
	StorageFreePct int      `json:"storage_free_pct,omitempty"`
	CPULoad        float64  `json:"cpu_load,omitempty"`
	MemoryUsedMB   int      `json:"memory_used_mb,omitempty"`
	MemoryTotalMB  int      `json:"memory_total_mb,omitempty"`
	ActiveModules  []string `json:"active_modules,omitempty"`
}

// PeerStatus tracks the real-time connectivity and health of a monitored node.
type PeerStatus struct {
	ID                  string
	URL                 string
	Token               string
	LastSeen            time.Time
	DowntimeStart       time.Time
	IsOnline            bool
	ConsecutiveFailures int
	AlertSent           bool
	LatestPayload       *NodeHealthPayload
	StorageWarningSent  bool
}

// Watcher manages background polling of one or more Allod nodes.
type Watcher struct {
	mu                  sync.Mutex
	peers               map[string]*PeerStatus
	alertThreshold      time.Duration
	checkInterval       time.Duration
	failureCountLimit   int
	stopChan            chan struct{}
	client              *http.Client
	OnAlert             func(nodeID, reason string, downtime time.Duration)
	OnRecover           func(nodeID string, totalDowntime time.Duration)
	OnWarning           func(nodeID, title, details string)
}

// NewWatcher creates an enhanced watcher.
func NewWatcher(alertThreshold, checkInterval time.Duration) *Watcher {
	if alertThreshold <= 0 {
		alertThreshold = 3 * time.Minute
	}
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}

	return &Watcher{
		peers:             make(map[string]*PeerStatus),
		alertThreshold:    alertThreshold,
		checkInterval:     checkInterval,
		failureCountLimit: 3,
		stopChan:          make(chan struct{}),
		client:            &http.Client{Timeout: 5 * time.Second},
	}
}

// AddNode registers a target node for periodic health inspection.
func (w *Watcher) AddNode(id, rawURL, token string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	targetURL := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}
	if !strings.HasSuffix(targetURL, "/api/health") {
		targetURL = strings.TrimSuffix(targetURL, "/") + "/api/health"
	}

	w.peers[id] = &PeerStatus{
		ID:                  id,
		URL:                 targetURL,
		Token:               token,
		LastSeen:            time.Now(),
		IsOnline:            true,
		ConsecutiveFailures: 0,
		AlertSent:           false,
	}
}

// AddPeer preserves backward-compatibility for PoC test callers.
func (w *Watcher) AddPeer(id, address string) {
	w.AddNode(id, address, "")
}

// Start begins the monitoring ticker loop in the background.
func (w *Watcher) Start() {
	go w.loop()
}

// Stop gracefully shuts down the watcher loop.
func (w *Watcher) Stop() {
	close(w.stopChan)
}

func (w *Watcher) loop() {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	// Initial check immediately on startup
	w.checkPeers()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.checkPeers()
		}
	}
}

type targetCheck struct {
	id    string
	url   string
	token string
}

func (w *Watcher) checkPeers() {
	w.mu.Lock()
	targets := make([]targetCheck, 0, len(w.peers))
	for id, peer := range w.peers {
		targets = append(targets, targetCheck{id: id, url: peer.URL, token: peer.Token})
	}
	w.mu.Unlock()

	type probeResult struct {
		id      string
		success bool
		err     error
		payload *NodeHealthPayload
	}

	results := make([]probeResult, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t targetCheck) {
			defer wg.Done()

			req, err := http.NewRequest(http.MethodGet, t.url, nil)
			if err != nil {
				results[idx] = probeResult{id: t.id, success: false, err: err}
				return
			}
			if t.token != "" {
				req.Header.Set("X-Allod-Watch-Token", t.token)
			}

			resp, err := w.client.Do(req)
			if err != nil {
				results[idx] = probeResult{id: t.id, success: false, err: err}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results[idx] = probeResult{
					id:      t.id,
					success: false,
					err:     fmt.Errorf("HTTP status %d", resp.StatusCode),
				}
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				results[idx] = probeResult{id: t.id, success: false, err: err}
				return
			}

			var payload NodeHealthPayload
			if err := json.Unmarshal(body, &payload); err == nil && payload.Status != "" {
				results[idx] = probeResult{id: t.id, success: true, payload: &payload}
			} else {
				// Success probe even if JSON parsing differs (e.g. plain 200 OK)
				results[idx] = probeResult{id: t.id, success: true}
			}
		}(i, target)
	}

	wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()

	for _, res := range results {
		peer, exists := w.peers[res.id]
		if !exists {
			continue
		}

		if res.success {
			if !peer.IsOnline {
				// Node just recovered from downtime!
				totalDowntime := now.Sub(peer.DowntimeStart)
				fmt.Printf("💚 [Watchdog] Rientro: Il nodo %s è tornato online! (Disservizio: %v)\n", peer.ID, totalDowntime.Round(time.Second))
				peer.IsOnline = true
				peer.AlertSent = false
				peer.ConsecutiveFailures = 0

				if w.OnRecover != nil {
					go w.OnRecover(peer.ID, totalDowntime)
				}
			} else {
				peer.ConsecutiveFailures = 0
			}

			peer.LastSeen = now
			if res.payload != nil {
				peer.LatestPayload = res.payload

				// Check for storage warnings
				if !res.payload.StorageOK && !peer.StorageWarningSent {
					peer.StorageWarningSent = true
					if w.OnWarning != nil {
						go w.OnWarning(peer.ID, "Storage Degradato", "Il pool di archiviazione segnala anomalie o spazio in esaurimento.")
					}
				} else if res.payload.StorageOK {
					peer.StorageWarningSent = false
				}
			}
		} else {
			peer.ConsecutiveFailures++
			if peer.IsOnline {
				if peer.ConsecutiveFailures == 1 {
					peer.DowntimeStart = now
				}
				downtime := now.Sub(peer.DowntimeStart)

				if (downtime >= w.alertThreshold || peer.ConsecutiveFailures >= w.failureCountLimit) && !peer.AlertSent {
					peer.IsOnline = false
					peer.AlertSent = true
					errMsg := "connessione rifiutata o timeout"
					if res.err != nil {
						errMsg = res.err.Error()
					}

					fmt.Printf("🔴 [Watchdog] ALLARME (PEER_LOST): Il nodo %s non risponde da %v! (Soglia: %v, Causa: %v)\n",
						peer.ID, downtime.Round(time.Second), w.alertThreshold, errMsg)

					if w.OnAlert != nil {
						go w.OnAlert(peer.ID, errMsg, downtime)
					}
				}
			}
		}
	}
}

// GetPeerStatus returns the latest status of a monitored node.
func (w *Watcher) GetPeerStatus(id string) (*PeerStatus, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	peer, exists := w.peers[id]
	if !exists {
		return nil, false
	}
	cp := *peer
	return &cp, true
}

// GetAllPeers returns copies of all monitored peers.
func (w *Watcher) GetAllPeers() map[string]PeerStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	res := make(map[string]PeerStatus, len(w.peers))
	for k, v := range w.peers {
		res[k] = *v
	}
	return res
}
