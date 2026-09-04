package quadlet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/allod-project/allod/internal/manifest"
)

// GenerateResult holds one or more generated Quadlet unit files for a module.
type GenerateResult struct {
	// Files maps filename (e.g. "photos.container") to file content.
	Files map[string]string
	// IsNative is true for modules that run on the host without a container image.
	IsNative bool
}

// Generate produces the Quadlet unit files for a module.
// For multi-image modules it generates one .container per image.
// For native modules (no images) it generates a .service unit instead.
func Generate(modID string, m *manifest.Manifest, levelName string) (*GenerateResult, error) {
	if levelName == "off" {
		return &GenerateResult{Files: map[string]string{}}, nil
	}

	level, exists := m.Levels[levelName]
	if !exists {
		return nil, fmt.Errorf("level %s not found in manifest for %s", levelName, modID)
	}

	result := &GenerateResult{Files: make(map[string]string)}

	EnsureStorageDirectories(modID)

	if len(m.Images) == 0 {
		// Native module: generate a .service unit instead of .container
		result.IsNative = true
		content := generateService(modID, m, level)
		result.Files[modID+".service"] = content
		return result, nil
	}

	if len(m.Images) == 1 {
		// Single-image module: generate one .container with ports
		content := generateContainer(modID, m, m.Images[0], level, true)
		result.Files[modID+".container"] = content
		return result, nil
	}

	// Multi-image module: generate one .container per image
	for i, img := range m.Images {
		isPrimary := (i == 0)
		suffix := ""
		if !isPrimary {
			parts := strings.Split(img.Ref, "/")
			shortName := parts[len(parts)-1]
			suffix = "-" + shortName
		}
		filename := modID + suffix + ".container"
		content := generateContainer(modID+suffix, m, img, level, isPrimary)
		result.Files[filename] = content
	}

	return result, nil
}

// EnsureStorageDirectories creates all host volume mount paths with permissive access.
func EnsureStorageDirectories(modID string) {
	baseDir := StorageBaseDir()
	var dirs []string
	switch modID {
	case "cloud":
		dirs = []string{
			filepath.Join(baseDir, "cloud", "html"),
			filepath.Join(baseDir, "cloud", "data"),
			filepath.Join(baseDir, "cloud", "postgres"),
		}
	case "photos":
		dirs = []string{
			filepath.Join(baseDir, "photos", "upload"),
			filepath.Join(baseDir, "photos", "postgres"),
			filepath.Join(baseDir, "photos", "valkey"),
		}
	case "backup":
		dirs = []string{
			filepath.Join(baseDir, "backup", "vault"),
		}
	case "shares":
		dirs = []string{
			filepath.Join(baseDir, "shares", "public"),
		}
	case "media":
		dirs = []string{
			filepath.Join(baseDir, "shares", "media", "movies"),
			filepath.Join(baseDir, "shares", "media", "tv"),
			filepath.Join(baseDir, "media", "config"),
		}
	case "network":
		hsCfgDir := filepath.Join(baseDir, "network", "headscale", "config")
		hsDataDir := filepath.Join(baseDir, "network", "headscale", "data")
		dirs = []string{
			hsCfgDir,
			hsDataDir,
		}
		for _, d := range dirs {
			_ = os.MkdirAll(d, 0777)
		}
		cfgFile := filepath.Join(hsCfgDir, "config.yaml")
		if _, err := os.Stat(cfgFile); err != nil {
			defaultHeadscaleYaml := `server_url: http://127.0.0.1:8085
listen_addr: 0.0.0.0:8085
metrics_listen_addr: 0.0.0.0:9095
grpc_listen_addr: 0.0.0.0:50443
grpc_allow_insecure: true
ip_prefixes:
  - 100.64.0.0/10
  - fd7a:115c:a1e0::/48
derp:
  server:
    enabled: true
    region_id: 999
    region_code: "allod"
    region_name: "Allod Embedded DERP"
    stun_listen_addr: "0.0.0.0:3478"
  urls:
    - https://controlplane.tailscale.com/derpmap/default
  auto_update_enabled: true
  update_frequency: 24h
disable_check_updates: true
ephemeral_node_inactivity_timeout: 30m
database:
  type: sqlite3
  sqlite:
    path: /var/lib/headscale/db.sqlite
dns:
  magic_dns: true
  base_domain: mesh.allod
  nameservers:
    split: {}
    global:
      - 1.1.1.1
      - 8.8.8.8
`
			_ = os.WriteFile(cfgFile, []byte(defaultHeadscaleYaml), 0644)
		}
	default:
		dirs = []string{
			filepath.Join(baseDir, modID),
		}
	}

	for _, d := range dirs {
		_ = os.MkdirAll(d, 0777)
	}
}

