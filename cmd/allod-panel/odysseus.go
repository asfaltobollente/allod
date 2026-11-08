package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	// Preferred pattern: "Temporary password: <value>" (case-insensitive, ':' or '=').
	tempPasswordRegex = regexp.MustCompile(`(?i)temporary\s+(?:admin\s+)?password\s*[:=]\s*(\S+)`)
	// Fallback pattern: any "password: <value>" / "password=<value>" line.
	genericPasswordRegex = regexp.MustCompile(`(?i)\bpassword\s*[:=]\s*(\S+)`)
)

// extractOdysseusTempPassword scans service/container logs and returns the most recent
// auto-generated admin password printed by Odysseus on first run. Returns "" if none found.
func extractOdysseusTempPassword(logs string) string {
	logs = ansiEscapeRegex.ReplaceAllString(logs, "")

	pick := func(re *regexp.Regexp) string {
		found := ""
		for _, line := range strings.Split(logs, "\n") {
			if m := re.FindStringSubmatch(line); m != nil {
				if v := strings.Trim(m[1], "\"'`"); v != "" {
					found = v
				}
			}
		}
		return found
	}

	if p := pick(tempPasswordRegex); p != "" {
		return p
	}
	return pick(genericPasswordRegex)
}

// readOdysseusLogs collects the logs of the odysseus unit/container, trying journald first
// and falling back to podman logs.
func readOdysseusLogs(ctx context.Context) string {
	var sb strings.Builder

	jctx, cancelJ := context.WithTimeout(ctx, 4*time.Second)
	out, _ := exec.CommandContext(jctx, "journalctl", "--user", "-u", "odysseus.service", "--no-pager", "-o", "cat").CombinedOutput()
	cancelJ()
	sb.Write(out)

	pctx, cancelP := context.WithTimeout(ctx, 4*time.Second)
	out, _ = exec.CommandContext(pctx, "podman", "logs", "odysseus").CombinedOutput()
	cancelP()
	sb.WriteString("\n")
	sb.Write(out)

	return sb.String()
}

// handleOdysseusTempPassword returns the Odysseus temporary admin password found in the logs.
// The route lives under /api/ and is therefore protected by the admin session middleware.
func handleOdysseusTempPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	password := extractOdysseusTempPassword(readOdysseusLogs(r.Context()))
	if password == "" {
		json.NewEncoder(w).Encode(PanelResponse{
			Status: "not_found",
			Message: "Nessuna password temporanea trovata nei log. Potrebbe essere già stata cambiata, " +
				"oppure i log sono stati ruotati. Per rigenerarla: ferma il modulo, elimina " +
				"/mnt/allod-storage/odysseus/data/auth.json e riavvialo.",
		})
		return
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status: "ok",
		Data:   map[string]string{"username": "admin", "password": password},
	})
}

// OdysseusDetectResult describes the detected Odysseus deployment configuration.
type OdysseusDetectResult struct {
	Path        string `json:"path"`
	Mode        string `json:"mode"` // "git", "docker", "quadlet"
	ModeSummary string `json:"mode_summary"`
	CanUpdate   bool   `json:"can_update"`
}

// OdysseusUpdateReport contains the full log output and status of an update execution.
type OdysseusUpdateReport struct {
	Mode    string   `json:"mode"`
	Path    string   `json:"path"`
	Steps   []string `json:"steps"`
	Output  string   `json:"output"`
	Success bool     `json:"success"`
}

// detectOdysseus finds the directory and deployment mode of Odysseus.
func detectOdysseus(customPath string) OdysseusDetectResult {
	if customPath != "" {
		if fi, err := os.Stat(customPath); err == nil && fi.IsDir() {
			mode := detectOdysseusMode(customPath)
			return OdysseusDetectResult{
				Path:        customPath,
				Mode:        mode,
				ModeSummary: formatModeSummary(mode, customPath),
				CanUpdate:   true,
			}
		}
	}

	var candidates []string
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		candidates = append(candidates, filepath.Join(u.HomeDir, "odysseus"))
	}
	if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, "odysseus"))
	}
	candidates = append(candidates,
		"/mnt/allod-storage/odysseus",
		"/mnt/allod-storage/odysseus/src",
		"/opt/odysseus",
		"/var/lib/odysseus",
		"./odysseus",
	)

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			mode := detectOdysseusMode(c)
			if mode == "git" || mode == "docker" {
				return OdysseusDetectResult{
					Path:        c,
					Mode:        mode,
					ModeSummary: formatModeSummary(mode, c),
					CanUpdate:   true,
				}
			}
		}
	}

	return OdysseusDetectResult{
		Path:        "/mnt/allod-storage/odysseus",
		Mode:        "quadlet",
		ModeSummary: "Container Podman / Quadlet (Allod Native Stack)",
		CanUpdate:   true,
	}
}

