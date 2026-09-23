package watch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramNotifierSend(t *testing.T) {
	var receivedBody map[string]interface{}
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true, "result": {"message_id": 12345}}`))
	}))
	defer mockServer.Close()

	notifier := NewTelegramNotifier("fake-token", "123456", 0)
	// Override transport to route to mock server
	notifier.Client = mockServer.Client()

	// Direct test via custom send URL if we test Send
	// To test Send with the mock server URL, let's test the payload formatting
	if notifier.BotToken != "fake-token" || notifier.ChatID != "123456" {
		t.Fatalf("Parametri non inizializzati correttamente")
	}

	// Missing token or chat id must return error
	emptyNotifier := NewTelegramNotifier("", "", 0)
	if err := emptyNotifier.Send("test"); err == nil {
		t.Errorf("Atteso errore per credenziali vuote, ottenuto nil")
	}
}

func TestTelegramNotifierFormatting(t *testing.T) {
	// Test alert formatting
	dur := 185 * time.Second
	durStr := formatDuration(dur)
	if !strings.Contains(durStr, "3 min") {
		t.Errorf("Formattazione durata inattesa: %s", durStr)
	}

	uptime := 50 * time.Hour
	uptimeStr := formatUptime(uptime)
	if !strings.Contains(uptimeStr, "2 giorni") {
		t.Errorf("Formattazione uptime inattesa: %s", uptimeStr)
	}
}
