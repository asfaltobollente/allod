package helper

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validDeviceRegex(dev string) bool {
	dev = strings.TrimSpace(dev)
	dev = strings.TrimPrefix(dev, "/dev/")
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

func ensureLinuxUser(username string) error {
	if !validNameRegex.MatchString(username) {
		return fmt.Errorf("invalid username '%s'", username)
	}

	// 1. Check if user already exists in Linux (/etc/passwd)
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

type Server struct {
	SocketPath string
}

type Client struct {
	SocketPath string
}

func (c *Client) Execute(action string, args map[string]interface{}, plan bool) (Response, error) {
	sock := c.SocketPath
	if sock == "" {
		sock = "/run/allod/helper.sock"
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		conn, err = net.Dial("unix", "allod-helper.sock")
		if err != nil {
			conn, err = net.Dial("tcp", "127.0.0.1:40000")
			if err != nil {
				return Response{Ok: false, Error: "cannot connect to helper"}, err
			}
		}
	}
	defer conn.Close()

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
	if dir := filepath.Dir(s.SocketPath); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	l, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		// Fallback locale per test se unix socket fallisce su win
		l, err = net.Listen("tcp", "127.0.0.1:40000")
		if err != nil {
			return err
		}
		fmt.Println("Ascolto su TCP 127.0.0.1:40000 (Fallback)")
	} else {
		_ = os.Chmod(s.SocketPath, 0666)
		fmt.Println("Ascolto su UNIX Socket:", s.SocketPath)
	}
	defer l.Close()

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
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Ok: false, Error: "Invalid JSON format"})
		return
	}

	res := s.processRequest(req)
	encoder.Encode(res)
}

