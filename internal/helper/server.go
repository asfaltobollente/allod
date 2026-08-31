package helper

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

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

func (s *Server) Start() error {
	os.Remove(s.SocketPath) // Pulizia vecchio socket
	l, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		// Fallback locale per test se unix socket fallisce su win
		l, err = net.Listen("tcp", "127.0.0.1:40000")
		if err != nil {
			return err
		}
		fmt.Println("Ascolto su TCP 127.0.0.1:40000 (Fallback)")
	} else {
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
			return Response{Ok: false, Error: "Invalid or missing 'name' (must be alphanumeric/hyphens)"}
		}
		path, _ := req.Args["path"].(string)
		if path != "" && (strings.Contains(path, "..") || (!strings.HasPrefix(path, "/") && !filepath.IsAbs(path))) {
			return Response{Ok: false, Error: "Invalid 'path' (must be absolute without traversal)"}
		}

		plan := []string{
			fmt.Sprintf("systemctl stop smb-%s", name),
			fmt.Sprintf("configure share %s at %s", name, path),
			fmt.Sprintf("systemctl start smb-%s", name),
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
		serial, ok := req.Args["serial"].(string)
		if !ok || !validNameRegex.MatchString(serial) {
			return Response{Ok: false, Error: "Invalid or missing 'serial' identifier"}
		}
		return Response{Ok: true, Applied: !req.Plan, Plan: []string{fmt.Sprintf("wipefs and format disk with serial %s", serial)}}

	default:
		// Rifiuta tassativamente tutto ciò che non è nella lista chiusa
		return Response{Ok: false, Error: "Action not allowed: " + req.Action}
	}
}
