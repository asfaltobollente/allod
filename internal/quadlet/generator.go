package quadlet

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asfaltobollente/allod/internal/manifest"
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

const (
	legacyCloudPostgresPassword  = "allod_secure_pass"
	legacyPhotosPostgresPassword = "postgres"
)

func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	return string(bytes), nil
}

// EnsureModuleSecret ensures that <storage>/<module>/secrets/<key>.env exists with mode 0600 (dir 0700).
// If it does not exist, but existing DB data exists, it migrates using legacy credentials.
// Otherwise it generates a 32-character random alphanumeric secret.
func EnsureModuleSecret(module, key, legacyValue string) error {
	return ensureModuleSecret(module, key, legacyValue)
}

func ensureModuleSecret(module, key, legacyValue string) error {
	resBaseDir := ResolvedStorageBaseDir()
	secDir := filepath.Join(resBaseDir, module, "secrets")
	if err := os.MkdirAll(secDir, 0700); err != nil {
		return err
	}
	secFile := filepath.Join(secDir, key+".env")
	if _, err := os.Stat(secFile); err == nil {
		return nil
	}

	dbDir := filepath.Join(resBaseDir, module, "postgres")
	isLegacy := false
	if entries, err := os.ReadDir(dbDir); err == nil && len(entries) > 0 {
		isLegacy = true
	}

	var password string
	if isLegacy {
		password = legacyValue
		log.Printf("NOTICE: migrating existing %s database using legacy credentials to %s; consider rotating password", module, secFile)
	} else {
		genPass, err := generateRandomPassword(32)
		if err != nil {
			return fmt.Errorf("failed to generate random secret for %s/%s: %w", module, key, err)
		}
		password = genPass
	}

	var envContent string
	switch module {
	case "cloud":
		envContent = fmt.Sprintf("POSTGRES_HOST=cloud-postgres\nPOSTGRES_DB=nextcloud\nPOSTGRES_USER=nextcloud\nPOSTGRES_PASSWORD=%s\n", password)
	case "photos":
		envContent = fmt.Sprintf("DB_HOSTNAME=photos-postgres\nDB_DATABASE_NAME=immich\nDB_USERNAME=postgres\nDB_PASSWORD=%s\nREDIS_HOSTNAME=photos-valkey\nPOSTGRES_DB=immich\nPOSTGRES_USER=postgres\nPOSTGRES_PASSWORD=%s\nPOSTGRES_INITDB_ARGS=--data-checksums\n", password, password)
	default:
		envContent = fmt.Sprintf("PASSWORD=%s\n", password)
	}

	return os.WriteFile(secFile, []byte(envContent), 0600)
}

