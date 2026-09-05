package preflight

import (
	"testing"

	"github.com/allod-project/allod/internal/config"
	"github.com/allod-project/allod/internal/manifest"
)

func TestPreflightPass(t *testing.T) {
	cfg := &config.Config{
		Modules: map[string]config.ModuleConfig{
			"storage": {Level: "basic"},
		},
	}

	m := &manifest.Manifest{
		ID: "shares",
		Levels: map[string]manifest.Level{
			"basic": {
				RAMMB: 100,
				Requires: manifest.Requires{
					Modules: []string{"storage"},
				},
			},
		},
	}

	res := Check(cfg, "shares", m, "basic")
	if !res.Pass {
		t.Errorf("expected preflight to pass, got message: %s", res.Message)
	}
}

func TestPreflightMissingDependency(t *testing.T) {
	// storage is missing or off
	cfg := &config.Config{
		Modules: map[string]config.ModuleConfig{
			"storage": {Level: "off"},
		},
	}

	m := &manifest.Manifest{
		ID: "shares",
		Levels: map[string]manifest.Level{
			"basic": {
				RAMMB: 100,
				Requires: manifest.Requires{
					Modules: []string{"storage"},
				},
			},
		},
	}

	res := Check(cfg, "shares", m, "basic")
	if res.Pass {
		t.Errorf("expected preflight to fail due to missing storage module")
	}
}

func TestPreflightInvalidLevel(t *testing.T) {
	cfg := &config.Config{Modules: map[string]config.ModuleConfig{}}
	m := &manifest.Manifest{
		ID: "photos",
		Levels: map[string]manifest.Level{
			"standard": {RAMMB: 1500},
		},
	}

	res := Check(cfg, "photos", m, "non_existent_level")
	if res.Pass {
		t.Errorf("expected preflight to fail on non-existent level")
	}
}

func TestPreflightNetworkHybrid(t *testing.T) {
	cfg := &config.Config{
		Modules: map[string]config.ModuleConfig{
			"storage": {Level: "basic"},
		},
	}
	m := &manifest.Manifest{
		ID: "network",
		Levels: map[string]manifest.Level{
			"hybrid": {
				RAMMB: 120,
				Requires: manifest.Requires{
					Modules: []string{"storage"},
				},
			},
		},
	}

	res := Check(cfg, "network", m, "hybrid")
	if !res.Pass {
		t.Errorf("expected network hybrid preflight to pass, got: %s", res.Message)
	}

	// Missing storage
	cfgNoStorage := &config.Config{Modules: map[string]config.ModuleConfig{}}
	resNoStorage := Check(cfgNoStorage, "network", m, "hybrid")
	if resNoStorage.Pass {
		t.Errorf("expected network hybrid to fail when storage is missing")
	}
}
