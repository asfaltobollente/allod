package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/asfaltobollente/allod/internal/helper"
)

// HelperUpgradeResult contains the outcome of an attempted helper daemon upgrade.
type HelperUpgradeResult struct {
	UpToDate      bool   `json:"up_to_date"`
	RestartQueued bool   `json:"restart_queued"`
	ManualCommand string `json:"manual_command,omitempty"`
	Message       string `json:"message"`
}

// computeFileSHA256 returns the hex-encoded SHA-256 hash of a file.
func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// UpgradeAndRestartHelper securely verifies and updates the allod-helperd daemon.
// The panel (running rootless) compiles the helper binary locally, compares its SHA-256
// with the installed binary in /usr/local/bin/allod-helperd. If they match, it simply
// triggers a restart via the helper socket. If they differ, it refuses to overwrite
// system directories and returns the exact manual sudo install command required.
func UpgradeAndRestartHelper(client *helper.Client, logReport *strings.Builder) (*HelperUpgradeResult, error) {
	// 1. Build newly compiled binary in panel directory
	localBinary := "allod-helperd"
	localNew := "allod-helperd.new"
	_ = os.Remove(localNew)

	buildOut, errBuild := buildGoBinary(localNew, "./cmd/allod-helperd")
	if logReport != nil {
		logReport.WriteString(fmt.Sprintf("[go build -o %s ./cmd/allod-helperd]\n%s\n", localNew, string(buildOut)))
	}
	if errBuild != nil {
		// Fallback to building directly
		buildOut2, errBuild2 := buildGoBinary(localBinary, "./cmd/allod-helperd")
		if errBuild2 != nil {
			return nil, fmt.Errorf("compilazione allod-helperd fallita: %w (%s)", errBuild2, string(buildOut2))
		}
	} else {
		_ = os.Remove(localBinary)
		_ = os.Rename(localNew, localBinary)
		_ = os.Chmod(localBinary, 0755)
	}

	absLocalBinary, _ := filepath.Abs(localBinary)

	// 2. Hash local binary
	localHash, errLocalHash := computeFileSHA256(localBinary)
	if errLocalHash != nil {
		return nil, fmt.Errorf("impossibile calcolare SHA256 del binario locale: %w", errLocalHash)
	}

	// 3. Hash installed binary in /usr/local/bin/allod-helperd
	installedPath := "/usr/local/bin/allod-helperd"
	installedHash, errInstalled := computeFileSHA256(installedPath)

	manualCmd := fmt.Sprintf("sudo install -o root -g root -m 0755 %s %s && sudo systemctl restart allod-helperd", absLocalBinary, installedPath)

	if errInstalled == nil && localHash == installedHash {
		// Identical binary: safe to trigger service restart
		if logReport != nil {
			logReport.WriteString("✓ Il binario installato in /usr/local/bin/allod-helperd è già identico (SHA256 matcha). Riavvio servizio in corso...\n")
		}
		if client != nil {
			res, errRestart := client.Execute("service.restart", map[string]interface{}{"unit": "allod-helperd"}, false)
			if errRestart == nil && res.Ok {
				return &HelperUpgradeResult{
					UpToDate:      true,
					RestartQueued: true,
					Message:       "allod-helperd già aggiornato, riavvio del servizio accodato con successo.",
				}, nil
			}
		}
		return &HelperUpgradeResult{
			UpToDate:      true,
			RestartQueued: false,
			ManualCommand: "sudo systemctl restart allod-helperd",
			Message:       "Binario allod-helperd aggiornato, eseguire il riavvio manuale del servizio.",
		}, nil
	}

	// Different binary or not yet installed: root privilege required
	if logReport != nil {
		logReport.WriteString(fmt.Sprintf("ℹ️ Il nuovo binario allod-helperd richiede privilegi root per l'installazione in %s.\nEsegui da terminale:\n%s\n", installedPath, manualCmd))
	}

	return &HelperUpgradeResult{
		UpToDate:      false,
		RestartQueued: false,
		ManualCommand: manualCmd,
		Message:       "Il binario del root helper è stato compilato ma richiede privilegi di root per essere installato in /usr/local/bin.",
	}, nil
}