// StorageBaseDir determines the root directory for persistent module volumes.
// If /mnt/allod-storage (the Btrfs NAS pool) exists, it is preferred.
func StorageBaseDir() string {
	if _, err := os.Stat("/mnt/allod-storage"); err == nil {
		return "/mnt/allod-storage"
	}
	return "%h/.local/share/allod/storage"
}

// GenerateNetwork returns the Quadlet .network definition for inter-container communication.
func GenerateNetwork() string {
	return `[Network]
NetworkName=allod
`
}

// EnsureAllodNetwork creates allod.network in the systemd quadlet directory if missing.
func EnsureAllodNetwork(outDir string) {
	_ = os.MkdirAll(outDir, 0755)
	_ = exec.Command("podman", "network", "create", "allod").Run()
	netFile := filepath.Join(outDir, "allod.network")
	if _, err := os.Stat(netFile); err != nil {
		_ = os.WriteFile(netFile, []byte(GenerateNetwork()), 0644)
	}
}

func generateContainer(unitName string, m *manifest.Manifest, img manifest.Image, level manifest.Level, isPrimary bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Generated by allod for unit: %s\n", unitName))
	sb.WriteString("[Unit]\n")
	sb.WriteString(fmt.Sprintf("Description=Allod Module: %s\n", unitName))
	sb.WriteString("After=network-online.target\n")
	if isPrimary && len(m.Images) > 1 {
		for i := 1; i < len(m.Images); i++ {
			parts := strings.Split(m.Images[i].Ref, "/")
			shortName := parts[len(parts)-1]
			sb.WriteString(fmt.Sprintf("Requires=%s-%s.service\n", m.ID, shortName))
			sb.WriteString(fmt.Sprintf("After=%s-%s.service\n", m.ID, shortName))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("[Container]\n")
	sb.WriteString(fmt.Sprintf("Image=%s:%s\n", img.Ref, img.Tag))
	sb.WriteString(fmt.Sprintf("ContainerName=%s\n", unitName))
	sb.WriteString("Network=allod\n")
	if len(img.Args) > 0 {
		sb.WriteString(fmt.Sprintf("Exec=%s\n", strings.Join(img.Args, " ")))
	}

	// Only publish host ports on the primary container of the module.
	// Secondary containers (databases, caches) communicate via Network=allod DNS.
	if isPrimary {
		for _, p := range m.Ports {
			cPort := p.N
			if p.ContainerPort > 0 {
				cPort = p.ContainerPort
			}
			sb.WriteString(fmt.Sprintf("PublishPort=%d:%d\n", p.N, cPort))
		}
	}

	// Persistent Storage Volumes
	baseDir := StorageBaseDir()
	switch m.ID {
	case "cloud":
		if isPrimary {
			sb.WriteString(fmt.Sprintf("Volume=%s/cloud/html:/var/www/html:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("Volume=%s/cloud/data:/var/www/html/data:Z\n", baseDir))
			if strings.Contains(img.Ref, "nextcloud") {
				sb.WriteString("Environment=POSTGRES_HOST=cloud-postgres\n")
				sb.WriteString("Environment=POSTGRES_DB=nextcloud\n")
				sb.WriteString("Environment=POSTGRES_USER=nextcloud\n")
				sb.WriteString("Environment=POSTGRES_PASSWORD=allod_secure_pass\n")
			}
		} else if strings.Contains(img.Ref, "postgres") {
			sb.WriteString(fmt.Sprintf("Volume=%s/cloud/postgres:/var/lib/postgresql/data:Z\n", baseDir))
			sb.WriteString("Environment=POSTGRES_DB=nextcloud\n")
			sb.WriteString("Environment=POSTGRES_USER=nextcloud\n")
			sb.WriteString("Environment=POSTGRES_PASSWORD=allod_secure_pass\n")
		}
	case "photos":
		if isPrimary {
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/upload:/usr/src/app/upload:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/upload:/data:Z\n", baseDir))
			sb.WriteString("Environment=DB_HOSTNAME=photos-postgres\n")
			sb.WriteString("Environment=DB_DATABASE_NAME=immich\n")
			sb.WriteString("Environment=DB_USERNAME=postgres\n")
			sb.WriteString("Environment=DB_PASSWORD=postgres\n")
			sb.WriteString("Environment=REDIS_HOSTNAME=photos-valkey\n")
		} else if strings.Contains(img.Ref, "postgres") {
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/postgres:/var/lib/postgresql/data:Z\n", baseDir))
			sb.WriteString("Environment=POSTGRES_DB=immich\n")
			sb.WriteString("Environment=POSTGRES_USER=postgres\n")
			sb.WriteString("Environment=POSTGRES_PASSWORD=postgres\n")
			sb.WriteString("Environment=POSTGRES_INITDB_ARGS=--data-checksums\n")
			sb.WriteString("ShmSize=128m\n")
		} else if strings.Contains(img.Ref, "valkey") {
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/valkey:/data:Z\n", baseDir))
		}
	case "backup":
		sb.WriteString(fmt.Sprintf("Volume=%s/backup/vault:/data:Z\n", baseDir))
	case "media":
		sb.WriteString(fmt.Sprintf("Volume=%s/media/config:/config:Z\n", baseDir))
		sb.WriteString(fmt.Sprintf("Volume=%s/shares/media:/media:Z\n", baseDir))
	case "network":
		if strings.Contains(img.Ref, "headscale") {
			sb.WriteString(fmt.Sprintf("Volume=%s/network/headscale/config:/etc/headscale:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("Volume=%s/network/headscale/data:/var/lib/headscale:Z\n", baseDir))
			sb.WriteString("Exec=serve\n")
		} else if strings.Contains(img.Ref, "cloudflared") {
			sb.WriteString("Exec=tunnel --no-autoupdate run\n")
			tokenFile := filepath.Join(baseDir, "network", "cloudflared.token")
			if tokBytes, err := os.ReadFile(tokenFile); err == nil && len(strings.TrimSpace(string(tokBytes))) > 0 {
				sb.WriteString(fmt.Sprintf("Environment=TUNNEL_TOKEN=%s\n", strings.TrimSpace(string(tokBytes))))
			}
		}
	default:
		sb.WriteString(fmt.Sprintf("Volume=%s/%s:/data:Z\n", baseDir, m.ID))
	}

	if m.Privileges.Userns == "host" {
		sb.WriteString("UserNS=host\n")
	}
	for _, dev := range m.Privileges.Devices {
		if _, err := os.Stat(dev); err == nil {
			sb.WriteString(fmt.Sprintf("AddDevice=%s\n", dev))
		}
	}
	for _, cap := range m.Privileges.Caps {
		sb.WriteString(fmt.Sprintf("AddCapability=%s\n", cap))
	}

	sb.WriteString("\n[Service]\n")
	sb.WriteString("Restart=always\n")
	sb.WriteString("RestartSec=5s\n")
	sb.WriteString("TimeoutStartSec=900s\n")
	sb.WriteString("SuccessExitStatus=0 143 137\n")
	if level.RAMMB > 0 {
		sb.WriteString(fmt.Sprintf("MemoryMax=%dM\n", level.RAMMB))
	}

	sb.WriteString("\n[Install]\n")
	sb.WriteString("WantedBy=default.target\n")

	return sb.String()
}

func generateService(modID string, m *manifest.Manifest, level manifest.Level) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Generated by allod for native module: %s\n", modID))
	sb.WriteString("[Unit]\n")
	sb.WriteString(fmt.Sprintf("Description=Allod Native Module: %s\n", modID))
	sb.WriteString("After=network-online.target\n\n")

	sb.WriteString("[Service]\n")
	sb.WriteString("Type=oneshot\n")
	sb.WriteString("RemainAfterExit=yes\n")
	sb.WriteString("ExecStart=/bin/true\n")
	if level.RAMMB > 0 {
		sb.WriteString(fmt.Sprintf("MemoryMax=%dM\n", level.RAMMB))
	}

	sb.WriteString("\n[Install]\n")
	sb.WriteString("WantedBy=default.target\n")

	return sb.String()
}

// GenerateContainer is the legacy single-file API kept for backward compatibility.
func GenerateContainer(modID string, m *manifest.Manifest, levelName string) (string, error) {
	res, err := Generate(modID, m, levelName)
	if err != nil {
		return "", err
	}
	for _, content := range res.Files {
		return content, nil
	}
	return "", nil
}
