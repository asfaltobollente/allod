//go:build linux

package helper

import (
	"log"
	"os"
	"path/filepath"
)

// RepairSystemBinPermissions tightens permissions on /usr/local/bin and /usr/local/sbin
// by removing write permissions for group and other (mode &^ 0022), restoring Allod
// binaries to root:root ownership and 0755 permissions, without following symlinks.
func RepairSystemBinPermissions() error {
	dirs := []string{"/usr/local/bin", "/usr/local/sbin"}
	for _, dir := range dirs {
		fi, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			log.Printf("[SECURITY] %s is a symlink, skipping permission tightening", dir)
			continue
		}
		if !fi.IsDir() {
			continue
		}
		currentMode := fi.Mode().Perm()
		if currentMode&0022 != 0 {
			newMode := currentMode &^ 0022
			if err := os.Chmod(dir, newMode); err == nil {
				log.Printf("[SECURITY] Permessi corretti su %s (da %04o a %04o)", dir, currentMode, newMode)
			} else {
				log.Printf("[SECURITY] Errore correzione permessi su %s: %v", dir, err)
			}
		}
	}

	allodBins := []string{"allod", "allod-helperd", "allod-panel", "allod-watch"}
	for _, bin := range allodBins {
		p := filepath.Join("/usr/local/bin", bin)
		fi, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			log.Printf("[SECURITY] %s is a symlink, skipping ownership/mode fix", p)
			continue
		}
		if fi.IsDir() {
			continue
		}
		// Ensure root:root ownership
		if err := os.Lchown(p, 0, 0); err == nil {
			log.Printf("[SECURITY] Proprietà di %s ripristinata a root:root", p)
		}
		// Ensure 0755 mode
		if fi.Mode().Perm() != 0755 {
			if err := os.Chmod(p, 0755); err == nil {
				log.Printf("[SECURITY] Permessi di %s impostati a 0755 (erano %04o)", p, fi.Mode().Perm())
			}
		}
	}
	return nil
}
