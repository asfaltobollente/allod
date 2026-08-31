package watch

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type PeerStatus struct {
	ID        string
	Address   string
	LastSeen  time.Time
	IsOnline  bool
	AlertSent bool
}

type Watcher struct {
	mu             sync.Mutex
	peers          map[string]*PeerStatus
	alertThreshold time.Duration
	checkInterval  time.Duration
	stopChan       chan struct{}
	client         *http.Client
}

func NewWatcher(alertThreshold time.Duration, checkInterval time.Duration) *Watcher {
	return &Watcher{
		peers:          make(map[string]*PeerStatus),
		alertThreshold: alertThreshold,
		checkInterval:  checkInterval,
		stopChan:       make(chan struct{}),
		client:         &http.Client{Timeout: 1 * time.Second},
	}
}

func (w *Watcher) AddPeer(id, address string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.peers[id] = &PeerStatus{
		ID:        id,
		Address:   address,
		LastSeen:  time.Now(),
		IsOnline:  true,
		AlertSent: false,
	}
}

func (w *Watcher) Start() {
	go w.loop()
}

func (w *Watcher) Stop() {
	close(w.stopChan)
}

func (w *Watcher) loop() {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.checkPeers()
		}
	}
}

type peerCheckTarget struct {
	id      string
	address string
}

func (w *Watcher) checkPeers() {
	// Snapshot peer list under lock to avoid holding the mutex during network I/O
	w.mu.Lock()
	targets := make([]peerCheckTarget, 0, len(w.peers))
	for id, peer := range w.peers {
		targets = append(targets, peerCheckTarget{id: id, address: peer.Address})
	}
	w.mu.Unlock()

	type checkResult struct {
		id      string
		success bool
	}

	results := make([]checkResult, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t peerCheckTarget) {
			defer wg.Done()
			resp, err := w.client.Head("http://" + t.address)
			if err == nil {
				resp.Body.Close()
				results[idx] = checkResult{id: t.id, success: true}
			} else {
				results[idx] = checkResult{id: t.id, success: false}
			}
		}(i, target)
	}

	wg.Wait()

	// Update state under lock
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, res := range results {
		peer, exists := w.peers[res.id]
		if !exists {
			continue
		}

		if res.success {
			peer.LastSeen = time.Now()
			if !peer.IsOnline {
				fmt.Printf("💚 [Watchdog] Rientro: Il nodo %s è tornato online!\n", peer.ID)
				peer.IsOnline = true
				peer.AlertSent = false
			}
		} else {
			offlineDuration := time.Since(peer.LastSeen)
			if offlineDuration > w.alertThreshold && !peer.AlertSent {
				fmt.Printf("🔴 [Watchdog] ALLARME (PEER_LOST): Il nodo %s non risponde da %v! (Soglia: %v)\n", peer.ID, offlineDuration.Round(time.Millisecond), w.alertThreshold)
				peer.IsOnline = false
				peer.AlertSent = true
			}
		}
	}
}
