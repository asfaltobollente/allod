package updater

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/asfaltobollente/allod/internal/config"
	"github.com/asfaltobollente/allod/internal/state"
)

func setupTestEnv(t *testing.T) (string, *config.Config, *state.Store, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "allod-updater-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "state.db")
	st, err := state.Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("state.Open failed: %v", err)
	}

	cfg := &config.Config{
		Modules: map[string]config.ModuleConfig{
			"photos": {Level: "standard"},
			"cloud":  {Level: "standard"},
		},
	}

	// Find modules dir relative to repo
	repoModulesDir := filepath.Join("..", "..", "modules")
	if _, err := os.Stat(repoModulesDir); err != nil {
		repoModulesDir = filepath.Join("modules")
	}

	cleanup := func() {
		st.Close()
		os.RemoveAll(tmpDir)
	}

	return repoModulesDir, cfg, st, cleanup
}

func TestCheckUpdates(t *testing.T) {
	modulesDir, cfg, st, cleanup := setupTestEnv(t)
	defer cleanup()

	u := NewUpdater("test_out")
	updates := u.CheckUpdates(modulesDir, cfg, st)

	if len(updates) == 0 {
		t.Fatalf("expected non-empty updates map")
	}

	// Verify system module
	storageInfo, ok := updates["storage"]
	if !ok {
		t.Fatalf("expected 'storage' in updates")
	}
	if storageInfo.Type != "system" || storageInfo.UpdateAvailable {
		t.Errorf("unexpected storage info: %+v", storageInfo)
	}

	// Verify photos module before setting target
	photosInfo, ok := updates["photos"]
	if !ok {
		t.Fatalf("expected 'photos' in updates")
	}
	if photosInfo.Type != "container" {
		t.Errorf("expected photos type 'container', got %s", photosInfo.Type)
	}
	if photosInfo.UpdateAvailable {
		t.Errorf("expected photos UpdateAvailable false initially")
	}

	// Set update target in state.db
	err := st.SetMeta("module_update_target_photos", "v4.0.0")
	if err != nil {
		t.Fatalf("SetMeta failed: %v", err)
	}

	// Recheck
	updates2 := u.CheckUpdates(modulesDir, cfg, st)
	photosInfo2 := updates2["photos"]
	if !photosInfo2.UpdateAvailable {
		t.Errorf("expected UpdateAvailable true after setting target in state.db")
	}
	if photosInfo2.LatestTag != "v4.0.0" {
		t.Errorf("expected LatestTag 'v4.0.0', got '%s'", photosInfo2.LatestTag)
	}
}

func TestExecuteUpdateSuccess(t *testing.T) {
	modulesDir, cfg, st, cleanup := setupTestEnv(t)
	defer cleanup()

	u := NewUpdater("test_out")
	ctx := context.Background()

	report, err := u.ExecuteUpdate(ctx, "photos", "v3.3.0", modulesDir, cfg, st)
	if err != nil {
		t.Fatalf("ExecuteUpdate returned error: %v", err)
	}
	if !report.Success {
		t.Fatalf("expected report.Success true, got false. Error: %s", report.ErrorMessage)
	}
	if report.FinalState != StateCommitted {
		t.Errorf("expected FinalState %s, got %s", StateCommitted, report.FinalState)
	}
	if report.TargetTag != "v3.3.0" {
		t.Errorf("expected TargetTag 'v3.3.0', got %s", report.TargetTag)
	}
}

func TestExecuteUpdateRollback(t *testing.T) {
	modulesDir, cfg, st, cleanup := setupTestEnv(t)
	defer cleanup()

	u := NewUpdater("test_out")
	ctx := context.Background()

	report, err := u.ExecuteUpdate(ctx, "photos", "fail-healthcheck", modulesDir, cfg, st)
	if err != nil {
		t.Fatalf("ExecuteUpdate returned unexpected Go error: %v", err)
	}
	if report.Success {
		t.Fatalf("expected report.Success false for fail-healthcheck")
	}
	if report.FinalState != StateRolledBack {
		t.Errorf("expected FinalState %s, got %s", StateRolledBack, report.FinalState)
	}
}

func TestSimulateUpdate(t *testing.T) {
	_, cfg, st, cleanup := setupTestEnv(t)
	defer cleanup()

	u := NewUpdater("test_out")

	rep, err := u.SimulateUpdate("cloud", "30-apache", false, cfg, st)
	if err != nil || !rep.Success {
		t.Fatalf("expected successful simulation: %v", err)
	}
	if rep.FinalState != StateCommitted {
		t.Errorf("expected StateCommitted, got %s", rep.FinalState)
	}

	repFail, _ := u.SimulateUpdate("cloud", "30-apache", true, cfg, st)
	if repFail.Success {
		t.Fatalf("expected failure for failHealthCheck=true")
	}
	if repFail.FinalState != StateRolledBack {
		t.Errorf("expected StateRolledBack, got %s", repFail.FinalState)
	}
}
