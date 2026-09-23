package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/asfaltobollente/allod/internal/config"
	"github.com/asfaltobollente/allod/internal/helper"
	"github.com/asfaltobollente/allod/internal/manifest"
	"github.com/asfaltobollente/allod/internal/panel"
	"github.com/asfaltobollente/allod/internal/preflight"
	"github.com/asfaltobollente/allod/internal/quadlet"
	"github.com/asfaltobollente/allod/internal/ring"
	"github.com/asfaltobollente/allod/internal/state"
	"github.com/asfaltobollente/allod/internal/updater"
	"github.com/asfaltobollente/allod/internal/version"
)

//go:embed web/*
var webFS embed.FS

var validTriadUserRegex = regexp.MustCompile(`^[a-z0-9_.-]+$`)

func getGitVersionBuildArgs() []string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	out, err := cmd.Output()
	if err == nil {
		v := strings.TrimSpace(string(out))
		if len(v) > 0 {
			return []string{"-ldflags", fmt.Sprintf("-X github.com/asfaltobollente/allod/internal/version.Version=%s", v)}
		}
	}
	return nil
}

func buildGoBinary(outputName, pkgPath string) ([]byte, error) {
	args := []string{"build"}
	args = append(args, getGitVersionBuildArgs()...)
	args = append(args, "-o", outputName, pkgPath)
	return exec.Command("go", args...).CombinedOutput()
}

type PanelResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type VolumeMountInfo struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	Exists        bool   `json:"exists"`
	SizeBytes     int64  `json:"size_bytes"`
	SizeHuman     string `json:"size_human"`
}

type ModuleInfo struct {
	ID            string             `json:"id"`
	Tier          string             `json:"tier"`
	CurrentLevel  string             `json:"current_level"`
	RuntimeStatus string             `json:"runtime_status"` // "running", "stopped", "failed", "off"
	Manifest      *manifest.Manifest `json:"manifest"`
	StoragePath   string             `json:"storage_path"`
	StorageSize   string             `json:"storage_size"`
	StorageBytes  int64              `json:"storage_bytes"`
	IsOnNASPool   bool               `json:"is_on_nas_pool"`
	Mounts        []VolumeMountInfo  `json:"mounts"`
}

func getConfigPath() string {
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return "configs/config.example.yaml"
}

func getModulesDir() string {
	candidates := []string{
		"modules",
		filepath.Join(".", "modules"),
		filepath.Join("..", "modules"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "allod", "modules"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "modules"))
		candidates = append(candidates, filepath.Join(exeDir, "..", "modules"))
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return "modules"
}

func getRingTopology(cfg *config.Config) (*ring.RingTopology, bool) {
	if _, err := os.Stat("ring.yaml"); err == nil {
		topo, err := ring.LoadTopology("ring.yaml")
		if err == nil {
			return topo, false
		}
	}
	// Standalone single-node topology if no ring.yaml has been configured
	nodeName := "allod-node"
	if cfg != nil && cfg.Node.Name != "" {
		nodeName = cfg.Node.Name
	}
	topo := ring.NewRingTopology("allod-standalone", 2)
	topo.AddMember(&ring.Member{
		ID:      nodeName,
		Address: "127.0.0.1 (Locale)",
		QuotaGB: 500,
		Datasets: []ring.Dataset{
			{ID: "photos", SizeGB: 40, Critical: true},
			{ID: "documents", SizeGB: 10, Critical: true},
		},
	})
	return topo, true
}

const dbPath = "state.db"

func getStorageInfo(modID string) (string, string, int64, bool, []VolumeMountInfo) {
	baseDir := "/mnt/allod-storage"
	isOnNAS := true
	if _, err := os.Stat(baseDir); err != nil {
		isOnNAS = false
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".local", "share", "allod", "storage")
	}

	modPath := filepath.Join(baseDir, modID)
	var mounts []VolumeMountInfo

	switch modID {
	case "cloud":
		mounts = []VolumeMountInfo{
			{HostPath: filepath.Join(modPath, "html"), ContainerPath: "/var/www/html"},
			{HostPath: filepath.Join(modPath, "data"), ContainerPath: "/var/www/html/data"},
			{HostPath: filepath.Join(modPath, "postgres"), ContainerPath: "/var/lib/postgresql/data"},
		}
	case "photos":
		mounts = []VolumeMountInfo{
			{HostPath: filepath.Join(modPath, "upload"), ContainerPath: "/usr/src/app/upload"},
			{HostPath: filepath.Join(modPath, "postgres"), ContainerPath: "/var/lib/postgresql/data"},
			{HostPath: filepath.Join(modPath, "valkey"), ContainerPath: "/data"},
		}
	case "backup":
		mounts = []VolumeMountInfo{
			{HostPath: filepath.Join(modPath, "vault"), ContainerPath: "/data"},
		}
	case "shares":
		mounts = []VolumeMountInfo{
			{HostPath: filepath.Join(modPath, "public"), ContainerPath: "/shares/public"},
		}
	case "media":
		mounts = []VolumeMountInfo{
			{HostPath: filepath.Join(modPath, "data"), ContainerPath: "/media"},
			{HostPath: filepath.Join(modPath, "config"), ContainerPath: "/config"},
		}
	default:
		mounts = []VolumeMountInfo{
			{HostPath: modPath, ContainerPath: "/data"},
		}
	}

	var totalBytes int64 = 0
	for i := range mounts {
		if fi, err := os.Stat(mounts[i].HostPath); err == nil {
			mounts[i].Exists = true
			if fi.IsDir() {
				s := dirSize(mounts[i].HostPath)
				mounts[i].SizeBytes = s
				mounts[i].SizeHuman = formatBytes(s)
				totalBytes += s
			}
		} else {
			mounts[i].Exists = false
			mounts[i].SizeHuman = "0 B"
		}
	}

	return modPath, formatBytes(totalBytes), totalBytes, isOnNAS, mounts
}

func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(b)/(1024*1024*1024))
}

func getModuleRuntimeStatus(modName string, level string, runningContainers map[string]bool) string {
	if modName == "storage" {
		// Storage is active if /mnt/allod-storage is mounted or single/raid1 level is configured
		if _, err := os.Stat("/mnt/allod-storage"); err == nil {
			return "running"
		}
		if level == "off" || level == "" {
			return "off"
		}
		return "stopped"
	}
	if modName == "shares" {
		if out, err := exec.Command("systemctl", "is-active", "smbd").Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
			return "running"
		}
		if level == "off" || level == "" {
			return "off"
		}
		return "stopped"
	}
	if modName == "network" {
		// 1. Check native systemd service first
		if out, err := exec.Command("systemctl", "is-active", "netbird").Output(); err == nil && strings.TrimSpace(string(out)) == "active" {
			client := helper.Client{SocketPath: "/run/allod/helper.sock"}
			if res, err := client.Execute("network.netbird_status", nil, false); err == nil && res.Ok {
				var stMap map[string]interface{}
				if err := json.Unmarshal([]byte(res.Output), &stMap); err == nil {
					if mgmt, ok := stMap["management"].(map[string]interface{}); ok {
						if conn, ok := mgmt["connected"].(bool); ok && !conn {
							return "stopped"
						}
					}
				}
			}
			return "running"
		}

		// 2. Check helper status (covers container or native when service active check differs)
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if res, err := client.Execute("network.netbird_status", nil, false); err == nil && res.Ok {
			var stMap map[string]interface{}
			if err := json.Unmarshal([]byte(res.Output), &stMap); err == nil {
				if mgmt, ok := stMap["management"].(map[string]interface{}); ok {
					if conn, ok := mgmt["connected"].(bool); ok && conn {
						return "running"
					}
				}
				if ip, ok := stMap["netbirdIp"].(string); ok && ip != "" {
					return "running"
				}
				if level == "off" || level == "" {
					return "off"
				}
				return "stopped"
			}
		}

		// 3. Check if running in a container
		if quadlet.IsModuleRunning("network", runningContainers) {
			return "running"
		}

		if level == "off" || level == "" {
			return "off"
		}
		return "stopped"
	}

	// Active rootless Podman containers check
	if quadlet.IsModuleRunning(modName, runningContainers) {
		return "running"
	}

	out, err := exec.Command("systemctl", "--user", "is-active", modName).Output()
	st := strings.TrimSpace(string(out))
	if err == nil && st == "active" {
		return "running"
	}
	if st == "failed" {
		return "failed"
	}
	if st == "activating" {
		return "starting"
	}
	if level == "off" || level == "" {
		return "off"
	}
	return "stopped"
}

func getTriadStatus() (allActive bool, sharesOk bool, photosOk bool, mediaOk bool, missing []string, reason string) {
	runningContainers := quadlet.GetRunningContainers()
	cfg, _ := config.LoadConfig(getConfigPath())

	lvlShares := "standard"
	lvlPhotos := "standard"
	lvlMedia := "basic"
	if cfg != nil && cfg.Modules != nil {
		if m, ok := cfg.Modules["shares"]; ok && m.Level != "" {
			lvlShares = m.Level
		}
		if m, ok := cfg.Modules["photos"]; ok && m.Level != "" {
			lvlPhotos = m.Level
		}
		if m, ok := cfg.Modules["media"]; ok && m.Level != "" {
			lvlMedia = m.Level
		}
	}

	stShares := getModuleRuntimeStatus("shares", lvlShares, runningContainers)
	stPhotos := getModuleRuntimeStatus("photos", lvlPhotos, runningContainers)
	stMedia := getModuleRuntimeStatus("media", lvlMedia, runningContainers)

	sharesOk = (stShares == "running")
	photosOk = (stPhotos == "running")
	mediaOk = (stMedia == "running")

	allActive = sharesOk && photosOk && mediaOk

	if !sharesOk {
		missing = append(missing, "Condivisione LAN (Samba)")
	}
	if !photosOk {
		missing = append(missing, "Foto & Backup (Immich)")
	}
	if !mediaOk {
		missing = append(missing, "Streaming Media (Jellyfin)")
	}

	if !allActive {
		reason = fmt.Sprintf("I seguenti servizi della Triade non sono attualmente attivi: %s. La Triade richiede che tutti e tre i servizi siano avviati per poter orchestrare automaticamente le cartelle personali, i permessi isolati e il bind-mount delle foto.", strings.Join(missing, ", "))
	}
	return
}

