package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultsAndEnvOverrides(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Intervals.CheckSeconds != 60 {
		t.Errorf("Intervallo di default atteso 60s, ottenuto %d", cfg.Intervals.CheckSeconds)
	}

	h, m := cfg.Digest.ParseDigestTime()
	if h != 8 || m != 30 {
		t.Errorf("Orario digest default atteso 08:30, ottenuto %02d:%02d", h, m)
	}

	// Test environment variable override
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-env-token")
	os.Setenv("TELEGRAM_CHAT_ID", "999888777")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")
	defer os.Unsetenv("TELEGRAM_CHAT_ID")

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "watch.yaml")
	os.WriteFile(cfgFile, []byte("intervals:\n  check_seconds: 45\n"), 0644)

	loaded, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("Errore caricamento config: %v", err)
	}

	if loaded.Intervals.CheckSeconds != 45 {
		t.Errorf("Atteso check_seconds=45, ottenuto %d", loaded.Intervals.CheckSeconds)
	}
	if loaded.Telegram.BotToken != "test-env-token" {
		t.Errorf("Atteso bot token da env 'test-env-token', ottenuto %s", loaded.Telegram.BotToken)
	}
	if loaded.Telegram.ChatID != "999888777" {
		t.Errorf("Atteso chat_id da env '999888777', ottenuto %s", loaded.Telegram.ChatID)
	}
}
