package helper

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// AllowedActions is the single source of truth for the 16 actions supported by the root helper.
var AllowedActions = []string{
	"shares.apply",
	"shares.bind_photos",
	"shares.set_password",
	"users.create",
	"users.passwd",
	"firewall.apply",
	"snapshots.create",
	"snapshots.prune",
	"smart.read",
	"service.restart",
	"storage.init",
	"storage.diagnostics",
	"network.netbird_status",
	"network.netbird_cli",
	"network.netbird_up",
	"network.netbird_down",
	"network.install_native",
	"network.server_up",
	"network.server_down",
	"network.server_status",
}

// AllowedServiceUnits defines systemd service units allowed to be restarted via service.restart.
var AllowedServiceUnits = []string{
	"allod-helperd",
	"allod-panel",
	"smbd",
	"smb",
	"network",
	"network-netbird",
	"netbird",
	"cloud",
	"cloud-postgres",
	"photos",
	"photos-postgres",
	"photos-valkey",
	"media",
	"backup",
	"storage",
	"nftables",
}

func isAllowedAction(action string) bool {
	for _, a := range AllowedActions {
		if a == action {
			return true
		}
	}
	return false
}

func isAllowedServiceUnit(unit string) bool {
	clean := strings.TrimSuffix(unit, ".service")
	for _, u := range AllowedServiceUnits {
		if u == unit || u == clean {
			return true
		}
	}
	return false
}

func isAllowedPath(p string) bool {
	if p == "" {
		return true
	}
	if strings.Contains(p, "..") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(p))
	if strings.HasPrefix(clean, "/mnt/allod-storage") ||
		strings.HasPrefix(clean, "/mnt") ||
		strings.HasPrefix(clean, "/data") ||
		clean == "/usr/local/bin" {
		return true
	}
	if envStorage := os.Getenv("ALLOD_STORAGE_DIR"); envStorage != "" && strings.HasPrefix(clean, filepath.ToSlash(filepath.Clean(envStorage))) {
		return true
	}
	return false
}

func hashArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return "none"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "err"
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}

var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var validUnitRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)

func validDeviceRegex(dev string) bool {
	dev = strings.TrimSpace(dev)
	dev = strings.TrimPrefix(dev, "/dev/")
	dev = strings.TrimPrefix(dev, "disk/by-id/")
	return validNameRegex.MatchString(dev)
}

func resolveExecutable(preferred string, fallbacks ...string) string {
	if p, err := exec.LookPath(preferred); err == nil {
		return p
	}
	for _, fb := range fallbacks {
		if _, err := os.Stat(fb); err == nil {
			return fb
		}
	}
	return preferred
}

// GetInterfaceIPv4 extracts the first non-loopback IPv4 address for a given network interface.
func GetInterfaceIPv4(ifaceName string) string {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
			return ip.String()
		}
	}
	return ""
}

func ensureLinuxUser(username string) error {
	if !validNameRegex.MatchString(username) {
		return fmt.Errorf("invalid username '%s'", username)
	}

	// 1. Check if user already exists in Linux (/etc/passwd)
	if passwdData, err := os.ReadFile("/etc/passwd"); err == nil {
		for _, line := range strings.Split(string(passwdData), "\n") {
			parts := strings.Split(line, ":")
			if len(parts) > 0 && parts[0] == username {
				return nil // User already exists in OS!
			}
		}
	}
	idBin := resolveExecutable("id", "/usr/bin/id", "/bin/id")
	if err := exec.Command(idBin, "-u", username).Run(); err == nil {
		return nil // User already exists in OS!
	}

	// 2. Resolve nologin shell
	nologinShell := "/usr/sbin/nologin"
	if _, err := os.Stat("/usr/sbin/nologin"); err != nil {
		if _, err := os.Stat("/sbin/nologin"); err == nil {
			nologinShell = "/sbin/nologin"
		} else {
			nologinShell = "/bin/false"
		}
	}

	// 3. Resolve useradd binary
	useraddBin := resolveExecutable("useradd", "/usr/sbin/useradd", "/sbin/useradd", "/bin/useradd")

	// Try standard headless NAS user creation (-M no home, -s nologin)
	cmd := exec.Command(useraddBin, "-M", "-s", nologinShell, username)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// 3b. Try useradd without creating a group of same name (-N)
	cmd1b := exec.Command(useraddBin, "-M", "-N", "-s", nologinShell, username)
	if _, err1b := cmd1b.CombinedOutput(); err1b == nil {
		return nil
	}

	// 3c. Try useradd with existing group 'users'
	cmd1c := exec.Command(useraddBin, "-M", "-g", "users", "-s", nologinShell, username)
	if _, err1c := cmd1c.CombinedOutput(); err1c == nil {
		return nil
	}

	// 4. Try with useradd -m
	cmd2 := exec.Command(useraddBin, "-m", username)
	out2, err2 := cmd2.CombinedOutput()
	if err2 == nil {
		return nil
	}

	// 5. Try adduser (Debian/Ubuntu helper)
	adduserBin := resolveExecutable("adduser", "/usr/sbin/adduser", "/sbin/adduser")
	cmd3 := exec.Command(adduserBin, "--disabled-password", "--gecos", "", "--no-create-home", username)
	out3, err3 := cmd3.CombinedOutput()
	if err3 == nil {
		return nil
	}

	// 6. Check if user exists despite any warnings/non-zero codes
	if errCheck := exec.Command(idBin, "-u", username).Run(); errCheck == nil {
		return nil
	}

	// 7. Last-resort fallback when running as root: direct /etc/passwd and /etc/group entry
	if os.Geteuid() == 0 {
		if passwdData, errP := os.ReadFile("/etc/passwd"); errP == nil {
			sPasswd := string(passwdData)
			maxUID := 1000
			for _, line := range strings.Split(sPasswd, "\n") {
				fields := strings.Split(line, ":")
				if len(fields) >= 3 {
					if uid, errU := strconv.Atoi(fields[2]); errU == nil && uid >= 1000 && uid < 60000 {
						if uid > maxUID {
							maxUID = uid
						}
					}
				}
			}
			newUID := maxUID + 1
			homeDir := filepath.Join("/mnt/allod-storage/shares", username)
			entry := fmt.Sprintf("%s:x:%d:%d:%s:%s:%s\n", username, newUID, newUID, username, homeDir, nologinShell)
			if f, errA := os.OpenFile("/etc/passwd", os.O_APPEND|os.O_WRONLY, 0644); errA == nil {
				_, _ = f.WriteString(entry)
				f.Close()
			}
			groupEntry := fmt.Sprintf("%s:x:%d:\n", username, newUID)
			if fg, errG := os.OpenFile("/etc/group", os.O_APPEND|os.O_WRONLY, 0644); errG == nil {
				_, _ = fg.WriteString(groupEntry)
				fg.Close()
			}
			if shadow, errS := os.OpenFile("/etc/shadow", os.O_APPEND|os.O_WRONLY, 0640); errS == nil {
				_, _ = shadow.WriteString(fmt.Sprintf("%s:*:19000:0:99999:7:::\n", username))
				shadow.Close()
			}
			return nil
		}
	}

	return fmt.Errorf("user creation failed: %v (%s) / %v (%s) / %v (%s)",
		err, strings.TrimSpace(string(out)),
		err2, strings.TrimSpace(string(out2)),
		err3, strings.TrimSpace(string(out3)))
}

type Request struct {
	Action string                 `json:"action"`
	Plan   bool                   `json:"plan"`
	Args   map[string]interface{} `json:"args"`
}

