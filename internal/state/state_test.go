package state

import (
	"path/filepath"
	"testing"
	"time"
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

func TestFamilyMembersLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-family.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open state db: %v", err)
	}
	defer store.Close()

	// 1. Create family member
	m := &FamilyMember{
		Username:     "mario",
		FirstName:    "Mario",
		LastName:     "Rossi",
		Email:        "mario@example.com",
		Role:         "admin",
		AvatarColor:  "#38bdf8",
		Notes:        "iPhone 15 + ThinkPad",
		SmbActive:    true,
		PhotosLinked: true,
	}
	if err := store.CreateFamilyMember(m); err != nil {
		t.Fatalf("failed to create member: %v", err)
	}
	if m.ID == 0 {
		t.Errorf("expected non-zero member ID")
	}

	// 2. Get member
	fetched, err := store.GetFamilyMember("mario")
	if err != nil || fetched == nil {
		t.Fatalf("failed to get member: %v", err)
	}
	if fetched.FirstName != "Mario" || fetched.LastName != "Rossi" || fetched.Role != "admin" {
		t.Errorf("unexpected member fields: %+v", fetched)
	}

	// 3. List members
	list, err := store.ListFamilyMembers()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 member in list, got %d", len(list))
	}

	// 4. Update member
	fetched.Role = "member"
	fetched.Notes = "Updated device"
	if err := store.UpdateFamilyMember(fetched); err != nil {
		t.Fatalf("failed to update member: %v", err)
	}
	updated, _ := store.GetFamilyMember("mario")
	if updated.Role != "member" || updated.Notes != "Updated device" {
		t.Errorf("expected updated role and notes, got: %+v", updated)
	}

	// 5. Create & Validate Reset / Onboarding Token
	token, err := store.CreateResetToken("mario", 2*time.Hour)
	if err != nil || token == "" {
		t.Fatalf("failed to create token: %v", err)
	}
	tokenMember, err := store.ValidateResetToken(token)
	if err != nil || tokenMember == nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if tokenMember.Username != "mario" {
		t.Errorf("expected token for mario, got %s", tokenMember.Username)
	}

	// 6. Consume Token
	if err := store.ConsumeResetToken(token); err != nil {
		t.Fatalf("failed to consume token: %v", err)
	}
	_, err = store.ValidateResetToken(token)
	if err == nil {
		t.Errorf("expected error for consumed token, got nil")
	}

	// 7. Delete Member
	if err := store.DeleteFamilyMember("mario"); err != nil {
		t.Fatalf("failed to delete member: %v", err)
	}
	deleted, _ := store.GetFamilyMember("mario")
	if deleted != nil {
		t.Errorf("expected member to be deleted, got: %+v", deleted)
	}
}
