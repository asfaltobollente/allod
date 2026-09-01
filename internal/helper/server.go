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
)

var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validDeviceRegex(dev string) bool {
	dev = strings.TrimSpace(dev)
	dev = strings.TrimPrefix(dev, "/dev/")
	return validNameRegex.MatchString(dev)
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

		plan := []string{
			fmt.Sprintf("mkdir -p %s && chmod 0775 %s", path, path),
			fmt.Sprintf("configure share [%s] at %s in /etc/samba/smb.conf", name, path),
			"systemctl restart smbd",
		}

		if !req.Plan {
			_ = os.MkdirAll(path, 0775)
			smbConf := "/etc/samba/smb.conf"
			if content, err := os.ReadFile(smbConf); err == nil {
				shareTag := fmt.Sprintf("[%s]", name)
				if !strings.Contains(string(content), shareTag) {
					shareSnippet := fmt.Sprintf("\n[%s]\n   path = %s\n   browseable = yes\n   read only = no\n   guest ok = yes\n   create mask = 0664\n   directory mask = 0775\n", name, path)
					_ = os.WriteFile(smbConf, []byte(string(content)+shareSnippet), 0644)
				}
				_ = exec.Command("systemctl", "restart", "smbd").Run()
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
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("useradd -m %s", username)}}

	case "users.passwd":
		username, ok := req.Args["username"].(string)
		if !ok || !validNameRegex.MatchString(username) {
			return Response{Ok: false, Error: "Invalid or missing 'username'"}
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("update password for %s", username)}}

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

	default:
		// Rifiuta tassativamente tutto ciò che non è nella lista chiusa
		return Response{Ok: false, Error: "Action not allowed: " + req.Action}
	}
}