type Response struct {
	Ok      bool     `json:"ok"`
	Applied bool     `json:"applied"`
	Plan    []string `json:"plan,omitempty"`
	Output  string   `json:"output,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// PeerCredChecker determines if the caller of a connection is authorized.
type PeerCredChecker interface {
	CheckPeer(conn net.Conn) (uid uint32, allowed bool, err error)
}

type Server struct {
	SocketPath  string
	CredChecker PeerCredChecker
}

type Client struct {
	SocketPath string
}

func (c *Client) Execute(action string, args map[string]interface{}, plan bool) (Response, error) {
	sock := c.SocketPath
	if sock == "" {
		sock = "/run/allod/helper.sock"
	}

	conn, err := net.DialTimeout("unix", sock, 3*time.Second)
	if err != nil {
		conn, err = net.DialTimeout("unix", "allod-helper.sock", 3*time.Second)
		if err != nil {
			return Response{Ok: false, Error: fmt.Sprintf("cannot connect to helper socket %s: %v", sock, err)}, err
		}
	}
	defer conn.Close()

	timeout := 15 * time.Second
	if action == "network.install_native" || action == "storage.init" || action == "system.update" {
		timeout = 60 * time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := Request{
		Action: action,
		Plan:   plan,
		Args:   args,
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{Ok: false, Error: err.Error()}, err
	}

	var res Response
	if err := json.NewDecoder(conn).Decode(&res); err != nil {
		return Response{Ok: false, Error: err.Error()}, err
	}

	return res, nil
}

func (s *Server) Start() error {
	os.Remove(s.SocketPath) // Pulizia vecchio socket

	// Ensure group 'allod' exists if running as root on Linux
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		_ = exec.Command("groupadd", "-f", "allod").Run()
		// Automatically add active logged-in users (UID >= 1000) to 'allod' group
		if userDirs, err := filepath.Glob("/run/user/[0-9]*"); err == nil {
			for _, d := range userDirs {
				uidStr := filepath.Base(d)
				if uid, errU := strconv.Atoi(uidStr); errU == nil && uid >= 1000 {
					if u, errL := user.LookupId(uidStr); errL == nil {
						_ = exec.Command("usermod", "-aG", "allod", u.Username).Run()
					}
				}
			}
		}
	}

	if dir := filepath.Dir(s.SocketPath); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
		if runtime.GOOS == "linux" && os.Geteuid() == 0 {
			_ = exec.Command("chown", "root:allod", dir).Run()
			_ = os.Chmod(dir, 0755)
		}
	}

	l, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %w", s.SocketPath, err)
	}
	defer l.Close()

	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		_ = exec.Command("chown", "root:allod", s.SocketPath).Run()
	}
	if err := os.Chmod(s.SocketPath, 0666); err != nil {
		log.Printf("Warning: failed to chmod 0666 on %s: %v", s.SocketPath, err)
	}
	fmt.Println("Ascolto su UNIX Socket:", s.SocketPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	checker := s.CredChecker
	if checker == nil {
		checker = defaultPeerCredCheckerInstance()
	}

	uid, allowed, credErr := checker.CheckPeer(conn)

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Ok: false, Error: "Invalid JSON format"})
		return
	}

	if credErr != nil || !allowed {
		log.Printf("[SECURITY] Rejected helper request: caller UID %d is not in group allod (action: %s, error: %v)", uid, req.Action, credErr)
		encoder.Encode(Response{Ok: false, Error: "caller not in group allod"})
		return
	}

	res := s.processRequest(req)
	argsHash := hashArgs(req.Args)
	log.Printf("[AUDIT] time=%s uid=%d action=%s args_hash=%s ok=%v applied=%v",
		time.Now().UTC().Format(time.RFC3339), uid, req.Action, argsHash, res.Ok, res.Applied)
	encoder.Encode(res)
}

func (s *Server) processRequest(req Request) Response {
	if !isAllowedAction(req.Action) {
		return Response{Ok: false, Error: "Action not allowed: " + req.Action}
	}

	switch req.Action {
	case "shares.apply":
		name, _ := req.Args["name"].(string)
		if name == "" {
			name = "shares"
		} else if !validNameRegex.MatchString(name) {
			return Response{Ok: false, Error: "Invalid share 'name'"}
		}
		path, _ := req.Args["path"].(string)
		if path == "" {
			path = "/mnt/allod-storage/shares"
		}
		if !isAllowedPath(path) {
			return Response{Ok: false, Error: "Invalid 'path' (must be under /mnt/allod-storage without traversal)"}
		}

		enabled := true
		if en, ok := req.Args["enabled"].(bool); ok {
			enabled = en
		}

		plan := []string{
			fmt.Sprintf("mkdir -p %s && chmod -R 0777 %s", path, path),
			fmt.Sprintf("configure share [%s] at %s in /etc/samba/smb.conf", name, path),
			"systemctl restart smbd",
		}

		if !req.Plan {
			_ = os.MkdirAll(path, 0777)
			_ = exec.Command("chmod", "-R", "0777", path).Run()

			// If path is a system binary directory, do not configure it as a Samba share
			cleanPath := filepath.Clean(path)
			if cleanPath == "/usr/local/bin" || cleanPath == "/bin" || cleanPath == "/usr/bin" || cleanPath == "/sbin" || cleanPath == "/usr/sbin" {
				_ = os.RemoveAll(filepath.Join(cleanPath, "public"))
				return Response{Ok: true, Applied: true, Plan: []string{fmt.Sprintf("chmod -R 0777 %s", cleanPath)}}
			}

			// Ensure public folder and media subfolders
			pubPath := filepath.Join(path, "public")
			if strings.HasSuffix(path, "/public") {
				pubPath = path
			}
			_ = os.MkdirAll(pubPath, 0777)
			_ = exec.Command("chmod", "-R", "0777", pubPath).Run()
			for _, sub := range []string{"film", "musica", "serie", "movies", "tv", "music"} {
				subDir := filepath.Join(pubPath, sub)
				_ = os.MkdirAll(subDir, 0777)
				_ = exec.Command("chmod", "0777", subDir).Run()
			}

			smbConf := "/etc/samba/smb.conf"
			if content, err := os.ReadFile(smbConf); err == nil {
				sContent := string(content)
				if enabled {
					// Configure [public] share for guest access without password
					if !strings.Contains(sContent, "[public]") {
						publicSnippet := fmt.Sprintf("\n[public]\n   comment = Cartella pubblica Allod (Jellyfin Media)\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = yes\n   create mask = 0666\n   directory mask = 0777\n   force create mode = 0666\n   force directory mode = 0777\n   hide unreadable = yes\n", pubPath)
						sContent += publicSnippet
					}

					// Configure default share (e.g. [shares])
					shareTag := fmt.Sprintf("[%s]", name)
					if !strings.Contains(sContent, shareTag) {
						shareSnippet := fmt.Sprintf("\n[%s]\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = yes\n   create mask = 0666\n   directory mask = 0777\n   force create mode = 0666\n   force directory mode = 0777\n   hide unreadable = yes\n", name, path)
						sContent += shareSnippet
					}

					sContent = PatchSambaConfig(sContent)
					_ = os.WriteFile(smbConf, []byte(sContent), 0644)
					_ = exec.Command("systemctl", "restart", "smbd").Run()
				} else {
					_ = exec.Command("systemctl", "stop", "smbd").Run()
				}
			} else if !enabled {
				_ = exec.Command("systemctl", "stop", "smbd").Run()
			}
		}

		return Response{Ok: true, Applied: !req.Plan, Plan: plan}

	case "shares.bind_photos":
		enabled := true
		if en, ok := req.Args["enabled"].(bool); ok {
			enabled = en
		}

		targetUser, _ := req.Args["username"].(string)
		if targetUser != "" && !validNameRegex.MatchString(targetUser) {
			return Response{Ok: false, Error: "Invalid 'username' for bind_photos"}
		}
		basePhotos := "/mnt/allod-storage/photos/upload/library"
		baseShares := "/mnt/allod-storage/shares"

		var targets [][2]string // [source, target]

		if targetUser != "" && validNameRegex.MatchString(targetUser) {
			targets = append(targets, [2]string{
				filepath.Join(basePhotos, targetUser),
				filepath.Join(baseShares, targetUser, "photos"),
			})
		} else {
			// Shared / global photos library
			targets = append(targets, [2]string{
				basePhotos,
				filepath.Join(baseShares, "photos"),
			})
			// Per-user photo shares for every registered user directory in shares
			if entries, err := os.ReadDir(baseShares); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					uName := e.Name()
					if uName == "public" || uName == "photos" || uName == "media" || uName == "lost+found" || strings.HasPrefix(uName, ".") {
						continue
					}
					targets = append(targets, [2]string{
						filepath.Join(basePhotos, uName),
						filepath.Join(baseShares, uName, "photos"),
					})
				}
			}
		}

		plan := []string{}
		for _, pair := range targets {
			plan = append(plan, fmt.Sprintf("bind mount %s to %s (enabled=%v)", pair[0], pair[1], enabled))
		}

		if !req.Plan {
			mounts, _ := os.ReadFile("/proc/mounts")
			mountsStr := string(mounts)

			for _, pair := range targets {
				src := pair[0]
				tgt := pair[1]

				if enabled {
					_ = os.MkdirAll(src, 0770)
					_ = os.MkdirAll(tgt, 0770)
					if strings.Contains(mountsStr, tgt) {
						_ = exec.Command("umount", tgt).Run()
					}
					cmd := exec.Command("mount", "--bind", src, tgt)
					if out, err := cmd.CombinedOutput(); err != nil {
						return Response{Ok: false, Error: fmt.Sprintf("mount --bind %s to %s failed: %v (%s)", src, tgt, err, strings.TrimSpace(string(out)))}
					}
					_ = exec.Command("chmod", "-R", "0770", tgt).Run()
					_ = exec.Command("chmod", "-R", "go-rwx", tgt).Run()
				} else {
					if strings.Contains(mountsStr, tgt) {
						_ = exec.Command("umount", tgt).Run()
					}
					_ = exec.Command("chmod", "0770", tgt).Run()
				}
			}
		}

		return Response{Ok: true, Applied: !req.Plan, Plan: plan}

	case "snapshots.create":
		subvol, _ := req.Args["subvolume"].(string)
		if subvol == "" {
			subvol = "data"
		} else if !validNameRegex.MatchString(subvol) {
			return Response{Ok: false, Error: "Invalid 'subvolume' name"}
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("btrfs subvolume snapshot /data/%s /data/.snapshots/%s", subvol, subvol)}}

	case "snapshots.prune":
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{"btrfs subvolume delete old snapshots"}}

	case "users.create":
		username, ok := req.Args["username"].(string)
		if !ok || !validNameRegex.MatchString(username) {
			return Response{Ok: false, Error: "Invalid or missing 'username'"}
		}
		if !req.Plan {
			if err := ensureLinuxUser(username); err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Failed to create Linux user '%s': %v", username, err)}
			}
			userSharePath := filepath.Join("/mnt/allod-storage/shares", username)
			_ = os.MkdirAll(userSharePath, 0770)
			chmodBin := resolveExecutable("chmod", "/bin/chmod", "/usr/bin/chmod")
			_ = exec.Command(chmodBin, "0770", userSharePath).Run()
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("useradd -M -s /usr/sbin/nologin %s", username)}}

	case "users.passwd", "shares.set_password":
		username, ok := req.Args["username"].(string)
		if !ok || !validNameRegex.MatchString(username) {
			return Response{Ok: false, Error: "Invalid or missing 'username'"}
		}
		password, _ := req.Args["password"].(string)
		if password == "" {
			return Response{Ok: false, Error: "Password cannot be empty"}
		}

		plan := []string{fmt.Sprintf("smbpasswd -a -s %s", username)}

		if !req.Plan {
			// Auto-heal: Ensure Linux user exists in /etc/passwd first so smbpasswd can register them!
			if err := ensureLinuxUser(username); err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Failed to ensure Linux user '%s': %v", username, err)}
			}

			smbpasswdBin := resolveExecutable("smbpasswd", "/usr/bin/smbpasswd", "/bin/smbpasswd")

			cmd := exec.Command(smbpasswdBin, "-a", "-s", username)
			cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
			if out, err := cmd.CombinedOutput(); err != nil {
				// If adding failed, try updating existing entry
				cmd2 := exec.Command(smbpasswdBin, "-s", username)
				cmd2.Stdin = strings.NewReader(password + "\n" + password + "\n")
				if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
					return Response{Ok: false, Error: fmt.Sprintf("smbpasswd error: %v (%s)", err, strings.TrimSpace(string(out)+"\n"+string(out2)))}
				}
			}
			_ = exec.Command(smbpasswdBin, "-e", username).Run()

			// Ensure user directory /mnt/allod-storage/shares/<username> exists
			userSharePath := filepath.Join("/mnt/allod-storage/shares", username)
			_ = os.MkdirAll(userSharePath, 0770)
			chmodBin := resolveExecutable("chmod", "/bin/chmod", "/usr/bin/chmod")
			_ = exec.Command(chmodBin, "0770", userSharePath).Run()
			chownBin := resolveExecutable("chown", "/bin/chown", "/usr/bin/chown")
			_ = exec.Command(chownBin, "-R", fmt.Sprintf("%s:%s", username, username), userSharePath).Run()

			// Ensure private share configuration in /etc/samba/smb.conf
			smbConf := "/etc/samba/smb.conf"
			if content, err := os.ReadFile(smbConf); err == nil {
				sContent := string(content)
				userTag := fmt.Sprintf("[%s]", username)
				if !strings.Contains(sContent, userTag) {
					userSnippet := fmt.Sprintf("\n[%s]\n   comment = Cartella privata di %s\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = no\n   valid users = %s\n   create mask = 0660\n   directory mask = 0770\n   access based share enum = yes\n   hide unreadable = yes\n", username, username, userSharePath, username)
					sContent += userSnippet
				}
				sContent = PatchSambaConfig(sContent)
				_ = os.WriteFile(smbConf, []byte(sContent), 0644)
			}

			systemctlBin := resolveExecutable("systemctl", "/bin/systemctl", "/usr/bin/systemctl")
			_ = exec.Command(systemctlBin, "restart", "smbd").Run()
		}

		return Response{Ok: true, Applied: !req.Plan, Plan: plan}

	case "firewall.apply":
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{"nftables reload /etc/allod/nftables.conf"}}

	case "smart.read":
		disk, ok := req.Args["disk"].(string)
		if !ok || disk == "" || !validDeviceRegex(disk) {
			return Response{Ok: false, Error: "Invalid or missing 'disk' identifier"}
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("smartctl -H /dev/disk/by-id/%s", disk)}}

	case "service.restart":
		unit, ok := req.Args["unit"].(string)
		if !ok || !validUnitRegex.MatchString(unit) || !isAllowedServiceUnit(unit) {
			return Response{Ok: false, Error: "Service unit not allowed or invalid: " + unit}
		}
		if !req.Plan {
			if unit == "allod-helperd" {
				// 1. Copy newly built binary to /usr/local/bin/allod-helperd if available
				cwd, _ := os.Getwd()
				candidates := []string{
					filepath.Join(cwd, "allod-helperd"),
					"allod-helperd",
				}
				if entries, err := filepath.Glob("/home/*/allod/allod-helperd"); err == nil {
					candidates = append(candidates, entries...)
				}
				if entries, err := filepath.Glob("/home/*/*/allod-helperd"); err == nil {
					candidates = append(candidates, entries...)
				}
				if entries, err := filepath.Glob("/home/*/*/*/allod-helperd"); err == nil {
					candidates = append(candidates, entries...)
				}
				candidates = append(candidates, "/tmp/allod-helperd-update", "/tmp/allod-helperd")
				for _, cand := range candidates {
					if info, err := os.Stat(cand); err == nil && !info.IsDir() {
						if data, err := os.ReadFile(cand); err == nil && len(data) > 0 {
							tmpDst := "/usr/local/bin/allod-helperd.tmp"
							_ = os.Remove(tmpDst)
							if errW := os.WriteFile(tmpDst, data, 0755); errW == nil {
								_ = os.Rename(tmpDst, "/usr/local/bin/allod-helperd")
							} else {
								_ = os.Remove("/usr/local/bin/allod-helperd")
								_ = os.WriteFile("/usr/local/bin/allod-helperd", data, 0755)
							}
							_ = os.Chmod("/usr/local/bin/allod-helperd", 0755)
						}
						break
					}
				}

				// 2. Restart systemd service asynchronously so we can return the response first
				systemctlBin := resolveExecutable("systemctl", "/bin/systemctl", "/usr/bin/systemctl")
				go func() {
					time.Sleep(200 * time.Millisecond)
					_ = exec.Command(systemctlBin, "--no-block", "restart", "allod-helperd").Run()
				}()
				return Response{Ok: true, Applied: true, Plan: []string{"systemctl restart --no-block allod-helperd (queued)"}}

				// 3. Fallback for manual run: spawn self in background and exit
				binaryPath, errExe := os.Executable()
				if errExe == nil {
					go func() {
						time.Sleep(500 * time.Millisecond)
						cmd := exec.Command(binaryPath)
						_ = cmd.Start()
						os.Exit(0)
					}()
					return Response{Ok: true, Applied: true, Plan: []string{"re-spawn allod-helperd binary"}}
				}
			}

			systemctlBin := resolveExecutable("systemctl", "/bin/systemctl", "/usr/bin/systemctl")
			_ = exec.Command(systemctlBin, "restart", unit).Run()
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("systemctl restart %s", unit)}}

	case "storage.init":
		var disks []string
		if dList, ok := req.Args["disks"].([]interface{}); ok {
			for _, d := range dList {
				s, ok := d.(string)
				if ok && validDeviceRegex(s) {
					disks = append(disks, s)
				} else {
					return Response{Ok: false, Error: fmt.Sprintf("Invalid disk identifier: %v", d)}
				}
			}
		} else if dStr, ok := req.Args["disks"].(string); ok {
			for _, s := range strings.Split(dStr, ",") {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				if validDeviceRegex(s) {
					disks = append(disks, s)
				} else {
					return Response{Ok: false, Error: "Invalid disk identifier: " + s}
				}
			}
		}

		if len(disks) == 0 {
			return Response{Ok: false, Error: "Nessun disco valido specificato per storage.init"}
		}

		mode, _ := req.Args["mode"].(string)
		if mode == "" {
			mode = "raid1"
		} else if mode != "single" && mode != "raid1" {
			return Response{Ok: false, Error: "Invalid storage mode (must be 'single' or 'raid1')"}
		}

		mountPoint, _ := req.Args["mount"].(string)
		if mountPoint == "" {
			mountPoint = "/mnt/allod-storage"
		} else if !isAllowedPath(mountPoint) {
			return Response{Ok: false, Error: "Invalid mount path: " + mountPoint}
		}

		username, _ := req.Args["user"].(string)
		if username != "" && !validNameRegex.MatchString(username) {
			return Response{Ok: false, Error: "Invalid username: " + username}
		}
		if username == "" || username == "root" {
			if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
				username = sudoUser
			} else {
				if entries, err := os.ReadDir("/home"); err == nil {
					for _, e := range entries {
						if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
							username = e.Name()
							break
						}
					}
				}
			}
		}
		if username == "" {
			username = "root"
		}

		plan := []string{
			fmt.Sprintf("mkfs.btrfs -d %s -m %s -f %s", mode, mode, strings.Join(disks, " ")),
			fmt.Sprintf("mkdir -p %s", mountPoint),
			fmt.Sprintf("mount %s %s", disks[0], mountPoint),
			fmt.Sprintf("mkdir -p %s/{cloud,photos,shares,backup,media}", mountPoint),
			fmt.Sprintf("chmod -R 0777 %s", mountPoint),
			fmt.Sprintf("chown -R %s:%s %s", username, username, mountPoint),
		}

		if !req.Plan {
			for _, d := range disks {
				devPath := d
				if !strings.HasPrefix(devPath, "/dev/") {
					devPath = "/dev/" + devPath
				}
				_ = exec.Command("umount", devPath).Run()
			}

			args := []string{"-d", mode, "-m", mode, "-f"}
			for _, d := range disks {
				if !strings.HasPrefix(d, "/dev/") {
					d = "/dev/" + d
				}
				args = append(args, d)
			}
			if err := exec.Command("mkfs.btrfs", args...).Run(); err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore mkfs.btrfs: %v", err)}
			}
			_ = os.MkdirAll(mountPoint, 0777)
			firstDisk := disks[0]
			if !strings.HasPrefix(firstDisk, "/dev/") {
				firstDisk = "/dev/" + firstDisk
			}
			_ = exec.Command("mount", firstDisk, mountPoint).Run()

			subdirs := []string{
				"cloud", "cloud/html", "cloud/data", "cloud/postgres",
				"photos", "photos/upload", "photos/postgres", "photos/valkey",
				"shares", "shares/public",
				"backup", "backup/vault",
				"media", "media/data", "media/config",
			}
			for _, sub := range subdirs {
				_ = os.MkdirAll(filepath.Join(mountPoint, sub), 0777)
			}
			_ = exec.Command("chmod", "-R", "0777", mountPoint).Run()
			if username != "root" {
				_ = exec.Command("chown", "-R", fmt.Sprintf("%s:%s", username, username), mountPoint).Run()
			}
		}

		return Response{Ok: true, Applied: !req.Plan, Plan: plan}

	case "storage.diagnostics":
		mountPoint, _ := req.Args["mount"].(string)
		if mountPoint == "" {
			mountPoint = "/mnt/allod-storage"
		} else if !isAllowedPath(mountPoint) {
			return Response{Ok: false, Error: "Invalid mount path: " + mountPoint}
		}

		usageOut, _ := exec.Command("btrfs", "filesystem", "usage", mountPoint).CombinedOutput()
		statsOut, _ := exec.Command("btrfs", "device", "stats", mountPoint).CombinedOutput()
		dfOut, _ := exec.Command("btrfs", "filesystem", "df", mountPoint).CombinedOutput()

		type diagResult struct {
			Usage string `json:"usage"`
			Stats string `json:"stats"`
			Df    string `json:"df"`
		}
		raw, _ := json.Marshal(diagResult{
			Usage: string(usageOut),
			Stats: string(statsOut),
			Df:    string(dfOut),
		})

		return Response{
			Ok:      true,
			Applied: true,
			Output:  string(raw),
			Plan: []string{
				fmt.Sprintf("btrfs filesystem usage %s", mountPoint),
				fmt.Sprintf("btrfs device stats %s", mountPoint),
			},
		}

	case "network.netbird_status":
		if nbBin, err := exec.LookPath("netbird"); err == nil {
			fullCmd := []string{nbBin, "status", "--json"}
			plan := []string{strings.Join(fullCmd, " ")}
			if !req.Plan {
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				out, err := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...).CombinedOutput()
				cancel()
				strOut := strings.TrimSpace(string(out))
				if err == nil && len(strOut) > 0 {
					start := strings.Index(strOut, "{")
					end := strings.LastIndex(strOut, "}")
					if start >= 0 && end > start {
						return Response{Ok: true, Applied: true, Output: strOut[start : end+1], Plan: plan}
					}
				}
				return Response{Ok: false, Error: fmt.Sprintf("NetBird nativo non risponde allo status JSON: %s", strOut)}
			} else {
				return Response{Ok: true, Applied: false, Plan: plan}
			}
		}

		targetContainer, runuserPrefix, isRunning, exists := findNetBirdTarget()
		var fullCmd []string
		if len(runuserPrefix) > 0 {
			fullCmd = append(runuserPrefix, "podman", "exec", targetContainer, "netbird", "status", "--json")
		} else {
			fullCmd = []string{"podman", "exec", targetContainer, "netbird", "status", "--json"}
		}
		plan := []string{strings.Join(fullCmd, " ")}
		if !req.Plan {
			if !isRunning {
				if exists {
					var startCmd []string
					if len(runuserPrefix) > 0 {
						startCmd = append(runuserPrefix, "podman", "start", targetContainer)
					} else {
						startCmd = []string{"podman", "start", targetContainer}
					}
					ctxS, cancelS := context.WithTimeout(context.Background(), 4*time.Second)
					out, err := exec.CommandContext(ctxS, startCmd[0], startCmd[1:]...).CombinedOutput()
					cancelS()
					if err == nil {
						time.Sleep(1 * time.Second)
						isRunning = true
					} else {
						return Response{Ok: false, Error: fmt.Sprintf("Container NetBird (%s) è arrestato (tentativo avvio: %s). Clicca 'Avvia' nel modulo Rete per riavviarlo.", targetContainer, strings.TrimSpace(string(out)))}
					}
				} else {
					return Response{Ok: false, Error: "Container NetBird non configurato o non presente. Avvia il modulo network dal pannello."}
				}
			}

			ctxE, cancelE := context.WithTimeout(context.Background(), 4*time.Second)
			out, err := exec.CommandContext(ctxE, fullCmd[0], fullCmd[1:]...).CombinedOutput()
			cancelE()
			strOut := strings.TrimSpace(string(out))
			if err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore status NetBird (%s): %v (%s)", targetContainer, err, strOut)}
			}
			start := strings.Index(strOut, "{")
			end := strings.LastIndex(strOut, "}")
			if start >= 0 && end > start {
				return Response{Ok: true, Applied: true, Output: strOut[start : end+1], Plan: plan}
			}
			return Response{Ok: false, Error: fmt.Sprintf("Errore status NetBird: %s", strOut)}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.netbird_cli":
		cmdType, _ := req.Args["command"].(string)
		nbBin, nbErr := exec.LookPath("netbird")
		isNative := (nbErr == nil)

		if cmdType == "logs" {
			// If native NetBird binary is installed on the host, prioritize journalctl logs
			if isNative {
				ctxJ, cancelJ := context.WithTimeout(context.Background(), 4*time.Second)
				outJ, errJ := exec.CommandContext(ctxJ, "journalctl", "-u", "netbird", "-n", "30", "--no-pager").CombinedOutput()
				cancelJ()
				strJ := strings.TrimSpace(string(outJ))
				if errJ == nil && len(strJ) > 0 && !strings.Contains(strJ, "No entries") {
					return Response{Ok: true, Applied: true, Output: strJ, Plan: []string{"journalctl -u netbird"}}
				}
			}

			targetContainer, runuserPrefix, isRunning, exists := findNetBirdTarget()
			if !exists {
				if isNative {
					ctxJ, cancelJ := context.WithTimeout(context.Background(), 4*time.Second)
					outJ, _ := exec.CommandContext(ctxJ, "journalctl", "-u", "netbird", "-n", "30", "--no-pager").CombinedOutput()
					cancelJ()
					return Response{Ok: true, Applied: true, Output: strings.TrimSpace(string(outJ)), Plan: []string{"journalctl -u netbird"}}
				}
				return Response{Ok: true, Applied: true, Output: "Nessun container o servizio NetBird trovato. Avvia il modulo Rete dal pannello.", Plan: nil}
			}
			var logCmd []string
			if len(runuserPrefix) > 0 {
				logCmd = append(runuserPrefix, "podman", "logs", "--tail", "30", targetContainer)
			} else {
				logCmd = []string{"podman", "logs", "--tail", "30", targetContainer}
			}
			plan := []string{strings.Join(logCmd, " ")}
			if !req.Plan {
				ctxL, cancelL := context.WithTimeout(context.Background(), 4*time.Second)
				out, err := exec.CommandContext(ctxL, logCmd[0], logCmd[1:]...).CombinedOutput()
				cancelL()
				strLogs := strings.TrimSpace(string(out))
				if !isRunning && strLogs != "" {
					strLogs = fmt.Sprintf("[ATTENZIONE: Il container %s è attualmente ARRESTATO]\n%s", targetContainer, strLogs)
				}
				if err == nil && len(strLogs) > 0 {
					return Response{Ok: true, Applied: true, Output: strLogs, Plan: plan}
				}
				return Response{Ok: true, Applied: true, Output: "In attesa di log da NetBird.", Plan: plan}
			}
			return Response{Ok: true, Applied: false, Plan: plan}
		}

		var args []string
		switch cmdType {
		case "status":
			args = []string{"status"}
		case "status_detail":
			args = []string{"status", "--detail"}
		default:
			args = []string{"status", "--json"}
		}

		if isNative {
			fullCmd := append([]string{nbBin}, args...)
			plan := []string{strings.Join(fullCmd, " ")}
			if !req.Plan {
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				out, err := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...).CombinedOutput()
				cancel()
				trimmed := strings.TrimSpace(string(out))
				if err == nil && len(trimmed) > 0 {
					return Response{Ok: true, Applied: true, Output: trimmed, Plan: plan}
				}
				// If status detail failed or daemon is stopped, query systemctl status netbird for actionable diagnostics
				ctxS, cancelS := context.WithTimeout(context.Background(), 3*time.Second)
				outS, _ := exec.CommandContext(ctxS, "systemctl", "status", "netbird").CombinedOutput()
				cancelS()
				sysStatus := strings.TrimSpace(string(outS))
				if len(sysStatus) > 0 {
					if len(trimmed) > 0 {
						return Response{Ok: true, Applied: true, Output: fmt.Sprintf("%s\n\n--- systemctl status netbird ---\n%s", trimmed, sysStatus), Plan: plan}
					}
					return Response{Ok: true, Applied: true, Output: sysStatus, Plan: plan}
				}
				if len(trimmed) > 0 {
					return Response{Ok: true, Applied: true, Output: trimmed, Plan: plan}
				}
				return Response{Ok: false, Error: "NetBird nativo non risponde alla CLI. Verifica lo stato con 'systemctl status netbird'."}
			} else {
				return Response{Ok: true, Applied: false, Plan: plan}
			}
		}

		targetContainer, runuserPrefix, isRunning, exists := findNetBirdTarget()
		var fullCmd []string
		if len(runuserPrefix) > 0 {
			fullCmd = append(runuserPrefix, "podman", "exec", targetContainer, "netbird")
			fullCmd = append(fullCmd, args...)
		} else {
			fullCmd = append([]string{"podman", "exec", targetContainer, "netbird"}, args...)
		}
		plan := []string{strings.Join(fullCmd, " ")}
		if !req.Plan {
			if !isRunning {
				if exists {
					return Response{Ok: false, Error: fmt.Sprintf("Il container %s è arrestato. Clicca su 'Avvia' nel modulo Rete per avviarlo.", targetContainer)}
				}
				return Response{Ok: false, Error: "Nessun container NetBird configurato. Clicca su 'Avvia' nel modulo Rete per configurare e avviare."}
			}

			ctxE, cancelE := context.WithTimeout(context.Background(), 4*time.Second)
			out, err := exec.CommandContext(ctxE, fullCmd[0], fullCmd[1:]...).CombinedOutput()
			cancelE()
			trimmed := strings.TrimSpace(string(out))
			if err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore CLI NetBird (%s): %v (%s)", targetContainer, err, trimmed)}
			}
			return Response{Ok: true, Applied: true, Output: trimmed, Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.netbird_up":
		// Ensure kernel tun and wireguard drivers are loaded
		_ = exec.Command("modprobe", "tun").Run()
		_ = exec.Command("modprobe", "wireguard").Run()

		// Ensure host sysctls for WireGuard mesh forwarding
		_ = exec.Command("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1").Run()
		_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
		_ = exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()

		baseDir := "/mnt/allod-storage"
		if envBase := os.Getenv("ALLOD_STORAGE_DIR"); envBase != "" {
			baseDir = envBase
		}
		dataDir := filepath.Join(baseDir, "network", "netbird")
		secDir := filepath.Join(baseDir, "network", "secrets")
		envFile := filepath.Join(secDir, "netbird.env")

		_ = os.MkdirAll(dataDir, 0755)
		_ = os.MkdirAll(secDir, 0700)

		if key, ok := req.Args["setup_key"].(string); ok && key != "" {
			mgmt, _ := req.Args["management_url"].(string)
			if mgmt == "" {
				mgmt = "https://api.netbird.io:443"
			}
			envContent := fmt.Sprintf("# NetBird Sovereign Mesh Configuration\nNB_SETUP_KEY=%s\nNB_MANAGEMENT_URL=%s\nNB_DISABLE_DNS=true\n", key, mgmt)
			_ = os.WriteFile(envFile, []byte(envContent), 0600)
		} else if _, err := os.Stat(envFile); err != nil {
			defaultEnv := "# NetBird Sovereign Mesh Configuration\nNB_SETUP_KEY=\nNB_MANAGEMENT_URL=\nNB_DISABLE_DNS=true\n"
			_ = os.WriteFile(envFile, []byte(defaultEnv), 0600)
		}

		if nbBin, err := exec.LookPath("netbird"); err == nil {
			var cmdArgs []string
			cmdArgs = append(cmdArgs, "up", "--disable-dns")
			if key, ok := req.Args["setup_key"].(string); ok && key != "" {
				cmdArgs = append(cmdArgs, "--setup-key", key)
			}
			if mgmt, ok := req.Args["management_url"].(string); ok && mgmt != "" && mgmt != "https://api.netbird.io:443" {
				cmdArgs = append(cmdArgs, "--management-url", mgmt)
			}
			fullCmd := append([]string{nbBin}, cmdArgs...)
			plan := []string{strings.Join(fullCmd, " ")}
			if !req.Plan {
				// Clean up any lingering container client now that host binary is active
				_ = exec.Command("podman", "stop", "allod-netbird").Run()
				_ = exec.Command("podman", "rm", "-f", "allod-netbird").Run()

				// Ensure netbird systemd service is enabled and running
				_ = exec.Command("systemctl", "enable", "--now", "netbird").Run()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				out, err := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...).CombinedOutput()
				if err != nil {
					return Response{Ok: false, Error: fmt.Sprintf("Errore netbird up nativo: %v (%s)", err, strings.TrimSpace(string(out)))}
				}
				return Response{Ok: true, Applied: true, Output: strings.TrimSpace(string(out)), Plan: plan}
			}
			return Response{Ok: true, Applied: false, Plan: plan}
		}

		// Clean up any legacy rootless user containers
		if userDirs, err := filepath.Glob("/run/user/[0-9]*"); err == nil {
			for _, dir := range userDirs {
				uid := filepath.Base(dir)
				_ = exec.Command("runuser", "-u", "#"+uid, "--", "podman", "rm", "-f", "network").Run()
				_ = exec.Command("runuser", "-u", "#"+uid, "--", "podman", "rm", "-f", "systemd-network").Run()
			}
		}

		// Clean up any lingering or orphan wt0 interface before creating container
		_ = exec.Command("ip", "link", "delete", "wt0").Run()

		// Stop and remove any prior root container to restart cleanly
		_ = exec.Command("podman", "rm", "-f", "allod-netbird").Run()

		// Clean up stale socket and lock files from prior runs in dataDir so new container doesn't fail on bind
		_ = os.Remove(filepath.Join(dataDir, "netbird.sock"))
		_ = os.Remove(filepath.Join(dataDir, "default.sock"))
		_ = os.Remove(filepath.Join(dataDir, "netbird.sock.lock"))

		// Enable podman-restart service so containers with --restart=always restart on boot
		_ = exec.Command("systemctl", "enable", "--now", "podman-restart.service").Run()

		fullCmd := []string{
			"podman", "run", "-d",
			"--name", "allod-netbird",
			"--replace",
			"--restart=always",
			"--privileged",
			"--network=host",
			"--device=/dev/net/tun",
			"-v", fmt.Sprintf("%s:/var/lib/netbird:Z", dataDir),
			"--env-file", envFile,
			"--env", "NB_DISABLE_DNS=true",
			"docker.io/netbirdio/netbird:0.79.0",
		}
		plan := []string{strings.Join(fullCmd, " ")}
		if !req.Plan {
			out, err := exec.Command(fullCmd[0], fullCmd[1:]...).CombinedOutput()
			if err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore avvio container NetBird: %v (%s)", err, strings.TrimSpace(string(out)))}
			}
			return Response{Ok: true, Applied: true, Output: strings.TrimSpace(string(out)), Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.install_native":
		plan := []string{
			"curl -fsSL https://pkgs.netbird.io/install.sh | sh",
			"podman rm -f allod-netbird",
		}
		if !req.Plan {
			out, err := exec.Command("sh", "-c", "curl -fsSL https://pkgs.netbird.io/install.sh | sh").CombinedOutput()
			if err != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore installazione NetBird nativo: %v (%s)", err, strings.TrimSpace(string(out)))}
			}
			// Stop and remove any prior container client now that host binary is installed
			_ = exec.Command("podman", "stop", "allod-netbird").Run()
			_ = exec.Command("podman", "rm", "-f", "allod-netbird").Run()
			_ = exec.Command("ip", "link", "delete", "wt0").Run()

			// Auto-connect native NetBird if setup key is configured
			baseDir := "/mnt/allod-storage"
			if envBase := os.Getenv("ALLOD_STORAGE_DIR"); envBase != "" {
				baseDir = envBase
			}
			envFile := filepath.Join(baseDir, "network", "secrets", "netbird.env")
			setupKey := ""
			mgmtURL := "https://api.netbird.io:443"
			if envBytes, err := os.ReadFile(envFile); err == nil {
				for _, line := range strings.Split(string(envBytes), "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "NB_SETUP_KEY=") {
						setupKey = strings.Trim(strings.TrimPrefix(trimmed, "NB_SETUP_KEY="), `"'`)
					} else if strings.HasPrefix(trimmed, "NB_MANAGEMENT_URL=") {
						val := strings.Trim(strings.TrimPrefix(trimmed, "NB_MANAGEMENT_URL="), `"'`)
						if val != "" {
							mgmtURL = val
						}
					}
				}
			}
			if setupKey != "" {
				if nbBin, err := exec.LookPath("netbird"); err == nil {
					_ = exec.Command("systemctl", "enable", "--now", "netbird").Run()
					cmdArgs := []string{"up", "--disable-dns", "--setup-key", setupKey}
					if mgmtURL != "" && mgmtURL != "https://api.netbird.io:443" {
						cmdArgs = append(cmdArgs, "--management-url", mgmtURL)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					_ = exec.CommandContext(ctx, nbBin, cmdArgs...).Run()
					cancel()
				}
			}
			return Response{Ok: true, Applied: true, Output: string(out), Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.server_up":
		baseDir := "/mnt/allod-storage"
		if envBase := os.Getenv("ALLOD_STORAGE_DIR"); envBase != "" {
			baseDir = envBase
		}
		srvDir := filepath.Join(baseDir, "network", "server")
		dataDir := filepath.Join(srvDir, "data")
		cfgDir := filepath.Join(srvDir, "config")
		_ = os.MkdirAll(dataDir, 0755)
		_ = os.MkdirAll(cfgDir, 0755)

		domain := "127.0.0.1"
		if d, ok := req.Args["domain"].(string); ok && strings.TrimSpace(d) != "" {
			domain = strings.TrimSpace(d)
		}
		port := 33073
		if p, ok := req.Args["port"].(float64); ok && p > 0 && p <= 65535 {
			port = int(p)
		} else if p, ok := req.Args["port"].(int); ok && p > 0 && p <= 65535 {
			port = p
		}
		dashPort := 8088
		if dp, ok := req.Args["dash_port"].(float64); ok && dp > 0 && dp <= 65535 {
			dashPort = int(dp)
		} else if dp, ok := req.Args["dash_port"].(int); ok && dp > 0 && dp <= 65535 {
			dashPort = dp
		}
		stunPort := 3478
		if sp, ok := req.Args["stun_port"].(float64); ok && sp > 0 && sp <= 65535 {
			stunPort = int(sp)
		} else if sp, ok := req.Args["stun_port"].(int); ok && sp > 0 && sp <= 65535 {
			stunPort = sp
		}

		exposedURL := fmt.Sprintf("http://%s:%d", domain, port)
		if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
			exposedURL = domain
		} else if strings.Contains(domain, ".") && !strings.Contains(domain, "192.168.") && !strings.Contains(domain, "10.") && !strings.Contains(domain, "127.0.") {
			exposedURL = fmt.Sprintf("https://%s:%d", domain, port)
		}

		cfgFile := filepath.Join(cfgDir, "config.yaml")
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			authSecret := generateRandomKeyString(32)
			sessionKey := generateRandomKeyString(32)
			dataKey := generateRandomKeyString(32)

			cfgContent := fmt.Sprintf(`# NetBird Managed Server Configuration (Allod)
server:
  listenAddress: ":80"
  exposedAddress: "%s"
  stunPorts:
    - %d
  metricsPort: 9090
  healthcheckAddress: ":9000"
  logLevel: "info"
  logFile: "console"
  authSecret: "%s"
  dataDir: "/var/lib/netbird"
  auth:
    issuer: "%s/oauth2"
    signKeyRefreshEnabled: true
    sessionCookieEncryptionKey: "%s"
    dashboardRedirectURIs:
      - "http://%s:%d/nb-auth"
      - "http://%s:%d/nb-silent-auth"
    cliRedirectURIs:
      - "http://localhost:53000/"
  store:
    engine: "sqlite"
    encryptionKey: "%s"
`, exposedURL, stunPort, authSecret, exposedURL, sessionKey, domain, dashPort, domain, dashPort, dataKey)
			_ = os.WriteFile(cfgFile, []byte(cfgContent), 0600)
		}

		serverCmd := []string{
			"podman", "run", "-d",
			"--name", "allod-netbird-server",
			"--replace",
			"--restart=always",
			"-v", fmt.Sprintf("%s:/var/lib/netbird:Z", dataDir),
			"-v", fmt.Sprintf("%s:/etc/netbird/config.yaml:Z", cfgFile),
			"-p", fmt.Sprintf("%d:80", port),
			"-p", fmt.Sprintf("%d:%d/udp", stunPort, stunPort),
			"docker.io/netbirdio/netbird-server:0.79.0",
			"--config", "/etc/netbird/config.yaml",
		}

		dashCmd := []string{
			"podman", "run", "-d",
			"--name", "allod-netbird-dashboard",
			"--replace",
			"--restart=always",
			"-p", fmt.Sprintf("%d:80", dashPort),
			"--env", fmt.Sprintf("NETBIRD_MGMT_API_ENDPOINT=%s", exposedURL),
			"--env", fmt.Sprintf("NETBIRD_MGMT_GRPC_API_ENDPOINT=%s", exposedURL),
			"--env", "AUTH_AUDIENCE=netbird-dashboard",
			"--env", "AUTH_CLIENT_ID=netbird-dashboard",
			"--env", fmt.Sprintf("AUTH_AUTHORITY=%s/oauth2", exposedURL),
			"--env", "USE_AUTH0=false",
			"--env", "AUTH_SUPPORTED_SCOPES=openid profile email groups",
			"--env", "AUTH_REDIRECT_URI=/nb-auth",
			"--env", "AUTH_SILENT_REDIRECT_URI=/nb-silent-auth",
			"docker.io/netbirdio/dashboard:latest",
		}

		plan := []string{
			strings.Join(serverCmd, " "),
			strings.Join(dashCmd, " "),
		}

		if !req.Plan {
			_ = exec.Command("podman", "rm", "-f", "allod-netbird-server").Run()
			_ = exec.Command("podman", "rm", "-f", "allod-netbird-dashboard").Run()

			outSrv, errSrv := exec.Command(serverCmd[0], serverCmd[1:]...).CombinedOutput()
			if errSrv != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore avvio allod-netbird-server: %v (%s)", errSrv, strings.TrimSpace(string(outSrv)))}
			}
			outDash, errDash := exec.Command(dashCmd[0], dashCmd[1:]...).CombinedOutput()
			if errDash != nil {
				return Response{Ok: false, Error: fmt.Sprintf("Errore avvio allod-netbird-dashboard: %v (%s)", errDash, strings.TrimSpace(string(outDash)))}
			}
			return Response{
				Ok:      true,
				Applied: true,
				Output:  fmt.Sprintf("Server NetBird gestito avviato con successo!\nManagement URL: %s\nDashboard: http://%s:%d", exposedURL, domain, dashPort),
				Plan:    plan,
			}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.server_down":
		plan := []string{
			"podman rm -f allod-netbird-server",
			"podman rm -f allod-netbird-dashboard",
		}
		if !req.Plan {
			_ = exec.Command("podman", "stop", "allod-netbird-server").Run()
			_ = exec.Command("podman", "rm", "-f", "allod-netbird-server").Run()
			_ = exec.Command("podman", "stop", "allod-netbird-dashboard").Run()
			_ = exec.Command("podman", "rm", "-f", "allod-netbird-dashboard").Run()
			return Response{Ok: true, Applied: true, Output: "Server NetBird gestito e Dashboard rimossi con successo", Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.server_status":
		plan := []string{
			"podman ps --filter name=allod-netbird-server --format {{.Names}}",
			"podman ps --filter name=allod-netbird-dashboard --format {{.Names}}",
		}
		if !req.Plan {
			srvRunning := false
			dashRunning := false
			if out, err := exec.Command("podman", "ps", "--filter", "name=allod-netbird-server", "--format", "{{.Names}}").Output(); err == nil {
				if strings.Contains(string(out), "allod-netbird-server") {
					srvRunning = true
				}
			}
			if out, err := exec.Command("podman", "ps", "--filter", "name=allod-netbird-dashboard", "--format", "{{.Names}}").Output(); err == nil {
				if strings.Contains(string(out), "allod-netbird-dashboard") {
					dashRunning = true
				}
			}
			statusPayload, _ := json.Marshal(map[string]interface{}{
				"server_running":    srvRunning,
				"dashboard_running": dashRunning,
				"running":           srvRunning && dashRunning,
			})
			return Response{Ok: true, Applied: true, Output: string(statusPayload), Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	case "network.netbird_down":
		if nbBin, err := exec.LookPath("netbird"); err == nil {
			fullCmd := []string{nbBin, "down"}
			plan := []string{strings.Join(fullCmd, " ")}
			if !req.Plan {
				out, _ := exec.Command(fullCmd[0], fullCmd[1:]...).CombinedOutput()
				_ = exec.Command("ip", "link", "delete", "wt0").Run()
				return Response{Ok: true, Applied: true, Output: strings.TrimSpace(string(out)), Plan: plan}
			}
			return Response{Ok: true, Applied: false, Plan: plan}
		}

		fullCmd := []string{"podman", "stop", "allod-netbird"}
		plan := []string{strings.Join(fullCmd, " ")}
		if !req.Plan {
			_ = exec.Command("podman", "stop", "allod-netbird").Run()
			_ = exec.Command("podman", "rm", "-f", "allod-netbird").Run()
			_ = exec.Command("ip", "link", "delete", "wt0").Run()
			return Response{Ok: true, Applied: true, Output: "allod-netbird fermato", Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	default:
		// Rifiuta tassativamente tutto ciò che non è nella lista chiusa
		return Response{Ok: false, Error: "Action not allowed: " + req.Action}
	}
}

// findNetBirdTarget looks for a NetBird container in root podman or in active user namespaces via runuser.
// Returns (containerName, runuserPrefix, isRunning, exists).
func findNetBirdTarget() (string, []string, bool, bool) {
	isMatch := func(name string) bool {
		return name == "allod-netbird" || name == "network" || name == "systemd-network" ||
			name == "network-netbird" || name == "systemd-network-netbird" ||
			strings.HasPrefix(name, "network-") || strings.HasPrefix(name, "systemd-network-")
	}

	// 1. Try root podman ps (running)
	if out, err := exec.Command("podman", "ps", "--format", "{{.Names}}").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if isMatch(name) {
				return name, nil, true, true
			}
		}
	}

	// 2. Try rootless users in /run/user/<uid> (running)
	if userDirs, err := filepath.Glob("/run/user/[0-9]*"); err == nil {
		for _, dir := range userDirs {
			uid := filepath.Base(dir)
			prefix := []string{"runuser", "-u", "#" + uid, "--"}
			args := append(prefix, "podman", "ps", "--format", "{{.Names}}")
			if out, err := exec.Command(args[0], args[1:]...).Output(); err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					name := strings.TrimSpace(line)
					if isMatch(name) {
						return name, prefix, true, true
					}
				}
			}
		}
	}

	// 3. Try root podman ps -a (exists but stopped)
	if out, err := exec.Command("podman", "ps", "-a", "--format", "{{.Names}}").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if isMatch(name) {
				return name, nil, false, true
			}
		}
	}

	// 4. Try rootless users in /run/user/<uid> (exists but stopped)
	if userDirs, err := filepath.Glob("/run/user/[0-9]*"); err == nil {
		for _, dir := range userDirs {
			uid := filepath.Base(dir)
			prefix := []string{"runuser", "-u", "#" + uid, "--"}
			args := append(prefix, "podman", "ps", "-a", "--format", "{{.Names}}")
			if out, err := exec.Command(args[0], args[1:]...).Output(); err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					name := strings.TrimSpace(line)
					if isMatch(name) {
						return name, prefix, false, true
					}
				}
			}
		}
	}

	return "allod-netbird", nil, false, false
}

