package state

import (
	"path/filepath"
	"testing"
)

func TestStateStoreLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-state.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open state db: %v", err)
	}
	defer store.Close()

	// 1. Save module
	err = store.SaveModule("photos", "standard", "hash123")
	if err != nil {
		t.Fatalf("failed to save module: %v", err)
	}

	// 2. Get module
	mod, err := store.GetModule("photos")
	if err != nil || mod == nil {
		t.Fatalf("failed to get module: %v", err)
	}
	if mod.Level != "standard" || mod.ContentHash != "hash123" {
		t.Errorf("unexpected module data: %+v", mod)
	}

	// 3. List modules
	mods, err := store.ListModules()
	if err != nil || len(mods) != 1 {
		t.Fatalf("expected 1 module in list, got %d", len(mods))
	}

	// 4. Node meta
	err = store.SetMeta("test_key", "test_value")
	if err != nil {
		t.Fatalf("failed to set meta: %v", err)
	}
	val, err := store.GetMeta("test_key")
	if err != nil || val != "test_value" {
		t.Errorf("expected meta 'test_value', got %q", val)
	}

	// 5. Delete module
	err = store.DeleteModule("photos")
	if err != nil {
		t.Fatalf("failed to delete module: %v", err)
	}
	modAfter, _ := store.GetModule("photos")
	if modAfter != nil {
		t.Errorf("expected module to be deleted, found: %+v", modAfter)
	}
}
