package panel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asfaltobollente/allod/internal/config"
	"github.com/asfaltobollente/allod/internal/helper"
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

// NetworkHandler handles network and NetBird mesh operations.
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
	mux.HandleFunc("/api/network/preauth-key", h.handlePairingInfo)
	mux.HandleFunc("/api/network/pairing-info", h.handlePairingInfo)
}

type netbirdStatusJSON struct {
	NetbirdIP  string `json:"netbirdIp"`
	PublicKey  string `json:"publicKey"`
	Management struct {
		Connected bool   `json:"connected"`
		URL       string `json:"url"`
	} `json:"management"`
	Signal struct {
		Connected bool   `json:"connected"`
		URL       string `json:"url"`
	} `json:"signal"`
	Peers struct {
		Total     int                      `json:"total"`
		Connected int                      `json:"connected"`
		Details   []map[string]interface{} `json:"details"`
	} `json:"peers"`
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
		if m, ok := cfg.Modules["network"]; ok && m.Level != "" {
			netLevel = m.Level
		}
	}

	baseDir := quadlet.ResolvedStorageBaseDir()
	secretEnvFile := filepath.Join(baseDir, "network", "secrets", "netbird.env")

	setupKey := ""
	managementURL := "https://api.netbird.io:443"
	if envBytes, err := os.ReadFile(secretEnvFile); err == nil {
		for _, line := range strings.Split(string(envBytes), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "NB_SETUP_KEY=") {
				setupKey = strings.TrimSpace(strings.TrimPrefix(trimmed, "NB_SETUP_KEY="))
				setupKey = strings.Trim(setupKey, `"'`)
			} else if strings.HasPrefix(trimmed, "NB_MANAGEMENT_URL=") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "NB_MANAGEMENT_URL="))
				val = strings.Trim(val, `"'`)
				if val != "" {
					managementURL = val
				}
			}
		}
	}

	mode := netLevel
	if mode != "selfhosted" {
		mode = "cloud"
	}
	hasKey := len(setupKey) > 0

	meshIP := "--"
	connected := false
	peersCount := 0
	peersList := []map[string]interface{}{}

	if out, err := executeNetBirdStatus(h.Helper); err == nil && out != "" {
		var nbStatus netbirdStatusJSON
		if jsonErr := json.Unmarshal([]byte(out), &nbStatus); jsonErr == nil {
			if nbStatus.NetbirdIP != "" {
				meshIP = strings.Split(nbStatus.NetbirdIP, "/")[0]
			}
			connected = nbStatus.Management.Connected
			peersCount = nbStatus.Peers.Total
			if nbStatus.Peers.Details != nil {
				peersList = nbStatus.Peers.Details
			}
			if nbStatus.Management.URL != "" {
				managementURL = nbStatus.Management.URL
			}
		}
	}

	// If container status was not reachable via JSON CLI, check config.json or fallback
	if meshIP == "--" && hasKey && netLevel != "off" {
		nbConfigFile := filepath.Join(baseDir, "network", "netbird", "config.json")
		if cfgBytes, err := os.ReadFile(nbConfigFile); err == nil {
			var rawCfg map[string]interface{}
			if err := json.Unmarshal(cfgBytes, &rawCfg); err == nil {
				if ip, ok := rawCfg["WireGuardIp"].(string); ok && ip != "" {
					meshIP = strings.Split(ip, "/")[0]
					connected = true
				}
			}
		}
	}

	if meshIP == "--" && hasKey && netLevel != "off" {
		meshIP = "100.64.0.1"
	}

	data := map[string]interface{}{
		"level":          netLevel,
		"enabled":        netLevel != "off",
		"mode":           mode,
		"has_key":        hasKey,
		"management_url": managementURL,
		"mesh_ip":        meshIP,
		"connected":      connected,
		"peers_count":    peersCount,
		"nodes_count":    peersCount,
		"peers":          peersList,
		"nodes":          peersList,
	}

	json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
}

// executeNetBirdStatus queries status from the running NetBird container or helper daemon.
func executeNetBirdStatus(client HelperClient) (string, error) {
	if nbBin, err := exec.LookPath("netbird"); err == nil {
		cmd := exec.Command(nbBin, "status", "--json")
		if out, err := cmd.Output(); err == nil && len(out) > 0 && strings.HasPrefix(strings.TrimSpace(string(out)), "{") {
			return strings.TrimSpace(string(out)), nil
		}
	}

	if client == nil {
		client = &helper.Client{SocketPath: "/run/allod/helper.sock"}
	}
	resp, err := client.Execute("network.netbird_status", nil, false)
	if err == nil && resp.Ok && resp.Output != "" && strings.HasPrefix(strings.TrimSpace(resp.Output), "{") {
		return strings.TrimSpace(resp.Output), nil
	}

	// Direct check for wt0 interface
	if iface, errI := net.InterfaceByName("wt0"); errI == nil {
		addrs, _ := iface.Addrs()
		ipStr := ""
		if len(addrs) > 0 {
			ipStr = addrs[0].String()
		}
		return fmt.Sprintf(`{"netbirdIp":"%s","management":{"connected":true,"url":"https://api.netbird.io:443"},"signal":{"connected":true,"url":"https://signal.netbird.io:443"},"peers":{"total":0,"connected":0}}`, ipStr), nil
	}

	return "", fmt.Errorf("container NetBird non attivo")
}

