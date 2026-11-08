package backup

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/asfaltobollente/allod/internal/config"
	"github.com/asfaltobollente/allod/internal/ring"
	"github.com/asfaltobollente/allod/internal/state"
)

var (
	ErrNoRemoteVault = errors.New("nessun vault remoto collegato nel Ring")
	ErrNoPaths       = errors.New("nessun percorso valido selezionato per il backup")
	ErrResticMissing = errors.New("il client restic non è installato sul sistema")
	ErrAlreadyRun    = errors.New("un processo di backup è già in esecuzione")
)

// Snapshot represents a verified snapshot stored on a remote vault.
type Snapshot struct {
	ID            string    `json:"id"`
	ShortID       string    `json:"short_id"`
	Time          time.Time `json:"time"`
	TimeFormatted string    `json:"time_formatted"`
	Paths         []string  `json:"paths"`
	Hostname      string    `json:"hostname"`
	Username      string    `json:"username"`
	SizeFormatted string    `json:"size_formatted,omitempty"`
	FilesCount    int       `json:"files_count,omitempty"`
}

// Status represents the comprehensive backup status across the node and vaults.
type Status struct {
	IsRunning       bool       `json:"is_running"`
	CurrentAction   string     `json:"current_action,omitempty"`
	CurrentProgress string     `json:"current_progress,omitempty"`
	LastRun         string     `json:"last_run,omitempty"`
	LastStatus      string     `json:"last_status"` // "never", "running", "success", "error"
	LastError       string     `json:"last_error,omitempty"`
	LastSnapshotID  string     `json:"last_snapshot_id,omitempty"`
	RemotePeerName  string     `json:"remote_peer_name,omitempty"`
	RemoteAddress   string     `json:"remote_address,omitempty"`
	RemotePort      int        `json:"remote_port,omitempty"`
	SelectedPaths   []string   `json:"selected_paths"`
	TotalSnapshots    int        `json:"total_snapshots"`
	Snapshots         []Snapshot `json:"snapshots"`
	EncryptionMode    string     `json:"encryption_mode"`
	HasRestic         bool       `json:"has_restic"`
	CLIUpdateOneliner string     `json:"cli_update_oneliner,omitempty"`
	PrimaryMeshIP     string     `json:"primary_mesh_ip,omitempty"`
}

// Engine coordinates client-side Restic backups towards remote append-only vaults.
type Engine struct {
	dbPath     string
	configPath string
	ringPath   string

	mu              sync.Mutex
	isRunning       bool
	currentAction   string
	currentProgress string
	lastError       string
}

// NewEngine initializes a backup coordinator instance.
func NewEngine(dbPath, configPath, ringPath string) *Engine {
	if dbPath == "" {
		dbPath = "state.db"
	}
	if configPath == "" {
		configPath = "config.yaml"
	}
	if ringPath == "" {
		ringPath = "ring.yaml"
	}
	return &Engine{
		dbPath:     dbPath,
		configPath: configPath,
		ringPath:   ringPath,
	}
}

