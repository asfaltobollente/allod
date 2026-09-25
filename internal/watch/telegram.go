package watch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

// TelegramNotifier handles sending alerts and morning digests via Telegram Bot API.
type TelegramNotifier struct {
	BotToken string
	ChatID   string
	TopicID  int
	Client   *http.Client
}

// NewTelegramNotifier creates a new Telegram notifier instance.
func NewTelegramNotifier(botToken, chatID string, topicID int) *TelegramNotifier {
	return &TelegramNotifier{
		BotToken: strings.TrimSpace(botToken),
		ChatID:   strings.TrimSpace(chatID),
		TopicID:  topicID,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type telegramSendPayload struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode"`
	MessageThreadID       int    `json:"message_thread_id,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// Send sends an HTML-formatted message to the configured chat.
func (t *TelegramNotifier) Send(htmlText string) error {
	if t.BotToken == "" || t.ChatID == "" {
		return fmt.Errorf("bot_token o chat_id non configurati")
	}

	payload := telegramSendPayload{
		ChatID:                t.ChatID,
		Text:                  htmlText,
		ParseMode:             "HTML",
		MessageThreadID:       t.TopicID,
		DisableWebPagePreview: true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("errore serializzazione messaggio telegram: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("errore creazione richiesta telegram: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.Client.Do(req)
	if err != nil {
		return fmt.Errorf("errore invio richiesta a telegram: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var tgResp telegramResponse
	if err := json.Unmarshal(respBytes, &tgResp); err == nil && !tgResp.OK {
		return fmt.Errorf("telegram API ha restituito errore: %s", tgResp.Description)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram HTTP error %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

// SendAlert sends an urgent blackout / offline incident alert.
func (t *TelegramNotifier) SendAlert(nodeName, reason string, downtime time.Duration) error {
	msg := fmt.Sprintf(
		"🚨 <b>ALLERTA ALLOD: NODO NON RAGGIUNGIBILE</b>\n\n"+
			"🏷️ <b>Nodo:</b> <code>%s</code>\n"+
			"⏱️ <b>Stato:</b> Non risponde da <b>%s</b>\n"+
			"⚠️ <b>Dettaglio:</b> %s\n\n"+
			"<i>Possibile blackout elettrico, riavvio o interruzione della connettività di rete.</i>",
		html.EscapeString(nodeName),
		formatDuration(downtime),
		html.EscapeString(reason),
	)
	return t.Send(msg)
}

// SendRecovery sends a recovery message when the node comes back online.
func (t *TelegramNotifier) SendRecovery(nodeName string, totalDowntime time.Duration) error {
	msg := fmt.Sprintf(
		"💚 <b>RIENTRO ALLOD: NODO TORNATO ONLINE</b>\n\n"+
			"🏷️ <b>Nodo:</b> <code>%s</code>\n"+
			"✅ <b>Stato:</b> Operativo e rispondente\n"+
			"⏱️ <b>Durata disservizio:</b> %s\n\n"+
			"<i>Tutti i servizi locali e mesh sono nuovamente operativi.</i>",
		html.EscapeString(nodeName),
		formatDuration(totalDowntime),
	)
	return t.Send(msg)
}

// SendWarning sends a non-fatal warning (e.g. disk space low).
func (t *TelegramNotifier) SendWarning(nodeName, warningTitle, details string) error {
	msg := fmt.Sprintf(
		"⚠️ <b>AVVISO DI SISTEMA ALLOD</b>\n\n"+
			"🏷️ <b>Nodo:</b> <code>%s</code>\n"+
			"📌 <b>Evento:</b> %s\n"+
			"📝 <b>Dettaglio:</b> %s\n",
		html.EscapeString(nodeName),
		html.EscapeString(warningTitle),
		html.EscapeString(details),
	)
	return t.Send(msg)
}

// DigestReport aggregates telemetry and status for the daily morning digest.
type DigestReport struct {
	NodeName      string
	Uptime        time.Duration
	StorageStatus string
	StorageUsed   string
	StorageFree   string
	RAMUsedMB     int
	RAMTotalMB    int
	CPULoad       float64
	CPUTemp       float64
	ActiveModules []string
	WeatherInfo   string // e.g. "☀️ Roma: Sereno, min 16°C / max 25°C, Pioggia: 0%"
}

// SendDailyDigest sends the morning positive digest with server health & weather.
func (t *TelegramNotifier) SendDailyDigest(report DigestReport) error {
	var modulesStr string
	if len(report.ActiveModules) > 0 {
		modulesStr = strings.Join(report.ActiveModules, ", ")
	} else {
		modulesStr = "Nessun modulo aggiuntivo attivo"
	}

	weatherSection := ""
	if report.WeatherInfo != "" {
		weatherSection = fmt.Sprintf("🌤️ <b>Meteo di Oggi:</b>\n%s\n\n", html.EscapeString(report.WeatherInfo))
	}

	vitalsInfo := ""
	var vitalsParts []string
	if report.CPUTemp > 0 {
		vitalsParts = append(vitalsParts, fmt.Sprintf("Temp CPU: %.1f°C", report.CPUTemp))
	}
	if report.CPULoad > 0 {
		vitalsParts = append(vitalsParts, fmt.Sprintf("Carico: %.2f", report.CPULoad))
	}
	if len(vitalsParts) > 0 {
		vitalsInfo = fmt.Sprintf("🌡️ <b>Hardware:</b> %s\n", html.EscapeString(strings.Join(vitalsParts, " • ")))
	}

	msg := fmt.Sprintf(
		"☀️ <b>Buongiorno! Resoconto Allod</b>\n\n"+
			"%s"+
			"🏷️ <b>Nodo:</b> <code>%s</code>\n"+
			"⏱️ <b>Uptime:</b> %s\n"+
			"💾 <b>Pool Storage:</b> %s (Usati: %s, Liberi: %s)\n"+
			"🧠 <b>Memoria RAM:</b> %d MB / %d MB\n"+
			"%s"+
			"🧩 <b>Servizi:</b> %s\n\n"+
			"✓ <i>Tutto regolare. I tuoi dati personali sono al sicuro.</i>",
		weatherSection,
		html.EscapeString(report.NodeName),
		formatUptime(report.Uptime),
		html.EscapeString(report.StorageStatus),
		html.EscapeString(report.StorageUsed),
		html.EscapeString(report.StorageFree),
		report.RAMUsedMB,
		report.RAMTotalMB,
		vitalsInfo,
		html.EscapeString(modulesStr),
	)
	return t.Send(msg)
}

// Test sends a quick verification message to test Telegram bot credentials.
func (t *TelegramNotifier) Test() error {
	msg := "🤖 <b>Allod Sentinel: Test di Connessione</b>\n\n" +
		"✓ Il bot Telegram è configurato correttamente!\n" +
		"Questa chat riceverà gli allarmi di disconnessione, i rientri online e il resoconto mattutino con il meteo."
	return t.Send(msg)
}

// TelegramChat contains information about an active chat discovered via Telegram Bot API updates.
type TelegramChat struct {
	ChatID    string `json:"chat_id"`
	Type      string `json:"type"`
	FirstName string `json:"first_name,omitempty"`
	Username  string `json:"username,omitempty"`
	Title     string `json:"title,omitempty"`
	Label     string `json:"label"`
}

// GetChats queries the Telegram Bot API getUpdates endpoint to find active chats.
func GetChats(botToken string) ([]TelegramChat, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", strings.TrimSpace(botToken))
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("errore connessione telegram getUpdates: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

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
		ChannelPost *struct {
			Chat struct {
				ID    int64  `json:"id"`
				Type  string `json:"type"`
				Title string `json:"title,omitempty"`
			} `json:"chat"`
		} `json:"channel_post"`
	}

	type getUpdatesResponse struct {
		OK     bool        `json:"ok"`
		Result []updateMsg `json:"result"`
	}

	var res getUpdatesResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("errore parsing getUpdates: %w", err)
	}

	var chats []TelegramChat
	seen := make(map[int64]bool)

	for _, u := range res.Result {
		var id int64
		var chatType, username, title, firstName string

		if u.Message != nil && u.Message.Chat.ID != 0 {
			id = u.Message.Chat.ID
			chatType = u.Message.Chat.Type
			username = u.Message.Chat.Username
			if username == "" {
				username = u.Message.From.Username
			}
			title = u.Message.Chat.Title
			firstName = u.Message.Chat.FirstName
			if firstName == "" {
				firstName = u.Message.From.FirstName
			}
		} else if u.MyChatMember != nil && u.MyChatMember.Chat.ID != 0 {
			id = u.MyChatMember.Chat.ID
			chatType = u.MyChatMember.Chat.Type
			username = u.MyChatMember.Chat.Username
			if username == "" {
				username = u.MyChatMember.From.Username
			}
			title = u.MyChatMember.Chat.Title
			firstName = u.MyChatMember.Chat.FirstName
			if firstName == "" {
				firstName = u.MyChatMember.From.FirstName
			}
		} else if u.ChannelPost != nil && u.ChannelPost.Chat.ID != 0 {
			id = u.ChannelPost.Chat.ID
			chatType = u.ChannelPost.Chat.Type
			title = u.ChannelPost.Chat.Title
		}

		if id != 0 && !seen[id] {
			seen[id] = true
			label := fmt.Sprintf("ID: %d (Tipo: %s", id, chatType)
			if title != "" {
				label += fmt.Sprintf(", Gruppo: %s", title)
			}
			if firstName != "" {
				label += fmt.Sprintf(", Utente: %s", firstName)
			} else if username != "" {
				label += fmt.Sprintf(", Utente: @%s", username)
			}
			label += ")"

			chats = append(chats, TelegramChat{
				ChatID:    fmt.Sprintf("%d", id),
				Type:      chatType,
				FirstName: firstName,
				Username:  username,
				Title:     title,
				Label:     label,
			})
		}
	}

	return chats, nil
}

// GetChatIDs queries the Telegram Bot API getUpdates endpoint to help users find their chat ID (returns formatted strings for CLI).
func GetChatIDs(botToken string) ([]string, error) {
	chats, err := GetChats(botToken)
	if err != nil {
		return nil, err
	}
	var res []string
	for _, c := range chats {
		res = append(res, c.Label)
	}
	return res, nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m == 0 {
		return fmt.Sprintf("%d secondi", s)
	}
	h := m / 60
	m = m % 60
	if h == 0 {
		return fmt.Sprintf("%d min, %d sec", m, s)
	}
	return fmt.Sprintf("%d ore, %d min", h, m)
}

func formatUptime(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%d giorni, %d ore", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d ore, %d min", hours, mins)
	}
	return fmt.Sprintf("%d minuti", mins)
}