func main() {
	port := 8080
	cfgPath := getConfigPath()
	fmt.Printf("Avvio allod-panel (Web Dashboard Integrata) su porta %d (config: %s) ...\n", port, cfgPath)

	mux := http.NewServeMux()

	// 1. Static Web Assets from embed.FS
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Errore caricamento asset embeddati: %v\n", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	// Portal and Welcome Routes for Family Members
	mux.HandleFunc("/portal", func(w http.ResponseWriter, r *http.Request) {
		f, err := subFS.Open("portal.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.Copy(w, f)
	})
	mux.HandleFunc("/welcome", func(w http.ResponseWriter, r *http.Request) {
		target := "/portal"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})

	// 2. API Status
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg, err := config.LoadConfig(getConfigPath())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		realRAM := preflight.GetRealRAMStats()
		committed, _ := preflight.CommittedRAMInfo(cfg, "")

		helperConnected, helperPermDenied := checkHelperConnectivity()
		topo, isStandalone := getRingTopology(cfg)
		storageTopo := preflight.DetectStorageTopology()

		uid := os.Getuid()
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = fmt.Sprintf("uid-%d", uid)
		}
		isRootless := uid != 0

		data := map[string]interface{}{
			"version":                  version.Get(),
			"node_name":                cfg.Node.Name,
			"channel":                  cfg.Node.Channel,
			"ram_total_mb":             realRAM.TotalMB,
			"ram_used_mb":              realRAM.UsedMB,
			"ram_available_mb":         realRAM.AvailableMB,
			"ram_free_mb":              realRAM.FreeMB,
			"core_reserved_mb":         preflight.CoreReservedMB,
			"ram_committed_mb":         committed,
			"helper_connected":         helperConnected,
			"helper_permission_denied": helperPermDenied,
			"group_repo":               cfg.Node.Group,
			"is_standalone":            isStandalone,
			"ring_members":             len(topo.Members),
			"is_rootless":              isRootless,
			"uid":                      uid,
			"current_user":             currentUser,
			"storage":                  storageTopo,
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
	})

	// 3. API Modules list
	mux.HandleFunc("/api/modules", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg, err := config.LoadConfig(getConfigPath())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		modDir := getModulesDir()
		entries, err := os.ReadDir(modDir)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		runningContainers := quadlet.GetRunningContainers()
		var modules []ModuleInfo
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			modName := e.Name()
			mPath := filepath.Join(modDir, modName, "module.yaml")
			m, err := manifest.LoadManifest(mPath)
			if err != nil {
				continue
			}

			curLevel := "off"
			if modCfg, exists := cfg.Modules[modName]; exists && modCfg.Level != "" {
				curLevel = modCfg.Level
			}

			runtimeStatus := getModuleRuntimeStatus(modName, curLevel, runningContainers)
			sPath, sSize, sBytes, isNAS, mounts := getStorageInfo(modName)

			modules = append(modules, ModuleInfo{
				ID:            modName,
				Tier:          m.Tier,
				CurrentLevel:  curLevel,
				RuntimeStatus: runtimeStatus,
				Manifest:      m,
				StoragePath:   sPath,
				StorageSize:   sSize,
				StorageBytes:  sBytes,
				IsOnNASPool:   isNAS,
				Mounts:        mounts,
			})
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: modules})
	})

	// 4. API Modules Start
	mux.HandleFunc("/api/modules/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Module string `json:"module"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Module == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Invalid JSON"})
			return
		}

		// Auto-generate Quadlets if missing or needed
		cfgPath := getConfigPath()
		cfg, _ := config.LoadConfig(cfgPath)
		level := "standard"
		if cfg != nil && cfg.Modules != nil {
			if mcfg, ok := cfg.Modules[req.Module]; ok && mcfg.Level != "" && mcfg.Level != "off" {
				level = mcfg.Level
			}
		}

		modDir := getModulesDir()
		mPath := filepath.Join(modDir, req.Module, "module.yaml")
		m, err := manifest.LoadManifest(mPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Manifest non trovato per '%s' in %s: %v", req.Module, mPath, err)})
			return
		}

		if _, ok := m.Levels[level]; !ok {
			if _, ok := m.Levels["hybrid"]; ok {
				level = "hybrid"
			} else if _, ok := m.Levels["basic"]; ok {
				level = "basic"
			} else if _, ok := m.Levels["standard"]; ok {
				level = "standard"
			} else {
				for k := range m.Levels {
					if k != "off" {
						level = k
						break
					}
				}
			}
		}

		genRes, err := quadlet.Generate(req.Module, m, level)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore generazione Quadlet per %s: %v", req.Module, err)})
			return
		}

		home, _ := os.UserHomeDir()
		if home != "" {
			quadDir := filepath.Join(home, ".config", "containers", "systemd")
			systemdUserDir := filepath.Join(home, ".config", "systemd", "user")
			_ = os.MkdirAll(quadDir, 0755)
			_ = os.MkdirAll(systemdUserDir, 0755)
			quadlet.EnsureAllodNetwork(quadDir)
			for fname, content := range genRes.Files {
				if strings.HasSuffix(fname, ".service") {
					_ = os.WriteFile(filepath.Join(systemdUserDir, fname), []byte(content), 0644)
				}
				if err := os.WriteFile(filepath.Join(quadDir, fname), []byte(content), 0644); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore scrittura file %s: %v", fname, err)})
					return
				}
			}
		}

		if req.Module == "shares" {
			client := helper.Client{SocketPath: "/run/allod/helper.sock"}
			res, err := client.Execute("shares.apply", map[string]interface{}{
				"name": "shares",
				"path": "/mnt/allod-storage/shares",
			}, false)
			if err != nil {
				client.SocketPath = "allod-helper.sock"
				res, err = client.Execute("shares.apply", map[string]interface{}{
					"name": "shares",
					"path": "/mnt/allod-storage/shares",
				}, false)
			}
			if err != nil || !res.Ok {
				w.WriteHeader(http.StatusInternalServerError)
				errMsg := "Errore comunicazione helper Samba (allod-helperd è avviato con sudo?)"
				if err != nil {
					errMsg = err.Error()
				} else if res.Error != "" {
					errMsg = res.Error
				}
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
				return
			}
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Condivisione Samba avviata con successo su /mnt/allod-storage/shares"})
			return
		}

		if req.Module == "network" {
			client := helper.Client{SocketPath: "/run/allod/helper.sock"}
			res, err := client.Execute("network.netbird_up", nil, false)
			if err != nil {
				client.SocketPath = "allod-helper.sock"
				res, err = client.Execute("network.netbird_up", nil, false)
			}
			if err != nil || !res.Ok {
				w.WriteHeader(http.StatusInternalServerError)
				errMsg := "Errore comunicazione helper NetBird (verifica che allod-helperd sia in esecuzione come root)"
				if err != nil {
					errMsg = err.Error()
				} else if res.Error != "" {
					errMsg = res.Error
				}
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
				return
			}
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Modulo network (NetBird) avviato con successo tramite allod-helperd"})
			return
		}

		baseDir := quadlet.ResolvedStorageBaseDir()
		if req.Module == "photos" {
			_ = os.MkdirAll(filepath.Join(baseDir, "photos", "upload"), 0777)
			_ = os.MkdirAll(filepath.Join(baseDir, "photos", "postgres"), 0777)
			_ = os.MkdirAll(filepath.Join(baseDir, "photos", "valkey"), 0777)
		} else if req.Module == "cloud" {
			_ = os.MkdirAll(filepath.Join(baseDir, "cloud", "html"), 0777)
			_ = os.MkdirAll(filepath.Join(baseDir, "cloud", "data"), 0777)
			_ = os.MkdirAll(filepath.Join(baseDir, "cloud", "postgres"), 0777)
		} else if req.Module == "media" {
			mediaDirs := []string{
				filepath.Join(baseDir, "media", "config"),
				filepath.Join(baseDir, "shares", "media", "movies"),
				filepath.Join(baseDir, "shares", "media", "tv"),
				filepath.Join(baseDir, "shares", "media", "music"),
				filepath.Join(baseDir, "shares", "movies"),
				filepath.Join(baseDir, "shares", "tv"),
				filepath.Join(baseDir, "shares", "music"),
				filepath.Join(baseDir, "shares", "film"),
				filepath.Join(baseDir, "shares", "musica"),
			}
			for _, md := range mediaDirs {
				_ = os.MkdirAll(md, 0777)
				_ = os.Chmod(md, 0777)
			}
			_ = exec.Command("chmod", "-R", "0777", filepath.Join(baseDir, "shares")).Run()
		} else {
			_ = os.MkdirAll(filepath.Join(baseDir, req.Module), 0777)
		}

		reloadOut, _ := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput()
		_ = exec.Command("systemctl", "--user", "reset-failed").Run()

		// Start secondary containers first if any
		_ = exec.Command("systemctl", "--user", "start", req.Module+"-postgres").Run()
		_ = exec.Command("systemctl", "--user", "start", req.Module+"-valkey").Run()

		startOut, err := exec.Command("systemctl", "--user", "start", req.Module).CombinedOutput()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := strings.TrimSpace(string(startOut))
			if errMsg == "" {
				errMsg = strings.TrimSpace(string(reloadOut))
			}
			if errMsg == "" {
				errMsg = err.Error()
			}
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore avvio servizio %s: %s", req.Module, errMsg)})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: fmt.Sprintf("Avvio del modulo %s avviato con successo", req.Module)})
	})

	// 5. API Modules Stop
	mux.HandleFunc("/api/modules/stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Module string `json:"module"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Module == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Invalid JSON"})
			return
		}

		if req.Module == "shares" {
			client := helper.Client{SocketPath: "/run/allod/helper.sock"}
			res, err := client.Execute("shares.apply", map[string]interface{}{
				"name":    "shares",
				"path":    "/mnt/allod-storage/shares",
				"enabled": false,
			}, false)
			if err != nil {
				client.SocketPath = "allod-helper.sock"
				res, err = client.Execute("shares.apply", map[string]interface{}{
					"name":    "shares",
					"path":    "/mnt/allod-storage/shares",
					"enabled": false,
				}, false)
			}
			if err != nil || !res.Ok {
				_ = exec.Command("systemctl", "stop", "smbd").Run()
			}
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Condivisione Samba fermata con successo"})
			return
		}

		if req.Module == "network" {
			go func() {
				time.Sleep(150 * time.Millisecond)
				client := helper.Client{SocketPath: "/run/allod/helper.sock"}
				_, err := client.Execute("network.netbird_down", nil, false)
				if err != nil {
					client.SocketPath = "allod-helper.sock"
					_, _ = client.Execute("network.netbird_down", nil, false)
				}
				_ = exec.Command("systemctl", "--user", "stop", "network").Run()
				_ = exec.Command("podman", "rm", "-f", "network").Run()
				_ = exec.Command("podman", "rm", "-f", "allod-netbird").Run()
			}()
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Modulo network (NetBird) fermato con successo"})
			return
		}

		_ = quadlet.StopAndRemoveContainers(req.Module, false)
		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: fmt.Sprintf("Modulo %s fermato e container rimossi con successo", req.Module)})
	})

	// 5a. API System Sweep (Podman ghost containers & dangling images sweeper)
	mux.HandleFunc("/api/system/sweep", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// 1. Prune dead / exited user containers
		cntPruneOut, _ := exec.Command("podman", "container", "prune", "-f").CombinedOutput()

		// 2. Prune dangling unused user images
		imgPruneOut, _ := exec.Command("podman", "image", "prune", "-f").CombinedOutput()

		// 3. Prune root containers and dangling images via privileged helper
		rootCntPruned := ""
		rootImgPruned := ""
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if resp, err := client.Execute("containers.prune", nil, false); err == nil && resp.Ok && resp.Output != "" {
			var rootRes map[string]string
			if err := json.Unmarshal([]byte(resp.Output), &rootRes); err == nil {
				rootCntPruned = rootRes["containers_pruned"]
				rootImgPruned = rootRes["images_pruned"]
			}
		}

		// 4. Remove stale .cid files
		uid := os.Getuid()
		cidDir := fmt.Sprintf("/run/user/%d", uid)
		var cleanedCids []string
		if entries, err := os.ReadDir(cidDir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".cid") {
					cidPath := filepath.Join(cidDir, entry.Name())
					_ = os.Remove(cidPath)
					cleanedCids = append(cleanedCids, entry.Name())
				}
			}
		}

		// 5. Terminate any orphan containers for modules configured as 'off'
		var orphansCleaned []string
		cfg, _ := config.LoadConfig(getConfigPath())
		if cfg != nil {
			for modID, modCfg := range cfg.Modules {
				if modCfg.Level == "off" || modCfg.Level == "" {
					if quadlet.IsModuleRunning(modID, nil) {
						_ = quadlet.StopAndRemoveContainers(modID, true)
						orphansCleaned = append(orphansCleaned, modID)
					}
				}
			}
		}

		// 6. Reset failed systemd units
		_ = exec.Command("systemctl", "--user", "reset-failed").Run()

		data := map[string]interface{}{
			"containers_pruned":      strings.TrimSpace(string(cntPruneOut)),
			"images_pruned":          strings.TrimSpace(string(imgPruneOut)),
			"root_containers_pruned": rootCntPruned,
			"root_images_pruned":     rootImgPruned,
			"cleaned_cids":           cleanedCids,
			"orphans_cleaned":        orphansCleaned,
			"timestamp":              time.Now().Format("2006-01-02 15:04:05"),
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Sweeper completato con successo: container morti e immagini orfane rimossi!",
			Data:    data,
		})
	})

	// 5a-2. API System Reload (Regenerate Quadlets + systemctl --user daemon-reload)
	mux.HandleFunc("/api/system/reload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		home, _ := os.UserHomeDir()
		var writtenFiles []string
		cfg, _ := config.LoadConfig(getConfigPath())
		if home != "" {
			quadDir := filepath.Join(home, ".config", "containers", "systemd")
			systemdUserDir := filepath.Join(home, ".config", "systemd", "user")
			_ = os.MkdirAll(quadDir, 0755)
			_ = os.MkdirAll(systemdUserDir, 0755)
			quadlet.EnsureAllodNetwork(quadDir)

			modDir := getModulesDir()
			entries, _ := os.ReadDir(modDir)
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				modID := entry.Name()
				mPath := filepath.Join(modDir, modID, "module.yaml")
				m, err := manifest.LoadManifest(mPath)
				if err != nil {
					continue
				}

				curLevel := "off"
				if cfg != nil && cfg.Modules != nil {
					if modCfg, exists := cfg.Modules[modID]; exists && modCfg.Level != "" {
						curLevel = modCfg.Level
					}
				}

				if curLevel == "off" || curLevel == "" {
					// Clean up any stale unit files or orphan containers for disabled modules
					_ = quadlet.StopAndRemoveContainers(modID, true)
					continue
				}

				if genRes, err := quadlet.Generate(modID, m, curLevel); err == nil {
					for fname, content := range genRes.Files {
						if strings.HasSuffix(fname, ".service") {
							_ = os.WriteFile(filepath.Join(systemdUserDir, fname), []byte(content), 0644)
						}
						if err := os.WriteFile(filepath.Join(quadDir, fname), []byte(content), 0644); err == nil {
							writtenFiles = append(writtenFiles, fname)
						}
					}
				}
			}
		}

		reloadOut, _ := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput()
		_ = exec.Command("systemctl", "--user", "reset-failed").Run()

		data := map[string]interface{}{
			"written_files": writtenFiles,
			"reload_output": strings.TrimSpace(string(reloadOut)),
			"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Quadlet rigenerati (%d unità) e daemon-reload eseguito con successo!", len(writtenFiles)),
			Data:    data,
		})
	})

	// 5a-3. API System Git Pull
	mux.HandleFunc("/api/system/git-pull", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		out, err := exec.Command("git", "pull").CombinedOutput()
		outputStr := strings.TrimSpace(string(out))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Git pull fallito: %v\n%s", err, outputStr),
				Data:    map[string]interface{}{"output": outputStr},
			})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Git pull completato con successo",
			Data:    map[string]interface{}{"output": outputStr},
		})
	})

	// 5a-4. API System Go Build
	mux.HandleFunc("/api/system/go-build", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Build both allod CLI and allod-panel
		var report strings.Builder
		out1, err1 := buildGoBinary("allod", "./cmd/allod")
		report.WriteString("[go build -o allod ./cmd/allod]\n")
		report.WriteString(string(out1))

		out2, err2 := buildGoBinary("allod-panel", "./cmd/allod-panel")
		report.WriteString("\n[go build -o allod-panel ./cmd/allod-panel]\n")
		report.WriteString(string(out2))

		out3, err3 := buildGoBinary("allod-helperd", "./cmd/allod-helperd")
		report.WriteString("\n[go build -o allod-helperd ./cmd/allod-helperd]\n")
		report.WriteString(string(out3))

		if err1 != nil || err2 != nil || err3 != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: "Errore durante la compilazione Go",
				Data:    map[string]interface{}{"output": report.String()},
			})
			return
		}

		report.WriteString("\n✓ Compilazione completata con successo per 'allod', 'allod-panel' e 'allod-helperd'.")
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Compilazione Go completata con successo",
			Data:    map[string]interface{}{"output": report.String()},
		})
	})

	linuxUserExists := func(username string) bool {
		if username == "" {
			return false
		}
		data, err := os.ReadFile("/etc/passwd")
		if err != nil {
			idBin := "id"
			if p, errP := exec.LookPath("id"); errP == nil {
				idBin = p
			}
			return exec.Command(idBin, "-u", username).Run() == nil
		}
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Split(line, ":")
			if len(parts) > 0 && parts[0] == username {
				return true
			}
		}
		return false
	}

	ensureSystemUser := func(client *helper.Client, username string) error {
		if username == "" || !validTriadUserRegex.MatchString(username) {
			return fmt.Errorf("invalid username '%s'", username)
		}

		// 1. Quick check: does user already exist in Linux (/etc/passwd)?
		if linuxUserExists(username) {
			return nil
		}

		// 2. Authoritative creation via privileged root helper
		if client == nil {
			return fmt.Errorf("helper daemon is not connected")
		}

		if err := client.CreateUser(username); err != nil {
			return fmt.Errorf("failed to create Linux user '%s': %w", username, err)
		}

		if linuxUserExists(username) {
			return nil
		}
		return fmt.Errorf("user '%s' was not found in /etc/passwd after creation", username)
	}

	upgradeAndRestartHelper := func(client *helper.Client, logReport *strings.Builder) error {
		cwd, _ := os.Getwd()

		// 1. Where could the new helper binary be?
		tmpHelper := filepath.Join(os.TempDir(), "allod-helperd-update")
		candidates := []string{
			tmpHelper,
			filepath.Join(cwd, "allod-helperd"),
			"allod-helperd",
			filepath.Join(cwd, "allod-helperd.new"),
		}
		var helperBytes []byte
		var foundPath string
		for _, p := range candidates {
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				helperBytes = data
				foundPath = p
				if logReport != nil {
					logReport.WriteString(fmt.Sprintf("✓ Trovato binario helper: %s (%d byte)\n", p, len(data)))
				}
				break
			}
		}

		if len(helperBytes) == 0 {
			// Compile now if not found
			cmdBuild := exec.Command("go", "build", "-o", tmpHelper, "./cmd/allod-helperd")
			if out, errB := cmdBuild.CombinedOutput(); errB == nil {
				if data, errR := os.ReadFile(tmpHelper); errR == nil && len(data) > 0 {
					helperBytes = data
					foundPath = tmpHelper
					if logReport != nil {
						logReport.WriteString(fmt.Sprintf("✓ Compilato helper in %s (%d byte)\n", tmpHelper, len(data)))
					}
				}
			} else if logReport != nil {
				logReport.WriteString(fmt.Sprintf("⚠️ Errore compilazione helper: %v (%s)\n", errB, strings.TrimSpace(string(out))))
			}
		}

		if len(helperBytes) > 0 {
			// Step 1: Ensure /usr/local/bin is writable via helper shares.apply
			// Pass BOTH 'name' and 'path' to work with older and newer helper daemons!
			if client != nil {
				_, _ = client.Execute("shares.apply", map[string]interface{}{
					"name": "shares",
					"path": "/usr/local/bin",
				}, false)
			}

			// Step 2: Write to /usr/local/bin/allod-helperd.tmp and atomic rename to bypass ETXTBSY
			tmpDst := "/usr/local/bin/allod-helperd.tmp"
			_ = os.Remove(tmpDst)
			copied := false
			if errW := os.WriteFile(tmpDst, helperBytes, 0755); errW == nil {
				_ = os.Rename(tmpDst, "/usr/local/bin/allod-helperd")
				copied = true
			} else {
				// Direct unlink and copy
				_ = os.Remove("/usr/local/bin/allod-helperd")
				_ = exec.Command("sudo", "-n", "rm", "-f", "/usr/local/bin/allod-helperd").Run()
				if errDirect := os.WriteFile("/usr/local/bin/allod-helperd", helperBytes, 0755); errDirect == nil {
					copied = true
				} else if errCp := exec.Command("cp", "-f", foundPath, "/usr/local/bin/allod-helperd").Run(); errCp == nil {
					copied = true
				} else if errSudo := exec.Command("sudo", "-n", "cp", "-f", foundPath, "/usr/local/bin/allod-helperd").Run(); errSudo == nil {
					copied = true
				}
			}

			// Ensure permissions
			_ = os.Chmod("/usr/local/bin/allod-helperd", 0755)
			_ = exec.Command("chmod", "0755", "/usr/local/bin/allod-helperd").Run()
			_ = exec.Command("sudo", "-n", "chmod", "0755", "/usr/local/bin/allod-helperd").Run()

			// Also copy allod cli if available
			_ = os.Remove("/usr/local/bin/allod")
			_ = exec.Command("cp", "-f", "allod", "/usr/local/bin/allod").Run()
			_ = exec.Command("sudo", "-n", "cp", "-f", "allod", "/usr/local/bin/allod").Run()

			// Restore /usr/local/bin permissions
			_ = exec.Command("chmod", "0755", "/usr/local/bin").Run()
			_ = exec.Command("sudo", "-n", "chmod", "0755", "/usr/local/bin").Run()

			if copied {
				if logReport != nil {
					logReport.WriteString("✓ allod-helperd aggiornato con successo in /usr/local/bin/allod-helperd\n")
				}
			} else if logReport != nil {
				logReport.WriteString("ℹ️ Impossibile scrivere in /usr/local/bin/allod-helperd\n")
			}
		}

		// Step 3: Restart allod-helperd
		if client != nil {
			res, err := client.Execute("service.restart", map[string]interface{}{"unit": "allod-helperd"}, false)
			if err != nil || !res.Ok {
				_ = exec.Command("sudo", "-n", "systemctl", "restart", "allod-helperd").Run()
				_ = exec.Command("sudo", "-n", "chmod", "0666", "/run/allod/helper.sock").Run()
			}
		}
		return nil
	}

	// 5a-5. API System Self-Update & Restart
	mux.HandleFunc("/api/system/self-update", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var logReport strings.Builder
		// 1. git pull
		gitOut, err := exec.Command("git", "pull").CombinedOutput()
		logReport.WriteString("[git pull]\n" + string(gitOut) + "\n")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore git pull: %v", err),
				Data:    map[string]interface{}{"output": logReport.String()},
			})
			return
		}

		// 2. go build allod-panel to temp first, then rename (atomic replace prevents ETXTBSY)
		_ = os.Remove("allod-panel.new")
		buildOut, err := buildGoBinary("allod-panel.new", "./cmd/allod-panel")
		logReport.WriteString("[go build -o allod-panel.new ./cmd/allod-panel]\n" + string(buildOut) + "\n")
		if err != nil {
			buildOut2, err2 := buildGoBinary("allod-panel", "./cmd/allod-panel")
			logReport.WriteString("[go build -o allod-panel fallback]\n" + string(buildOut2) + "\n")
			if err2 != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(PanelResponse{
					Status:  "error",
					Message: fmt.Sprintf("Errore compilazione allod-panel: %v", err2),
					Data:    map[string]interface{}{"output": logReport.String()},
				})
				return
			}
		} else {
			_ = os.Remove("allod-panel")
			_ = os.Rename("allod-panel.new", "allod-panel")
			_ = os.Chmod("allod-panel", 0755)
		}

		// Build allod cli too
		_ = os.Remove("allod.new")
		buildCliOut, _ := buildGoBinary("allod.new", "./cmd/allod")
		logReport.WriteString("[go build -o allod ./cmd/allod]\n" + string(buildCliOut) + "\n")
		_ = os.Remove("allod")
		_ = os.Rename("allod.new", "allod")
		_ = os.Chmod("allod", 0755)

		// Build allod-helperd to /tmp/allod-helperd-update (guaranteed write access)
		tmpHelper := filepath.Join(os.TempDir(), "allod-helperd-update")
		_ = os.Remove(tmpHelper)
		buildHelperOut, errH := buildGoBinary(tmpHelper, "./cmd/allod-helperd")
		logReport.WriteString(fmt.Sprintf("[go build -o %s ./cmd/allod-helperd]\n%s\n", tmpHelper, string(buildHelperOut)))
		if errH == nil {
			_ = os.Remove("allod-helperd")
			_ = exec.Command("cp", "-f", tmpHelper, "allod-helperd").Run()
		}

		// Upgrade and restart allod-helperd
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if err := upgradeAndRestartHelper(&client, &logReport); err == nil {
			logReport.WriteString("✓ Demone Root Helper (allod-helperd) aggiornato e riavviato!\n")
		} else {
			logReport.WriteString(fmt.Sprintf("ℹ️ Riavvio allod-helperd: %v\n", err))
		}

		// Sync existing family members into Linux OS so accounts like 'mario' exist in /etc/passwd
		if st, err := state.Open(dbPath); err == nil {
			if members, err := st.ListFamilyMembers(); err == nil {
				for _, m := range members {
					if errU := ensureSystemUser(&client, m.Username); errU == nil {
						logReport.WriteString(fmt.Sprintf("✓ Utente di sistema '%s' verificato nel SO.\n", m.Username))
					} else {
						logReport.WriteString(fmt.Sprintf("⚠️ Utente '%s': %v\n", m.Username, errU))
					}
				}
			}
			st.Close()
		}

		logReport.WriteString("✓ Binari aggiornati. Riavvio pannello web in background...\n")

		// Send success response immediately before terminating
		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Aggiornamento completato! Riavvio del pannello web e di allod-helperd in corso...",
			Data:    map[string]interface{}{"output": logReport.String()},
		})

		// Asynchronous restart after 800ms
		go func() {
			time.Sleep(800 * time.Millisecond)
			if err := exec.Command("systemctl", "--user", "restart", "allod-panel").Run(); err != nil {
				cmd := exec.Command("sh", "-c", "nohup ./allod-panel > panel.log 2>&1 &")
				_ = cmd.Start()
			}
			os.Exit(0)
		}()
	})

	// 5a-5b. API System Helper Restart
	mux.HandleFunc("/api/system/helper-restart", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var logReport strings.Builder
		// Compile allod-helperd to temp path first
		tmpHelper := filepath.Join(os.TempDir(), "allod-helperd-update")
		_ = os.Remove(tmpHelper)
		buildHelperOut, errH := exec.Command("go", "build", "-o", tmpHelper, "./cmd/allod-helperd").CombinedOutput()
		logReport.WriteString(fmt.Sprintf("[go build -o %s ./cmd/allod-helperd]\n%s\n", tmpHelper, string(buildHelperOut)))
		if errH == nil {
			_ = os.Remove("allod-helperd")
			_ = exec.Command("cp", "-f", tmpHelper, "allod-helperd").Run()
		}

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if err := upgradeAndRestartHelper(&client, &logReport); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: "Errore riavvio helper: " + err.Error(),
				Data:    map[string]interface{}{"log": logReport.String(), "output": logReport.String()},
			})
			return
		}

		// Sync all family members
		if st, err := state.Open(dbPath); err == nil {
			if members, err := st.ListFamilyMembers(); err == nil {
				for _, m := range members {
					if errU := ensureSystemUser(&client, m.Username); errU == nil {
						logReport.WriteString(fmt.Sprintf("✓ Utente di sistema '%s' sincronizzato nel SO.\n", m.Username))
					} else {
						logReport.WriteString(fmt.Sprintf("⚠️ Sincronizzazione '%s': %v\n", m.Username, errU))
					}
				}
			}
			st.Close()
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "✓ Demone Root Helper (allod-helperd) aggiornato, riavviato e utenti sincronizzati!",
			Data:    map[string]interface{}{"log": logReport.String(), "output": logReport.String()},
		})
	})

	// 5a-6. API System Reset Failed
	mux.HandleFunc("/api/system/reset-failed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		out, err := exec.Command("systemctl", "--user", "reset-failed").CombinedOutput()
		outputStr := strings.TrimSpace(string(out))
		if outputStr == "" {
			outputStr = "✓ systemctl --user reset-failed eseguito. Tutti i contatori di errore azzerati."
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore reset-failed: %v", err),
				Data:    map[string]interface{}{"output": outputStr},
			})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Reset-failed completato con successo",
			Data:    map[string]interface{}{"output": outputStr},
		})
	})

	// 5a-7. API System Modules Control (Start only configured != off, or stop all)
	mux.HandleFunc("/api/system/modules-control", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Action string `json:"action"` // "start" or "stop"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Action != "start" && req.Action != "stop") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Action deve essere 'start' oppure 'stop'"})
			return
		}

		var report strings.Builder
		report.WriteString(fmt.Sprintf("=== AZIONE GLOBALE: %s ===\n", strings.ToUpper(req.Action)))

		// Read active configuration
		cfg, _ := config.LoadConfig("configs/config.example.yaml")
		modConfigs := make(map[string]string)
		if cfg != nil {
			for mName, mCfg := range cfg.Modules {
				modConfigs[mName] = mCfg.Level
			}
		}

		// Also check state.db
		if st, err := state.Open("state.db"); err == nil {
			if list, err := st.ListModules(); err == nil {
				for mName, sMod := range list {
					if _, exists := modConfigs[mName]; !exists || modConfigs[mName] == "" {
						modConfigs[mName] = sMod.Level
					}
				}
			}
			st.Close()
		}

		// Read all existing modules
		modDir := getModulesDir()
		entries, _ := os.ReadDir(modDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			modID := entry.Name()
			curLevel := modConfigs[modID]
			if curLevel == "" {
				curLevel = "off"
			}

			if req.Action == "start" {
				// USER REQUIREMENT: Only start modules configured != "off"!
				if curLevel == "off" {
					report.WriteString(fmt.Sprintf("  ⏸ %-12s SALTATO (livello: off)\n", modID))
					continue
				}

				if modID == "shares" {
					_ = exec.Command("systemctl", "start", "smbd").Run()
					report.WriteString(fmt.Sprintf("  ▶ %-12s Avviato (Samba native)\n", modID))
					continue
				}

				if modID == "network" {
					client := helper.Client{SocketPath: "/run/allod/helper.sock"}
					res, err := client.Execute("network.netbird_up", nil, false)
					if err != nil {
						client.SocketPath = "allod-helper.sock"
						res, err = client.Execute("network.netbird_up", nil, false)
					}
					if err != nil || !res.Ok {
						report.WriteString(fmt.Sprintf("  ✗ %-12s Errore avvio NetBird: %v\n", modID, err))
					} else {
						report.WriteString(fmt.Sprintf("  ▶ %-12s Avviato (NetBird native)\n", modID))
					}
					continue
				}

				_ = exec.Command("systemctl", "--user", "start", "--no-block", modID+"-postgres").Run()
				_ = exec.Command("systemctl", "--user", "start", "--no-block", modID+"-valkey").Run()
				err := exec.Command("systemctl", "--user", "start", "--no-block", modID).Run()
				if err != nil {
					report.WriteString(fmt.Sprintf("  ✗ %-12s Errore avvio: %v\n", modID, err))
				} else {
					report.WriteString(fmt.Sprintf("  ▶ %-12s Avviato (livello: %s)\n", modID, curLevel))
				}
			} else if req.Action == "stop" {
				// Stop module and its secondary containers
				if modID == "shares" {
					_ = exec.Command("systemctl", "stop", "smbd").Run()
					report.WriteString(fmt.Sprintf("  ⏹ %-12s Fermato (Samba native)\n", modID))
					continue
				}

				if modID == "network" {
					client := helper.Client{SocketPath: "/run/allod/helper.sock"}
					_, err := client.Execute("network.netbird_down", nil, false)
					if err != nil {
						client.SocketPath = "allod-helper.sock"
						_, _ = client.Execute("network.netbird_down", nil, false)
					}
					report.WriteString(fmt.Sprintf("  ⏹ %-12s Fermato (NetBird native)\n", modID))
					continue
				}

				_ = exec.Command("systemctl", "--user", "stop", modID).Run()
				_ = exec.Command("systemctl", "--user", "stop", modID+"-postgres").Run()
				_ = exec.Command("systemctl", "--user", "stop", modID+"-valkey").Run()
				report.WriteString(fmt.Sprintf("  ⏹ %-12s Fermato\n", modID))
			}
		}

		_ = exec.Command("systemctl", "--user", "reset-failed").Run()
		report.WriteString("\n✓ Operazione completata.")

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Azione %s completata per i moduli", req.Action),
			Data:    map[string]interface{}{"output": report.String()},
		})
	})

	// 5a-8. API System Enable Autostart & Linger
	mux.HandleFunc("/api/system/enable-autostart", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var report strings.Builder
		report.WriteString("=== ABILITAZIONE AVVIO AUTOMATICO AL BOOT ===\n")

		// 1. Enable linger for current user
		lingerCmd := exec.Command("loginctl", "enable-linger")
		lingerOut, err := lingerCmd.CombinedOutput()
		if err != nil {
			// Try with explicit username
			uName := os.Getenv("USER")
			if uName != "" {
				lingerOut, err = exec.Command("loginctl", "enable-linger", uName).CombinedOutput()
			}
		}
		if err != nil {
			report.WriteString(fmt.Sprintf("⚠️ loginctl enable-linger: %v (%s)\n  (Suggerimento: eseguire da terminale: sudo loginctl enable-linger $USER)\n", err, strings.TrimSpace(string(lingerOut))))
		} else {
			report.WriteString("✓ Systemd User Linger abilitato (i servizi utente e container girano 24/7 al boot senza login).\n")
		}

		// 2. Install allod-panel.service into ~/.config/systemd/user/
		home, _ := os.UserHomeDir()
		if home != "" {
			systemdUserDir := filepath.Join(home, ".config", "systemd", "user")
			_ = os.MkdirAll(systemdUserDir, 0755)

			serviceContent := `[Unit]
Description=Allod Web Dashboard Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=%h/allod
ExecStart=%h/allod/allod-panel
Restart=always
RestartSec=5s

[Install]
WantedBy=default.target
`
			panelSvcPath := filepath.Join(systemdUserDir, "allod-panel.service")
			if err := os.WriteFile(panelSvcPath, []byte(serviceContent), 0644); err == nil {
				report.WriteString("✓ File allod-panel.service installato in " + panelSvcPath + "\n")
			}

			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
			enableOut, err := exec.Command("systemctl", "--user", "enable", "allod-panel").CombinedOutput()
			if err != nil {
				report.WriteString(fmt.Sprintf("⚠️ systemctl --user enable allod-panel: %v (%s)\n", err, strings.TrimSpace(string(enableOut))))
			} else {
				report.WriteString("✓ allod-panel.service abilitato in systemd (si avvierà automaticamente ad ogni reboot).\n")
			}
		}

		// 3. Try to enable smbd for Samba
		_ = exec.Command("systemctl", "enable", "smbd").Run()
		report.WriteString("✓ Servizio Samba (smbd) registrato per l'avvio.\n")

		// 4. Reset failed
		_ = exec.Command("systemctl", "--user", "reset-failed").Run()
		report.WriteString("\n✓ Configurazione completata! Al prossimo riavvio sia la dashboard che i container partiranno da soli.")

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Avvio automatico al boot configurato con successo!",
			Data:    map[string]interface{}{"output": report.String()},
		})
	})

	// 5a-9. API System Setup Status (Checks linger and systemd user services)
	mux.HandleFunc("/api/system/setup-status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		uName := os.Getenv("USER")
		if uName == "" {
			uName = "user"
		}

		// 1. Check if linger is enabled
		lingerEnabled := false
		if _, err := os.Stat("/var/lib/systemd/linger/" + uName); err == nil {
			lingerEnabled = true
		} else {
			out, err := exec.Command("loginctl", "show-user", uName, "--property=Linger").CombinedOutput()
			if err == nil && strings.Contains(string(out), "Linger=yes") {
				lingerEnabled = true
			}
		}

		// 2. Check if allod-panel.service is enabled
		panelEnabled := false
		out, err := exec.Command("systemctl", "--user", "is-enabled", "allod-panel").CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) == "enabled" {
			panelEnabled = true
		}

		allReady := lingerEnabled && panelEnabled

		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data: map[string]interface{}{
				"linger_enabled": lingerEnabled,
				"panel_enabled":  panelEnabled,
				"all_ready":      allReady,
				"user":           uName,
			},
		})
	})

	// 5a-10. API Photos & Shares Integration (Bind mount /mnt/allod-storage/photos/upload to /mnt/allod-storage/shares/photos)
	mux.HandleFunc("/api/modules/photos/shares-integration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		targetPath := "/mnt/allod-storage/shares/photos"

		if r.Method == http.MethodGet {
			isMounted := false
			if mounts, err := os.ReadFile("/proc/mounts"); err == nil {
				isMounted = strings.Contains(string(mounts), targetPath)
			}

			savedEnabled := isMounted
			if st, err := state.Open("state.db"); err == nil {
				if val, err := st.GetMeta("photos_shares_integration"); err == nil && val != "" {
					savedEnabled = (val == "true")
				}
				st.Close()
			}

			sharesActive := false
			out, err := exec.Command("systemctl", "is-active", "smbd").CombinedOutput()
			if err == nil && strings.TrimSpace(string(out)) == "active" {
				sharesActive = true
			}

			json.NewEncoder(w).Encode(PanelResponse{
				Status: "ok",
				Data: map[string]interface{}{
					"enabled":       savedEnabled,
					"mounted":       isMounted,
					"shares_active": sharesActive,
					"target_path":   targetPath,
				},
			})
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Enabled  bool   `json:"enabled"`
				Username string `json:"username"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
				return
			}

			if st, err := state.Open("state.db"); err == nil {
				val := "false"
				if req.Enabled {
					val = "true"
				}
				_ = st.SetMeta("photos_shares_integration", val)
				st.Close()
			}

			client := helper.Client{SocketPath: "/run/allod/helper.sock"}
			bindArgs := map[string]interface{}{
				"enabled": req.Enabled,
			}
			if req.Username != "" {
				bindArgs["username"] = req.Username
			}
			resp, err := client.Execute("shares.bind_photos", bindArgs, false)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(PanelResponse{
					Status:  "error",
					Message: fmt.Sprintf("Root Helper non raggiungibile: %v. Assicurati che allod-helperd sia avviato.", err),
				})
				return
			}

			if !resp.Ok {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(PanelResponse{
					Status:  "error",
					Message: fmt.Sprintf("Errore applicazione bind mount: %s", resp.Error),
				})
				return
			}

			msg := "Collegamento Photos ➔ Shares attivato con successo!"
			if !req.Enabled {
				msg = "Collegamento Photos ➔ Shares disattivato."
			}

			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "ok",
				Message: msg,
				Data: map[string]interface{}{
					"enabled": req.Enabled,
				},
			})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// 5a-11. API Shares Set Samba Password
	mux.HandleFunc("/api/modules/shares/set-password", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
			return
		}

		if req.Username == "" {
			req.Username = os.Getenv("USER")
			if req.Username == "" {
				req.Username = "user"
			}
		}

		if len(req.Password) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La password deve contenere almeno 4 caratteri"})
			return
		}

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		// Ensure system user exists in OS first
		_ = ensureSystemUser(&client, req.Username)
		resp, err := client.Execute("shares.set_password", map[string]interface{}{
			"username": req.Username,
			"password": req.Password,
		}, false)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Root Helper non raggiungibile: %v. Assicurati che allod-helperd sia avviato.", err),
			})
			return
		}

		if !resp.Ok {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore impostazione password Samba: %s", resp.Error),
			})
			return
		}

		// If photos-shares integration is active, bind this user's photo folder automatically
		if st, err := state.Open("state.db"); err == nil {
			if val, _ := st.GetMeta("photos_shares_integration"); val == "true" {
				_, _ = client.Execute("shares.bind_photos", map[string]interface{}{
					"enabled":  true,
					"username": req.Username,
				}, false)
			}
			st.Close()
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Password Samba per l'utente '%s' configurata con successo (condivisione privata \\\\allod\\%s abilitata)!", req.Username, req.Username),
			Data: map[string]interface{}{
				"username": req.Username,
			},
		})
	})

	// 5a-12. API Triad Status (Samba + Immich + Jellyfin)
	mux.HandleFunc("/api/triad/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		allActive, sharesOk, photosOk, mediaOk, missing, reason := getTriadStatus()
		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data: map[string]interface{}{
				"all_active":    allActive,
				"shares_active": sharesOk,
				"photos_active": photosOk,
				"media_active":  mediaOk,
				"missing":       missing,
				"reason":        reason,
			},
		})
	})

	// 5a-13. API Triad Create User (Automates SMB + Immich + Jellyfin folders and mounts)
	mux.HandleFunc("/api/triad/create-user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		allActive, _, _, _, missing, _ := getTriadStatus()
		if !allActive {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Impossibile procedere: i seguenti servizi della Triade non sono attivi: %s. Tutti e 3 i servizi devono essere attivi per garantire la corretta impostazione di cartelle, permessi e bind-mount.", strings.Join(missing, ", ")),
			})
			return
		}

		var req struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			LinkPhotos *bool  `json:"link_photos"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
			return
		}

		req.Username = strings.ToLower(strings.TrimSpace(req.Username))
		if !validTriadUserRegex.MatchString(req.Username) || len(req.Username) < 2 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Nome utente non valido (solo lettere minuscole, numeri e trattini, min 2 caratteri)"})
			return
		}

		if len(req.Password) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La password deve contenere almeno 4 caratteri"})
			return
		}

		linkPhotos := true
		if req.LinkPhotos != nil {
			linkPhotos = *req.LinkPhotos
		}

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}

		// 1. Create Linux system user
		if errUser := ensureSystemUser(&client, req.Username); errUser != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore creazione utente Linux: " + errUser.Error()})
			return
		}

		// 2. Set Samba password and create private share [<username>]
		if errSmb := client.SetSambaPassword(req.Username, req.Password); errSmb != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore configurazione password Samba: %v", errSmb),
			})
			return
		}

		// 3. Pre-create subdirectories: photos, media, documents with strict permissions (0770)
		baseDir := quadlet.ResolvedStorageBaseDir()
		userShareDir := filepath.Join(baseDir, "shares", req.Username)
		_ = os.MkdirAll(userShareDir, 0770)
		_ = os.Chmod(userShareDir, 0770)
		for _, sub := range []string{"photos", "media", "documents"} {
			subPath := filepath.Join(userShareDir, sub)
			_ = os.MkdirAll(subPath, 0770)
			_ = os.Chmod(subPath, 0770)
		}

		// 4. Pre-create Immich library folder with proper permissions (0770)
		immichUserDir := filepath.Join(baseDir, "photos", "upload", "library", req.Username)
		_ = os.MkdirAll(immichUserDir, 0770)
		_ = os.Chmod(immichUserDir, 0770)

		// 5. Execute bind mount for user photos if requested
		if linkPhotos {
			_ = client.BindPhotos(req.Username, true)
		}

		host := r.Host
		if colon := strings.Index(host, ":"); colon != -1 {
			host = host[:colon]
		}
		if host == "" {
			host = "allod"
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Utente Triade '%s' creato e predisposto con successo!", req.Username),
			Data: map[string]interface{}{
				"username":             req.Username,
				"smb_path":             fmt.Sprintf("\\\\%s\\%s", host, req.Username),
				"photos_linked":        linkPhotos,
				"immich_storage_label": req.Username,
				"jellyfin_user":        req.Username,
			},
		})
	})

	// 5a-14. API Triad List Users
	mux.HandleFunc("/api/triad/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		baseDir := quadlet.ResolvedStorageBaseDir()
		sharesDir := filepath.Join(baseDir, "shares")

		type TriadUserInfo struct {
			Username     string `json:"username"`
			SmbPath      string `json:"smb_path"`
			PhotosLinked bool   `json:"photos_linked"`
			HasLibrary   bool   `json:"has_library"`
		}

		var users []TriadUserInfo
		mounts, _ := os.ReadFile("/proc/mounts")
		mountsStr := string(mounts)

		host := r.Host
		if colon := strings.Index(host, ":"); colon != -1 {
			host = host[:colon]
		}
		if host == "" {
			host = "allod"
		}

		if entries, err := os.ReadDir(sharesDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				uName := e.Name()
				if uName == "public" || uName == "photos" || uName == "media" || uName == "lost+found" || strings.HasPrefix(uName, ".") {
					continue
				}

				targetMount := filepath.Join(sharesDir, uName, "photos")
				photosLinked := strings.Contains(mountsStr, targetMount)

				libraryPath := filepath.Join(baseDir, "photos", "upload", "library", uName)
				_, errLib := os.Stat(libraryPath)

				users = append(users, TriadUserInfo{
					Username:     uName,
					SmbPath:      fmt.Sprintf("\\\\%s\\%s", host, uName),
					PhotosLinked: photosLinked,
					HasLibrary:   errLib == nil,
				})
			}
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data:   users,
		})
	})

	// 5a-15. API Triad Toggle Photos Link
	mux.HandleFunc("/api/triad/toggle-photos-link", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
			Enabled  bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
			return
		}

		req.Username = strings.ToLower(strings.TrimSpace(req.Username))
		if req.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Nome utente richiesto"})
			return
		}

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if err := client.BindPhotos(req.Username, req.Enabled); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore configurazione bind photos: %v", err),
			})
			return
		}

		msg := fmt.Sprintf("Cartella foto Immich collegata con successo su \\\\allod\\%s\\photos", req.Username)
		if !req.Enabled {
			msg = fmt.Sprintf("Collegamento foto su Samba rimosso per '%s'. Le foto rimangono protette solo nell'app Immich.", req.Username)
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: msg,
			Data: map[string]interface{}{
				"username":      req.Username,
				"photos_linked": req.Enabled,
			},
		})
	})

	// 5a-16. API Family Members List
	mux.HandleFunc("/api/family/members", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore apertura database: " + err.Error()})
			return
		}
		defer st.Close()

		baseDir := quadlet.ResolvedStorageBaseDir()
		sharesDir := filepath.Join(baseDir, "shares")
		mounts, _ := os.ReadFile("/proc/mounts")
		mountsStr := string(mounts)

		host := r.Host
		if colon := strings.Index(host, ":"); colon != -1 {
			host = host[:colon]
		}
		if host == "" {
			host = "allod"
		}

		proto := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "https"
		}

		// Auto-import any filesystem Samba users not yet in state.db (one-time migration only)
		if imported, _ := st.GetMeta("family_shares_imported"); imported != "true" {
			if entries, err := os.ReadDir(sharesDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					uName := e.Name()
					if uName == "public" || uName == "photos" || uName == "media" || uName == "lost+found" || strings.HasPrefix(uName, ".") || strings.Contains(uName, ".deleted") {
						continue
					}
					existing, _ := st.GetFamilyMember(uName)
					if existing == nil {
						targetMount := filepath.Join(sharesDir, uName, "photos")
						isLinked := strings.Contains(mountsStr, targetMount)
						_ = st.CreateFamilyMember(&state.FamilyMember{
							Username:     uName,
							FirstName:    strings.Title(uName),
							LastName:     "",
							Role:         "member",
							AvatarColor:  "#38bdf8",
							SmbActive:    true,
							PhotosLinked: isLinked,
						})
					}
				}
			}
			_ = st.SetMeta("family_shares_imported", "true")
		}

		members, err := st.ListFamilyMembers()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore lettura membri: " + err.Error()})
			return
		}

		type FamilyMemberDTO struct {
			ID           int64  `json:"id"`
			Username     string `json:"username"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name"`
			DisplayName  string `json:"display_name"`
			Email        string `json:"email"`
			Role         string `json:"role"`
			AvatarColor  string `json:"avatar_color"`
			Notes        string `json:"notes"`
			SmbPath      string `json:"smb_path"`
			SmbActive    bool   `json:"smb_active"`
			PhotosLinked bool   `json:"photos_linked"`
			HasLibrary   bool   `json:"has_library"`
			HasInvite    bool   `json:"has_invite"`
			InviteURL    string `json:"invite_url,omitempty"`
			CreatedAt    string `json:"created_at"`
		}

		var dtos []FamilyMemberDTO
		for _, m := range members {
			targetMount := filepath.Join(sharesDir, m.Username, "photos")
			photosLinked := strings.Contains(mountsStr, targetMount)

			libraryPath := filepath.Join(baseDir, "photos", "upload", "library", m.Username)
			_, errLib := os.Stat(libraryPath)

			disp := m.FirstName
			if m.LastName != "" {
				disp += " " + m.LastName
			}

			var inviteURL string
			hasInvite := false
			if m.OnboardingToken != "" && m.OnboardingExpires != nil && time.Now().Before(*m.OnboardingExpires) {
				hasInvite = true
				inviteURL = fmt.Sprintf("%s://%s/portal?invite=%s", proto, r.Host, m.OnboardingToken)
			}

			dtos = append(dtos, FamilyMemberDTO{
				ID:           m.ID,
				Username:     m.Username,
				FirstName:    m.FirstName,
				LastName:     m.LastName,
				DisplayName:  disp,
				Email:        m.Email,
				Role:         m.Role,
				AvatarColor:  m.AvatarColor,
				Notes:        m.Notes,
				SmbPath:      fmt.Sprintf("\\\\%s\\%s", host, m.Username),
				SmbActive:    m.SmbActive,
				PhotosLinked: photosLinked,
				HasLibrary:   errLib == nil,
				HasInvite:    hasInvite,
				InviteURL:    inviteURL,
				CreatedAt:    m.CreatedAt.Format("02/01/2006"),
			})
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data:   dtos,
		})
	})

	// 5a-17. API Family Member Create
	mux.HandleFunc("/api/family/create", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		allActive, _, _, _, _, reason := getTriadStatus()
		if !allActive {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Impossibile creare il membro: i 3 moduli della Triade devono essere tutti operativi. %s", reason),
			})
			return
		}

		var req struct {
			Username       string `json:"username"`
			FirstName      string `json:"first_name"`
			LastName       string `json:"last_name"`
			Email          string `json:"email"`
			Role           string `json:"role"`
			AvatarColor    string `json:"avatar_color"`
			Notes          string `json:"notes"`
			Password       string `json:"password"`
			LinkPhotos     *bool  `json:"link_photos"`
			GenerateInvite bool   `json:"generate_invite"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
			return
		}

		req.Username = strings.ToLower(strings.TrimSpace(req.Username))
		req.FirstName = strings.TrimSpace(req.FirstName)
		req.LastName = strings.TrimSpace(req.LastName)
		req.Email = strings.TrimSpace(req.Email)

		if req.Username == "" || !validTriadUserRegex.MatchString(req.Username) || len(req.Username) < 2 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Nome utente non valido (solo lettere minuscole, numeri e trattini, min 2 caratteri)"})
			return
		}

		if req.FirstName == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Il nome è obbligatorio"})
			return
		}

		if req.Password != "" && len(req.Password) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La password deve contenere almeno 4 caratteri"})
			return
		}

		// If no password provided, always generate an invite link for autonomous onboarding
		if req.Password == "" {
			req.GenerateInvite = true
		}

		linkPhotos := true
		if req.LinkPhotos != nil {
			linkPhotos = *req.LinkPhotos
		}

		if req.Role == "" {
			req.Role = "member"
		}
		if req.AvatarColor == "" {
			req.AvatarColor = "#38bdf8"
		}

		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore database: " + err.Error()})
			return
		}
		defer st.Close()

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}

		// 1. Create Linux system user
		if errU := ensureSystemUser(&client, req.Username); errU != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore registrazione utente Linux: " + errU.Error()})
			return
		}

		// 2. Set Samba password if provided
		if req.Password != "" {
			if errSmb := client.SetSambaPassword(req.Username, req.Password); errSmb != nil {
				// Retry with auto-heal
				_ = ensureSystemUser(&client, req.Username)
				if errRetry := client.SetSambaPassword(req.Username, req.Password); errRetry != nil {
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(PanelResponse{
						Status:  "error",
						Message: fmt.Sprintf("Errore configurazione password Samba: %v", errRetry),
					})
					return
				}
			}
		}

		// 3. Pre-create subdirectories: photos, media, documents with strict permissions (0770)
		baseDir := quadlet.ResolvedStorageBaseDir()
		userShareDir := filepath.Join(baseDir, "shares", req.Username)
		_ = os.MkdirAll(userShareDir, 0770)
		_ = os.Chmod(userShareDir, 0770)
		for _, sub := range []string{"photos", "media", "documents"} {
			subPath := filepath.Join(userShareDir, sub)
			_ = os.MkdirAll(subPath, 0770)
			_ = os.Chmod(subPath, 0770)
		}

		// 4. Pre-create Immich library folder with proper permissions (0770)
		immichUserDir := filepath.Join(baseDir, "photos", "upload", "library", req.Username)
		_ = os.MkdirAll(immichUserDir, 0770)
		_ = os.Chmod(immichUserDir, 0770)

		// 5. Execute bind mount for user photos if requested
		if linkPhotos {
			_ = client.BindPhotos(req.Username, true)
		}

		// 6. Save member in state.db
		member := &state.FamilyMember{
			Username:     req.Username,
			FirstName:    req.FirstName,
			LastName:     req.LastName,
			Email:        req.Email,
			Role:         req.Role,
			AvatarColor:  req.AvatarColor,
			Notes:        req.Notes,
			SmbActive:    true,
			PhotosLinked: linkPhotos,
		}

		if err := st.CreateFamilyMember(member); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore salvataggio membro: " + err.Error()})
			return
		}

		var inviteToken string
		var inviteURL string
		proto := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "https"
		}
		host := r.Host
		if colon := strings.Index(host, ":"); colon != -1 {
			host = host[:colon]
		}
		if host == "" {
			host = "allod"
		}

		if req.GenerateInvite {
			token, errTok := st.CreateResetToken(req.Username, 72*time.Hour)
			if errTok == nil {
				inviteToken = token
				inviteURL = fmt.Sprintf("%s://%s/portal?invite=%s", proto, r.Host, token)
			}
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Membro della famiglia '%s %s' (%s) creato con successo!", req.FirstName, req.LastName, req.Username),
			Data: map[string]interface{}{
				"username":             req.Username,
				"first_name":           req.FirstName,
				"last_name":            req.LastName,
				"role":                 req.Role,
				"smb_path":             fmt.Sprintf("\\\\%s\\%s", host, req.Username),
				"photos_linked":        linkPhotos,
				"has_invite":           req.GenerateInvite,
				"invite_token":         inviteToken,
				"invite_url":           inviteURL,
				"immich_storage_label": req.Username,
				"jellyfin_user":        req.Username,
			},
		})
	})

	// 5a-18. API Family Generate Reset Link
	mux.HandleFunc("/api/family/generate-reset-link", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Nome utente richiesto"})
			return
		}

		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore database: " + err.Error()})
			return
		}
		defer st.Close()

		token, err := st.CreateResetToken(req.Username, 48*time.Hour)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		proto := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "https"
		}
		inviteURL := fmt.Sprintf("%s://%s/portal?invite=%s", proto, r.Host, token)

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: "Link di invito/ripristino generato con successo!",
			Data: map[string]interface{}{
				"username":   req.Username,
				"token":      token,
				"invite_url": inviteURL,
				"expires_in": "48 ore",
			},
		})
	})

	// 5a-19. API Family Member Delete
	mux.HandleFunc("/api/family/delete", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Nome utente richiesto"})
			return
		}
		req.Username = strings.ToLower(strings.TrimSpace(req.Username))

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		// 1. Unbind photos
		_ = client.BindPhotos(req.Username, false)

		// 2. Delete member and lock auto-import
		st, err := state.Open(dbPath)
		if err == nil {
			_ = st.DeleteFamilyMember(req.Username)
			_ = st.SetMeta("family_shares_imported", "true")
			st.Close()
		}

		// 3. Rename user shares directory to avoid re-detection
		baseDir := quadlet.ResolvedStorageBaseDir()
		userShareDir := filepath.Join(baseDir, "shares", req.Username)
		_ = os.Rename(userShareDir, userShareDir+".deleted."+time.Now().Format("20060102150405"))

		// 4. Remove user from Samba via helper
		_ = client.DeleteSambaUser(req.Username)

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Membro della famiglia '%s' rimosso con successo.", req.Username),
		})
	})

	// 5a-20. API Portal Verify Token
	mux.HandleFunc("/api/portal/verify-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Token mancante"})
			return
		}

		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore database: " + err.Error()})
			return
		}
		defer st.Close()

		member, err := st.ValidateResetToken(token)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data: map[string]interface{}{
				"username":   member.Username,
				"first_name": member.FirstName,
				"last_name":  member.LastName,
				"role":       member.Role,
			},
		})
	})

	// 5a-21. API Portal Set Password
	mux.HandleFunc("/api/portal/set-password", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Token       string `json:"token"`
			Username    string `json:"username"`
			NewPassword string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Payload JSON non valido"})
			return
		}

		if len(req.NewPassword) < 4 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "La nuova password deve contenere almeno 4 caratteri"})
			return
		}

		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore database: " + err.Error()})
			return
		}
		defer st.Close()

		targetUser := ""
		if req.Token != "" {
			member, errVal := st.ValidateResetToken(req.Token)
			if errVal != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errVal.Error()})
				return
			}
			targetUser = member.Username
		} else if req.Username != "" {
			targetUser = strings.ToLower(strings.TrimSpace(req.Username))
			m, errMem := st.GetFamilyMember(targetUser)
			if errMem != nil || m == nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Utente non trovato"})
				return
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Token o nome utente richiesto"})
			return
		}

		// Set Samba password via root helper
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		// Ensure system user exists in Linux (/etc/passwd) first
		if errEnsure := ensureSystemUser(&client, targetUser); errEnsure != nil {
			log.Printf("[portal] Avviso ensureSystemUser: %v", errEnsure)
		}

		errSmb := client.SetSambaPassword(targetUser, req.NewPassword)
		if errSmb != nil {
			// Retry once after ensuring user
			_ = ensureSystemUser(&client, targetUser)
			errSmb = client.SetSambaPassword(targetUser, req.NewPassword)
		}

		if errSmb != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore aggiornamento password Samba: %v", errSmb),
			})
			return
		}

		// Consume token if one was used
		if req.Token != "" {
			_ = st.ConsumeResetToken(req.Token)
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Password personale per '%s' impostata con successo!", targetUser),
			Data: map[string]interface{}{
				"username": targetUser,
			},
		})
	})

	// 5b. API Storage Init
	mux.HandleFunc("/api/storage/init", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Disks []string `json:"disks"`
			Mode  string   `json:"mode"`
			Mount string   `json:"mount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Optional body
		}

		if len(req.Disks) == 0 {
			topo := preflight.DetectStorageTopology()
			for _, d := range topo.DataDisks {
				req.Disks = append(req.Disks, d.Name)
			}
		}

		if len(req.Disks) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Nessun disco dati secondario rilevato"})
			return
		}

		if req.Mode == "" {
			if len(req.Disks) >= 2 {
				req.Mode = "raid1"
			} else {
				req.Mode = "single"
			}
		}
		if req.Mount == "" {
			req.Mount = "/mnt/allod-storage"
		}

		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if err := client.InitStorage(req.Mode, req.Disks, req.Mount, os.Getenv("USER")); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{
				Status:  "error",
				Message: fmt.Sprintf("Errore inizializzazione storage: %v", err),
			})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Pool Btrfs %s inizializzato con successo su %s!", strings.ToUpper(req.Mode), req.Mount),
		})
	})

	// 5c. API Storage Diagnostics (Live Btrfs usage & device stats)
	mux.HandleFunc("/api/storage/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		mountPoint := "/mnt/allod-storage"
		var usageStr, statsStr, dfStr string
		isMounted := true

		// Try privileged helper first for 100% complete root device stats and detailed chunks
		client := helper.Client{SocketPath: "/run/allod/helper.sock"}
		if output, err := client.StorageDiagnostics(mountPoint); err == nil && output != "" {
			var diagData struct {
				Usage string `json:"usage"`
				Stats string `json:"stats"`
				Df    string `json:"df"`
			}
			if err := json.Unmarshal([]byte(output), &diagData); err == nil {
				usageStr = diagData.Usage
				statsStr = diagData.Stats
				dfStr = diagData.Df
			}
		}

		// Fallback to unprivileged exec if helper didn't provide results
		if usageStr == "" {
			usageOut, err := exec.Command("btrfs", "filesystem", "usage", mountPoint).CombinedOutput()
			if err != nil {
				usageOut = []byte(fmt.Sprintf("btrfs filesystem usage error: %v\nOutput: %s", err, string(usageOut)))
			}
			usageStr = string(usageOut)
			isMounted = (err == nil)
		}

		if statsStr == "" {
			statsOut, err := exec.Command("btrfs", "device", "stats", mountPoint).CombinedOutput()
			if err != nil {
				statsOut = []byte(fmt.Sprintf("btrfs device stats error: %v\nOutput: %s", err, string(statsOut)))
			}
			statsStr = string(statsOut)
		}

		if dfStr == "" {
			dfOut, err := exec.Command("btrfs", "filesystem", "df", mountPoint).CombinedOutput()
			if err != nil {
				dfOut = []byte(fmt.Sprintf("btrfs filesystem df error: %v\nOutput: %s", err, string(dfOut)))
			}
			dfStr = string(dfOut)
		}

		data := map[string]interface{}{
			"mount_point": mountPoint,
			"usage":       usageStr,
			"stats":       statsStr,
			"df":          dfStr,
			"is_mounted":  isMounted,
			"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
	})

	// 5d. API Module Diagnostics (Live systemd status & container logs)
	mux.HandleFunc("/api/modules/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		modName := r.URL.Query().Get("module")
		if modName == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Missing module query parameter"})
			return
		}

		var statusOut, logsOut []byte
		if modName == "shares" {
			statusOut, _ = exec.Command("systemctl", "status", "smbd").CombinedOutput()
			if len(statusOut) == 0 {
				statusOut, _ = exec.Command("systemctl", "status", "smb").CombinedOutput()
			}
			logsOut, _ = exec.Command("journalctl", "-u", "smbd", "-n", "30", "--no-pager").CombinedOutput()
			if len(logsOut) == 0 || strings.Contains(string(logsOut), "No entries") {
				logsOut, _ = exec.Command("testparm", "-s").CombinedOutput()
			}
		} else if modName == "network" {
			client := helper.Client{SocketPath: "/run/allod/helper.sock"}
			statusRes, errS := client.Execute("network.netbird_cli", map[string]interface{}{"command": "status_detail"}, false)
			if errS != nil {
				client.SocketPath = "allod-helper.sock"
				statusRes, errS = client.Execute("network.netbird_cli", map[string]interface{}{"command": "status_detail"}, false)
			}
			if errS == nil && statusRes.Ok && len(statusRes.Output) > 0 && !strings.Contains(statusRes.Output, "cannot exec in a stopped container") {
				statusOut = []byte(statusRes.Output)
			} else {
				ctxSys, cancelSys := context.WithTimeout(r.Context(), 3*time.Second)
				sysOut, _ := exec.CommandContext(ctxSys, "systemctl", "status", "netbird").CombinedOutput()
				cancelSys()
				if len(sysOut) > 0 {
					statusOut = sysOut
				} else if iface, err := net.InterfaceByName("wt0"); err == nil {
					statusOut = []byte(fmt.Sprintf("✓ Interfaccia kernel wt0 attiva (MTU: %d, Flags: %v)\nNetBird WireGuard mesh attivo a livello host.", iface.MTU, iface.Flags))
				} else {
					errMsg := statusRes.Error
					if errMsg == "" && len(statusRes.Output) > 0 {
						errMsg = statusRes.Output
					}
					statusOut = []byte("NetBird non attivo o in attesa di avvio.\n" + errMsg)
				}
			}

			logsRes, errL := client.Execute("network.netbird_cli", map[string]interface{}{"command": "logs"}, false)
			if errL != nil {
				client.SocketPath = "allod-helper.sock"
				logsRes, errL = client.Execute("network.netbird_cli", map[string]interface{}{"command": "logs"}, false)
			}
			if errL == nil && logsRes.Ok && len(logsRes.Output) > 0 {
				logsOut = []byte(logsRes.Output)
			} else {
				ctxJ, cancelJ := context.WithTimeout(r.Context(), 3*time.Second)
				logsOut, _ = exec.CommandContext(ctxJ, "journalctl", "-u", "netbird", "-n", "30", "--no-pager").CombinedOutput()
				cancelJ()
				if len(logsOut) == 0 || strings.Contains(string(logsOut), "No entries") {
					ctxP, cancelP := context.WithTimeout(r.Context(), 3*time.Second)
					logsOut, _ = exec.CommandContext(ctxP, "podman", "logs", "--tail", "30", "allod-netbird").CombinedOutput()
					cancelP()
				}
			}
		} else {
			statusOut, _ = exec.Command("systemctl", "--user", "status", modName).CombinedOutput()
			logsOut, _ = exec.Command("podman", "logs", "--tail", "30", modName).CombinedOutput()
			if len(logsOut) == 0 || strings.Contains(string(logsOut), "no container") {
				logsOut, _ = exec.Command("podman", "logs", "--tail", "30", "systemd-"+modName).CombinedOutput()
			}
			if len(logsOut) == 0 || strings.Contains(string(logsOut), "no container") {
				logsOut, _ = exec.Command("journalctl", "--user", "-u", modName, "-n", "30", "--no-pager").CombinedOutput()
			}
		}

		data := map[string]interface{}{
			"module":      modName,
			"status_text": string(statusOut),
			"logs":        string(logsOut),
			"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
	})

	// 5e. API Network Status & NetBird Control (extracted to internal/panel/network.go)
	panel.RegisterNetworkRoutes(mux, &panel.NetworkHandler{
		GetConfigPath: getConfigPath,
		GetModulesDir: getModulesDir,
		Helper:        &helper.Client{SocketPath: "/run/allod/helper.sock"},
	})

	// 5f. API Speedtest (Ping, Download, Upload)
	mux.HandleFunc("/api/speedtest/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "timestamp": time.Now().UnixMilli()})
	})

	mux.HandleFunc("/api/speedtest/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")

		chunk := make([]byte, 64*1024)
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}

		totalChunks := 480 // ~30 MB stream for high-speed measurement
		flusher, ok := w.(http.Flusher)
		for i := 0; i < totalChunks; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if ok && i%10 == 0 {
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/api/speedtest/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		start := time.Now()
		bytesRead, _ := io.Copy(io.Discard, io.LimitReader(r.Body, 100*1024*1024))
		duration := time.Since(start)

		mbps := 0.0
		if duration.Seconds() > 0 && bytesRead > 0 {
			mbps = (float64(bytesRead) * 8.0) / (duration.Seconds() * 1000 * 1000)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"bytes":      bytesRead,
			"duration_s": duration.Seconds(),
			"mbps":       mbps,
		})
	})

	// 6. API Modules Set Level
	mux.HandleFunc("/api/modules/set", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Module     string `json:"module"`
			Level      string `json:"level"`
			AcceptRisk bool   `json:"accept_risk"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Invalid JSON"})
			return
		}

		cfgPath := getConfigPath()
		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		mPath := filepath.Join(getModulesDir(), req.Module, "module.yaml")
		m, err := manifest.LoadManifest(mPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Modulo inesistente: " + err.Error()})
			return
		}

		if req.Level != "off" {
			if _, exists := m.Levels[req.Level]; !exists {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Livello '%s' non valido", req.Level)})
				return
			}
		}

		res := preflight.Check(cfg, req.Module, m, req.Level)
		if !res.Pass && !req.AcceptRisk {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(PanelResponse{Status: "rejected", Message: res.Message, Data: res.Message})
			return
		}

		if cfg.Modules == nil {
			cfg.Modules = make(map[string]config.ModuleConfig)
		}
		modCfg := cfg.Modules[req.Module]
		modCfg.Level = req.Level
		cfg.Modules[req.Module] = modCfg

		if err := cfg.Save(cfgPath); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		home, _ := os.UserHomeDir()
		if home != "" {
			quadDir := filepath.Join(home, ".config", "containers", "systemd")
			systemdUserDir := filepath.Join(home, ".config", "systemd", "user")
			_ = os.MkdirAll(quadDir, 0755)
			_ = os.MkdirAll(systemdUserDir, 0755)
			if req.Level == "off" {
				if req.Module == "network" {
					client := helper.Client{SocketPath: "/run/allod/helper.sock"}
					_, _ = client.Execute("network.netbird_down", nil, false)
				} else if req.Module == "shares" {
					client := helper.Client{SocketPath: "/run/allod/helper.sock"}
					_, _ = client.Execute("shares.apply", map[string]interface{}{
						"name":    "shares",
						"path":    "/mnt/allod-storage/shares",
						"enabled": false,
					}, false)
				}
				_ = quadlet.StopAndRemoveContainers(req.Module, true)
			} else {
				quadlet.EnsureAllodNetwork(quadDir)
				if genRes, err := quadlet.Generate(req.Module, m, req.Level); err == nil {
					for fname, content := range genRes.Files {
						if strings.HasSuffix(fname, ".service") {
							_ = os.WriteFile(filepath.Join(systemdUserDir, fname), []byte(content), 0644)
						}
						_ = os.WriteFile(filepath.Join(quadDir, fname), []byte(content), 0644)
					}
				}
			}
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: fmt.Sprintf("Modulo %s impostato a %s", req.Module, req.Level)})
	})

	// 6c. API Purge Module (Clean restart / reset with double-lock safety)
	mux.HandleFunc("/api/modules/purge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Module string `json:"module"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Module == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Modulo non specificato"})
			return
		}

		modID := req.Module

		// 1. Forcibly stop and remove Podman containers and Quadlet units
		_ = quadlet.StopAndRemoveContainers(modID, true)

		// 5. Clean state.db entry
		if st, err := state.Open(dbPath); err == nil {
			st.DeleteModule(modID)
			st.Close()
		}

		// 6. Clean and re-initialize storage folder
		home, _ := os.UserHomeDir()
		baseDir := "/mnt/allod-storage"
		if _, err := os.Stat(baseDir); err != nil {
			baseDir = filepath.Join(home, ".local", "share", "allod", "storage")
		}
		modStorage := filepath.Join(baseDir, modID)
		_ = exec.Command("podman", "unshare", "rm", "-rf", modStorage).Run()
		_ = os.RemoveAll(modStorage)
		quadlet.EnsureStorageDirectories(modID)

		// 7. Daemon reload
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

		json.NewEncoder(w).Encode(PanelResponse{
			Status:  "ok",
			Message: fmt.Sprintf("Modulo '%s' cancellato e ripristinato con successo. Cartelle pulite su %s.", modID, modStorage),
		})
	})

	// 7. API Ring Status
	mux.HandleFunc("/api/ring", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg, _ := config.LoadConfig(getConfigPath())
		topo, isStandalone := getRingTopology(cfg)

		placements := topo.CalculatePlacement()

		data := map[string]interface{}{
			"name":            topo.Name,
			"target_replicas": topo.TargetReplicas,
			"members":         topo.Members,
			"placements":      placements,
			"is_standalone":   isStandalone,
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
	})

	// 8. API Ring Simulate Removal
	mux.HandleFunc("/api/ring/simulate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Member string `json:"member"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cfg, _ := config.LoadConfig(getConfigPath())
		topo, _ := getRingTopology(cfg)

		impact, err := topo.SimulateRemoval(req.Member)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: impact})
	})

	// 9. API Update Rollback Simulation
	mux.HandleFunc("/api/update/simulate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Module string `json:"module"`
			Tag    string `json:"tag"`
			Fail   bool   `json:"fail"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cfg, err := config.LoadConfig(getConfigPath())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore caricamento config: " + err.Error()})
			return
		}
		st, err := state.Open(dbPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Errore apertura state.db: " + err.Error()})
			return
		}
		defer st.Close()

		u := updater.NewUpdater("out_quadlet")
		report, err := u.SimulateUpdate(req.Module, req.Tag, req.Fail, cfg, st)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: report})
	})

	// 10. API Test Helper
	mux.HandleFunc("/api/test-helper", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		conn, err := net.Dial("unix", "/run/allod/helper.sock")
		if err != nil {
			conn, err = net.Dial("unix", "allod-helper.sock")
			if err != nil {
				w.WriteHeader(500)
				msg := "Impossibile contattare allod-helperd (è avviato?)"
				if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
					msg = "Permesso negato (EACCES): l'utente del pannello non appartiene al gruppo 'allod'. Esegui: sudo usermod -aG allod $USER"
				}
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: msg})
				return
			}
		}
		defer conn.Close()

		reqBody := `{"action": "shares.apply", "plan": true, "args": {"name": "video", "path": "/data/video"}}`
		conn.Write([]byte(reqBody + "\n"))
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))

		var respBuf bytes.Buffer
		io.Copy(&respBuf, conn)

		json.NewEncoder(w).Encode(PanelResponse{
			Status: "ok",
			Data:   fmt.Sprintf("Risposta dall'helper root: %s", respBuf.String()),
		})
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Background startup synchronization of all family members to ensure Linux users exist
	go func() {
		time.Sleep(1 * time.Second)
		if st, err := state.Open(dbPath); err == nil {
			defer st.Close()
			if members, err := st.ListFamilyMembers(); err == nil {
				c := helper.Client{SocketPath: "/run/allod/helper.sock"}
				for _, m := range members {
					_ = ensureSystemUser(&c, m.Username)
				}
			}
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Errore server: %v\n", err)
		os.Exit(1)
	}
}

func checkHelperConnectivity() (connected bool, permissionDenied bool) {
	conn, err := net.DialTimeout("unix", "/run/allod/helper.sock", 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true, false
	}
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		return false, true
	}
	if _, statErr := os.Stat("/run/allod/helper.sock"); statErr != nil {
		if os.IsPermission(statErr) || strings.Contains(strings.ToLower(statErr.Error()), "permission denied") {
			return false, true
		}
	}

	conn, err = net.DialTimeout("unix", "allod-helper.sock", 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true, false
	}
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		return false, true
	}

	return false, false
}
