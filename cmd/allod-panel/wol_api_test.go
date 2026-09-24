package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/asfaltobollente/allod/internal/state"
	"github.com/asfaltobollente/allod/internal/wol"
)

func TestWoLAPIRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(originalWd) }()

	st, err := state.Open("state.db")
	if err != nil {
		t.Fatalf("failed to open state.db: %v", err)
	}
	defer st.Close()

	mux := http.NewServeMux()

	// Register WoL handlers matching main.go
	mux.HandleFunc("/api/wol/devices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s, err := state.Open("state.db")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer s.Close()

		switch r.Method {
		case http.MethodGet:
			devs, _ := s.ListWoLDevices()
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: devs})
		case http.MethodPost:
			var dev state.WoLDevice
			_ = json.NewDecoder(r.Body).Decode(&dev)
			hw, err := wol.ParseMAC(dev.MACAddress)
			if err != nil {
				json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
				return
			}
			dev.MACAddress = hw.String()
			_ = s.SaveWoLDevice(&dev)
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Data: dev})
		case http.MethodDelete:
			id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
			_ = s.DeleteWoLDevice(id)
			json.NewEncoder(w).Encode(PanelResponse{Status: "ok"})
		}
	})

	mux.HandleFunc("/api/wol/wake", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ID          int64  `json:"id"`
			MACAddress  string `json:"mac_address"`
			BroadcastIP string `json:"broadcast_ip"`
			Port        int    `json:"port"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		s, _ := state.Open("state.db")
		defer s.Close()

		targetMAC := req.MACAddress
		if req.ID > 0 {
			dev, _ := s.GetWoLDevice(req.ID)
			if dev != nil {
				targetMAC = dev.MACAddress
			}
		}

		res, err := wol.Send(targetMAC, "127.0.0.1", 9999)
		if err != nil {
			json.NewEncoder(w).Encode(PanelResponse{Status: "error", Message: err.Error()})
			return
		}
		if req.ID > 0 {
			_ = s.RecordWoLWake(req.ID)
		}
		json.NewEncoder(w).Encode(PanelResponse{Status: "ok", Message: "Magic packet sent", Data: res})
	})

	// 1. GET /api/wol/devices (empty initially)
	req := httptest.NewRequest("GET", "/api/wol/devices", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// 2. POST /api/wol/devices (create device)
	createBody := `{"name":"PC Studio","mac_address":"00:D8:61:33:0E:1F"}`
	reqCreate := httptest.NewRequest("POST", "/api/wol/devices", strings.NewReader(createBody))
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)

	var createResp struct {
		Status string          `json:"status"`
		Data   state.WoLDevice `json:"data"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &createResp)
	if createResp.Status != "ok" || createResp.Data.ID <= 0 {
		t.Fatalf("failed to create device via API: %+v", createResp)
	}

	// 3. POST /api/wol/wake with device ID
	wakeBody := `{"id":` + strconv.FormatInt(createResp.Data.ID, 10) + `}`
	reqWake := httptest.NewRequest("POST", "/api/wol/wake", strings.NewReader(wakeBody))
	recWake := httptest.NewRecorder()
	mux.ServeHTTP(recWake, reqWake)

	var wakeResp PanelResponse
	_ = json.Unmarshal(recWake.Body.Bytes(), &wakeResp)
	if wakeResp.Status != "ok" {
		t.Errorf("expected wake status ok, got: %+v", wakeResp)
	}

	// 4. Verify wake recorded in DB
	updatedDev, _ := st.GetWoLDevice(createResp.Data.ID)
	if updatedDev.LastWakeAt == nil {
		t.Errorf("expected LastWakeAt to be set after wake API call")
	}

	// 5. DELETE /api/wol/devices?id=...
	reqDel := httptest.NewRequest("DELETE", "/api/wol/devices?id="+strconv.FormatInt(createResp.Data.ID, 10), nil)
	recDel := httptest.NewRecorder()
	mux.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Errorf("expected delete status 200, got %d", recDel.Code)
	}
}
