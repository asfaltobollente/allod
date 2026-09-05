package quadlet

import (
	"strings"
	"testing"

	"github.com/allod-project/allod/internal/manifest"
)

func TestGenerateNativeModule(t *testing.T) {
	m := &manifest.Manifest{
		ID:   "storage",
		Tier: "core",
		Levels: map[string]manifest.Level{
			"basic": {RAMMB: 50},
		},
		Images: []manifest.Image{},
	}

	res, err := Generate("storage", m, "basic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.IsNative {
		t.Errorf("expected module to be identified as native")
	}

	content, exists := res.Files["storage.service"]
	if !exists {
		t.Fatalf("expected storage.service to be generated")
	}

	if !strings.Contains(content, "Description=Allod Native Module: storage") {
		t.Errorf("expected native description in unit file, got:\n%s", content)
	}
}

func TestGenerateMultiImageModule(t *testing.T) {
	m := &manifest.Manifest{
		ID:   "photos",
		Tier: "recommended",
		Levels: map[string]manifest.Level{
			"standard": {RAMMB: 1500},
		},
		Images: []manifest.Image{
			{Ref: "ghcr.io/immich-app/immich-server", Tag: "v1.118.0", Channel: "patch"},
			{Ref: "ghcr.io/immich-app/postgres", Tag: "14", Channel: "pinned"},
			{Ref: "docker.io/valkey/valkey", Tag: "9", Channel: "pinned"},
		},
	}

	res, err := Generate("photos", m, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Files) != 3 {
		t.Fatalf("expected 3 generated container units, got %d", len(res.Files))
	}

	expectedFiles := []string{"photos.container", "photos-postgres.container", "photos-valkey.container"}
	for _, ef := range expectedFiles {
		if _, ok := res.Files[ef]; !ok {
			t.Errorf("expected generated file %s not found", ef)
		}
	}
}

func TestIsModuleRunning(t *testing.T) {
	mockContainers := map[string]bool{
		"cloud":                true,
		"cloud-postgres":       true,
		"photos-valkey":        true,
		"systemd-network-derp": true,
	}

	if !IsModuleRunning("cloud", mockContainers) {
		t.Errorf("expected cloud to be detected as running")
	}

	if !IsModuleRunning("photos", mockContainers) {
		t.Errorf("expected photos to be detected as running (photos-valkey exists)")
	}

	if !IsModuleRunning("network", mockContainers) {
		t.Errorf("expected network to be detected as running (systemd-network-derp exists)")
	}

	if IsModuleRunning("media", mockContainers) {
		t.Errorf("expected media to NOT be detected as running")
	}
}

func TestGenerateNetworkHybrid(t *testing.T) {
	m := &manifest.Manifest{
		ID:   "network",
		Tier: "recommended",
		Levels: map[string]manifest.Level{
			"hybrid": {
				RAMMB: 120,
				Requires: manifest.Requires{
					Modules: []string{"storage"},
				},
			},
		},
		Ports: []manifest.Port{
			{N: 8085, Scope: "localhost"},
		},
		Privileges: manifest.Privileges{
			Userns: "rootless",
			Caps:   []string{"NET_ADMIN"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/headscale/headscale", Tag: "0.25.1", Channel: "patch"},
			{Ref: "docker.io/cloudflare/cloudflared", Tag: "latest", Channel: "patch"},
		},
	}

	res, err := Generate("network", m, "hybrid")
	if err != nil {
		t.Fatalf("unexpected error generating network: %v", err)
	}

	if len(res.Files) != 2 {
		t.Fatalf("expected 2 units (headscale + cloudflared), got %d", len(res.Files))
	}

	headscaleUnit, ok := res.Files["network.container"]
	if !ok {
		t.Fatalf("network.container not generated")
	}
	if !strings.Contains(headscaleUnit, "Image=docker.io/headscale/headscale:0.25.1") {
		t.Errorf("expected headscale image in unit, got:\n%s", headscaleUnit)
	}
	if !strings.Contains(headscaleUnit, "PublishPort=127.0.0.1:8085:8085") {
		t.Errorf("expected 127.0.0.1:8085 localhost publish port, got:\n%s", headscaleUnit)
	}
	if !strings.Contains(headscaleUnit, "MemoryMax=120M") {
		t.Errorf("expected MemoryMax=120M, got:\n%s", headscaleUnit)
	}
	if !strings.Contains(headscaleUnit, "AddCapability=NET_ADMIN") {
		t.Errorf("expected AddCapability=NET_ADMIN, got:\n%s", headscaleUnit)
	}

	cfUnit, ok := res.Files["network-cloudflared.container"]
	if !ok {
		t.Fatalf("network-cloudflared.container not generated")
	}
	if !strings.Contains(cfUnit, "Image=docker.io/cloudflare/cloudflared:latest") {
		t.Errorf("expected cloudflared image in unit, got:\n%s", cfUnit)
	}
	if !strings.Contains(cfUnit, "Network=host") {
		t.Errorf("expected Network=host for cloudflared, got:\n%s", cfUnit)
	}
	if !strings.Contains(cfUnit, "Exec=tunnel --no-autoupdate run") {
		t.Errorf("expected tunnel run exec, got:\n%s", cfUnit)
	}
}

