package helper

import (
	"encoding/json"
	"net"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCommandRunnerInterface(t *testing.T) {
	var _ CommandRunner = (*Client)(nil)
}

func TestCommandRunnerMethods(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping unix domain socket test on windows")
	}

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test-runner.sock")

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}
	defer l.Close()

	// Mock server loop responding to requests
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			var req Request
			if err := json.NewDecoder(conn).Decode(&req); err == nil {
				resp := Response{Ok: true, Applied: true}
				if req.Action == "storage.diagnostics" {
					resp.Output = "mock diagnostics output"
				}
				_ = json.NewEncoder(conn).Encode(resp)
			}
			conn.Close()
		}
	}()

	client := &Client{SocketPath: sockPath}

	if err := client.CreateUser("testuser"); err != nil {
		t.Errorf("CreateUser failed: %v", err)
	}
	if err := client.DeleteUser("testuser"); err != nil {
		t.Errorf("DeleteUser failed: %v", err)
	}
	if err := client.SetSambaPassword("testuser", "secret"); err != nil {
		t.Errorf("SetSambaPassword failed: %v", err)
	}
	if err := client.AddSambaUser("testuser"); err != nil {
		t.Errorf("AddSambaUser failed: %v", err)
	}
	if err := client.DeleteSambaUser("testuser"); err != nil {
		t.Errorf("DeleteSambaUser failed: %v", err)
	}
	if err := client.ApplyShare("docs", "/data/docs"); err != nil {
		t.Errorf("ApplyShare failed: %v", err)
	}
	if err := client.BindPhotos("testuser", false); err != nil {
		t.Errorf("BindPhotos failed: %v", err)
	}
	if err := client.RestartService("allod-helperd"); err != nil {
		t.Errorf("RestartService failed: %v", err)
	}
	if err := client.InitStorage("single", []string{"/dev/sdb"}, "/mnt/storage", "allod"); err != nil {
		t.Errorf("InitStorage failed: %v", err)
	}
	out, err := client.StorageDiagnostics("/mnt/storage")
	if err != nil || out != "mock diagnostics output" {
		t.Errorf("StorageDiagnostics failed: out=%q, err=%v", out, err)
	}
	if err := client.SnapshotCreate("photos", "snap1"); err != nil {
		t.Errorf("SnapshotCreate failed: %v", err)
	}
}
