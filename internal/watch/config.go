package watch

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SentinelNodeConfig defines a single Allod node monitored by the sentinel.
type SentinelNodeConfig struct {
	ID    string `yaml:"id"`
	URL   string `yaml:"url"`
	Token string `yaml:"token,omitempty"`
}

// TelegramConfig defines Telegram Bot settings.
type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
	TopicID  int    `yaml:"topic_id,omitempty"`
}

// WeatherConfig defines weather forecast settings for the morning digest.
type WeatherConfig struct {
	Enabled bool   `yaml:"enabled"`
	City    string `yaml:"city"`
}

// IntervalsConfig defines polling intervals and outage thresholds.
type IntervalsConfig struct {
	CheckSeconds         int `yaml:"check_seconds"`
	DownThresholdSeconds int `yaml:"down_threshold_seconds"`
}

// DigestConfig defines morning digest schedule.
type DigestConfig struct {
	Enabled bool   `yaml:"enabled"`
	Time    string `yaml:"time"` // "HH:MM" e.g. "08:30"
}

// SentinelConfig is the root configuration structure for allod-watch.
type SentinelConfig struct {
	Nodes     []SentinelNodeConfig `yaml:"nodes"`
	Telegram  TelegramConfig       `yaml:"telegram"`
	Weather   WeatherConfig        `yaml:"weather"`
	Intervals IntervalsConfig      `yaml:"intervals"`
	Digest    DigestConfig         `yaml:"digest"`
}

// LoadConfig loads the sentinel configuration from a YAML file, applying environment variable overrides.
func LoadConfig(path string) (*SentinelConfig, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("errore lettura file config %s: %w", path, err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("errore parsing yaml config %s: %w", path, err)
			}
		}
	}

	// Environment variable overrides (useful in containerized / cloud VPS environments)
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		cfg.Telegram.BotToken = token
		cfg.Telegram.Enabled = true
	}
	if chatID := os.Getenv("TELEGRAM_CHAT_ID"); chatID != "" {
		cfg.Telegram.ChatID = chatID
		cfg.Telegram.Enabled = true
	}
	if city := os.Getenv("WEATHER_CITY"); city != "" {
		cfg.Weather.City = city
		cfg.Weather.Enabled = true
	}
	if nodeURL := os.Getenv("ALLOD_NODE_URL"); nodeURL != "" {
		nodeID := os.Getenv("ALLOD_NODE_ID")
		if nodeID == "" {
			nodeID = "allod-node"
		}
		cfg.Nodes = []SentinelNodeConfig{
			{ID: nodeID, URL: nodeURL, Token: os.Getenv("ALLOD_WATCH_TOKEN")},
		}
	}

	return cfg, nil
}

// DefaultConfig provides sensible defaults.
func DefaultConfig() *SentinelConfig {
	return &SentinelConfig{
		Nodes: []SentinelNodeConfig{
			{ID: "mio-allod", URL: "http://127.0.0.1:8080/api/health"},
		},
		Telegram: TelegramConfig{
			Enabled:  false,
			BotToken: "",
			ChatID:   "",
			TopicID:  0,
		},
		Weather: WeatherConfig{
			Enabled: true,
			City:    "Roma",
		},
		Intervals: IntervalsConfig{
			CheckSeconds:         60,
			DownThresholdSeconds: 180, // 3 minutes
		},
		Digest: DigestConfig{
			Enabled: true,
			Time:    "08:30",
		},
	}
}

// ParseDigestTime parses the "HH:MM" digest schedule.
func (c *DigestConfig) ParseDigestTime() (hour, minute int) {
	parts := strings.Split(strings.TrimSpace(c.Time), ":")
	if len(parts) == 2 {
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil && h >= 0 && h <= 23 && m >= 0 && m <= 59 {
			return h, m
		}
	}
	return 8, 30
}

// CheckDuration returns the polling interval as time.Duration.
func (c *IntervalsConfig) CheckDuration() time.Duration {
	if c.CheckSeconds < 5 {
		return 30 * time.Second
	}
	return time.Duration(c.CheckSeconds) * time.Second
}

// DownThresholdDuration returns the outage threshold as time.Duration.
func (c *IntervalsConfig) DownThresholdDuration() time.Duration {
	if c.DownThresholdSeconds < 10 {
		return 3 * time.Minute
	}
	return time.Duration(c.DownThresholdSeconds) * time.Second
}