// FindRestic looks for the restic executable on the host system.
func FindRestic() string {
	if p, err := exec.LookPath("restic"); err == nil && p != "" {
		return p
	}
	candidates := []string{
		"/usr/bin/restic",
		"/usr/local/bin/restic",
		"/bin/restic",
		"/opt/allod/bin/restic",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// GetOrCreateEncryptionKey fetches the sovereign AES-256 backup key or creates one.
func (e *Engine) GetOrCreateEncryptionKey(st *state.Store) (string, error) {
	if st == nil {
		return "", errors.New("database state store is nil")
	}
	existing, err := st.GetMeta("backup_encryption_key")
	if err == nil && strings.TrimSpace(existing) != "" {
		return strings.TrimSpace(existing), nil
	}

	buf := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("failed generating cryptographic key: %w", err)
	}
	newKey := hex.EncodeToString(buf)
	if err := st.SetMeta("backup_encryption_key", newKey); err != nil {
		return "", fmt.Errorf("failed persisting encryption key: %w", err)
	}
	return newKey, nil
}

// FindRemoteVault scans the ring topology for the first active remote peer vault.
func (e *Engine) FindRemoteVault(cfg *config.Config) (name string, addr string, port int, err error) {
	topo, err := ring.LoadTopology(e.ringPath)
	if err != nil || topo == nil || len(topo.Members) <= 1 {
		return "", "", 0, ErrNoRemoteVault
	}

	localName := "allod-node"
	if cfg != nil && cfg.Node.Name != "" {
		localName = cfg.Node.Name
	} else if h, errH := os.Hostname(); errH == nil && h != "" {
		localName = h
	}

	for id, m := range topo.Members {
		if id != localName && m != nil && strings.TrimSpace(m.Address) != "" {
			host := strings.TrimSpace(m.Address)
			p := 8000
			if strings.Contains(host, ":") && !strings.HasSuffix(host, "]") {
				parts := strings.Split(host, ":")
				if len(parts) == 2 {
					host = parts[0]
					fmt.Sscanf(parts[1], "%d", &p)
				}
			}
			return id, host, p, nil
		}
	}

	return "", "", 0, ErrNoRemoteVault
}

// FormatRepoURL builds the rest:http://<host>:<port>/ restic repository URL.
func FormatRepoURL(host string, port int) string {
	cleanHost := strings.TrimSpace(host)
	if port <= 0 {
		port = 8000
	}
	if strings.Contains(cleanHost, ":") && !strings.HasPrefix(cleanHost, "[") {
		cleanHost = "[" + cleanHost + "]"
	}
	return fmt.Sprintf("rest:http://%s:%d/", cleanHost, port)
}

// GetStatus returns the current backup status and lists existing remote snapshots.
func (e *Engine) GetStatus(ctx context.Context) (*Status, error) {
	st, err := state.Open(e.dbPath)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	cfg, _ := config.LoadConfig(e.configPath)
	vName, vAddr, vPort, _ := e.FindRemoteVault(cfg)

	lastRun, _ := st.GetMeta("backup_last_run")
	lastStatus, _ := st.GetMeta("backup_last_status")
	if lastStatus == "" {
		lastStatus = "never"
	}
	lastErr, _ := st.GetMeta("backup_last_error")
	lastSnap, _ := st.GetMeta("backup_last_snapshot")

	var selectedPaths []string
	selPathsJSON, _ := st.GetMeta("backup_selected_paths")
	if selPathsJSON != "" {
		_ = json.Unmarshal([]byte(selPathsJSON), &selectedPaths)
	}

	resticBin := FindRestic()
	hasRestic := resticBin != ""

	e.mu.Lock()
	isRunning := e.isRunning
	curAction := e.currentAction
	curProg := e.currentProgress
	if e.lastError != "" {
		lastErr = e.lastError
	}
	e.mu.Unlock()

	status := &Status{
		IsRunning:       isRunning,
		CurrentAction:   curAction,
		CurrentProgress: curProg,
		LastRun:         lastRun,
		LastStatus:      lastStatus,
		LastError:       lastErr,
		LastSnapshotID:  lastSnap,
		RemotePeerName:  vName,
		RemoteAddress:   vAddr,
		RemotePort:      vPort,
		SelectedPaths:   selectedPaths,
		EncryptionMode:  "Zero-Knowledge AES-256 (Append-Only)",
		HasRestic:       hasRestic,
		Snapshots:       []Snapshot{},
	}

	if isRunning {
		status.LastStatus = "running"
	}

	// Query remote snapshots if remote vault is known and restic is available
	if hasRestic && vAddr != "" {
		repoURL := FormatRepoURL(vAddr, vPort)
		key, _ := e.GetOrCreateEncryptionKey(st)
		if key != "" {
			snaps, errS := e.querySnapshots(ctx, resticBin, repoURL, key)
			if errS == nil {
				status.Snapshots = snaps
				status.TotalSnapshots = len(snaps)
				if len(snaps) > 0 && status.LastSnapshotID == "" {
					status.LastSnapshotID = snaps[0].ShortID
				}
			}
		}
	}

	return status, nil
}

// querySnapshots calls `restic snapshots --json` against the remote vault.
func (e *Engine) querySnapshots(ctx context.Context, resticBin, repoURL, password string) ([]Snapshot, error) {
	qCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(qCtx, resticBin, "-r", repoURL, "snapshots", "--json")
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	type rawSnapshot struct {
		ID       string    `json:"id"`
		ShortID  string    `json:"short_id"`
		Time     time.Time `json:"time"`
		Paths    []string  `json:"paths"`
		Hostname string    `json:"hostname"`
		Username string    `json:"username"`
		Summary  *struct {
			TotalBytesProcessed int64 `json:"total_bytes_processed"`
			TotalFilesProcessed int   `json:"total_files_processed"`
		} `json:"summary"`
	}

	var rawList []rawSnapshot
	if err := json.Unmarshal(out, &rawList); err != nil {
		return nil, err
	}

	var result []Snapshot
	for _, r := range rawList {
		sizeStr := ""
		filesCount := 0
		if r.Summary != nil {
			sizeStr = formatBytes(r.Summary.TotalBytesProcessed)
			filesCount = r.Summary.TotalFilesProcessed
		}
		result = append(result, Snapshot{
			ID:            r.ID,
			ShortID:       r.ShortID,
			Time:          r.Time,
			TimeFormatted: r.Time.Format("02 Jan 2006, 15:04"),
			Paths:         r.Paths,
			Hostname:      r.Hostname,
			Username:      r.Username,
			SizeFormatted: sizeStr,
			FilesCount:    filesCount,
		})
	}

	// Sort newest first
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.After(result[j].Time)
	})

	return result, nil
}