// PatchSambaConfig ensures that smb.conf has multi-user isolation enabled:
// 1. [global] has "access based share enum = yes" so users only see shares they have access to
// 2. Every share with "valid users" has "access based share enum = yes" and "hide unreadable = yes"
// 3. Shared directories like [shares] or [public] have "hide unreadable = yes"
func PatchSambaConfig(content string) string {
	lines := strings.Split(content, "\n")
	var sectionOrder []string
	sectionLines := make(map[string][]string)
	var preLines []string
	currentSection := ""

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed[1 : len(trimmed)-1]
			sectionOrder = append(sectionOrder, currentSection)
			sectionLines[currentSection] = []string{l}
		} else if currentSection == "" {
			preLines = append(preLines, l)
		} else {
			sectionLines[currentSection] = append(sectionLines[currentSection], l)
		}
	}

	if len(sectionOrder) == 0 {
		if strings.TrimSpace(content) == "" {
			return "[global]\n   access based share enum = yes\n"
		}
		return "[global]\n   access based share enum = yes\n\n" + content
	}

	hasGlobal := false
	for _, sec := range sectionOrder {
		if strings.EqualFold(sec, "global") {
			hasGlobal = true
			break
		}
	}

	var output []string
	if !hasGlobal {
		output = append(output, "[global]", "   access based share enum = yes", "")
	}
	output = append(output, preLines...)

	for _, sec := range sectionOrder {
		secLower := strings.ToLower(sec)
		sLines := sectionLines[sec]

		if secLower == "global" {
			hasAccessBased := false
			for _, sl := range sLines {
				st := strings.TrimSpace(sl)
				if strings.HasPrefix(st, "access based share enum") {
					hasAccessBased = true
					break
				}
			}
			if !hasAccessBased {
				var newSec []string
				newSec = append(newSec, sLines[0])
				newSec = append(newSec, "   # Allod Multi-User Privacy & ACL Isolation")
				newSec = append(newSec, "   access based share enum = yes")
				newSec = append(newSec, sLines[1:]...)
				sLines = newSec
			}
		} else {
			hasValidUsers := false
			hasAccessBased := false
			hasHideUnreadable := false

			for _, sl := range sLines {
				st := strings.TrimSpace(sl)
				if strings.HasPrefix(st, "valid users") {
					hasValidUsers = true
				}
				if strings.HasPrefix(st, "access based share enum") {
					hasAccessBased = true
				}
				if strings.HasPrefix(st, "hide unreadable") {
					hasHideUnreadable = true
				}
			}

			if hasValidUsers {
				if !hasAccessBased {
					sLines = append(sLines, "   access based share enum = yes")
				}
				if !hasHideUnreadable {
					sLines = append(sLines, "   hide unreadable = yes")
				}
			} else if secLower == "shares" || secLower == "public" {
				if !hasHideUnreadable {
					sLines = append(sLines, "   hide unreadable = yes")
				}
			}
		}

		output = append(output, sLines...)
	}

	return strings.Join(output, "\n")
}

// AutoHealSambaConfig checks /etc/samba/smb.conf and patches it if needed.
func AutoHealSambaConfig() error {
	smbConf := "/etc/samba/smb.conf"
	data, err := os.ReadFile(smbConf)
	if err != nil {
		return nil // Samba not installed or file doesn't exist
	}
	original := string(data)
	patched := PatchSambaConfig(original)
	if patched != original {
		if err := os.WriteFile(smbConf, []byte(patched), 0644); err != nil {
			return err
		}
		systemctlBin := resolveExecutable("systemctl", "/bin/systemctl", "/usr/bin/systemctl")
		_ = exec.Command(systemctlBin, "restart", "smbd").Run()
	}
	return nil
}

// generateRandomKeyString generates a cryptographically secure random base64 string.
func generateRandomKeyString(numBytes int) string {
	b := make([]byte, numBytes)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
