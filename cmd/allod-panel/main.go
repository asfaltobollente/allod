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
	"path/filepath"
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
	ID           string             `json:"id"`
	Tier         string             `json:"tier"`
	CurrentLevel string             `json:"current_level"`
	Manifest     *manifest.Manifest `json:"manifest"`
}

func getConfigPath() string {
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return "configs/config.example.yaml"
}

func getRingPath() string {
	if _, err := os.Stat("ring.yaml"); err == nil {
		return "ring.yaml"
	}
	return "configs/ring.example.yaml"
}

const dbPath = "state.db"

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

		// Test helper socket connectivity
		helperConnected := checkHelperConnectivity()

		data := map[string]interface{}{
			"node_name":          cfg.Node.Name,
			"channel":            cfg.Node.Channel,
			"ram_total_mb":       preflight.GetSystemRAMMB(),
			"core_reserved_mb":   preflight.CoreReservedMB,
			"ram_used_mb":        usedMB,
			"helper_connected":   helperConnected,
			"group_repo":         cfg.Node.Group,
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

			modules = append(modules, ModuleInfo{
				ID:           modName,
				Tier:         m.Tier,
				CurrentLevel: curLevel,
				Manifest:     m,
			})
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: modules})
	})

	// 4. API Modules Set Level
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

	// 5. API Ring Status
	mux.HandleFunc("/api/ring", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		topo, err := ring.LoadTopology(getRingPath())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		placements := topo.CalculatePlacement()

		data := map[string]interface{}{
			"name":            topo.Name,
			"target_replicas": topo.TargetReplicas,
			"members":         topo.Members,
			"placements":      placements,
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: data})
	})

	// 6. API Ring Simulate Removal
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

		topo, err := ring.LoadTopology(getRingPath())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		impact, err := topo.SimulateRemoval(req.Member)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: impact})
	})

	// 7. API Update Rollback Simulation
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

	// 8. API Test Helper
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
	conn, err := net.DialTimeout("tcp", "127.0.0.1:40000", 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	conn, err = net.DialTimeout("unix", "allod-helper.sock", 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