// Run executes the full backup workflow towards the remote peer.
func (e *Engine) Run(ctx context.Context, progressCb func(action, progress string)) error {
	e.mu.Lock()
	if e.isRunning {
		e.mu.Unlock()
		return ErrAlreadyRun
	}
	e.isRunning = true
	e.currentAction = "Preparazione backup..."
	e.currentProgress = "0%"
	e.lastError = ""
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.isRunning = false
		e.currentAction = ""
		e.currentProgress = ""
		e.mu.Unlock()
	}()

	st, err := state.Open(e.dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	cfg, _ := config.LoadConfig(e.configPath)
	vName, vAddr, vPort, errV := e.FindRemoteVault(cfg)
	if errV != nil {
		e.recordFailure(st, errV.Error())
		return errV
	}

	resticBin := FindRestic()
	if resticBin == "" {
		e.recordFailure(st, ErrResticMissing.Error())
		return ErrResticMissing
	}

	key, errK := e.GetOrCreateEncryptionKey(st)
	if errK != nil {
		e.recordFailure(st, errK.Error())
		return errK
	}

	// Resolve target paths
	var candidatePaths []string
	selPathsJSON, _ := st.GetMeta("backup_selected_paths")
	if selPathsJSON != "" {
		_ = json.Unmarshal([]byte(selPathsJSON), &candidatePaths)
	}

	var validPaths []string
	for _, p := range candidatePaths {
		if fi, errS := os.Stat(p); errS == nil && fi.IsDir() {
			validPaths = append(validPaths, p)
		}
	}
	if len(validPaths) == 0 {
		e.recordFailure(st, ErrNoPaths.Error())
		return ErrNoPaths
	}

	repoURL := FormatRepoURL(vAddr, vPort)

	// 1. Check if repository is initialized
	e.updateProgress(progressCb, fmt.Sprintf("Connessione a %s (%s:%d)...", vName, vAddr, vPort), "10%")
	checkCmd := exec.CommandContext(ctx, resticBin, "-r", repoURL, "snapshots", "--json")
	checkCmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+key)
	if errCheck := checkCmd.Run(); errCheck != nil {
		// Attempt initialization
		e.updateProgress(progressCb, fmt.Sprintf("Inizializzazione repository cifrato su %s...", vName), "25%")
		initCmd := exec.CommandContext(ctx, resticBin, "-r", repoURL, "init")
		initCmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+key)
		if outInit, errInit := initCmd.CombinedOutput(); errInit != nil {
			msg := fmt.Sprintf("Inizializzazione fallita: %v (%s)", errInit, strings.TrimSpace(string(outInit)))
			e.recordFailure(st, msg)
			return errors.New(msg)
		}
	}

	// 2. Execute restic backup with JSON streaming
	e.updateProgress(progressCb, fmt.Sprintf("Invio snapshot cifrato verso %s...", vName), "35%")

	backupArgs := append([]string{"-r", repoURL, "backup", "--json"}, validPaths...)
	cmd := exec.CommandContext(ctx, resticBin, backupArgs...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+key)

	stdoutPipe, errPipe := cmd.StdoutPipe()
	if errPipe != nil {
		e.recordFailure(st, errPipe.Error())
		return errPipe
	}

	if errStart := cmd.Start(); errStart != nil {
		e.recordFailure(st, errStart.Error())
		return errStart
	}

	scanner := bufio.NewScanner(stdoutPipe)
	lastSnapshotID := ""
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg struct {
			MessageType string  `json:"message_type"`
			PercentDone float64 `json:"percent_done"`
			TotalFiles  int     `json:"total_files"`
			FilesDone   int     `json:"files_done"`
			SnapshotID  string  `json:"snapshot_id"`
		}
		if errJ := json.Unmarshal(line, &msg); errJ == nil {
			if msg.MessageType == "status" {
				pct := int(msg.PercentDone * 100)
				progText := fmt.Sprintf("%d%% (%d/%d file)", pct, msg.FilesDone, msg.TotalFiles)
				e.updateProgress(progressCb, fmt.Sprintf("Backup in corso su %s...", vName), progText)
			} else if msg.MessageType == "summary" && msg.SnapshotID != "" {
				lastSnapshotID = msg.SnapshotID
			}
		}
	}

	if errWait := cmd.Wait(); errWait != nil {
		msg := fmt.Sprintf("Backup fallito: %v", errWait)
		e.recordFailure(st, msg)
		return errors.New(msg)
	}

	// 3. Success recording
	shortID := lastSnapshotID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	nowISO := time.Now().UTC().Format(time.RFC3339)

	_ = st.SetMeta("backup_last_run", nowISO)
	_ = st.SetMeta("backup_last_status", "success")
	_ = st.SetMeta("backup_last_error", "")
	if shortID != "" {
		_ = st.SetMeta("backup_last_snapshot", shortID)
	}

	e.updateProgress(progressCb, "Completato con successo!", "100%")
	return nil
}

func (e *Engine) updateProgress(cb func(action, progress string), action, progress string) {
	e.mu.Lock()
	e.currentAction = action
	e.currentProgress = progress
	e.mu.Unlock()
	if cb != nil {
		cb(action, progress)
	}
}

func (e *Engine) recordFailure(st *state.Store, msg string) {
	e.mu.Lock()
	e.lastError = msg
	e.mu.Unlock()
	if st != nil {
		_ = st.SetMeta("backup_last_status", "error")
		_ = st.SetMeta("backup_last_error", msg)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// CheckVaultReachable tests if the remote vault HTTP rest-server port is reachable.
func CheckVaultReachable(host string, port int, timeout time.Duration) bool {
	if port <= 0 {
		port = 8000
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