func detectOdysseusMode(dir string) string {
	composeFiles := []string{
		filepath.Join(dir, "docker-compose.yml"),
		filepath.Join(dir, "docker-compose.yaml"),
		filepath.Join(dir, "compose.yml"),
		filepath.Join(dir, "compose.yaml"),
	}
	for _, cf := range composeFiles {
		if _, err := os.Stat(cf); err == nil {
			return "docker"
		}
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return "git"
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return "git"
	}

	return "unknown"
}

func formatModeSummary(mode, path string) string {
	switch mode {
	case "git":
		return fmt.Sprintf("Repository Git con Dipendenze Python (%s)", path)
	case "docker":
		return fmt.Sprintf("Stack Docker / Podman Compose (%s)", path)
	case "quadlet":
		return "Container Podman Quadlet (ghcr.io/odysseus-dev/odysseus)"
	default:
		return fmt.Sprintf("Personalizzato (%s)", path)
	}
}

// executeOdysseusUpdate runs the step-by-step update process.
func executeOdysseusUpdate(ctx context.Context, det OdysseusDetectResult) *OdysseusUpdateReport {
	report := &OdysseusUpdateReport{
		Mode:    det.Mode,
		Path:    det.Path,
		Steps:   []string{},
		Success: true,
	}

	var sb strings.Builder
	logStep := func(step, output string, err error) {
		report.Steps = append(report.Steps, step)
		sb.WriteString("=== " + step + " ===\n")
		if strings.TrimSpace(output) != "" {
			sb.WriteString(strings.TrimSpace(output) + "\n")
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("❌ Errore: %v\n", err))
			report.Success = false
		} else {
			sb.WriteString("✓ OK\n")
		}
		sb.WriteString("\n")
	}

	switch det.Mode {
	case "git":
		// Step 1: git pull
		cmdGit := exec.CommandContext(ctx, "git", "-C", det.Path, "pull", "origin", "main")
		outGit, errGit := cmdGit.CombinedOutput()
		if errGit != nil {
			cmdGit2 := exec.CommandContext(ctx, "git", "-C", det.Path, "pull")
			outGit2, errGit2 := cmdGit2.CombinedOutput()
			if errGit2 != nil {
				logStep(fmt.Sprintf("git pull in %s", det.Path), string(outGit)+"\n"+string(outGit2), errGit2)
			} else {
				logStep(fmt.Sprintf("git pull in %s", det.Path), string(outGit2), nil)
			}
		} else {
			logStep(fmt.Sprintf("git pull origin main in %s", det.Path), string(outGit), nil)
		}

		// Step 2: pip install -r requirements.txt
		reqPath := filepath.Join(det.Path, "requirements.txt")
		if _, errReq := os.Stat(reqPath); errReq == nil {
			pipBin := "pip3"
			venvPips := []string{
				filepath.Join(det.Path, "venv", "bin", "pip"),
				filepath.Join(det.Path, ".venv", "bin", "pip"),
				filepath.Join(det.Path, "env", "bin", "pip"),
			}
			for _, vp := range venvPips {
				if fi, errV := os.Stat(vp); errV == nil && !fi.IsDir() {
					pipBin = vp
					break
				}
			}
			cmdPip := exec.CommandContext(ctx, pipBin, "install", "-r", "requirements.txt")
			cmdPip.Dir = det.Path
			outPip, errPip := cmdPip.CombinedOutput()
			logStep(fmt.Sprintf("%s install -r requirements.txt", pipBin), string(outPip), errPip)
		}

		// Step 3: Restart Odysseus
		cmdRestart := exec.CommandContext(ctx, "systemctl", "--user", "restart", "odysseus")
		outRestart, errRestart := cmdRestart.CombinedOutput()
		if errRestart != nil {
			cmdPodman := exec.CommandContext(ctx, "podman", "restart", "odysseus")
			outPodman, errPodman := cmdPodman.CombinedOutput()
			if errPodman != nil {
				logStep("Riavvio servizio Odysseus (systemctl/podman)", string(outRestart)+"\n"+string(outPodman), errPodman)
			} else {
				logStep("Riavvio container Odysseus (podman restart)", string(outPodman), nil)
			}
		} else {
			logStep("Riavvio servizio Odysseus (systemctl --user restart odysseus)", string(outRestart), nil)
		}

	case "docker":
		composeFile := "docker-compose.yml"
		for _, cf := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
			if _, err := os.Stat(filepath.Join(det.Path, cf)); err == nil {
				composeFile = cf
				break
			}
		}

		composeBin := "docker"
		composeArgs := []string{"compose", "-f", composeFile}
		if _, errD := exec.LookPath("docker"); errD != nil {
			if _, errP := exec.LookPath("podman-compose"); errP == nil {
				composeBin = "podman-compose"
				composeArgs = []string{"-f", composeFile}
			} else if _, errP2 := exec.LookPath("podman"); errP2 == nil {
				composeBin = "podman"
				composeArgs = []string{"compose", "-f", composeFile}
			}
		}

		// docker compose down
		cmdDown := exec.CommandContext(ctx, composeBin, append(composeArgs, "down")...)
		cmdDown.Dir = det.Path
		outDown, _ := cmdDown.CombinedOutput()
		logStep(fmt.Sprintf("%s down", composeBin), string(outDown), nil)

		// docker compose pull
		cmdPull := exec.CommandContext(ctx, composeBin, append(composeArgs, "pull")...)
		cmdPull.Dir = det.Path
		outPull, errPull := cmdPull.CombinedOutput()
		logStep(fmt.Sprintf("%s pull", composeBin), string(outPull), errPull)

		// docker compose up -d --build
		cmdUp := exec.CommandContext(ctx, composeBin, append(composeArgs, "up", "-d", "--build")...)
		cmdUp.Dir = det.Path
		outUp, errUp := cmdUp.CombinedOutput()
		logStep(fmt.Sprintf("%s up -d --build", composeBin), string(outUp), errUp)

	case "quadlet":
		imageRef := "ghcr.io/odysseus-dev/odysseus:latest"
		cmdPull := exec.CommandContext(ctx, "podman", "pull", imageRef)
		outPull, errPull := cmdPull.CombinedOutput()
		logStep("podman pull "+imageRef, string(outPull), errPull)

		_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
		cmdRestart := exec.CommandContext(ctx, "systemctl", "--user", "restart", "odysseus.service")
		outRestart, errRestart := cmdRestart.CombinedOutput()
		if errRestart != nil {
			cmdRestart2 := exec.CommandContext(ctx, "podman", "restart", "odysseus")
			outRestart2, errRestart2 := cmdRestart2.CombinedOutput()
			logStep("Riavvio container odysseus", string(outRestart)+"\n"+string(outRestart2), errRestart2)
		} else {
			logStep("systemctl --user restart odysseus.service", string(outRestart), nil)
		}
	}

	report.Output = sb.String()
	return report
}

// handleOdysseusUpdate inspects or triggers the Odysseus update process.
func handleOdysseusUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		customPath := strings.TrimSpace(r.URL.Query().Get("path"))
		det := detectOdysseus(customPath)
		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data:   det,
		})
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	det := detectOdysseus(strings.TrimSpace(req.Path))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	report := executeOdysseusUpdate(ctx, det)
	if !report.Success {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "error",
			Message: "Aggiornamento di Odysseus completato con segnalazioni. Consulta i log per i dettagli.",
			Data:    report,
		})
		return
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Odysseus aggiornato e riavviato con successo!",
		Data:    report,
	})
}
