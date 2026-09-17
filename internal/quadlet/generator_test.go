package quadlet

import (
	"os"
	"path/filepath"
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
			{Ref: "docker.io/cloudflare/cloudflared", Tag: "2026.9.1", Channel: "patch"},
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
	if !strings.Contains(cfUnit, "Image=docker.io/cloudflare/cloudflared:2026.9.1") {
		t.Errorf("expected cloudflared image in unit, got:\n%s", cfUnit)
	}
	if !strings.Contains(cfUnit, "Network=host") {
		t.Errorf("expected Network=host for cloudflared, got:\n%s", cfUnit)
	}
	if !strings.Contains(cfUnit, "Exec=tunnel --no-autoupdate run") {
		t.Errorf("expected tunnel run exec, got:\n%s", cfUnit)
	}
	if !strings.Contains(cfUnit, "EnvironmentFile=") || !strings.Contains(cfUnit, "secrets/cloudflared.env") {
		t.Errorf("expected EnvironmentFile pointing to secrets/cloudflared.env, got:\n%s", cfUnit)
	}
	if strings.Contains(cfUnit, "Environment=TUNNEL_TOKEN=") {
		t.Errorf("expected TUNNEL_TOKEN to NOT be inlined as Environment=, got:\n%s", cfUnit)
	}
}

func TestGenerateMediaJellyfin(t *testing.T) {
	m := &manifest.Manifest{
		ID:   "media",
		Tier: "optional",
		Levels: map[string]manifest.Level{
			"basic": {
				RAMMB: 500,
				Requires: manifest.Requires{
					Modules: []string{"storage"},
				},
			},
		},
		Ports: []manifest.Port{
			{N: 8096, Scope: "mesh"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/jellyfin/jellyfin", Tag: "10.10", Channel: "patch"},
		},
	}

	res, err := Generate("media", m, "basic")
	if err != nil {
		t.Fatalf("unexpected error generating media: %v", err)
	}

	unit, ok := res.Files["media.container"]
	if !ok {
		t.Fatalf("media.container not generated")
	}

	if !strings.Contains(unit, ":/shares:z") {
		t.Errorf("expected /shares:z mount in media unit, got:\n%s", unit)
	}
	if !strings.Contains(unit, ":/media:z") {
		t.Errorf("expected /media:z mount in media unit, got:\n%s", unit)
	}
	if !strings.Contains(unit, "shares/public:/shares/public:z") {
		t.Errorf("expected shares/public:/shares/public:z mount in media unit, got:\n%s", unit)
	}
	if !strings.Contains(unit, "MemoryMax=500M") {
		t.Errorf("expected MemoryMax=500M, got:\n%s", unit)
	}
}

func TestDatabaseSecretsDynamic(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	mPhotos := &manifest.Manifest{
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

	resPhotos, err := Generate("photos", mPhotos, "standard")
	if err != nil {
		t.Fatalf("unexpected error generating photos: %v", err)
	}

	for _, unitName := range []string{"photos.container", "photos-postgres.container"} {
		content := resPhotos.Files[unitName]
		if strings.Contains(content, "POSTGRES_PASSWORD=") || strings.Contains(content, "DB_PASSWORD=") {
			t.Errorf("expected unit %s to NOT contain plaintext password, got:\n%s", unitName, content)
		}
		if !strings.Contains(content, "EnvironmentFile=") || !strings.Contains(content, "photos/secrets/postgres.env") {
			t.Errorf("expected unit %s to contain EnvironmentFile pointing to secrets/postgres.env, got:\n%s", unitName, content)
		}
	}

	// Verify secret file exists and is populated
	secretPath := filepath.Join(tempDir, "photos", "secrets", "postgres.env")
	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("expected secret file to exist: %v", err)
	}
	// Check content has 32-char generated password and required variables
	secBytes, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("failed to read secret file: %v", err)
	}
	secStr := string(secBytes)
	if strings.Contains(secStr, "PASSWORD=postgres") {
		t.Errorf("expected freshly generated password, not legacy 'postgres', got: %s", secStr)
	}
	if !strings.Contains(secStr, "DB_PASSWORD=") || !strings.Contains(secStr, "POSTGRES_PASSWORD=") {
		t.Errorf("expected DB_PASSWORD and POSTGRES_PASSWORD in secret env, got:\n%s", secStr)
	}
	_ = info

	// Test cloud module
	mCloud := &manifest.Manifest{
		ID:   "cloud",
		Tier: "recommended",
		Levels: map[string]manifest.Level{
			"basic": {RAMMB: 1000},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/nextcloud", Tag: "30-apache", Channel: "patch"},
			{Ref: "docker.io/postgres", Tag: "16", Channel: "pinned"},
		},
	}

	resCloud, err := Generate("cloud", mCloud, "basic")
	if err != nil {
		t.Fatalf("unexpected error generating cloud: %v", err)
	}

	for _, unitName := range []string{"cloud.container", "cloud-postgres.container"} {
		content := resCloud.Files[unitName]
		if strings.Contains(content, "POSTGRES_PASSWORD=") {
			t.Errorf("expected unit %s to NOT contain plaintext password, got:\n%s", unitName, content)
		}
		if !strings.Contains(content, "EnvironmentFile=") || !strings.Contains(content, "cloud/secrets/postgres.env") {
			t.Errorf("expected unit %s to contain EnvironmentFile pointing to secrets/postgres.env, got:\n%s", unitName, content)
		}
	}
}

