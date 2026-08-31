package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveConfigAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Node: NodeConfig{
			Name:    "test-node",
			Channel: "stable",
			Group:   "test-group",
		},
		Modules: map[string]ModuleConfig{
			"photos": {Level: "standard"},
			"backup": {Level: "peers"},
		},
	}

	if err := cfg.Save(cfgFile); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Node.Name != "test-node" {
		t.Errorf("expected node name 'test-node', got %q", loaded.Node.Name)
	}
	if loaded.Modules["photos"].Level != "standard" {
		t.Errorf("expected photos level 'standard', got %q", loaded.Modules["photos"].Level)
	}

	// Verify temporary file was cleaned up by atomic save
	if _, err := os.Stat(cfgFile + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected .tmp file to be removed after atomic save")
	}
}
