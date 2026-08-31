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
	"github.com/allod-project/allod/internal/manifest"
	"github.com/allod-project/allod/internal/preflight"
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

type ModuleInfo struct {
	ID            string             `json:"id"`
	Tier          string             `json:"tier"`
	CurrentLevel  string             `json:"current_level"`
	RuntimeStatus string             `json:"runtime_status"` // "running", "stopped", "failed", "off"
	Manifest      *manifest.Manifest `json:"manifest"`
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

func getModuleRuntimeStatus(modName string, level string) string {
	if level == "off" || level == "" {
		return "off"
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

			modules = append(modules, ModuleInfo{
				ID:            modName,
				Tier:          m.Tier,
				CurrentLevel:  curLevel,
				RuntimeStatus: runtimeStatus,
				Manifest:      m,
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
		cmd := exec.Command("systemctl", "--user", "start", req.Module)
		if err := cmd.Run(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: fmt.Sprintf("Errore avvio servizio %s: %v", req.Module, err)})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: fmt.Sprintf("Modulo %s avviato con successo", req.Module)})
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