func TestDatabaseSecretsLegacyMigration(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	// Simulate existing cloud database directory with data
	cloudDbDir := filepath.Join(tempDir, "cloud", "postgres")
	if err := os.MkdirAll(cloudDbDir, 0755); err != nil {
		t.Fatalf("failed to create simulated db dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cloudDbDir, "PG_VERSION"), []byte("16\n"), 0644); err != nil {
		t.Fatalf("failed to create PG_VERSION: %v", err)
	}

	// Generate cloud units
	mCloud := &manifest.Manifest{
		ID:   "cloud",
		Tier: "recommended",
		Levels: map[string]manifest.Level{
			"basic": {RAMMB: 1000},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/nextcloud", Tag: "30-apache", Channel: "patch"},
			{Ref: "docker.io/postgres", Tag: "16", Channel: "pinned"},
		},
	}

	if _, err := Generate("cloud", mCloud, "basic"); err != nil {
		t.Fatalf("unexpected error generating cloud: %v", err)
	}

	cloudSecBytes, err := os.ReadFile(filepath.Join(tempDir, "cloud", "secrets", "postgres.env"))
	if err != nil {
		t.Fatalf("failed to read cloud secret file: %v", err)
	}
	if !strings.Contains(string(cloudSecBytes), "POSTGRES_PASSWORD=allod_secure_pass") {
		t.Errorf("expected legacy password 'allod_secure_pass' in migrated secret, got:\n%s", string(cloudSecBytes))
	}

	// Simulate existing photos database directory with data
	photosDbDir := filepath.Join(tempDir, "photos", "postgres")
	if err := os.MkdirAll(photosDbDir, 0755); err != nil {
		t.Fatalf("failed to create simulated photos db dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(photosDbDir, "PG_VERSION"), []byte("14\n"), 0644); err != nil {
		t.Fatalf("failed to create PG_VERSION: %v", err)
	}

	mPhotos := &manifest.Manifest{
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

	if _, err := Generate("photos", mPhotos, "standard"); err != nil {
		t.Fatalf("unexpected error generating photos: %v", err)
	}

	photosSecBytes, err := os.ReadFile(filepath.Join(tempDir, "photos", "secrets", "postgres.env"))
	if err != nil {
		t.Fatalf("failed to read photos secret file: %v", err)
	}
	photosSecStr := string(photosSecBytes)
	if !strings.Contains(photosSecStr, "POSTGRES_PASSWORD=postgres") || !strings.Contains(photosSecStr, "DB_PASSWORD=postgres") {
		t.Errorf("expected legacy password 'postgres' in migrated secret, got:\n%s", photosSecStr)
	}
}
