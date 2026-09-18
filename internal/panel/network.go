package panel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asfaltobollente/allod/internal/config"
	"github.com/asfaltobollente/allod/internal/helper"
	"github.com/asfaltobollente/allod/internal/manifest"
	"github.com/asfaltobollente/allod/internal/quadlet"
)

// HelperClient provides an interface to execute root helper actions.
type HelperClient interface {
	Execute(action string, args map[string]interface{}, plan bool) (helper.Response, error)
}

// PanelResponse represents a standardized JSON response for the panel API.
type PanelResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// NetworkHandler handles network and Headscale mesh operations.
type NetworkHandler struct {
	Helper        HelperClient
	GetConfigPath func() string
	GetModulesDir func() string
}

// RegisterNetworkRoutes registers /api/network/* endpoints on the given ServeMux.
func RegisterNetworkRoutes(mux *http.ServeMux, h *NetworkHandler) {
	if h == nil {
		h = &NetworkHandler{}
	}
	if h.Helper == nil {
		h.Helper = &helper.Client{SocketPath: "/run/allod/helper.sock"}
	}

	mux.HandleFunc("/api/network/status", h.handleStatus)
	mux.HandleFunc("/api/network/configure", h.handleConfigure)
	mux.HandleFunc("/api/network/preauth-key", h.handlePreauthKey)
}

func (h *NetworkHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cfgPath := ""
	if h.GetConfigPath != nil {
		cfgPath = h.GetConfigPath()
	}
	cfg, _ := config.LoadConfig(cfgPath)

	netLevel := "off"
	if cfg != nil && cfg.Modules != nil {
		if m, ok := cfg.Modules["network"]; ok {
			netLevel = m.Level
		}
	}

	baseDir := quadlet.ResolvedStorageBaseDir()
	tokenFile := filepath.Join(baseDir, "network", "cloudflared.token")
	secretEnvFile := filepath.Join(baseDir, "network", "secrets", "cloudflared.env")
	tokBytes, _ := os.ReadFile(secretEnvFile)
	if len(tokBytes) == 0 {
		tokBytes, _ = os.ReadFile(tokenFile)
	}
	hasToken := len(strings.TrimSpace(string(tokBytes))) > 0

	hsConfigFile := filepath.Join(baseDir, "network", "headscale", "config", "config.yaml")
	domain := ""
	if hsBytes, err := os.ReadFile(hsConfigFile); err == nil {
		lines := strings.Split(string(hsBytes), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "server_url:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					domain = strings.TrimSpace(parts[1])
					domain = strings.Trim(domain, `"'`)
				}
				break
			}
		}
	}

	nodesList := []map[string]interface{}{}
	meshIP := "100.64.0.1"
	if out, err := executeHeadscale("nodes_list", h.Helper); err == nil && out != "" {
		_ = json.Unmarshal([]byte(out), &nodesList)
	}

	data := map[string]interface{}{
		"level":       netLevel,
		"enabled":     netLevel != "off",
		"has_token":   hasToken,
		"server_url":  domain,
		"mesh_ip":     meshIP,
		"nodes_count": len(nodesList),
		"nodes":       nodesList,
	}

	json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
}

// executeHeadscale attempts to execute a Headscale command directly in the rootless Podman
// session first. If direct execution fails or container is not running locally, it falls back
// to the root helper client.
func executeHeadscale(cmdType string, client HelperClient) (string, error) {
	isMatch := func(name string) bool {
		if strings.Contains(name, "cloudflared") {
			return false
		}
		return name == "network" || name == "systemd-network" ||
			name == "network-headscale" || name == "systemd-network-headscale" ||
			strings.HasPrefix(name, "network-") || strings.HasPrefix(name, "systemd-network-")
	}

	target := ""
	if out, err := exec.Command("podman", "ps", "--format", "{{.Names}}").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if isMatch(name) {
				target = name
				break
			}
		}
	}

	if target != "" {
		switch cmdType {
		case "preauthkey_create":
			// Ensure default user exists
			_ = exec.Command("podman", "exec", target, "headscale", "users", "create", "default").Run()
			cmd := exec.Command("podman", "exec", target, "headscale", "preauthkeys", "create", "-u", "default", "--reusable=false", "--expiration", "1h")
			if out, err := cmd.Output(); err == nil {
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				for i := len(lines) - 1; i >= 0; i-- {
					l := strings.TrimSpace(lines[i])
					if l != "" {
						return l, nil
					}
				}
			}
		case "nodes_list":
			cmd := exec.Command("podman", "exec", target, "headscale", "nodes", "list", "--output", "json")
			if out, err := cmd.Output(); err == nil {
				return strings.TrimSpace(string(out)), nil
			}
		case "users_list":
			cmd := exec.Command("podman", "exec", target, "headscale", "users", "list", "--output", "json")
			if out, err := cmd.Output(); err == nil {
				return strings.TrimSpace(string(out)), nil
			}
		}
	}

	// Fallback to helper client
	if client != nil {
		resp, err := client.Execute("network.headscale_cli", map[string]interface{}{"command": cmdType}, false)
		if err == nil && resp.Ok {
			raw := strings.TrimSpace(resp.Output)
			if cmdType == "preauthkey_create" {
				lines := strings.Split(raw, "\n")
				for i := len(lines) - 1; i >= 0; i-- {
					l := strings.TrimSpace(lines[i])
					if l != "" {
						return l, nil
					}
				}
			}
			return raw, nil
		}
		if resp.Error != "" {
			return "", fmt.Errorf("%s", resp.Error)
		}
		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("impossibile eseguire il comando headscale: container non trovato")
}