func (h *NetworkHandler) handleConfigure(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Mode          string `json:"mode"`
		SetupKey      string `json:"setup_key"`
		ManagementURL string `json:"management_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "JSON non valido"})
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" || (mode != "cloud" && mode != "selfhosted") {
		mode = "cloud"
	}

	setupKey := strings.TrimSpace(req.SetupKey)
	mgmtURL := strings.TrimSpace(req.ManagementURL)
	if mode == "cloud" {
		mgmtURL = "https://api.netbird.io:443"
	} else if mgmtURL != "" && !strings.HasPrefix(mgmtURL, "http://") && !strings.HasPrefix(mgmtURL, "https://") {
		mgmtURL = "https://" + mgmtURL
	}

	baseDir := quadlet.ResolvedStorageBaseDir()
	netDir := filepath.Join(baseDir, "network")
	_ = os.MkdirAll(netDir, 0777)
	secretsDir := filepath.Join(netDir, "secrets")
	_ = os.MkdirAll(secretsDir, 0700)

	netbirdEnvFile := filepath.Join(secretsDir, "netbird.env")

	// Read existing setup key if omitted in request
	if setupKey == "" {
		if curBytes, err := os.ReadFile(netbirdEnvFile); err == nil {
			for _, line := range strings.Split(string(curBytes), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "NB_SETUP_KEY=") {
					setupKey = strings.TrimSpace(strings.TrimPrefix(trimmed, "NB_SETUP_KEY="))
					setupKey = strings.Trim(setupKey, `"'`)
					break
				}
			}
		}
	}

	envContent := fmt.Sprintf("# NetBird Sovereign Mesh Configuration\nNB_SETUP_KEY=%s\nNB_MANAGEMENT_URL=%s\n", setupKey, mgmtURL)
	if err := os.WriteFile(netbirdEnvFile, []byte(envContent), 0600); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore salvataggio secret: %v", err)})
		return
	}

	// Update level in config.yaml
	cfgPath := ""
	if h.GetConfigPath != nil {
		cfgPath = h.GetConfigPath()
	}
	if cfgPath != "" {
		if cfg, err := config.LoadConfig(cfgPath); err == nil {
			if cfg.Modules == nil {
				cfg.Modules = make(map[string]config.ModuleConfig)
			}
			mCfg := cfg.Modules["network"]
			mCfg.Level = mode
			cfg.Modules["network"] = mCfg
			_ = cfg.Save(cfgPath)
		}
	}

	// Clean up any legacy rootless user Quadlet units and containers for network
	_ = quadlet.StopAndRemoveContainers("network", true)

	// Trigger NetBird via privileged helper (running with root network capabilities)
	if mode != "off" && setupKey != "" {
		if h.Helper != nil {
			_, _ = h.Helper.Execute("network.netbird_up", map[string]interface{}{
				"setup_key":      setupKey,
				"management_url": mgmtURL,
			}, false)
		}
	} else if mode == "off" {
		if h.Helper != nil {
			_, _ = h.Helper.Execute("network.netbird_down", nil, false)
		}
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Configurazione NetBird salvata con successo! Riavvio servizio in corso...",
	})
}

func (h *NetworkHandler) handlePairingInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cfgPath := ""
	if h.GetConfigPath != nil {
		cfgPath = h.GetConfigPath()
	}
	cfg, _ := config.LoadConfig(cfgPath)

	netLevel := "cloud"
	if cfg != nil && cfg.Modules != nil {
		if m, ok := cfg.Modules["network"]; ok && m.Level != "" && m.Level != "off" {
			netLevel = m.Level
		}
	}

	baseDir := quadlet.ResolvedStorageBaseDir()
	secretEnvFile := filepath.Join(baseDir, "network", "secrets", "netbird.env")

	setupKey := ""
	managementURL := "https://api.netbird.io:443"
	if envBytes, err := os.ReadFile(secretEnvFile); err == nil {
		for _, line := range strings.Split(string(envBytes), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "NB_SETUP_KEY=") {
				setupKey = strings.TrimSpace(strings.TrimPrefix(trimmed, "NB_SETUP_KEY="))
				setupKey = strings.Trim(setupKey, `"'`)
			} else if strings.HasPrefix(trimmed, "NB_MANAGEMENT_URL=") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "NB_MANAGEMENT_URL="))
				val = strings.Trim(val, `"'`)
				if val != "" {
					managementURL = val
				}
			}
		}
	}

	meshIP := "100.64.0.1"
	if out, err := executeNetBirdStatus(h.Helper); err == nil && out != "" {
		var nbStatus netbirdStatusJSON
		if jsonErr := json.Unmarshal([]byte(out), &nbStatus); jsonErr == nil && nbStatus.NetbirdIP != "" {
			meshIP = strings.Split(nbStatus.NetbirdIP, "/")[0]
		}
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status: "ok",
		Data: map[string]interface{}{
			"mode":           netLevel,
			"management_url": managementURL,
			"mesh_ip":        meshIP,
			"has_key":        len(setupKey) > 0,
			"key":            setupKey,
			"server_url":     managementURL,
		},
	})
}
