package panel

import (
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asfaltobollente/allod/internal/state"
)

func TestGetPrimaryLANIP(t *testing.T) {
	ip := GetPrimaryLANIP()
	if ip == "" {
		t.Fatalf("expected non-empty IP")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Fatalf("invalid IP returned: %s", ip)
	}
}

func TestGetZeroConfigHostname(t *testing.T) {
	// 1. With nil state
	if h := GetZeroConfigHostname(nil); h != "allod" {
		t.Errorf("expected 'allod', got '%s'", h)
	}

	// 2. With state DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	st, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open state DB: %v", err)
	}
	defer st.Close()

	if h := GetZeroConfigHostname(st); h != "allod" {
		t.Errorf("expected 'allod', got '%s'", h)
	}

	_ = st.SetMeta("mdns_hostname", "mynas")
	if h := GetZeroConfigHostname(st); h != "mynas" {
		t.Errorf("expected 'mynas', got '%s'", h)
	}
}

func TestBuildZeroConfigModes(t *testing.T) {
	modes := BuildZeroConfigModes("mario", "192.168.1.50", "allod.local", "100.64.0.1")

	// Verify mdns mode
	mdns, ok := modes["mdns"]
	if !ok {
		t.Fatalf("expected mdns mode")
	}
	if mdns.WinPath != `\\allod.local\mario` {
		t.Errorf("unexpected mdns WinPath: %s", mdns.WinPath)
	}
	if mdns.MacPath != "smb://allod.local/mario" {
		t.Errorf("unexpected mdns MacPath: %s", mdns.MacPath)
	}
	if mdns.ImmichURL != "http://allod.local:2283" {
		t.Errorf("unexpected mdns ImmichURL: %s", mdns.ImmichURL)
	}
	if mdns.JellyfinURL != "http://allod.local:8096" {
		t.Errorf("unexpected mdns JellyfinURL: %s", mdns.JellyfinURL)
	}
	if !mdns.Recommended {
		t.Errorf("expected mdns to be recommended")
	}

	// Verify lan_ip mode
	lan, ok := modes["lan_ip"]
	if !ok {
		t.Fatalf("expected lan_ip mode")
	}
	if lan.WinPath != `\\192.168.1.50\mario` {
		t.Errorf("unexpected lan_ip WinPath: %s", lan.WinPath)
	}
	if lan.MacPath != "smb://192.168.1.50/mario" {
		t.Errorf("unexpected lan_ip MacPath: %s", lan.MacPath)
	}

	// Verify mesh mode
	mesh, ok := modes["mesh"]
	if !ok {
		t.Fatalf("expected mesh mode")
	}
	if mesh.WinPath != `\\100.64.0.1\mario` {
		t.Errorf("unexpected mesh WinPath: %s", mesh.WinPath)
	}
	if mesh.MacPath != "smb://100.64.0.1/mario" {
		t.Errorf("unexpected mesh MacPath: %s", mesh.MacPath)
	}
}

func TestGenerateAvahiHosts(t *testing.T) {
	out := GenerateAvahiHosts("192.168.1.50", "allod")
	expected := "192.168.1.50 allod.local allod\n"
	if !strings.Contains(out, expected) {
		t.Errorf("expected %q to contain %q", out, expected)
	}

	// Test when .local is passed
	out2 := GenerateAvahiHosts("192.168.1.50", "allod.local")
	if !strings.Contains(out2, expected) {
		t.Errorf("expected %q to contain %q", out2, expected)
	}
}

func TestGenerateAvahiService(t *testing.T) {
	xml := GenerateAvahiService("Allod NAS", 445, 8080)
	if !strings.Contains(xml, "<type>_smb._tcp</type>") {
		t.Errorf("missing _smb._tcp in xml: %s", xml)
	}
	if !strings.Contains(xml, "<port>445</port>") {
		t.Errorf("missing port 445 in xml: %s", xml)
	}
	if !strings.Contains(xml, "<txt-record>model=RackMac</txt-record>") {
		t.Errorf("missing RackMac txt-record in xml: %s", xml)
	}
}

func TestGenerateWsddDefaultConfig(t *testing.T) {
	conf := GenerateWsddDefaultConfig("allod", "workgroup")
	if !strings.Contains(conf, `WSDD_PARAMS="-n ALLOD -w WORKGROUP"`) {
		t.Errorf("unexpected wsdd config: %s", conf)
	}
}
