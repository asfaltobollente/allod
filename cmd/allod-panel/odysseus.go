package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
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
