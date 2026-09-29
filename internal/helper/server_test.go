package helper

import (
	"encoding/json"
	"fmt"
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
	if res.Ok {
		t.Fatalf("expected shares.apply for system binary directory /usr/local/bin to be rejected, but it succeeded")
	}
}

func TestIsAllowedPathSecurity(t *testing.T) {
	allowed := []string{
		"/mnt/allod-storage",
		"/mnt/allod-storage/shares",
		"/mnt/allod-storage/photos/upload",
		"/data",
		"/data/shares",
	}
	for _, p := range allowed {
		if !isAllowedPath(p) {
			t.Errorf("expected isAllowedPath(%q) to be true, got false", p)
		}
	}

	disallowed := []string{
		"/usr/local/bin",
		"/bin",
		"/usr/bin",
		"/etc",
		"/tmp",
		"/mnt",
		"/mnt/other",
		"/mnt/allod-storage-fake",
		"/mnt/allod-storage/../etc",
	}
	for _, p := range disallowed {
		if isAllowedPath(p) {
			t.Errorf("expected isAllowedPath(%q) to be false, got true", p)
		}
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
   guest ok = yes
   valid users = bob
   create mask = 0660
   directory mask = 0770
`

	patched := PatchSambaConfig(sampleConf)

	// Verify global section has access based share enum and netbios name
	if !strings.Contains(patched, "access based share enum = yes") {
		t.Errorf("expected access based share enum = yes in patched config")
	}
	if !strings.Contains(patched, "netbios name = ALLOD") {
		t.Errorf("expected netbios name = ALLOD in patched config")
	}

	// Verify alice and bob have access based share enum and hide unreadable
	if !strings.Contains(patched, "valid users = alice") {
		t.Errorf("expected valid users = alice to be preserved")
	}
	if !strings.Contains(patched, "hide unreadable = yes") {
		t.Errorf("expected hide unreadable = yes to be added to user shares and shares")
	}

	// Verify [shares] is browseable = no (hidden from root network view) and disallows guest
	if !strings.Contains(patched, "browseable = no") {
		t.Errorf("expected browseable = no for [shares] in patched config")
	}

	// Verify [public] is NOT automatically injected when not present
	if strings.Contains(patched, "[public]") {
		t.Errorf("expected [public] share to NOT be auto-injected into patched config")
	}

	// Verify all data shares with valid users have guest ok = no (bob's guest ok = yes overridden)
	if strings.Contains(patched, "guest ok = yes") {
		t.Errorf("expected no guest ok = yes in patched data shares, got:\n%s", patched)
	}

	// Idempotency: patching already patched config shouldn't duplicate lines
	patchedTwice := PatchSambaConfig(patched)
	countGlobalEnum := strings.Count(patchedTwice, "access based share enum = yes")
	// 1 in [global] + 1 in [alice] + 1 in [bob] = 3
	if countGlobalEnum != 3 {
		t.Errorf("expected exactly 3 occurrences of access based share enum = yes, got %d", countGlobalEnum)
	}
}

func TestComputeNTLMHash(t *testing.T) {
	tests := []struct {
		password string
		expected string
	}{
		{"", "31D6CFE0D16AE931B73C59D7E0C089C0"},
		{"password", "8846F7EAEE8FB117AD06BDD830B7586C"},
		{"admin", "209C6174DA490CAEB422F3FA5A7AE634"},
	}

	for _, tc := range tests {
		hash := ComputeNTLMHash(tc.password)
		if hash != tc.expected {
			t.Errorf("ComputeNTLMHash(%q) = %s; want %s", tc.password, hash, tc.expected)
		}
	}
}

func TestZeroConfigStatusPlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "network.zeroconfig_status",
		Plan:   true,
		Args:   map[string]interface{}{},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected zeroconfig_status to succeed: %s", res.Error)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(res.Output), &data); err != nil {
		t.Fatalf("failed to parse output json: %v", err)
	}
	if data["mdns_name"] != "allod.local" {
		t.Errorf("expected mdns_name 'allod.local', got %v", data["mdns_name"])
	}
	if data["netbios_name"] != "ALLOD" {
		t.Errorf("expected netbios_name 'ALLOD', got %v", data["netbios_name"])
	}
}

func TestZeroConfigSetupPlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "network.zeroconfig_setup",
		Plan:   true,
		Args: map[string]interface{}{
			"hostname": "allod",
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected zeroconfig_setup plan to succeed: %s", res.Error)
	}
	if len(res.Plan) < 5 {
		t.Errorf("expected at least 5 plan steps, got %d", len(res.Plan))
	}
}

func TestPatchAvahiDaemonConfig(t *testing.T) {
	conf := "[server]\nuse-ipv4=yes\n"
	patched := PatchAvahiDaemonConfig(conf)
	if !strings.Contains(patched, "enable-reflector=yes") {
		t.Errorf("expected enable-reflector=yes to be added")
	}
	if !strings.Contains(patched, "host-name=allod") {
		t.Errorf("expected host-name=allod to be added")
	}

	// Custom hostname
	patchedCustom := PatchAvahiDaemonConfig(conf, "mynas")
	if !strings.Contains(patchedCustom, "host-name=mynas") {
		t.Errorf("expected host-name=mynas, got %s", patchedCustom)
	}

	// Idempotency
	patchedTwice := PatchAvahiDaemonConfig(patched)
	if strings.Count(patchedTwice, "enable-reflector=yes") != 1 {
		t.Errorf("expected exactly 1 enable-reflector=yes, got %d", strings.Count(patchedTwice, "enable-reflector=yes"))
	}
	if strings.Count(patchedTwice, "host-name=allod") != 1 {
		t.Errorf("expected exactly 1 host-name=allod, got %d", strings.Count(patchedTwice, "host-name=allod"))
	}
}

func TestFindImmichPhotosSource(t *testing.T) {
	tmpDir := t.TempDir()
	basePhotos := filepath.Join(tmpDir, "photos", "upload")

	// 1. Without any files, returns fallback
	res := findImmichPhotosSource(basePhotos, "davide")
	expectedFallback := filepath.Join(basePhotos, "library", "davide")
	if res != expectedFallback {
		t.Errorf("expected fallback %s, got %s", expectedFallback, res)
	}

	// 2. With UUID folder in upload/ containing photos
	uuid := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	userUploadDir := filepath.Join(basePhotos, "upload", uuid)
	_ = os.MkdirAll(userUploadDir, 0777)
	_ = os.WriteFile(filepath.Join(userUploadDir, "photo1.jpg"), []byte("fake jpg"), 0644)

	resWithUpload := findImmichPhotosSource(basePhotos, "davide")
	if resWithUpload != userUploadDir {
		t.Errorf("expected upload dir %s, got %s", userUploadDir, resWithUpload)
	}

	// 3. With Storage Template library/ containing photos, library is preferred over upload
	userLibraryDir := filepath.Join(basePhotos, "library", uuid, "2026", "09")
	_ = os.MkdirAll(userLibraryDir, 0777)
	_ = os.WriteFile(filepath.Join(userLibraryDir, "photo2.jpg"), []byte("fake jpg 2"), 0644)

	resWithLibrary := findImmichPhotosSource(basePhotos, "davide")
	expectedLibRoot := filepath.Join(basePhotos, "library", uuid)
	if resWithLibrary != expectedLibRoot {
		t.Errorf("expected library dir %s, got %s", expectedLibRoot, resWithLibrary)
	}
}

func TestCountMediaFiles(t *testing.T) {
	tmpDir := t.TempDir()

	if cnt := countMediaFiles(tmpDir); cnt != 0 {
		t.Errorf("expected 0 for empty dir, got %d", cnt)
	}

	// Non-media files
	_ = os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("text"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "server.log"), []byte("log"), 0644)
	if cnt := countMediaFiles(tmpDir); cnt != 0 {
		t.Errorf("expected 0 for non-media files, got %d", cnt)
	}

	// Media files
	_ = os.WriteFile(filepath.Join(tmpDir, "photo.jpg"), []byte("jpg"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "video.mp4"), []byte("mp4"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "apple.HEIC"), []byte("heic"), 0644)
	if cnt := countMediaFiles(tmpDir); cnt != 3 {
		t.Errorf("expected 3 media files, got %d", cnt)
	}

	// Thumbs and encoded-video subdirectories should be skipped
	thumbsDir := filepath.Join(tmpDir, "thumbs")
	_ = os.MkdirAll(thumbsDir, 0755)
	_ = os.WriteFile(filepath.Join(thumbsDir, "thumb.jpg"), []byte("thumb"), 0644)

	profileDir := filepath.Join(tmpDir, "profile")
	_ = os.MkdirAll(profileDir, 0755)
	_ = os.WriteFile(filepath.Join(profileDir, "avatar.png"), []byte("avatar"), 0644)

	if cnt := countMediaFiles(tmpDir); cnt != 3 {
		t.Errorf("expected still 3 media files after adding thumbs and profile, got %d", cnt)
	}

	// Nested user subfolder with media should be counted
	subDir := filepath.Join(tmpDir, "2026", "09")
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "pic.png"), []byte("png"), 0644)

	if cnt := countMediaFiles(tmpDir); cnt != 4 {
		t.Errorf("expected 4 media files, got %d", cnt)
	}
}

func TestIsValidUUID(t *testing.T) {
	valid := []string{
		"123e4567-e89b-12d3-a456-426614174000",
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"00000000-0000-0000-0000-000000000000",
		"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	}
	for _, u := range valid {
		if !isValidUUID(u) {
			t.Errorf("expected valid UUID for %q, got false", u)
		}
	}

	invalid := []string{
		"",
		"not-a-uuid",
		"123e4567-e89b-12d3-a456-42661417400",   // too short
		"123e4567-e89b-12d3-a456-4266141740000", // too long
		"123e4567_e89b_12d3_a456_426614174000",  // underscores
		"../../../../etc/passwd",
		"123e4567-e89b-12d3-a456-42661417400g", // non-hex 'g'
		"'; drop table users; --",
	}
	for _, u := range invalid {
		if isValidUUID(u) {
			t.Errorf("expected invalid UUID for %q, got true", u)
		}
	}
}

func TestParseImmichUserUUID(t *testing.T) {
	steveUUID := "11111111-1111-1111-1111-111111111111"
	eveUUID := "22222222-2222-2222-2222-222222222222"

	// 1. Substring collision: 'eve' must NEVER match 'steve@example.com'
	steveOnlyDB := fmt.Sprintf("%s|steve@example.com|Steve\n", steveUUID)
	if res := parseImmichUserUUID(steveOnlyDB, "eve"); res != "" {
		t.Errorf("SECURITY: 'eve' matched 'steve@example.com', got UUID %s", res)
	}

	// 2. Exact match on email local-part or name
	multiUserDB := fmt.Sprintf("%s|steve@example.com|Steve\n%s|eve@example.com|Eve\n", steveUUID, eveUUID)
	if res := parseImmichUserUUID(multiUserDB, "eve"); res != eveUUID {
		t.Errorf("expected exact match for 'eve' to return %s, got %s", eveUUID, res)
	}
	if res := parseImmichUserUUID(multiUserDB, "steve"); res != steveUUID {
		t.Errorf("expected exact match for 'steve' to return %s, got %s", steveUUID, res)
	}

	// 3. Exact match by full email
	if res := parseImmichUserUUID(multiUserDB, "eve@example.com"); res != eveUUID {
		t.Errorf("expected match for 'eve@example.com' to return %s, got %s", eveUUID, res)
	}

	// 4. Exact match by UUID directly
	if res := parseImmichUserUUID(multiUserDB, steveUUID); res != steveUUID {
		t.Errorf("expected match for steveUUID to return %s, got %s", steveUUID, res)
	}

	// 5. Ambiguous match (>1 candidates) must be rejected
	ambiguousDB := fmt.Sprintf(
		"11111111-1111-1111-1111-111111111111|mario@example.com|Mario A\n" +
			"22222222-2222-2222-2222-222222222222|mario@example.org|Mario B\n",
	)
	if res := parseImmichUserUUID(ambiguousDB, "mario"); res != "" {
		t.Errorf("SECURITY: ambiguous user 'mario' returned UUID %s instead of rejecting", res)
	}

	// 6. Malformed UUID in DB must be rejected
	malformedDB := "bad-uuid|mario@example.com|Mario\n"
	if res := parseImmichUserUUID(malformedDB, "mario"); res != "" {
		t.Errorf("expected malformed UUID to be rejected, got %s", res)
	}

	// 7. Path traversal in UUID in DB must be rejected
	traversalDB := "../../../etc/passwd|mario@example.com|Mario\n"
	if res := parseImmichUserUUID(traversalDB, "mario"); res != "" {
		t.Errorf("expected traversal UUID to be rejected, got %s", res)
	}

	// 8. Empty username must return empty string
	if res := parseImmichUserUUID(multiUserDB, ""); res != "" {
		t.Errorf("expected empty username to return empty string, got %s", res)
	}

	// 9. Single user in DB must NOT be returned when searching for a different non-matching user
	singleUserDB := fmt.Sprintf("%s|admin@example.org|Administrator\n", steveUUID)
	if res := parseImmichUserUUID(singleUserDB, "luigi"); res != "" {
		t.Errorf("SECURITY: non-matching user 'luigi' fell back to single DB user %s", res)
	}
}

func TestFindImmichPhotosSourceNoDangerousFallback(t *testing.T) {
	tmpDir := t.TempDir()
	basePhotos := filepath.Join(tmpDir, "photos", "upload")

	steveUUID := "11111111-1111-1111-1111-111111111111"
	eveUUID := "22222222-2222-2222-2222-222222222222"

	// Steve has 10 photos in upload/<steveUUID>
	steveUploadDir := filepath.Join(basePhotos, "upload", steveUUID)
	_ = os.MkdirAll(steveUploadDir, 0755)
	for i := 0; i < 10; i++ {
		_ = os.WriteFile(filepath.Join(steveUploadDir, fmt.Sprintf("photo_%d.jpg", i)), []byte("photo"), 0644)
	}

	// Eve has 0 photos in upload/<eveUUID>
	eveUploadDir := filepath.Join(basePhotos, "upload", eveUUID)
	_ = os.MkdirAll(eveUploadDir, 0755)

	// Since there are 2 UUID folders and no DB running, 'eve' must NEVER get Steve's folder
	resEve := findImmichPhotosSource(basePhotos, "eve")
	if strings.Contains(resEve, steveUUID) {
		t.Fatalf("SECURITY: findImmichPhotosSource for 'eve' fell back to Steve's folder with more photos: %s", resEve)
	}

	// An unknown user must also NEVER get Steve's folder
	resUnknown := findImmichPhotosSource(basePhotos, "unknown")
	if strings.Contains(resUnknown, steveUUID) {
		t.Fatalf("SECURITY: findImmichPhotosSource for 'unknown' fell back to Steve's folder: %s", resUnknown)
	}
}

func TestIsAuthorizedCallerOnlyAllodGroup(t *testing.T) {
	allodGid := "1002"
	sudoGid := "27"
	wheelGid := "10"
	adminGid := "1001"

	// 1. Caller only in 'allod' group -> authorized
	if !isAuthorizedCaller([]string{allodGid}, allodGid) {
		t.Errorf("expected caller in 'allod' group to be authorized")
	}

	// 2. Caller in 'allod' and other groups -> authorized
	if !isAuthorizedCaller([]string{"1000", sudoGid, allodGid}, allodGid) {
		t.Errorf("expected caller in 'allod' plus other groups to be authorized")
	}

	// 3. Caller in 'sudo' only -> must be REJECTED
	if isAuthorizedCaller([]string{sudoGid}, allodGid) {
		t.Errorf("SECURITY: caller with only 'sudo' group was authorized to call root helper")
	}

	// 4. Caller in 'wheel' and 'admin' only -> must be REJECTED
	if isAuthorizedCaller([]string{wheelGid, adminGid}, allodGid) {
		t.Errorf("SECURITY: caller with 'wheel'/'admin' groups was authorized to call root helper")
	}

	// 5. Empty allodGid -> must be REJECTED
	if isAuthorizedCaller([]string{sudoGid}, "") {
		t.Errorf("expected rejection when allodGid is empty")
	}
}
