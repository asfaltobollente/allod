package watch

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DailyDigestScheduler triggers a positive morning report once per day at a designated time.
type DailyDigestScheduler struct {
	mu           sync.Mutex
	stopChan     chan struct{}
	hour         int
	minute       int
	city         string
	lastSentDate string
	watcher      *Watcher
	telegram     *TelegramNotifier
	weather      *WeatherClient
}

// NewDailyDigestScheduler creates a new morning digest scheduler.
func NewDailyDigestScheduler(hour, minute int, city string, w *Watcher, tg *TelegramNotifier, wc *WeatherClient) *DailyDigestScheduler {
	if hour < 0 || hour > 23 {
		hour = 8
	}
	if minute < 0 || minute > 59 {
		minute = 30
	}
	if city == "" {
		city = "Roma"
	}

	return &DailyDigestScheduler{
		stopChan: make(chan struct{}),
		hour:     hour,
		minute:   minute,
		city:     city,
		watcher:  w,
		telegram: tg,
		weather:  wc,
	}
}

// Start launches the daily digest ticker.
func (s *DailyDigestScheduler) Start() {
	go s.loop()
}

// Stop halts the daily digest ticker.
func (s *DailyDigestScheduler) Stop() {
	close(s.stopChan)
}

func (s *DailyDigestScheduler) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case now := <-ticker.C:
			s.checkAndTrigger(now)
		}
	}
}

func (s *DailyDigestScheduler) checkAndTrigger(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := now.Format("2006-01-02")
	if s.lastSentDate == today {
		return // Already sent today
	}

	if now.Hour() == s.hour && now.Minute() >= s.minute {
		s.lastSentDate = today
		go s.dispatch()
	}
}

// TriggerNow manually fires the morning digest immediately (for CLI test or debugging).
func (s *DailyDigestScheduler) TriggerNow() error {
	return s.dispatch()
}

func (s *DailyDigestScheduler) dispatch() error {
	peers := s.watcher.GetAllPeers()
	if len(peers) == 0 {
		return fmt.Errorf("nessun nodo registrato per il digest")
	}

	// Fetch weather forecast
	var weatherStr string
	if s.weather != nil && s.city != "" {
		wForecast, err := s.weather.GetDailyForecast(s.city)
		if err == nil {
			weatherStr = wForecast
		} else {
			log.Printf("[Digest] Avviso recupero meteo: %v", err)
			weatherStr = fmt.Sprintf("🌤️ %s (Meteo temporaneamente non disponibile)", s.city)
		}
	}

	for _, peer := range peers {
		// Only send digest if node is online
		if !peer.IsOnline {
			continue
		}

		report := DigestReport{
			NodeName:      peer.ID,
			Uptime:        48 * time.Hour, // default fallback
			StorageStatus: "Integro (RAID 1)",
			StorageUsed:   "N/D",
			StorageFree:   "N/D",
			RAMUsedMB:     0,
			RAMTotalMB:    0,
			ActiveModules: nil,
			WeatherInfo:   weatherStr,
		}

		if peer.LatestPayload != nil {
			report.Uptime = time.Duration(peer.LatestPayload.UptimeSeconds) * time.Second
			if peer.LatestPayload.NodeName != "" {
				report.NodeName = peer.LatestPayload.NodeName
			}
			if peer.LatestPayload.StorageOK {
				report.StorageStatus = "Integro e Sano"
			} else {
				report.StorageStatus = "⚠️ Attenzione Richiesta"
			}
			if peer.LatestPayload.StorageUsed != "" {
				report.StorageUsed = peer.LatestPayload.StorageUsed
			}
			if peer.LatestPayload.StorageFree != "" {
				report.StorageFree = peer.LatestPayload.StorageFree
			}
			report.RAMUsedMB = peer.LatestPayload.MemoryUsedMB
			report.RAMTotalMB = peer.LatestPayload.MemoryTotalMB
			report.ActiveModules = peer.LatestPayload.ActiveModules
		}

		if err := s.telegram.SendDailyDigest(report); err != nil {
			log.Printf("[Digest] Errore invio digest Telegram per %s: %v", peer.ID, err)
			return err
		}
		log.Printf("✓ [Digest] Digest mattutino inviato con successo a Telegram per il nodo %s", peer.ID)
	}

	return nil
}