// EnsureStorageDirectories creates all host volume mount paths with permissive access.
func EnsureStorageDirectories(modID string) {
	baseDir := ResolvedStorageBaseDir()
	var dirs []string
	switch modID {
	case "cloud":
		dirs = []string{
			filepath.Join(baseDir, "cloud", "html"),
			filepath.Join(baseDir, "cloud", "data"),
			filepath.Join(baseDir, "cloud", "postgres"),
		}
		for _, d := range dirs {
			_ = os.MkdirAll(d, 0777)
			_ = os.Chmod(d, 0777)
		}
		_ = os.MkdirAll(filepath.Join(baseDir, "cloud", "secrets"), 0700)
		_ = ensureModuleSecret("cloud", "postgres", legacyCloudPostgresPassword)
		return
	case "photos":
		dirs = []string{
			filepath.Join(baseDir, "photos", "upload"),
			filepath.Join(baseDir, "photos", "postgres"),
			filepath.Join(baseDir, "photos", "valkey"),
		}
		for _, d := range dirs {
			_ = os.MkdirAll(d, 0777)
			_ = os.Chmod(d, 0777)
		}
		_ = os.MkdirAll(filepath.Join(baseDir, "photos", "secrets"), 0700)
		_ = ensureModuleSecret("photos", "postgres", legacyPhotosPostgresPassword)
		return
	case "backup":
		dirs = []string{
			filepath.Join(baseDir, "backup", "vault"),
		}
	case "shares":
		dirs = []string{
			filepath.Join(baseDir, "shares"),
			filepath.Join(baseDir, "shares", "public"),
			filepath.Join(baseDir, "shares", "public", "film"),
			filepath.Join(baseDir, "shares", "public", "musica"),
			filepath.Join(baseDir, "shares", "public", "serie"),
			filepath.Join(baseDir, "shares", "public", "movies"),
			filepath.Join(baseDir, "shares", "public", "tv"),
			filepath.Join(baseDir, "shares", "public", "music"),
		}
		for _, d := range dirs {
			_ = os.MkdirAll(d, 0777)
			_ = os.Chmod(d, 0777)
		}
	case "media":
		dirs = []string{
			filepath.Join(baseDir, "media", "data"),
			filepath.Join(baseDir, "media", "cache"),
			filepath.Join(baseDir, "shares"),
			filepath.Join(baseDir, "shares", "public"),
			filepath.Join(baseDir, "shares", "public", "film"),
			filepath.Join(baseDir, "shares", "public", "musica"),
			filepath.Join(baseDir, "shares", "public", "serie"),
			filepath.Join(baseDir, "shares", "public", "movies"),
			filepath.Join(baseDir, "shares", "public", "tv"),
			filepath.Join(baseDir, "shares", "public", "music"),
			filepath.Join(baseDir, "shares", "media", "movies"),
			filepath.Join(baseDir, "shares", "media", "tv"),
			filepath.Join(baseDir, "shares", "media", "music"),
			filepath.Join(baseDir, "shares", "movies"),
			filepath.Join(baseDir, "shares", "tv"),
			filepath.Join(baseDir, "shares", "music"),
			filepath.Join(baseDir, "shares", "film"),
			filepath.Join(baseDir, "shares", "musica"),
			filepath.Join(baseDir, "media", "config"),
		}
		for _, d := range dirs {
			_ = os.MkdirAll(d, 0777)
			_ = os.Chmod(d, 0777)
		}
	case "network":
		nbCfgDir := filepath.Join(baseDir, "network", "netbird")
		netSecretsDir := filepath.Join(baseDir, "network", "secrets")
		dirs = []string{
			nbCfgDir,
		}
		for _, d := range dirs {
			_ = os.MkdirAll(d, 0777)
		}
		_ = os.MkdirAll(netSecretsDir, 0700)
		netbirdEnv := filepath.Join(netSecretsDir, "netbird.env")
		if _, err := os.Stat(netbirdEnv); err != nil {
			defaultNetBirdEnv := "# NetBird Sovereign Mesh Configuration\nNB_SETUP_KEY=\nNB_MANAGEMENT_URL=\n"
			_ = os.WriteFile(netbirdEnv, []byte(defaultNetBirdEnv), 0600)
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
	if env := os.Getenv("ALLOD_STORAGE_DIR"); env != "" {
		return env
	}
	if _, err := os.Stat("/mnt/allod-storage"); err == nil {
		return "/mnt/allod-storage"
	}
	return "%h/.local/share/allod/storage"
}

// ResolvedStorageBaseDir returns the actual filesystem path on disk for host operations,
// resolving systemd's %h specifier to the user's home directory.
func ResolvedStorageBaseDir() string {
	baseDir := StorageBaseDir()
	if strings.HasPrefix(baseDir, "%h") {
		if home, err := os.UserHomeDir(); err == nil {
			rel := strings.TrimPrefix(baseDir, "%h")
			rel = strings.TrimPrefix(rel, "/")
			rel = strings.TrimPrefix(rel, "\\")
			return filepath.Join(home, rel)
		}
	}
	return baseDir
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
	if strings.Contains(img.Ref, "cloudflared") || strings.Contains(img.Ref, "netbird") || m.ID == "network" {
		sb.WriteString("Network=host\n")
	} else {
		sb.WriteString("Network=allod\n")
	}
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
			if p.Scope == "localhost" || p.Scope == "loopback" {
				sb.WriteString(fmt.Sprintf("PublishPort=127.0.0.1:%d:%d\n", p.N, cPort))
			} else {
				sb.WriteString(fmt.Sprintf("PublishPort=%d:%d\n", p.N, cPort))
			}
		}
	}

	// Persistent Storage Volumes
	baseDir := StorageBaseDir()
	switch m.ID {
	case "cloud":
		_ = ensureModuleSecret("cloud", "postgres", legacyCloudPostgresPassword)
		if isPrimary {
			sb.WriteString(fmt.Sprintf("Volume=%s/cloud/html:/var/www/html:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("Volume=%s/cloud/data:/var/www/html/data:Z\n", baseDir))
			if strings.Contains(img.Ref, "nextcloud") {
				sb.WriteString(fmt.Sprintf("EnvironmentFile=%s/cloud/secrets/postgres.env\n", baseDir))
			}
		} else if strings.Contains(img.Ref, "postgres") {
			sb.WriteString(fmt.Sprintf("Volume=%s/cloud/postgres:/var/lib/postgresql/data:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("EnvironmentFile=%s/cloud/secrets/postgres.env\n", baseDir))
		}
	case "photos":
		_ = ensureModuleSecret("photos", "postgres", legacyPhotosPostgresPassword)
		if isPrimary {
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/upload:/usr/src/app/upload:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/upload:/data:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("EnvironmentFile=%s/photos/secrets/postgres.env\n", baseDir))
		} else if strings.Contains(img.Ref, "postgres") {
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/postgres:/var/lib/postgresql/data:Z\n", baseDir))
			sb.WriteString(fmt.Sprintf("EnvironmentFile=%s/photos/secrets/postgres.env\n", baseDir))
			sb.WriteString("ShmSize=128m\n")
		} else if strings.Contains(img.Ref, "valkey") {
			sb.WriteString(fmt.Sprintf("Volume=%s/photos/valkey:/data:Z\n", baseDir))
		}
	case "backup":
		sb.WriteString(fmt.Sprintf("Volume=%s/backup/vault:/data:Z\n", baseDir))
	case "media":
		sb.WriteString(fmt.Sprintf("Volume=%s/media/config:/config:Z\n", baseDir))
		sb.WriteString(fmt.Sprintf("Volume=%s/shares/public:/media:z\n", baseDir))
		sb.WriteString(fmt.Sprintf("Volume=%s/shares/public:/shares/public:z\n", baseDir))
		sb.WriteString(fmt.Sprintf("Volume=%s/shares:/shares:z\n", baseDir))
	case "network":
		sb.WriteString(fmt.Sprintf("Volume=%s/network/netbird:/etc/netbird:Z\n", baseDir))
		sb.WriteString(fmt.Sprintf("EnvironmentFile=%s/network/secrets/netbird.env\n", baseDir))
	default:
		sb.WriteString(fmt.Sprintf("Volume=%s/%s:/data:Z\n", baseDir, m.ID))
	}

	if m.Privileges.Userns == "host" {
		sb.WriteString("UserNS=host\n")
	}
	for _, dev := range m.Privileges.Devices {
		if _, err := os.Stat(dev); err == nil || dev == "/dev/net/tun" || strings.HasPrefix(dev, "/dev/net/") {
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

// GetRunningContainers returns a set of container names currently running in rootless Podman.
func GetRunningContainers() map[string]bool {
	active := make(map[string]bool)
	out, err := exec.Command("podman", "ps", "--format", "{{.Names}}").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			name := strings.TrimSpace(l)
			if name != "" {
				active[name] = true
			}
		}
	}
	return active
}

// IsModuleRunning checks if any container or systemd unit belonging to modID is actively running.
func IsModuleRunning(modID string, runningContainers map[string]bool) bool {
	if runningContainers == nil {
		runningContainers = GetRunningContainers()
	}
	for cName := range runningContainers {
		if cName == modID || strings.HasPrefix(cName, modID+"-") ||
			cName == "systemd-"+modID || strings.HasPrefix(cName, "systemd-"+modID+"-") {
			return true
		}
	}
	out, err := exec.Command("systemctl", "--user", "is-active", modID).Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		return true
	}
	return false
}

// StopAndRemoveContainers forcibly stops and terminates all systemd units and Podman containers
// associated with a module (including sub-containers like -postgres, -valkey, -headscale, etc.).
// If removeUnits is true, it also cleans up generated Quadlet .container and .service files.
func StopAndRemoveContainers(modID string, removeUnits bool) error {
	// 1. Gather all potential container names from active podman ps -a
	targets := make(map[string]bool)
	out, err := exec.Command("podman", "ps", "-a", "--format", "{{.Names}}").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			if name == modID || strings.HasPrefix(name, modID+"-") ||
				name == "systemd-"+modID || strings.HasPrefix(name, "systemd-"+modID+"-") {
				targets[name] = true
			}
		}
	}

	// Also always include well-known standard names
	standardNames := []string{
		modID,
		"systemd-" + modID,
		modID + "-postgres",
		"systemd-" + modID + "-postgres",
		modID + "-valkey",
		"systemd-" + modID + "-valkey",
		modID + "-netbird",
		"systemd-" + modID + "-netbird",
		modID + "-headscale",
		"systemd-" + modID + "-headscale",
		modID + "-cloudflared",
		"systemd-" + modID + "-cloudflared",
	}
	for _, sn := range standardNames {
		targets[sn] = true
	}

	// 2. Stop systemd units first
	for t := range targets {
		if !strings.HasPrefix(t, "systemd-") {
			_ = exec.Command("systemctl", "--user", "stop", t).Run()
		}
	}

	// 3. Forcibly stop and remove all matching Podman containers
	for t := range targets {
		_ = exec.Command("podman", "stop", "-t", "2", t).Run()
		_ = exec.Command("podman", "rm", "-f", t).Run()
	}

	// 4. Remove Quadlet and systemd unit files if requested
	home, _ := os.UserHomeDir()
	if removeUnits && home != "" {
		quadDir := filepath.Join(home, ".config", "containers", "systemd")
		systemdUserDir := filepath.Join(home, ".config", "systemd", "user")

		for _, dir := range []string{quadDir, systemdUserDir} {
			if entries, err := os.ReadDir(dir); err == nil {
				for _, e := range entries {
					name := e.Name()
					if strings.HasPrefix(name, modID+".") || strings.HasPrefix(name, modID+"-") {
						_ = os.Remove(filepath.Join(dir, name))
					}
				}
			}
		}
	}

	// 5. Remove stale CID files
	uid := os.Getuid()
	cidDir := fmt.Sprintf("/run/user/%d", uid)
	if entries, err := os.ReadDir(cidDir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), modID) && strings.HasSuffix(e.Name(), ".cid") {
				_ = os.Remove(filepath.Join(cidDir, e.Name()))
			}
		}
	}

	// 6. Reload systemd daemon & reset failed
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	_ = exec.Command("systemctl", "--user", "reset-failed").Run()

	return nil
}
