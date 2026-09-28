//go:build linux

package helper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinpermsTighteningLogic(t *testing.T) {
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "bin")
	if err := os.Mkdir(testDir, 0777); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	fi, err := os.Lstat(testDir)
	if err != nil {
		t.Fatalf("lstat failed: %v", err)
	}
	currentMode := fi.Mode().Perm()
	if currentMode&0022 != 0 {
		newMode := currentMode &^ 0022
		if err := os.Chmod(testDir, newMode); err != nil {
			t.Fatalf("chmod failed: %v", err)
		}
		fiAfter, _ := os.Lstat(testDir)
		if fiAfter.Mode().Perm()&0022 != 0 {
			t.Fatalf("expected 0022 bits to be removed, got: %04o", fiAfter.Mode().Perm())
		}
	}
}