func (h *NetworkHandler) handleConfigure(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain      string `json:"domain"`
		TunnelToken string `json:"tunnel_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "JSON non valido"})
		return
	}

	domain := strings.TrimSpace(req.Domain)
	if domain != "" && !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}

	baseDir := quadlet.ResolvedStorageBaseDir()
	netDir := filepath.Join(baseDir, "network")
	_ = os.MkdirAll(netDir, 0777)
	secretsDir := filepath.Join(netDir, "secrets")
	_ = os.MkdirAll(secretsDir, 0700)

	if req.TunnelToken != "" {
		tokClean := strings.TrimSpace(req.TunnelToken)
		secretEnv := filepath.Join(secretsDir, "cloudflared.env")
		_ = os.WriteFile(secretEnv, []byte(fmt.Sprintf("TUNNEL_TOKEN=%s\n", tokClean)), 0600)
		tokenFile := filepath.Join(netDir, "cloudflared.token")
		_ = os.WriteFile(tokenFile, []byte(tokClean), 0600)
		envFile := filepath.Join(netDir, "cloudflared.env")
		_ = os.WriteFile(envFile, []byte(fmt.Sprintf("TUNNEL_TOKEN=%s\n", tokClean)), 0600)
	}

	if domain != "" {
		hsConfigFile := filepath.Join(netDir, "headscale", "config", "config.yaml")
		if hsBytes, err := os.ReadFile(hsConfigFile); err == nil {
			content := string(hsBytes)
			lines := strings.Split(content, "\n")
			newLines := []string{}
			for _, l := range lines {
				if strings.HasPrefix(strings.TrimSpace(l), "server_url:") {
					newLines = append(newLines, fmt.Sprintf("server_url: %s", domain))
				} else {
					newLines = append(newLines, l)
				}
			}
			_ = os.WriteFile(hsConfigFile, []byte(strings.Join(newLines, "\n")), 0644)
		}
	}

	// Rigenera unità Quadlet per riflettere le modifiche
	home, _ := os.UserHomeDir()
	if home != "" {
		quadDir := filepath.Join(home, ".config", "containers", "systemd")
		modDir := "modules"
		if h.GetModulesDir != nil {
			modDir = h.GetModulesDir()
		}
		mPath := filepath.Join(modDir, "network", "module.yaml")
		if m, err := manifest.LoadManifest(mPath); err == nil {
			if genRes, err := quadlet.Generate("network", m, "hybrid"); err == nil {
				for fname, content := range genRes.Files {
					_ = os.WriteFile(filepath.Join(quadDir, fname), []byte(content), 0644)
				}
			}
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}

	// Riavvia micro-servizi se attivi
	_ = exec.Command("systemctl", "--user", "restart", "network", "network-headscale", "network-cloudflared").Run()

	json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Configurazione salvata"})
}

func (h *NetworkHandler) handlePreauthKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	key, err := executeHeadscale("preauthkey_create", h.Helper)
	if err != nil || key == "" {
		errMsg := "Errore esecuzione comando Headscale (verifica che il modulo network sia avviato)"
		if err != nil {
			errMsg = err.Error()
		}
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
		return
	}

	baseDir := quadlet.ResolvedStorageBaseDir()
	hsConfigFile := filepath.Join(baseDir, "network", "headscale", "config", "config.yaml")
	domain := ""
	if hsBytes, err := os.ReadFile(hsConfigFile); err == nil {
		lines := strings.Split(string(hsBytes), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "server_url:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					domain = strings.TrimSpace(parts[1])
					domain = strings.Trim(domain, `"'`)
				}
				break
			}
		}
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status: "ok",
		Data: map[string]interface{}{
			"key":        key,
			"server_url": domain,
			"expires_in": "1 ora",
		},
	})
}
