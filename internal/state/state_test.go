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

	// 7. Set & verify family member password
	m2 := &FamilyMember{
		Username:  "anna",
		FirstName: "Anna",
		LastName:  "Rossi",
	}
	if err := store.CreateFamilyMember(m2); err != nil {
		t.Fatalf("failed to create anna: %v", err)
	}
	if err := store.SetFamilyMemberPassword("anna", "fakehash123", "fakesalt456"); err != nil {
		t.Fatalf("failed to set password for anna: %v", err)
	}
	fetchedAnna, err := store.GetFamilyMember("anna")
	if err != nil || fetchedAnna == nil {
		t.Fatalf("failed to get anna: %v", err)
	}
	if fetchedAnna.PasswordHash != "fakehash123" || fetchedAnna.PasswordSalt != "fakesalt456" {
		t.Errorf("expected anna password fields to match, got hash=%q salt=%q", fetchedAnna.PasswordHash, fetchedAnna.PasswordSalt)
	}

	// 8. Delete Member
	if err := store.DeleteFamilyMember("mario"); err != nil {
		t.Fatalf("failed to delete member: %v", err)
	}
	deleted, _ := store.GetFamilyMember("mario")
	if deleted != nil {
		t.Errorf("expected member to be deleted, got: %+v", deleted)
	}
}

func TestAdminAuthLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-admin-auth.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer store.Close()

	hasAuth, err := store.HasAdminAuth()
	if err != nil {
		t.Fatalf("HasAdminAuth error: %v", err)
	}
	if hasAuth {
		t.Errorf("expected false for new db")
	}

	if err := store.SetAdminAuth("hash_xyz", "salt_123"); err != nil {
		t.Fatalf("failed to set admin auth: %v", err)
	}

	hasAuth, err = store.HasAdminAuth()
	if err != nil || !hasAuth {
		t.Errorf("expected true after setting admin auth")
	}

	h, s, err := store.GetAdminAuth()
	if err != nil || h != "hash_xyz" || s != "salt_123" {
		t.Errorf("unexpected admin auth: hash=%s, salt=%s", h, s)
	}

	if err := store.ClearAdminAuth(); err != nil {
		t.Fatalf("failed to clear admin auth: %v", err)
	}
	hasAuth, _ = store.HasAdminAuth()
	if hasAuth {
		t.Errorf("expected false after clear")
	}
}

func TestSessionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-session.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer store.Close()

	// 1. Create session
	token, err := store.CreateSession("admin", "admin", 1*time.Hour)
	if err != nil || token == "" {
		t.Fatalf("failed to create session: %v", err)
	}

	// 2. Get session
	sess, err := store.GetSession(token)
	if err != nil || sess == nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if sess.UserType != "admin" || sess.Username != "admin" {
		t.Errorf("unexpected session data: %+v", sess)
	}

	// 3. Expired session
	shortToken, err := store.CreateSession("family", "mario", -10*time.Second)
	if err != nil {
		t.Fatalf("failed to create expired session: %v", err)
	}
	expSess, err := store.GetSession(shortToken)
	if err != nil {
		t.Fatalf("unexpected error getting expired session: %v", err)
	}
	if expSess != nil {
		t.Errorf("expected nil for expired session, got: %+v", expSess)
	}

	// 4. Delete session
	if err := store.DeleteSession(token); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}
	delSess, _ := store.GetSession(token)
	if delSess != nil {
		t.Errorf("expected session to be deleted, got: %+v", delSess)
	}
}
