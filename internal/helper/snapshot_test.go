package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteSnapshotCreate_Validation(t *testing.T) {
	// Invalid subvolume traversal
	res := ExecuteSnapshotCreate("/mnt/allod-storage", "shares/../../etc", "snap1", true, true)
	if res.Ok {
		t.Errorf("expected failure for subvolume traversal, got ok: %v", res)
	}

	// Invalid snapshot name
	res2 := ExecuteSnapshotCreate("/mnt/allod-storage", "shares", "snap;rm -rf", true, true)
	if res2.Ok {
		t.Errorf("expected failure for invalid snapshot name, got ok: %v", res2)
	}

	// Valid plan
	res3 := ExecuteSnapshotCreate("/mnt/allod-storage", "shares", "shares-20261025", true, true)
	if !res3.Ok {
		t.Fatalf("expected plan success, got error: %s", res3.Error)
	}
	if len(res3.Plan) != 1 {
		t.Fatalf("expected 1 plan command, got %d", len(res3.Plan))
	}
	expectedCmd := "btrfs subvolume snapshot -r /mnt/allod-storage/shares /mnt/allod-storage/.snapshots/shares-20261025"
	if res3.Plan[0] != expectedCmd {
		t.Errorf("plan command mismatch:\nexpected: %s\ngot:      %s", expectedCmd, res3.Plan[0])
	}
}

func TestExecuteSnapshotPrune_PlanAndLogic(t *testing.T) {
	tmpDir := t.TempDir()
	snapDir := filepath.Join(tmpDir, ".snapshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		t.Fatalf("failed to create temp snapDir: %v", err)
	}

	// Create 5 mock snapshots with spaced timestamps
	for i := 1; i <= 5; i++ {
		p := filepath.Join(snapDir, fmt.Sprintf("shares-snap%d", i))
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatalf("failed to create mock snap %s: %v", p, err)
		}
		// Adjust mtime so snap1 is oldest, snap5 is newest
		mtime := time.Now().Add(time.Duration(i-5) * time.Hour)
		_ = os.Chtimes(p, mtime, mtime)
	}

	// Prune keeping 2 newest (snap5 and snap4), planning deletion of snap1, snap2, snap3
	resPlan := ExecuteSnapshotPrune(tmpDir, "shares", 2, 0, true)
	if !resPlan.Ok {
		t.Fatalf("expected prune plan success, got error: %s", resPlan.Error)
	}
	if len(resPlan.Plan) != 3 {
		t.Fatalf("expected 3 commands in plan, got %d: %v", len(resPlan.Plan), resPlan.Plan)
	}

	// Now execute prune keeping 2
	resExec := ExecuteSnapshotPrune(tmpDir, "shares", 2, 0, false)
	if !resExec.Ok {
		t.Fatalf("expected prune execution success, got error: %s", resExec.Error)
	}
	if !strings.Contains(resExec.Output, "Pruned 3 snapshot(s)") {
		t.Errorf("unexpected output: %s", resExec.Output)
	}

	// Remaining snapshots should be exactly 2
	remaining, err := os.ReadDir(snapDir)
	if err != nil {
		t.Fatalf("failed to read snapDir: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining snapshots, got %d", len(remaining))
	}
}

func TestExecuteSnapshotPrune_MissingDir(t *testing.T) {
	tmpDir := t.TempDir()
	res := ExecuteSnapshotPrune(tmpDir, "", 5, 0, true)
	if !res.Ok {
		t.Errorf("expected ok for missing snapshots directory, got error: %s", res.Error)
	}
}
