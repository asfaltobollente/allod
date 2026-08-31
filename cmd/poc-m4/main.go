package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func fileSHA256(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func main() {
	fmt.Println("=== Inizio Test M4: Backup Federato Append-Only & Ripristino Verificato ===")

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "allod-m4-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Workdir: %s\n", tmpDir)

	resticExe := filepath.Join(tmpDir, "restic.exe")
	restServerExe := filepath.Join(tmpDir, "rest-server.exe")

	// 1. Download restic
	fmt.Println("Scaricamento restic...")
	download("https://github.com/restic/restic/releases/download/v0.16.4/restic_0.16.4_windows_amd64.zip", filepath.Join(tmpDir, "restic.zip"))
	exec.Command("tar", "-xf", filepath.Join(tmpDir, "restic.zip"), "-C", tmpDir).Run()
	os.Rename(filepath.Join(tmpDir, "restic_0.16.4_windows_amd64.exe"), resticExe)

	// 2. Download rest-server
	fmt.Println("Scaricamento rest-server...")
	download("https://github.com/restic/rest-server/releases/download/v0.12.1/rest-server_0.12.1_windows_amd64.tar.gz", filepath.Join(tmpDir, "rs.tar.gz"))
	exec.Command("tar", "-xzf", filepath.Join(tmpDir, "rs.tar.gz"), "-C", tmpDir).Run()
	os.Rename(filepath.Join(tmpDir, "rest-server_0.12.1_windows_amd64", "rest-server.exe"), restServerExe)

	repoPath := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoPath, 0755)

	// 3. Start rest-server in append-only mode
	fmt.Println("Avvio rest-server in modalità append-only...")
	cmdServer := exec.Command(restServerExe, "--append-only", "--no-auth", "--path", repoPath)
	err = cmdServer.Start()
	if err != nil {
		panic(err)
	}
	defer func() {
		fmt.Println("Terminazione rest-server...")
		cmdServer.Process.Kill()
	}()

	// Wait for server to boot
	time.Sleep(2 * time.Second)

	// 4. Init repository
	fmt.Println("\n--- Inizializzazione Repository ---")
	repoURL := "rest:http://127.0.0.1:8000/"
	os.Setenv("RESTIC_PASSWORD", "allod-secret")

	cmdInit := exec.Command(resticExe, "-r", repoURL, "init")
	cmdInit.Stdout = os.Stdout
	cmdInit.Stderr = os.Stderr
	if err := cmdInit.Run(); err != nil {
		panic(fmt.Sprintf("Errore init restic: %v", err))
	}

	// 5. Create test file and compute original SHA256
	fmt.Println("\n--- Esecuzione Backup & Calcolo Hash Originale ---")
	testContent := "Documenti e ricordi insostituibili dell'utente Allod - 2026"
	testFile := filepath.Join(tmpDir, "prezioso.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		panic(err)
	}

	origHash, err := fileSHA256(testFile)
	if err != nil {
		panic(err)
	}
	fmt.Printf("File originale: %s\nSHA-256 originale: %s\n", testFile, origHash)

	cmdBackup := exec.Command(resticExe, "-r", repoURL, "backup", testFile)
	cmdBackup.Stdout = os.Stdout
	cmdBackup.Stderr = os.Stderr
	if err := cmdBackup.Run(); err != nil {
		panic(fmt.Sprintf("Errore backup: %v", err))
	}

	// 6. Test Verified Restore (Ripristino Verificato)
	fmt.Println("\n--- Ripristino Verificato: Restore e Comparazione Hash ---")
	restoreDir := filepath.Join(tmpDir, "restore_target")
	os.MkdirAll(restoreDir, 0755)

	cmdRestore := exec.Command(resticExe, "-r", repoURL, "restore", "latest", "--target", restoreDir)
	cmdRestore.Stdout = os.Stdout
	cmdRestore.Stderr = os.Stderr
	if err := cmdRestore.Run(); err != nil {
		panic(fmt.Sprintf("Errore restore: %v", err))
	}

	// Restic preserves absolute path hierarchy under restore target
	// Find the restored file
	var restoredFilePath string
	err = filepath.Walk(restoreDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "prezioso.txt" {
			restoredFilePath = path
			return io.EOF // Stop walking
		}
		return nil
	})
	if err != nil && err != io.EOF {
		panic(err)
	}

	if restoredFilePath == "" {
		fmt.Println("❌ ERRORE: File ripristinato non trovato nel target di restore!")
	} else {
		restoredHash, err := fileSHA256(restoredFilePath)
		if err != nil {
			panic(err)
		}
		fmt.Printf("File ripristinato: %s\nSHA-256 ripristinato: %s\n", restoredFilePath, restoredHash)

		if origHash == restoredHash {
			fmt.Println("✅ SUCCESSO: Integrità verificata! Gli hash SHA-256 coincidono esattamente.")
		} else {
			fmt.Println("❌ ERRORE: Gli hash non coincidono!")
		}
	}

	// 7. Attempt ransomware attack (raw HTTP DELETE)
	fmt.Println("\n--- Simulazione Ransomware: Tentativo di cancellazione diretta (HTTP DELETE) ---")
	cmdDelete := exec.Command("curl", "-s", "-X", "DELETE", "-i", "http://127.0.0.1:8000/config")

	out, _ := cmdDelete.CombinedOutput()
	outputStr := string(out)
	fmt.Println(outputStr)

	if strings.Contains(outputStr, "403 Forbidden") {
		fmt.Println("✅ SUCCESSO: La richiesta di cancellazione è stata respinta dal peer! (403 Forbidden).")
		fmt.Println("Il server in modalità append-only protegge i dati garantendo l'immutabilità.")
	} else {
		fmt.Println("❌ FALLIMENTO: Il comando non è stato respinto come atteso.")
	}
}

func download(url, dest string) {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	out, err := os.Create(dest)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	io.Copy(out, resp.Body)
}
