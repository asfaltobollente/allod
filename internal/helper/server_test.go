package helper

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		if perm != 0666 {
			t.Errorf("expected socket permissions 0666, got %04o", perm)
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

func TestAllowedActionsSchemaMatch(t *testing.T) {
	schemaPaths := []string{
		"../../schemas/helper-api.schema.json",
		"schemas/helper-api.schema.json",
	}
	var schemaBytes []byte
	var err error
	for _, p := range schemaPaths {
		schemaBytes, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("failed to read helper-api.schema.json: %v", err)
	}

	var schema struct {
		Properties struct {
			Request struct {
				Properties struct {
					Action struct {
						Enum []string `json:"enum"`
					} `json:"action"`
				} `json:"properties"`
			} `json:"request"`
		} `json:"properties"`
	}

	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("failed to parse helper-api.schema.json: %v", err)
	}

	schemaActions := schema.Properties.Request.Properties.Action.Enum
	if len(schemaActions) != len(AllowedActions) {
		t.Fatalf("mismatch in actions count: schema has %d, AllowedActions has %d",
			len(schemaActions), len(AllowedActions))
	}

	schemaSet := make(map[string]bool)
	for _, a := range schemaActions {
		schemaSet[a] = true
	}

	for _, a := range AllowedActions {
		if !schemaSet[a] {
			t.Errorf("AllowedAction %q not found in schema enum", a)
		}
	}
}

func TestInvalidArgumentRejection(t *testing.T) {
	s := &Server{}

	tests := []struct {
		name   string
		action string
		args   map[string]interface{}
	}{
		{
			name:   "shares.apply invalid name",
			action: "shares.apply",
			args:   map[string]interface{}{"name": "../bad_name"},
		},
		{
			name:   "shares.apply invalid path outside storage",
			action: "shares.apply",
			args:   map[string]interface{}{"path": "/etc/shadow"},
		},
		{
			name:   "shares.bind_photos invalid username",
			action: "shares.bind_photos",
			args:   map[string]interface{}{"username": "bad;whoami"},
		},
		{
			name:   "shares.set_password invalid username",
			action: "shares.set_password",
			args:   map[string]interface{}{"username": "user/../root", "password": "validpassword"},
		},
		{
			name:   "shares.set_password empty password",
			action: "shares.set_password",
			args:   map[string]interface{}{"username": "validuser", "password": ""},
		},
		{
			name:   "users.create invalid username",
			action: "users.create",
			args:   map[string]interface{}{"username": "user;rm -rf"},
		},
		{
			name:   "users.passwd empty password",
			action: "users.passwd",
			args:   map[string]interface{}{"username": "validuser", "password": ""},
		},
		{
			name:   "snapshots.create invalid subvolume",
			action: "snapshots.create",
			args:   map[string]interface{}{"subvolume": "sub/../escaped"},
		},
		{
			name:   "smart.read invalid disk",
			action: "smart.read",
			args:   map[string]interface{}{"disk": "sda;reboot"},
		},
		{
			name:   "service.restart disallowed unit",
			action: "service.restart",
			args:   map[string]interface{}{"unit": "sshd"},
		},
		{
			name:   "service.restart malicious unit",
			action: "service.restart",
			args:   map[string]interface{}{"unit": "allod-panel;rm -rf /"},
		},
		{
			name:   "storage.init empty disks",
			action: "storage.init",
			args:   map[string]interface{}{"disks": []interface{}{}},
		},
		{
			name:   "storage.init invalid disk",
			action: "storage.init",
			args:   map[string]interface{}{"disks": []interface{}{"sda;echo pwned"}},
		},
		{
			name:   "storage.init invalid mode",
			action: "storage.init",
			args:   map[string]interface{}{"disks": []interface{}{"sda"}, "mode": "raid0"},
		},
		{
			name:   "storage.init invalid mount",
			action: "storage.init",
			args:   map[string]interface{}{"disks": []interface{}{"sda"}, "mount": "/etc"},
		},
		{
			name:   "storage.diagnostics invalid mount",
			action: "storage.diagnostics",
			args:   map[string]interface{}{"mount": "/etc/shadow"},
		},
		{
			name:   "network.headscale_cli retired action",
			action: "network.headscale_cli",
			args:   map[string]interface{}{"command": "delete_all"},
		},
		{
			name:   "completely disallowed action",
			action: "system.shutdown",
			args:   map[string]interface{}{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := s.processRequest(Request{
				Action: tc.action,
				Plan:   true,
				Args:   tc.args,
			})
			if res.Ok {
				t.Errorf("expected %s to be rejected, but succeeded with plan: %v", tc.name, res.Plan)
			}
			if res.Error == "" {
				t.Errorf("expected non-empty error for %s", tc.name)
			}
		})
	}
}

func TestHelperNetBirdPlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "network.netbird_status",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected network.netbird_status plan to succeed, got error: %s", res.Error)
	}
	if len(res.Plan) != 1 {
		t.Errorf("expected 1 plan item, got %d", len(res.Plan))
	}

	reqUp := Request{
		Action: "network.netbird_up",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	resUp := s.processRequest(reqUp)
	if !resUp.Ok {
		t.Fatalf("expected network.netbird_up plan to succeed, got error: %s", resUp.Error)
	}

	reqDown := Request{
		Action: "network.netbird_down",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	resDown := s.processRequest(reqDown)
	if !resDown.Ok {
		t.Fatalf("expected network.netbird_down plan to succeed, got error: %s", resDown.Error)
	}

	reqNative := Request{
		Action: "network.install_native",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	resNative := s.processRequest(reqNative)
	if !resNative.Ok {
		t.Fatalf("expected network.install_native plan to succeed, got error: %s", resNative.Error)
	}

	reqSrvUp := Request{
		Action: "network.server_up",
		Plan:   true,
		Args: map[string]interface{}{
			"domain":    "mesh.example.com",
			"port":      33073,
			"dash_port": 8088,
		},
	}
	resSrvUp := s.processRequest(reqSrvUp)
	if !resSrvUp.Ok {
		t.Fatalf("expected network.server_up plan to succeed, got error: %s", resSrvUp.Error)
	}
	if len(resSrvUp.Plan) != 2 {
		t.Errorf("expected 2 plan items (server + dashboard), got %d", len(resSrvUp.Plan))
	}

	reqSrvDown := Request{
		Action: "network.server_down",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	resSrvDown := s.processRequest(reqSrvDown)
	if !resSrvDown.Ok {
		t.Fatalf("expected network.server_down plan to succeed, got error: %s", resSrvDown.Error)
	}

	reqSrvStatus := Request{
		Action: "network.server_status",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	resSrvStatus := s.processRequest(reqSrvStatus)
	if !resSrvStatus.Ok {
		t.Fatalf("expected network.server_status plan to succeed, got error: %s", resSrvStatus.Error)
	}
}

func TestPatchSambaConfig(t *testing.T) {
	sampleConf := `[global]
   workgroup = WORKGROUP
   server string = %h server

[shares]
   path = /mnt/allod-storage/shares
   browseable = yes
   read only = no
   guest ok = yes

[alice]
   comment = Cartella privata di alice
   path = /mnt/allod-storage/shares/alice
   browseable = yes
   read only = no
   guest ok = no
   valid users = alice
   create mask = 0660
   directory mask = 0770

[bob]
   comment = Cartella privata di bob
   path = /mnt/allod-storage/shares/bob
   browseable = yes
   read only = no
   guest ok = no
   valid users = bob
   create mask = 0660
   directory mask = 0770
`

	patched := PatchSambaConfig(sampleConf)

	// Verify global section has access based share enum
	if !strings.Contains(patched, "access based share enum = yes") {
		t.Errorf("expected access based share enum = yes in patched config")
	}

	// Verify alice and bob have access based share enum and hide unreadable
	if !strings.Contains(patched, "valid users = alice") {
		t.Errorf("expected valid users = alice to be preserved")
	}
	if !strings.Contains(patched, "hide unreadable = yes") {
		t.Errorf("expected hide unreadable = yes to be added to user shares and shares")
	}

	// Idempotency: patching already patched config shouldn't duplicate lines
	patchedTwice := PatchSambaConfig(patched)
	countGlobalEnum := strings.Count(patchedTwice, "access based share enum = yes")
	// 1 in [global] + 1 in [alice] + 1 in [bob] = 3
	if countGlobalEnum != 3 {
		t.Errorf("expected exactly 3 occurrences of access based share enum = yes, got %d", countGlobalEnum)
	}
}

