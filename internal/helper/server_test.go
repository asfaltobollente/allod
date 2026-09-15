package helper

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestHelperAllowedAction(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args: map[string]interface{}{
			"name": "documents",
			"path": "/data/documents",
		},
	}

	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected action to succeed, got error: %s", res.Error)
	}
	if len(res.Plan) != 3 {
		t.Errorf("expected 3 plan steps, got %d", len(res.Plan))
	}
}

func TestHelperDisallowedAction(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "dangerous.rmrf",
		Plan:   false,
		Args:   map[string]interface{}{},
	}

	res := s.processRequest(req)
	if res.Ok {
		t.Errorf("expected disallowed action to be rejected")
	}
}

func TestHelperPathTraversalRejected(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args: map[string]interface{}{
			"name": "attack",
			"path": "/data/../../etc/shadow",
		},
	}

	res := s.processRequest(req)
	if res.Ok {
		t.Errorf("expected path traversal to be rejected")
	}
}

func TestHelperSharesBindPhotosPlan(t *testing.T) {
	s := &Server{}
	// Global bind
	req := Request{
		Action: "shares.bind_photos",
		Plan:   true,
		Args: map[string]interface{}{
			"enabled": true,
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected shares.bind_photos to succeed, got: %s", res.Error)
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected at least 1 plan item for bind_photos, got 0")
	}

	// Per-user bind
	reqUser := Request{
		Action: "shares.bind_photos",
		Plan:   true,
		Args: map[string]interface{}{
			"enabled":  true,
			"username": "mario",
		},
	}
	resUser := s.processRequest(reqUser)
	if !resUser.Ok {
		t.Fatalf("expected per-user bind_photos to succeed, got: %s", resUser.Error)
	}
	if len(resUser.Plan) != 1 {
		t.Errorf("expected exactly 1 plan item for user mario, got %d", len(resUser.Plan))
	}
}

func TestHelperSharesSetPasswordPlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.set_password",
		Plan:   true,
		Args: map[string]interface{}{
			"username": "mario",
			"password": "supersecretpassword",
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected shares.set_password to succeed, got: %s", res.Error)
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected plan items, got 0")
	}
}

func TestHelperUsersCreatePlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "users.create",
		Plan:   true,
		Args: map[string]interface{}{
			"username": "mario",
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected users.create to succeed, got: %s", res.Error)
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected plan items for users.create, got 0")
	}
}

func TestEnsureLinuxUserValidation(t *testing.T) {
	if err := ensureLinuxUser("invalid;rm -rf /"); err == nil {
		t.Errorf("expected error on command injection in username, got nil")
	}
	if err := ensureLinuxUser(""); err == nil {
		t.Errorf("expected error on empty username, got nil")
	}
}

func TestHelperSharesApplySystemBinProtection(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args: map[string]interface{}{
			"name": "shares",
			"path": "/usr/local/bin",
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected shares.apply to succeed, got error: %s", res.Error)
	}
}

type mockCredChecker struct {
	uid     uint32
	allowed bool
	err     error
}

func (m mockCredChecker) CheckPeer(conn net.Conn) (uint32, bool, error) {
	return m.uid, m.allowed, m.err
}

func TestHelperSocketPermissions(t *testing.T) {
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "test-helper.sock")

	s := &Server{
		SocketPath: sockPath,
	}

	go func() {
		_ = s.Start()
	}()

	var fi os.FileInfo
	var err error
	for i := 0; i < 25; i++ {
		fi, err = os.Stat(sockPath)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("socket was not created: %v", err)
	}

	if runtime.GOOS != "windows" {
		perm := fi.Mode().Perm()
		if perm != 0660 {
			t.Errorf("expected socket permissions 0660, got %04o", perm)
		}
	}
}

func TestHelperCallerUIDUnauthorized(t *testing.T) {
	s := &Server{
		CredChecker: mockCredChecker{uid: 1001, allowed: false},
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go s.handle(serverConn)

	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args:   map[string]interface{}{"name": "shares", "path": "/data"},
	}
	if err := json.NewEncoder(clientConn).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	var res Response
	if err := json.NewDecoder(clientConn).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Ok {
		t.Errorf("expected unauthorized caller to be rejected, got Ok=true")
	}
	if res.Error != "caller not in group allod" {
		t.Errorf("expected error 'caller not in group allod', got %q", res.Error)
	}
}

func TestHelperCallerUIDAuthorized(t *testing.T) {
	s := &Server{
		CredChecker: mockCredChecker{uid: 1000, allowed: true},
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go s.handle(serverConn)

	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args:   map[string]interface{}{"name": "shares", "path": "/data"},
	}
	if err := json.NewEncoder(clientConn).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	var res Response
	if err := json.NewDecoder(clientConn).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !res.Ok {
		t.Errorf("expected authorized caller to succeed, got error: %s", res.Error)
	}
}
