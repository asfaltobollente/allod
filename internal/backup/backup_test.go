package backup

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/asfaltobollente/allod/internal/config"
	"github.com/asfaltobollente/allod/internal/ring"
	"github.com/asfaltobollente/allod/internal/state"
)

func TestFormatRepoURL(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{"100.100.66.117", 8000, "rest:http://100.100.66.117:8000/"},
		{"fd00::1", 8000, "rest:http://[fd00::1]:8000/"},
		{"[fd00::1]", 8000, "rest:http://[fd00::1]:8000/"},
		{"vault.mesh", 0, "rest:http://vault.mesh:8000/"},
	}

	for _, c := range cases {
		got := FormatRepoURL(c.host, c.port)
		if got != c.want {
			t.Errorf("FormatRepoURL(%q, %d) = %q, want %q", c.host, c.port, got, c.want)
		}
	}
}

func TestGetOrCreateEncryptionKey(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")
	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed opening state: %v", err)
	}
	defer st.Close()

	engine := NewEngine(dbPath, "config.yaml", "ring.yaml")

	// 1. Generate new key
	key1, err := engine.GetOrCreateEncryptionKey(st)
	if err != nil {
		t.Fatalf("expected key generation, got error: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32 hex chars, got %d (%s)", len(key1), key1)
	}

	// 2. Fetch existing key must return identical value
	key2, err := engine.GetOrCreateEncryptionKey(st)
	if err != nil {
		t.Fatalf("expected cached key, got error: %v", err)
	}
	if key1 != key2 {
		t.Fatalf("expected identical keys, got %s != %s", key1, key2)
	}
}

func TestFindRemoteVault(t *testing.T) {
	tempDir := t.TempDir()
	ringPath := filepath.Join(tempDir, "ring.yaml")

	topo := ring.NewRingTopology("allod-ring", 2)
	topo.AddMember(&ring.Member{
		ID:      "allod-ferretti",
		Address: "100.100.64.229",
		QuotaGB: 500,
	})
	topo.AddMember(&ring.Member{
		ID:      "Casa-Test",
		Address: "100.100.66.117",
		QuotaGB: 100,
	})
	if err := topo.Save(ringPath); err != nil {
		t.Fatalf("failed saving ring: %v", err)
	}

	engine := NewEngine("state.db", "config.yaml", ringPath)
	cfg := &config.Config{
		Node: config.NodeConfig{Name: "allod-ferretti"},
	}

	name, addr, port, err := engine.FindRemoteVault(cfg)
	if err != nil {
		t.Fatalf("expected remote vault, got error: %v", err)
	}
	if name != "Casa-Test" {
		t.Errorf("expected 'Casa-Test', got %q", name)
	}
	if addr != "100.100.66.117" {
		t.Errorf("expected '100.100.66.117', got %q", addr)
	}
	if port != 8000 {
		t.Errorf("expected port 8000, got %d", port)
	}
}

func TestGetStatusInitialState(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")
	ringPath := filepath.Join(tempDir, "ring.yaml")

	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed opening state: %v", err)
	}
	_ = st.Close()

	topo := ring.NewRingTopology("allod-ring", 2)
	topo.AddMember(&ring.Member{
		ID:      "allod-node",
		Address: "127.0.0.1",
		QuotaGB: 500,
	})
	topo.AddMember(&ring.Member{
		ID:      "Casa-Test",
		Address: "100.100.66.117",
		QuotaGB: 100,
	})
	_ = topo.Save(ringPath)

	engine := NewEngine(dbPath, "config.yaml", ringPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.IsRunning {
		t.Errorf("expected IsRunning false")
	}
	if status.RemotePeerName != "Casa-Test" {
		t.Errorf("expected remote peer 'Casa-Test', got %q", status.RemotePeerName)
	}
	if status.LastStatus != "never" {
		t.Errorf("expected last status 'never', got %q", status.LastStatus)
	}
}

func TestEngineLoggingAndDiagnostics(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "state.db")
	ringPath := filepath.Join(tempDir, "ring.yaml")

	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed opening state: %v", err)
	}
	_ = st.Close()

	topo := ring.NewRingTopology("allod-ring", 2)
	topo.AddMember(&ring.Member{
		ID:      "allod-ferretti",
		Address: "100.100.64.229",
		QuotaGB: 500,
	})
	topo.AddMember(&ring.Member{
		ID:      "Casa-Test",
		Address: "100.100.66.117",
		QuotaGB: 100,
	})
	_ = topo.Save(ringPath)

	engine := NewEngine(dbPath, "config.yaml", ringPath)

	// Add sample logs
	engine.AddLog("info", "Test log 1")
	engine.AddLog("warn", "Test warning 2")
	engine.AddLog("error", "Test error 3")

	st2, _ := state.Open(dbPath)
	logs := engine.GetLogs(st2)
	st2.Close()

	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
	if logs[0].Level != "INFO" || logs[0].Message != "Test log 1" {
		t.Errorf("unexpected log 0: %+v", logs[0])
	}
	if logs[2].Level != "ERROR" || logs[2].Message != "Test error 3" {
		t.Errorf("unexpected log 2: %+v", logs[2])
	}

	// Test GetDiagnostics
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	diag, errD := engine.GetDiagnostics(ctx)
	if errD != nil {
		t.Fatalf("GetDiagnostics failed: %v", errD)
	}
	if diag.RemoteVault != "Casa-Test" {
		t.Errorf("expected Casa-Test, got %s", diag.RemoteVault)
	}
	if len(diag.Logs) != 3 {
		t.Errorf("expected 3 logs in diag, got %d", len(diag.Logs))
	}
}
