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
