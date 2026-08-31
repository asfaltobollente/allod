package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/allod-project/allod/internal/config"
	"github.com/allod-project/allod/internal/helper"
	"github.com/allod-project/allod/internal/manifest"
	"github.com/allod-project/allod/internal/preflight"
	"github.com/allod-project/allod/internal/quadlet"
	"github.com/allod-project/allod/internal/ring"
	"github.com/allod-project/allod/internal/state"
	"github.com/allod-project/allod/internal/updater"
)

//go:embed web/*
var webFS embed.FS

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

func getModuleRuntimeStatus(modName string, level string) string {
	if level == "off" || level == "" {
		return "off"
	}
	if modName == "storage" {
		// Storage is active if /mnt/allod-storage is mounted or single/raid1 level is configured
		if _, err := os.Stat("/mnt/allod-storage"); err == nil {
			return "running"
		}
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
	return "stopped"
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

	// 2. API Status
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg, err := config.LoadConfig(getConfigPath())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		committed, _ := preflight.CommittedRAMInfo(cfg, "")
		usedMB := preflight.CoreReservedMB + committed

		helperConnected := checkHelperConnectivity()
		topo, isStandalone := getRingTopology(cfg)
		storageTopo := preflight.DetectStorageTopology()

		uid := os.Getuid()
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = fmt.Sprintf("uid-%d", uid)
		}
		isRootless := uid != 0

		data := map[string]interface{}{
			"node_name":        cfg.Node.Name,
			"channel":          cfg.Node.Channel,
			"ram_total_mb":     preflight.GetSystemRAMMB(),
			"core_reserved_mb": preflight.CoreReservedMB,
			"ram_used_mb":      usedMB,
			"helper_connected": helperConnected,
			"group_repo":       cfg.Node.Group,
			"is_standalone":    isStandalone,
			"ring_members":     len(topo.Members),
			"is_rootless":      isRootless,
			"uid":              uid,
			"current_user":     currentUser,
			"storage":          storageTopo,
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

		entries, err := os.ReadDir("modules")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		var modules []ModuleInfo
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			modName := e.Name()
			mPath := filepath.Join("modules", modName, "module.yaml")
			m, err := manifest.LoadManifest(mPath)
			if err != nil {
				continue
			}

			curLevel := "off"
			if modCfg, exists := cfg.Modules[modName]; exists && modCfg.Level != "" {
				curLevel = modCfg.Level
			}

			runtimeStatus := getModuleRuntimeStatus(modName, curLevel)
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

		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		cmd := exec.Command("systemctl", "--user", "start", "--no-block", req.Module)
		if err := cmd.Run(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore avvio servizio %s: %v", req.Module, err)})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: fmt.Sprintf("Avvio del modulo %s avviato in background", req.Module)})
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

		cmd := exec.Command("systemctl", "--user", "stop", req.Module)
		if err := cmd.Run(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore arresto %s: %v", req.Module, err)})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: fmt.Sprintf("Modulo %s fermato", req.Module)})
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
		res, err := client.Execute("storage.init", map[string]interface{}{
			"disks": req.Disks,
			"mode":  req.Mode,
			"mount": req.Mount,
			"user":  os.Getenv("USER"),
		}, false)

		if err != nil || !res.Ok {
			errMsg := "Errore comunicazione con allod-helperd (assicurati che sia avviato con sudo)"
			if err != nil {
				errMsg = err.Error()
			} else if res.Error != "" {
				errMsg = res.Error
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: errMsg})
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
		usageOut, err := exec.Command("btrfs", "filesystem", "usage", mountPoint).CombinedOutput()
		if err != nil {
			usageOut = []byte(fmt.Sprintf("btrfs filesystem usage error: %v\nOutput: %s", err, string(usageOut)))
		}

		statsOut, err := exec.Command("btrfs", "device", "stats", mountPoint).CombinedOutput()
		if err != nil {
			statsOut = []byte(fmt.Sprintf("btrfs device stats error: %v\nOutput: %s", err, string(statsOut)))
		}

		dfOut, err := exec.Command("btrfs", "filesystem", "df", mountPoint).CombinedOutput()
		if err != nil {
			dfOut = []byte(fmt.Sprintf("btrfs filesystem df error: %v\nOutput: %s", err, string(dfOut)))
		}

		data := map[string]interface{}{
			"mount_point":  mountPoint,
			"usage":        string(usageOut),
			"stats":        string(statsOut),
			"df":           string(dfOut),
			"is_mounted":   err == nil,
			"timestamp":    time.Now().Format("2006-01-02 15:04:05"),
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

		statusOut, _ := exec.Command("systemctl", "--user", "status", modName).CombinedOutput()
		logsOut, _ := exec.Command("podman", "logs", "--tail", "30", "systemd-"+modName).CombinedOutput()
		if len(logsOut) == 0 {
			logsOut, _ = exec.Command("journalctl", "--user", "-u", modName, "-n", "30", "--no-pager").CombinedOutput()
		}

		data := map[string]interface{}{
			"module":      modName,
			"status_text": string(statusOut),
			"logs":        string(logsOut),
			"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
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

		mPath := filepath.Join("modules", req.Module, "module.yaml")
		m, err := manifest.LoadManifest(mPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Modulo inesistente"})
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

		// 1. Stop systemd services
		_ = exec.Command("systemctl", "--user", "stop", modID).Run()
		_ = exec.Command("systemctl", "--user", "stop", modID+"-postgres").Run()
		_ = exec.Command("systemctl", "--user", "stop", modID+"-valkey").Run()

		// 2. Remove Podman containers
		_ = exec.Command("podman", "rm", "-f", "systemd-"+modID).Run()
		_ = exec.Command("podman", "rm", "-f", "systemd-"+modID+"-postgres").Run()
		_ = exec.Command("podman", "rm", "-f", "systemd-"+modID+"-valkey").Run()
		_ = exec.Command("podman", "rm", "-f", modID).Run()

		// 3. Reset failed state
		_ = exec.Command("systemctl", "--user", "reset-failed").Run()

		// 4. Remove generated Quadlet units
		home, _ := os.UserHomeDir()
		if home != "" {
			quadDir := filepath.Join(home, ".config", "containers", "systemd")
			_ = os.Remove(filepath.Join(quadDir, modID+".container"))
			_ = os.Remove(filepath.Join(quadDir, modID+"-postgres.container"))
			_ = os.Remove(filepath.Join(quadDir, modID+"-valkey.container"))
			_ = os.Remove(filepath.Join(quadDir, modID+".service"))
		}

		// 5. Clean state.db entry
		if st, err := state.Open(dbPath); err == nil {
			st.DeleteModule(modID)
			st.Close()
		}

		// 6. Clean and re-initialize storage folder
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
		conn, err := net.Dial("tcp", "127.0.0.1:40000")
		if err != nil {
			conn, err = net.Dial("unix", "allod-helper.sock")
			if err != nil {
				w.WriteHeader(500)
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: "Impossibile contattare allod-helperd (è avviato?)"})
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

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Errore server: %v\n", err)
		os.Exit(1)
	}
}

func checkHelperConnectivity() bool {
	conn, err := net.DialTimeout("unix", "/run/allod/helper.sock", 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	conn, err = net.DialTimeout("unix", "allod-helper.sock", 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	conn, err = net.DialTimeout("tcp", "127.0.0.1:40000", 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
