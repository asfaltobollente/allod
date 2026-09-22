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
	mux.HandleFunc("/api/network/install-native", h.handleInstallNative)
	mux.HandleFunc("/api/network/server/start", h.handleServerStart)
	mux.HandleFunc("/api/network/server/stop", h.handleServerStop)
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
	if mode == "selfhosted" {
		mode = "selfhosted_remote"
	}
	if mode != "selfhosted_remote" && mode != "selfhosted_managed" {
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

	// If container status was not reachable via JSON CLI, check wt0 interface directly for assigned IP
	if meshIP == "--" {
		if ip := helper.GetInterfaceIPv4("wt0"); ip != "" {
			meshIP = ip
		}
	}

	// If container status was not reachable via JSON CLI or wt0, check config.json for assigned IP
	if meshIP == "--" && hasKey && netLevel != "off" {
		nbConfigFile := filepath.Join(baseDir, "network", "netbird", "config.json")
		if cfgBytes, err := os.ReadFile(nbConfigFile); err == nil {
			var rawCfg map[string]interface{}
			if err := json.Unmarshal(cfgBytes, &rawCfg); err == nil {
				if ip, ok := rawCfg["WireGuardIp"].(string); ok && ip != "" {
					meshIP = strings.Split(ip, "/")[0]
				}
			}
		}
	}

	// Detect client runtime: native host binary vs podman container
	nativeInstalled := false
	if _, err := exec.LookPath("netbird"); err == nil {
		nativeInstalled = true
	} else if _, err := os.Stat("/usr/bin/netbird"); err == nil {
		nativeInstalled = true
	}

	clientRuntime := "container"
	if netLevel == "off" {
		clientRuntime = "stopped"
	} else if nativeInstalled {
		clientRuntime = "native"
	}

	// Managed server state
	serverManaged := map[string]interface{}{
		"running":           false,
		"server_running":    false,
		"dashboard_running": false,
		"domain":            "127.0.0.1",
		"port":              33073,
		"dashboard_port":    8088,
		"dashboard_url":     "http://127.0.0.1:8088",
		"management_url":    "http://127.0.0.1:33073",
	}

	if h.Helper != nil {
		if resp, err := h.Helper.Execute("network.server_status", nil, false); err == nil && resp.Ok && resp.Output != "" {
			var srvMap map[string]interface{}
			if err := json.Unmarshal([]byte(resp.Output), &srvMap); err == nil {
				for k, v := range srvMap {
					serverManaged[k] = v
				}
			}
		}
	}

	data := map[string]interface{}{
		"level":            netLevel,
		"enabled":          netLevel != "off",
		"mode":             mode,
		"client_runtime":   clientRuntime,
		"native_installed": nativeInstalled,
		"has_key":          hasKey,
		"management_url":   managementURL,
		"mesh_ip":          meshIP,
		"connected":        connected,
		"peers_count":      peersCount,
		"nodes_count":      peersCount,
		"peers":            peersList,
		"nodes":            peersList,
		"server_managed":   serverManaged,
	}

	json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
}

