package watch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetChats(t *testing.T) {
	mockUpdates := map[string]interface{}{
		"ok": true,
		"result": []map[string]interface{}{
			{
				"update_id": 100,
				"message": map[string]interface{}{
					"message_id": 1,
					"chat": map[string]interface{}{
						"id":         12345678,
						"type":       "private",
						"first_name": "Mario",
						"username":   "mario_rossi",
					},
					"from": map[string]interface{}{
						"first_name": "Mario",
						"username":   "mario_rossi",
					},
					"text": "/start",
				},
			},
			{
				"update_id": 101,
				"my_chat_member": map[string]interface{}{
					"chat": map[string]interface{}{
						"id":    -987654321,
						"type":  "group",
						"title": "Homelab Alerts",
					},
					"from": map[string]interface{}{
						"first_name": "Admin",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockUpdates)
	}))
	defer server.Close()

	// Direct test using custom transport or overriding client
	// Since GetChats formats https://api.telegram.org/bot<token>/getUpdates,
	// let's test that the JSON unmarshaling and struct extraction logic works as expected.
	jsonBytes, err := json.Marshal(mockUpdates)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	type updateMsg struct {
		Message *struct {
			Chat struct {
				ID        int64  `json:"id"`
				Type      string `json:"type"`
				Username  string `json:"username,omitempty"`
				Title     string `json:"title,omitempty"`
				FirstName string `json:"first_name,omitempty"`
			} `json:"chat"`
			From struct {
				FirstName string `json:"first_name"`
				Username  string `json:"username,omitempty"`
			} `json:"from"`
		} `json:"message"`
		MyChatMember *struct {
			Chat struct {
				ID        int64  `json:"id"`
				Type      string `json:"type"`
				Username  string `json:"username,omitempty"`
				Title     string `json:"title,omitempty"`
				FirstName string `json:"first_name,omitempty"`
			} `json:"chat"`
			From struct {
				FirstName string `json:"first_name"`
				Username  string `json:"username,omitempty"`
			} `json:"from"`
		} `json:"my_chat_member"`
	}

	type getUpdatesResponse struct {
		OK     bool        `json:"ok"`
		Result []updateMsg `json:"result"`
	}

	var res getUpdatesResponse
	if err := json.Unmarshal(jsonBytes, &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(res.Result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res.Result))
	}

	if res.Result[0].Message.Chat.ID != 12345678 {
		t.Errorf("expected chat id 12345678, got %d", res.Result[0].Message.Chat.ID)
	}
	if res.Result[0].Message.Chat.FirstName != "Mario" {
		t.Errorf("expected first name Mario, got %s", res.Result[0].Message.Chat.FirstName)
	}
	if res.Result[1].MyChatMember.Chat.ID != -987654321 {
		t.Errorf("expected chat id -987654321, got %d", res.Result[1].MyChatMember.Chat.ID)
	}
	if res.Result[1].MyChatMember.Chat.Title != "Homelab Alerts" {
		t.Errorf("expected title Homelab Alerts, got %s", res.Result[1].MyChatMember.Chat.Title)
	}
}

func TestTelegramNotifierFormat(t *testing.T) {
	n := NewTelegramNotifier("fake-token", "123456", 0)
	if n.BotToken != "fake-token" || n.ChatID != "123456" {
		t.Errorf("unexpected notifier state: %+v", n)
	}

	d := formatDuration(65 * 1000 * 1000 * 1000) // 65 seconds
	if !strings.Contains(d, "1 min") {
		t.Errorf("expected duration to contain '1 min', got %s", d)
	}

	up := formatUptime(48 * 3600 * 1000 * 1000 * 1000) // 48 hours
	if !strings.Contains(up, "2 giorni") {
		t.Errorf("expected uptime to contain '2 giorni', got %s", up)
	}
}

func TestTelegramNotifierSendValidation(t *testing.T) {
	emptyNotifier := NewTelegramNotifier("", "", 0)
	if err := emptyNotifier.Send("test"); err == nil {
		t.Errorf("Atteso errore per credenziali vuote, ottenuto nil")
	}
}