func (s *Server) processRequest(req Request) Response {
	// Lista chiusa di 9 azioni come da helper-api.schema.json
	switch req.Action {
	case "shares.apply":
		name, ok := req.Args["name"].(string)
		if !ok || !validNameRegex.MatchString(name) {
			name = "shares"
		}
		path, _ := req.Args["path"].(string)
		if path == "" {
			path = "/mnt/allod-storage/shares"
		}
		if path != "" && (strings.Contains(path, "..") || (!strings.HasPrefix(path, "/") && !filepath.IsAbs(path))) {
			return Response{Ok: false, Error: "Invalid 'path' (must be absolute without traversal)"}
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
						publicSnippet := fmt.Sprintf("\n[public]\n   comment = Cartella pubblica Allod (Jellyfin Media)\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = yes\n   create mask = 0666\n   directory mask = 0777\n   force create mode = 0666\n   force directory mode = 0777\n", pubPath)
						sContent += publicSnippet
					}

					// Configure default share (e.g. [shares])
					shareTag := fmt.Sprintf("[%s]", name)
					if !strings.Contains(sContent, shareTag) {
						shareSnippet := fmt.Sprintf("\n[%s]\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = yes\n   create mask = 0666\n   directory mask = 0777\n   force create mode = 0666\n   force directory mode = 0777\n", name, path)
						sContent += shareSnippet
					}

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
		if subvol != "" && !validNameRegex.MatchString(subvol) {
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

			// Ensure private share configuration in /etc/samba/smb.conf
			smbConf := "/etc/samba/smb.conf"
			if content, err := os.ReadFile(smbConf); err == nil {
				userTag := fmt.Sprintf("[%s]", username)
				if !strings.Contains(string(content), userTag) {
					userSnippet := fmt.Sprintf("\n[%s]\n   comment = Cartella privata di %s\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = no\n   valid users = %s\n   create mask = 0660\n   directory mask = 0770\n", username, username, userSharePath, username)
					_ = os.WriteFile(smbConf, []byte(string(content)+userSnippet), 0644)
				}
			}

			systemctlBin := resolveExecutable("systemctl", "/bin/systemctl", "/usr/bin/systemctl")
			_ = exec.Command(systemctlBin, "reload", "smbd").Run()
		}

		return Response{Ok: true, Applied: !req.Plan, Plan: plan}

	case "firewall.apply":
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{"nftables reload /etc/allod/nftables.conf"}}

	case "smart.read":
		disk, ok := req.Args["disk"].(string)
		if !ok || !validNameRegex.MatchString(disk) {
			return Response{Ok: false, Error: "Invalid or missing 'disk' identifier"}
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("smartctl -H /dev/disk/by-id/%s", disk)}}

	case "service.restart":
		unit, ok := req.Args["unit"].(string)
		if !ok || !validNameRegex.MatchString(unit) {
			return Response{Ok: false, Error: "Invalid or missing 'unit' name"}
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
				for _, cand := range candidates {
					if info, err := os.Stat(cand); err == nil && !info.IsDir() {
						if data, err := os.ReadFile(cand); err == nil {
							_ = os.WriteFile("/usr/local/bin/allod-helperd", data, 0755)
						}
						break
					}
				}

				// 2. Try restarting systemd service
				systemctlBin := resolveExecutable("systemctl", "/bin/systemctl", "/usr/bin/systemctl")
				if err := exec.Command(systemctlBin, "restart", "allod-helperd").Run(); err == nil {
					return Response{Ok: true, Applied: true, Plan: []string{"systemctl restart allod-helperd"}}
				}

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
				if s, ok := d.(string); ok && validDeviceRegex(s) {
					disks = append(disks, s)
				}
			}
		} else if dStr, ok := req.Args["disks"].(string); ok {
			for _, s := range strings.Split(dStr, ",") {
				s = strings.TrimSpace(s)
				if validDeviceRegex(s) {
					disks = append(disks, s)
				}
			}
		}

		if len(disks) == 0 {
			return Response{Ok: false, Error: "Nessun disco valido specificato per storage.init"}
		}

		mode, _ := req.Args["mode"].(string)
		if mode != "single" {
			mode = "raid1"
		}

		mountPoint, _ := req.Args["mount"].(string)
		if mountPoint == "" {
			mountPoint = "/mnt/allod-storage"
		}

		username, _ := req.Args["user"].(string)
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

	case "network.headscale_cli":
		cmdType, _ := req.Args["command"].(string)

		// Rileva dinamicamente il nome del container Headscale (network o network-headscale)
		targetContainer := "network"
		if out, err := exec.Command("podman", "ps", "--format", "{{.Names}}").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				name := strings.TrimSpace(line)
				if name == "network-headscale" || name == "systemd-network-headscale" {
					targetContainer = name
					break
				}
				if name == "network" || name == "systemd-network" {
					targetContainer = name
				}
			}
		}

		var args []string
		switch cmdType {
		case "preauthkey_create":
			// Assicura che l'utente 'default' esista in Headscale
			_ = exec.Command("podman", "exec", targetContainer, "headscale", "users", "create", "default").Run()
			args = []string{"exec", targetContainer, "headscale", "preauthkeys", "create", "-u", "default", "--reusable=false", "--expiration", "1h"}
		case "nodes_list":
			args = []string{"exec", targetContainer, "headscale", "nodes", "list", "--output", "json"}
		case "users_list":
			args = []string{"exec", targetContainer, "headscale", "users", "list", "--output", "json"}
		default:
			return Response{Ok: false, Error: "Comando Headscale non consentito: " + cmdType}
		}

		plan := []string{fmt.Sprintf("podman %s", strings.Join(args, " "))}
		if !req.Plan {
			out, err := exec.Command("podman", args...).CombinedOutput()
			if err != nil {
				// Fallback su comando host se disponibile
				if len(args) > 3 {
					directArgs := args[3:]
					if out2, err2 := exec.Command("headscale", directArgs...).CombinedOutput(); err2 == nil {
						return Response{Ok: true, Applied: true, Output: strings.TrimSpace(string(out2)), Plan: plan}
					}
				}
				return Response{Ok: false, Error: fmt.Sprintf("Errore esecuzione Headscale: %v (%s)", err, strings.TrimSpace(string(out)))}
			}
			return Response{Ok: true, Applied: true, Output: strings.TrimSpace(string(out)), Plan: plan}
		}
		return Response{Ok: true, Applied: false, Plan: plan}

	default:
		// Rifiuta tassativamente tutto ciò che non è nella lista chiusa
		return Response{Ok: false, Error: "Action not allowed: " + req.Action}
	}
}