// executeNetBirdStatus queries status from the running NetBird container or helper daemon.
func executeNetBirdStatus(client HelperClient) (string, error) {
	if nbBin, err := exec.LookPath("netbird"); err == nil {
		cmd := exec.Command(nbBin, "status", "--json")
		if out, err := cmd.Output(); err == nil && len(out) > 0 {
			strOut := strings.TrimSpace(string(out))
			start := strings.Index(strOut, "{")
			end := strings.LastIndex(strOut, "}")
			if start >= 0 && end > start {
				return strOut[start : end+1], nil
			}
		}
	}

	if client == nil {
		client = &helper.Client{SocketPath: "/run/allod/helper.sock"}
	}
	resp, err := client.Execute("network.netbird_status", nil, false)
	if err == nil && resp.Ok && resp.Output != "" {
		strOut := strings.TrimSpace(resp.Output)
		start := strings.Index(strOut, "{")
		end := strings.LastIndex(strOut, "}")
		if start >= 0 && end > start {
			return strOut[start : end+1], nil
		}
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
		Mode           string `json:"mode"`
		SetupKey       string `json:"setup_key"`
		ManagementURL  string `json:"management_url"`
		ServerDomain   string `json:"server_domain"`
		ServerPort     int    `json:"server_port"`
		ServerDashPort int    `json:"server_dash_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "JSON non valido"})
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "selfhosted" {
		mode = "selfhosted_remote"
	}
	if mode == "" || (mode != "cloud" && mode != "selfhosted_remote" && mode != "selfhosted_managed" && mode != "off") {
		mode = "cloud"
	}

	setupKey := strings.TrimSpace(req.SetupKey)
	mgmtURL := strings.TrimSpace(req.ManagementURL)
	serverDomain := strings.TrimSpace(req.ServerDomain)
	if serverDomain == "" {
		serverDomain = "127.0.0.1"
	}
	serverPort := req.ServerPort
	if serverPort <= 0 {
		serverPort = 33073
	}
	serverDashPort := req.ServerDashPort
	if serverDashPort <= 0 {
		serverDashPort = 8088
	}

	if mode == "cloud" {
		mgmtURL = "https://api.netbird.io:443"
	} else if mode == "selfhosted_managed" {
		if strings.HasPrefix(serverDomain, "http://") || strings.HasPrefix(serverDomain, "https://") {
			mgmtURL = serverDomain
		} else if strings.Contains(serverDomain, ".") && !strings.Contains(serverDomain, "192.168.") && !strings.Contains(serverDomain, "10.") && !strings.Contains(serverDomain, "127.0.") {
			mgmtURL = fmt.Sprintf("https://%s:%d", serverDomain, serverPort)
		} else {
			mgmtURL = fmt.Sprintf("http://%s:%d", serverDomain, serverPort)
		}
	} else if mode == "selfhosted_remote" {
		if mgmtURL != "" && !strings.HasPrefix(mgmtURL, "http://") && !strings.HasPrefix(mgmtURL, "https://") {
			mgmtURL = "https://" + mgmtURL
		}
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

	// If self-hosted managed, launch or restart the managed server container
	if mode == "selfhosted_managed" && h.Helper != nil {
		_, _ = h.Helper.Execute("network.server_up", map[string]interface{}{
			"domain":    serverDomain,
			"port":      serverPort,
			"dash_port": serverDashPort,
		}, false)
	}

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
			_, _ = h.Helper.Execute("network.server_down", nil, false)
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

	meshIP := "--"
	if out, err := executeNetBirdStatus(h.Helper); err == nil && out != "" {
		var nbStatus netbirdStatusJSON
		if jsonErr := json.Unmarshal([]byte(out), &nbStatus); jsonErr == nil && nbStatus.NetbirdIP != "" {
			meshIP = strings.Split(nbStatus.NetbirdIP, "/")[0]
		}
	}
	if meshIP == "--" {
		if ip := helper.GetInterfaceIPv4("wt0"); ip != "" {
			meshIP = ip
		}
	}

	modeLabel := "NetBird Cloud (EU)"
	dashURL := ""
	if netLevel == "selfhosted_remote" || netLevel == "selfhosted" {
		modeLabel = "NetBird Self-Hosted (Remoto)"
	} else if netLevel == "selfhosted_managed" {
		modeLabel = "NetBird Server Gestito (Allod)"
		dashURL = "http://127.0.0.1:8088"
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status: "ok",
		Data: map[string]interface{}{
			"mode":           netLevel,
			"mode_label":     modeLabel,
			"management_url": managementURL,
			"dashboard_url":  dashURL,
			"mesh_ip":        meshIP,
			"has_key":        len(setupKey) > 0,
			"key":            setupKey,
			"server_url":     managementURL,
		},
	})
}

func (h *NetworkHandler) handleInstallNative(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.Helper == nil {
		h.Helper = &helper.Client{SocketPath: "/run/allod/helper.sock"}
	}

	resp, err := h.Helper.Execute("network.install_native", nil, false)
	if err != nil || !resp.Ok {
		w.WriteHeader(http.StatusInternalServerError)
		errMsg := "Errore installazione NetBird nativo"
		if resp.Error != "" {
			errMsg = resp.Error
		} else if err != nil {
			errMsg = err.Error()
		}
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
		return
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "NetBird nativo installato con successo sul server! Rete mesh attiva.",
		Data: map[string]interface{}{
			"output": resp.Output,
		},
	})
}

func (h *NetworkHandler) handleServerStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain   string `json:"domain"`
		Port     int    `json:"port"`
		DashPort int    `json:"dash_port"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if h.Helper == nil {
		h.Helper = &helper.Client{SocketPath: "/run/allod/helper.sock"}
	}

	args := map[string]interface{}{}
	if req.Domain != "" {
		args["domain"] = req.Domain
	}
	if req.Port > 0 {
		args["port"] = req.Port
	}
	if req.DashPort > 0 {
		args["dash_port"] = req.DashPort
	}

	resp, err := h.Helper.Execute("network.server_up", args, false)
	if err != nil || !resp.Ok {
		w.WriteHeader(http.StatusInternalServerError)
		errMsg := "Errore avvio server NetBird gestito"
		if resp.Error != "" {
			errMsg = resp.Error
		} else if err != nil {
			errMsg = err.Error()
		}
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
		return
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: resp.Output,
	})
}

func (h *NetworkHandler) handleServerStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if h.Helper == nil {
		h.Helper = &helper.Client{SocketPath: "/run/allod/helper.sock"}
	}

	resp, err := h.Helper.Execute("network.server_down", nil, false)
	if err != nil || !resp.Ok {
		w.WriteHeader(http.StatusInternalServerError)
		errMsg := "Errore arresto server NetBird gestito"
		if resp.Error != "" {
			errMsg = resp.Error
		} else if err != nil {
			errMsg = err.Error()
		}
		json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
		return
	}

	json.NewEncoder(w).Encode(PanelResponse{
		Status:  "ok",
		Message: "Server NetBird gestito arrestato con successo",
	})
}
