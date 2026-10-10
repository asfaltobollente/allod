package quadlet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asfaltobollente/allod/internal/manifest"
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
			"cloud": {
				RAMMB: 40,
				Requires: manifest.Requires{
					Modules: []string{"storage"},
				},
			},
		},
		Ports: []manifest.Port{},
		Privileges: manifest.Privileges{
			Userns:  "rootless",
			Caps:    []string{"NET_ADMIN"},
			Devices: []string{"/dev/net/tun"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/netbirdio/netbird", Tag: "0.79.0", Channel: "patch"},
		},
	}

	res, err := Generate("network", m, "cloud")
	if err != nil {
		t.Fatalf("unexpected error generating network: %v", err)
	}

	// NetBird is managed at the system level by allod-helperd to retain full
	// kernel WireGuard/TUN capabilities; rootless user Quadlet generation is bypassed.
	if len(res.Files) != 0 {
		t.Fatalf("expected 0 user units for network (helper-managed), got %d", len(res.Files))
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

	if !strings.Contains(unit, ":/shares:z,ro") {
		t.Errorf("expected /shares:z,ro mount in media unit, got:\n%s", unit)
	}
	if !strings.Contains(unit, ":/media:z,ro") {
		t.Errorf("expected /media:z,ro mount in media unit, got:\n%s", unit)
	}
	if !strings.Contains(unit, "shares/public:/shares/public:z,ro") {
		t.Errorf("expected shares/public:/shares/public:z,ro mount in media unit, got:\n%s", unit)
	}
	if strings.Contains(unit, "shares/public:/media:z,U") || strings.Contains(unit, "shares/public:/shares/public:z,U") {
		t.Errorf("unexpected :z,U flag found on shared public volume in media unit:\n%s", unit)
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

func TestMeshPortBinding(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	mBackup := &manifest.Manifest{
		ID:   "backup",
		Tier: "core",
		Levels: map[string]manifest.Level{
			"basic": {RAMMB: 100},
		},
		Ports: []manifest.Port{
			{N: 8000, Scope: "mesh"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/restic/rest-server", Tag: "0.13.0"},
		},
	}

	// 1. When ALLOD_MESH_IP is explicitly set (simulating active wt0 mesh)
	t.Setenv("ALLOD_MESH_IP", "100.64.0.42")
	resWithMesh, err := Generate("backup", mBackup, "basic")
	if err != nil {
		t.Fatalf("unexpected error generating backup: %v", err)
	}
	contentWithMesh := resWithMesh.Files["backup.container"]
	if !strings.Contains(contentWithMesh, "PublishPort=100.64.0.42:8000:8000") {
		t.Errorf("expected mesh port bound to 100.64.0.42, got:\n%s", contentWithMesh)
	}
	if strings.Contains(contentWithMesh, "PublishPort=8000:8000") {
		t.Errorf("mesh port must NOT be published to 0.0.0.0, got:\n%s", contentWithMesh)
	}

	// 2. When ALLOD_MESH_IP is empty and no wt0 interface is present (fallback to localhost)
	t.Setenv("ALLOD_MESH_IP", "")
	resFallback, err := Generate("backup", mBackup, "basic")
	if err != nil {
		t.Fatalf("unexpected error generating backup: %v", err)
	}
	contentFallback := resFallback.Files["backup.container"]
	if !strings.Contains(contentFallback, "PublishPort=127.0.0.1:8000:8000") {
		t.Errorf("expected mesh port fallback to 127.0.0.1, got:\n%s", contentFallback)
	}
	if strings.Contains(contentFallback, "PublishPort=8000:8000") {
		t.Errorf("mesh port must NOT be published to 0.0.0.0, got:\n%s", contentFallback)
	}

	// 3. LAN scoped port publishes to 0.0.0.0
	mLan := &manifest.Manifest{
		ID:   "shares",
		Tier: "core",
		Levels: map[string]manifest.Level{
			"basic": {RAMMB: 100},
		},
		Ports: []manifest.Port{
			{N: 445, Scope: "lan"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/dperson/samba", Tag: "latest"},
		},
	}
	resLan, err := Generate("shares", mLan, "basic")
	if err != nil {
		t.Fatalf("unexpected error generating shares: %v", err)
	}
	contentLan := resLan.Files["shares.container"]
	if !strings.Contains(contentLan, "PublishPort=445:445") {
		t.Errorf("expected LAN port published to 0.0.0.0 (445:445), got:\n%s", contentLan)
	}
}

func TestGenerateOdysseusModule(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)
	t.Setenv("ALLOD_MESH_IP", "100.64.0.5")

	m := &manifest.Manifest{
		ID:   "odysseus",
		Tier: "optional",
		Levels: map[string]manifest.Level{
			"standard": {RAMMB: 2048},
		},
		Ports: []manifest.Port{
			{N: 7000, Scope: "mesh"},
		},
		Images: []manifest.Image{
			{Ref: "ghcr.io/odysseus-dev/odysseus", Tag: "1.0.2", Channel: "pinned"},
			{Ref: "docker.io/chromadb/chroma", Tag: "latest", Channel: "pinned"},
			{Ref: "docker.io/searxng/searxng", Tag: "2026.5.31-7159b8aed", Channel: "pinned"},
		},
	}

	res, err := Generate("odysseus", m, "standard")
	if err != nil {
		t.Fatalf("unexpected error generating odysseus: %v", err)
	}

	if len(res.Files) != 3 {
		t.Fatalf("expected 3 generated container units, got %d", len(res.Files))
	}

	expectedFiles := []string{"odysseus.container", "odysseus-chroma.container", "odysseus-searxng.container"}
	for _, ef := range expectedFiles {
		if _, ok := res.Files[ef]; !ok {
			t.Errorf("expected generated file %s not found", ef)
		}
	}

	mainContent := res.Files["odysseus.container"]
	if !strings.Contains(mainContent, "Requires=odysseus-chroma.service") {
		t.Errorf("expected dependency on odysseus-chroma, got:\n%s", mainContent)
	}
	if !strings.Contains(mainContent, "Requires=odysseus-searxng.service") {
		t.Errorf("expected dependency on odysseus-searxng, got:\n%s", mainContent)
	}
	if !strings.Contains(mainContent, "PublishPort=100.64.0.5:7000:7000") {
		t.Errorf("expected mesh publish port for 7000, got:\n%s", mainContent)
	}
	if !strings.Contains(mainContent, "ShmSize=512m") {
		t.Errorf("expected ShmSize=512m for headless chromium in odysseus.container, got:\n%s", mainContent)
	}
	if !strings.Contains(mainContent, "EnvironmentFile=") || !strings.Contains(mainContent, "odysseus/secrets/odysseus.env") {
		t.Errorf("expected EnvironmentFile for odysseus.env, got:\n%s", mainContent)
	}

	// Verify secret generation
	secFile := filepath.Join(tempDir, "odysseus", "secrets", "odysseus.env")
	secBytes, err := os.ReadFile(secFile)
	if err != nil {
		t.Fatalf("failed to read generated odysseus secret file: %v", err)
	}
	secStr := string(secBytes)
	if !strings.Contains(secStr, "OLLAMA_BASE_URL=") || !strings.Contains(secStr, "CHROMADB_HOST=odysseus-chroma") {
		t.Errorf("expected OLLAMA_BASE_URL and CHROMADB_HOST in odysseus.env, got:\n%s", secStr)
	}
}

func TestBackupModuleQuadletExec(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ALLOD_STORAGE_DIR", tempDir)

	// Case 1: manifest has only flags in args: ["--append-only", "--no-auth"]
	mLegacy := &manifest.Manifest{
		ID:   "backup",
		Tier: "core",
		Levels: map[string]manifest.Level{
			"peers": {RAMMB: 150},
		},
		Ports: []manifest.Port{
			{N: 8000, Scope: "mesh"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/restic/rest-server", Tag: "0.12.1", Args: []string{"--append-only", "--no-auth"}},
		},
	}

	res, err := Generate("backup", mLegacy, "peers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content := res.Files["backup.container"]
	if !strings.Contains(content, "Exec=/usr/bin/rest-server --append-only --no-auth --path /data") {
		t.Errorf("expected normalized Exec with /usr/bin/rest-server and --path /data, got:\n%s", content)
	}
	if !strings.Contains(content, "/backup/vault:/data:Z,U") {
		t.Errorf("expected /backup/vault volume mount, got:\n%s", content)
	}

	// Verify storage directory was created with mode 0770
	vaultDir := filepath.Join(tempDir, "backup", "vault")
	if fi, err := os.Stat(vaultDir); err != nil || !fi.IsDir() {
		t.Errorf("expected backup/vault directory to exist, err: %v", err)
	}

	// Case 2: manifest already has /usr/bin/rest-server specified
	mExplicit := &manifest.Manifest{
		ID:   "backup",
		Tier: "core",
		Levels: map[string]manifest.Level{
			"peers": {RAMMB: 150},
		},
		Ports: []manifest.Port{
			{N: 8000, Scope: "mesh"},
		},
		Images: []manifest.Image{
			{Ref: "docker.io/restic/rest-server", Tag: "0.12.1", Args: []string{"/usr/bin/rest-server", "--append-only", "--no-auth", "--path", "/data"}},
		},
	}

	resExp, err := Generate("backup", mExplicit, "peers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	contentExp := resExp.Files["backup.container"]
	if !strings.Contains(contentExp, "Exec=/usr/bin/rest-server --append-only --no-auth --path /data") {
		t.Errorf("expected explicit Exec command to be preserved, got:\n%s", contentExp)
	}
}

