package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type AppliedModule struct {
	Name        string
	Level       string
	ContentHash string
	AppliedAt   time.Time
}

type FamilyMember struct {
	ID                int64      `json:"id"`
	Username          string     `json:"username"`
	FirstName         string     `json:"first_name"`
	LastName          string     `json:"last_name"`
	Email             string     `json:"email"`
	Role              string     `json:"role"` // "admin", "member", "guest"
	AvatarColor       string     `json:"avatar_color"`
	Notes             string     `json:"notes"`
	SmbActive         bool       `json:"smb_active"`
	PhotosLinked      bool       `json:"photos_linked"`
	PasswordHash      string     `json:"-"`
	PasswordSalt      string     `json:"-"`
	OnboardingToken   string     `json:"onboarding_token,omitempty"`
	OnboardingExpires *time.Time `json:"onboarding_expires,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Session struct {
	Token     string    `json:"token"`
	UserType  string    `json:"user_type"` // "admin" or "family"
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type SentinelConfigRecord struct {
	Mode                 string    `json:"mode"`
	VPSHost              string    `json:"vps_host"`
	VPSPort              int       `json:"vps_port"`
	SecretToken          string    `json:"secret_token"`
	PushIntervalSeconds  int       `json:"push_interval_seconds"`
	TelegramBotToken     string    `json:"telegram_bot_token"`
	TelegramChatID       string    `json:"telegram_chat_id"`
	WeatherCity          string    `json:"weather_city"`
	DigestTime           string    `json:"digest_time"`
	DownThresholdSeconds int       `json:"down_threshold_seconds"`
	VPSSetupKey          string    `json:"vps_setup_key"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type WoLDevice struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	MACAddress  string     `json:"mac_address"`
	BroadcastIP string     `json:"broadcast_ip"`
	Port        int        `json:"port"`
	CreatedAt   time.Time  `json:"created_at"`
	LastWakeAt  *time.Time `json:"last_wake_at,omitempty"`
}

type SystemMetricRecord struct {
	Timestamp         int64   `json:"t"`
	CPUTempC          float64 `json:"cpu_temp"`
	CPUUsagePct       float64 `json:"cpu_usage"`
	RAMUsedMB         int64   `json:"ram_used_mb"`
	RAMTotalMB        int64   `json:"ram_total_mb"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
}

type Store struct {
	db *sql.DB
}

// Open initializes or connects to state.db
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS applied_modules (
		name TEXT PRIMARY KEY,
		level TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		applied_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS node_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS family_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		email TEXT DEFAULT '',
		role TEXT NOT NULL DEFAULT 'member',
		avatar_color TEXT DEFAULT '#38bdf8',
		notes TEXT DEFAULT '',
		smb_active BOOLEAN DEFAULT 1,
		photos_linked BOOLEAN DEFAULT 1,
		password_hash TEXT DEFAULT '',
		password_salt TEXT DEFAULT '',
		onboarding_token TEXT DEFAULT '',
		onboarding_expires DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_type TEXT NOT NULL,
		username TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sentinel_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		mode TEXT DEFAULT 'push',
		vps_host TEXT DEFAULT '',
		vps_port INTEGER DEFAULT 8443,
		secret_token TEXT DEFAULT '',
		push_interval_seconds INTEGER DEFAULT 60,
		telegram_bot_token TEXT DEFAULT '',
		telegram_chat_id TEXT DEFAULT '',
		weather_city TEXT DEFAULT 'Roma',
		digest_time TEXT DEFAULT '08:30',
		down_threshold_seconds INTEGER DEFAULT 300,
		vps_setup_key TEXT DEFAULT '',
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS wol_devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		mac_address TEXT NOT NULL,
		broadcast_ip TEXT DEFAULT '255.255.255.255',
		port INTEGER DEFAULT 9,
		created_at DATETIME NOT NULL,
		last_wake_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS metrics_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		cpu_temp_c REAL,
		cpu_usage_pct REAL,
		ram_used_mb INTEGER,
		ram_total_mb INTEGER,
		storage_used_bytes INTEGER,
		storage_total_bytes INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_metrics_history_ts ON metrics_history(timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Safe column additions for existing databases
	_, _ = db.Exec("ALTER TABLE family_members ADD COLUMN password_hash TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE family_members ADD COLUMN password_salt TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE sentinel_config ADD COLUMN vps_setup_key TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE sentinel_config ADD COLUMN mode TEXT DEFAULT 'push'")
	_, _ = db.Exec("ALTER TABLE sentinel_config ADD COLUMN vps_host TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE sentinel_config ADD COLUMN vps_port INTEGER DEFAULT 8443")
	_, _ = db.Exec("ALTER TABLE sentinel_config ADD COLUMN secret_token TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE sentinel_config ADD COLUMN push_interval_seconds INTEGER DEFAULT 60")

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetModule(name string) (*AppliedModule, error) {
	row := s.db.QueryRow("SELECT name, level, content_hash, applied_at FROM applied_modules WHERE name = ?", name)
	var mod AppliedModule
	var appliedAtStr string
	err := row.Scan(&mod.Name, &mod.Level, &mod.ContentHash, &appliedAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	mod.AppliedAt, _ = time.Parse(time.RFC3339, appliedAtStr)
	return &mod, nil
}

func (s *Store) ListModules() (map[string]AppliedModule, error) {
	rows, err := s.db.Query("SELECT name, level, content_hash, applied_at FROM applied_modules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]AppliedModule)
	for rows.Next() {
		var mod AppliedModule
		var appliedAtStr string
		if err := rows.Scan(&mod.Name, &mod.Level, &mod.ContentHash, &appliedAtStr); err != nil {
			return nil, err
		}
		mod.AppliedAt, _ = time.Parse(time.RFC3339, appliedAtStr)
		result[mod.Name] = mod
	}
	return result, nil
}

func (s *Store) SaveModule(name, level, hash string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
	INSERT INTO applied_modules (name, level, content_hash, applied_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		level = excluded.level,
		content_hash = excluded.content_hash,
		applied_at = excluded.applied_at;
	`
	_, err := s.db.Exec(query, name, level, hash, now)
	return err
}

func (s *Store) DeleteModule(name string) error {
	_, err := s.db.Exec("DELETE FROM applied_modules WHERE name = ?", name)
	return err
}

func (s *Store) SetMeta(key, value string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
	INSERT INTO node_meta (key, value, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET
		value = excluded.value,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.Exec(query, key, value, now)
	return err
}

func (s *Store) GetMeta(key string) (string, error) {
	row := s.db.QueryRow("SELECT value FROM node_meta WHERE key = ?", key)
	var val string
	err := row.Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// --- Family Members Methods ---

func (s *Store) CreateFamilyMember(m *FamilyMember) error {
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.Role == "" {
		m.Role = "member"
	}
	if m.AvatarColor == "" {
		m.AvatarColor = "#38bdf8"
	}

	var expiresStr *string
	if m.OnboardingExpires != nil {
		str := m.OnboardingExpires.Format(time.RFC3339)
		expiresStr = &str
	}

	query := `
	INSERT INTO family_members (username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, password_hash, password_salt, onboarding_token, onboarding_expires, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := s.db.Exec(query, m.Username, m.FirstName, m.LastName, m.Email, m.Role, m.AvatarColor, m.Notes, m.SmbActive, m.PhotosLinked, m.PasswordHash, m.PasswordSalt, m.OnboardingToken, expiresStr, m.CreatedAt.Format(time.RFC3339), m.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	return nil
}

func (s *Store) GetFamilyMember(username string) (*FamilyMember, error) {
	query := `
	SELECT id, username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, password_hash, password_salt, onboarding_token, onboarding_expires, created_at, updated_at
	FROM family_members WHERE username = ?
	`
	row := s.db.QueryRow(query, username)
	var m FamilyMember
	var expiresStr sql.NullString
	var createdStr, updatedStr string
	err := row.Scan(&m.ID, &m.Username, &m.FirstName, &m.LastName, &m.Email, &m.Role, &m.AvatarColor, &m.Notes, &m.SmbActive, &m.PhotosLinked, &m.PasswordHash, &m.PasswordSalt, &m.OnboardingToken, &expiresStr, &createdStr, &updatedStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if expiresStr.Valid && expiresStr.String != "" {
		t, _ := time.Parse(time.RFC3339, expiresStr.String)
		m.OnboardingExpires = &t
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &m, nil
}

func (s *Store) ListFamilyMembers() ([]FamilyMember, error) {
	query := `
	SELECT id, username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, password_hash, password_salt, onboarding_token, onboarding_expires, created_at, updated_at
	FROM family_members ORDER BY id ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []FamilyMember
	for rows.Next() {
		var m FamilyMember
		var expiresStr sql.NullString
		var createdStr, updatedStr string
		if err := rows.Scan(&m.ID, &m.Username, &m.FirstName, &m.LastName, &m.Email, &m.Role, &m.AvatarColor, &m.Notes, &m.SmbActive, &m.PhotosLinked, &m.PasswordHash, &m.PasswordSalt, &m.OnboardingToken, &expiresStr, &createdStr, &updatedStr); err != nil {
			return nil, err
		}
		if expiresStr.Valid && expiresStr.String != "" {
			t, _ := time.Parse(time.RFC3339, expiresStr.String)
			m.OnboardingExpires = &t
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		members = append(members, m)
	}
	return members, nil
}

func (s *Store) UpdateFamilyMember(m *FamilyMember) error {
	m.UpdatedAt = time.Now()
	var expiresStr *string
	if m.OnboardingExpires != nil {
		str := m.OnboardingExpires.Format(time.RFC3339)
		expiresStr = &str
	}
	query := `
	UPDATE family_members SET
		first_name = ?, last_name = ?, email = ?, role = ?, avatar_color = ?, notes = ?,
		smb_active = ?, photos_linked = ?, password_hash = ?, password_salt = ?,
		onboarding_token = ?, onboarding_expires = ?, updated_at = ?
	WHERE username = ?
	`
	_, err := s.db.Exec(query, m.FirstName, m.LastName, m.Email, m.Role, m.AvatarColor, m.Notes, m.SmbActive, m.PhotosLinked, m.PasswordHash, m.PasswordSalt, m.OnboardingToken, expiresStr, m.UpdatedAt.Format(time.RFC3339), m.Username)
	return err
}

func (s *Store) DeleteFamilyMember(username string) error {
	_, err := s.db.Exec("DELETE FROM family_members WHERE username = ?", username)
	return err
}

func (s *Store) CreateResetToken(username string, duration time.Duration) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(duration)
	expiresStr := expires.Format(time.RFC3339)

	query := `
	UPDATE family_members SET onboarding_token = ?, onboarding_expires = ?, updated_at = ?
	WHERE username = ?
	`
	res, err := s.db.Exec(query, token, expiresStr, time.Now().Format(time.RFC3339), username)
	if err != nil {
		return "", err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return "", fmt.Errorf("utente '%s' non trovato", username)
	}
	return token, nil
}

func (s *Store) ValidateResetToken(token string) (*FamilyMember, error) {
	if token == "" {
		return nil, fmt.Errorf("token non valido o mancante")
	}
	query := `
	SELECT id, username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, password_hash, password_salt, onboarding_token, onboarding_expires, created_at, updated_at
	FROM family_members WHERE onboarding_token = ?
	`
	row := s.db.QueryRow(query, token)
	var m FamilyMember
	var expiresStr sql.NullString
	var createdStr, updatedStr string
	err := row.Scan(&m.ID, &m.Username, &m.FirstName, &m.LastName, &m.Email, &m.Role, &m.AvatarColor, &m.Notes, &m.SmbActive, &m.PhotosLinked, &m.PasswordHash, &m.PasswordSalt, &m.OnboardingToken, &expiresStr, &createdStr, &updatedStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token non valido o già utilizzato")
		}
		return nil, err
	}
	if expiresStr.Valid && expiresStr.String != "" {
		exp, err := time.Parse(time.RFC3339, expiresStr.String)
		if err == nil {
			m.OnboardingExpires = &exp
			if time.Now().After(exp) {
				return nil, fmt.Errorf("questo link di onboarding è scaduto")
			}
		}
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &m, nil
}

func (s *Store) ConsumeResetToken(token string) error {
	query := `
	UPDATE family_members SET onboarding_token = '', onboarding_expires = NULL, updated_at = ?
	WHERE onboarding_token = ?
	`
	_, err := s.db.Exec(query, time.Now().Format(time.RFC3339), token)
	return err
}

func (s *Store) SetFamilyMemberPassword(username, passwordHash, passwordSalt string) error {
	now := time.Now().Format(time.RFC3339)
	query := `
	UPDATE family_members SET password_hash = ?, password_salt = ?, updated_at = ?
	WHERE username = ?
	`
	res, err := s.db.Exec(query, passwordHash, passwordSalt, now, username)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("utente '%s' non trovato", username)
	}
	return nil
}

func (s *Store) CreateSession(userType, username string, duration time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()
	expires := now.Add(duration)

	query := `
	INSERT INTO sessions (token, user_type, username, expires_at, created_at)
	VALUES (?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query, token, userType, username, expires.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) GetSession(token string) (*Session, error) {
	if token == "" {
		return nil, fmt.Errorf("token sessione mancante")
	}
	query := `
	SELECT token, user_type, username, expires_at, created_at
	FROM sessions WHERE token = ?
	`
	row := s.db.QueryRow(query, token)
	var sess Session
	var expStr, creStr string
	err := row.Scan(&sess.Token, &sess.UserType, &sess.Username, &expStr, &creStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	exp, _ := time.Parse(time.RFC3339, expStr)
	cre, _ := time.Parse(time.RFC3339, creStr)
	sess.ExpiresAt = exp
	sess.CreatedAt = cre

	if time.Now().After(sess.ExpiresAt) {
		_ = s.DeleteSession(token)
		return nil, nil
	}
	return &sess, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (s *Store) DeleteSessionsForUser(userType, username string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE user_type = ? AND username = ?", userType, username)
	return err
}

func (s *Store) CleanupExpiredSessions() error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec("DELETE FROM sessions WHERE expires_at < ?", now)
	return err
}

func (s *Store) SetAdminAuth(hash, salt string) error {
	if err := s.SetMeta("admin_password_hash", hash); err != nil {
		return err
	}
	return s.SetMeta("admin_password_salt", salt)
}

func (s *Store) GetAdminAuth() (string, string, error) {
	hash, err := s.GetMeta("admin_password_hash")
	if err != nil {
		return "", "", err
	}
	salt, err := s.GetMeta("admin_password_salt")
	if err != nil {
		return "", "", err
	}
	return hash, salt, nil
}

func (s *Store) HasAdminAuth() (bool, error) {
	hash, err := s.GetMeta("admin_password_hash")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(hash) != "", nil
}

func (s *Store) ClearAdminAuth() error {
	if err := s.SetMeta("admin_password_hash", ""); err != nil {
		return err
	}
	return s.SetMeta("admin_password_salt", "")
}

// --- Watch Sentinel Methods ---

// GetSentinelConfig returns the persisted sentinel configuration singleton.
func (s *Store) GetSentinelConfig() (*SentinelConfigRecord, error) {
	query := `
	SELECT mode, vps_host, vps_port, secret_token, push_interval_seconds,
	       telegram_bot_token, telegram_chat_id, weather_city, digest_time,
	       down_threshold_seconds, vps_setup_key, updated_at
	FROM sentinel_config WHERE id = 1
	`
	row := s.db.QueryRow(query)
	var cfg SentinelConfigRecord
	var updatedStr string
	err := row.Scan(
		&cfg.Mode, &cfg.VPSHost, &cfg.VPSPort, &cfg.SecretToken, &cfg.PushIntervalSeconds,
		&cfg.TelegramBotToken, &cfg.TelegramChatID, &cfg.WeatherCity, &cfg.DigestTime,
		&cfg.DownThresholdSeconds, &cfg.VPSSetupKey, &updatedStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return &SentinelConfigRecord{
				Mode:                 "push",
				VPSPort:              8443,
				PushIntervalSeconds:  60,
				WeatherCity:          "Roma",
				DigestTime:           "08:30",
				DownThresholdSeconds: 300,
				UpdatedAt:            time.Now(),
			}, nil
		}
		return nil, err
	}
	cfg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	if cfg.Mode == "" {
		cfg.Mode = "push"
	}
	if cfg.VPSPort <= 0 {
		cfg.VPSPort = 8443
	}
	if cfg.PushIntervalSeconds <= 0 {
		cfg.PushIntervalSeconds = 60
	}
	if cfg.WeatherCity == "" {
		cfg.WeatherCity = "Roma"
	}
	if cfg.DigestTime == "" {
		cfg.DigestTime = "08:30"
	}
	if cfg.DownThresholdSeconds <= 0 {
		cfg.DownThresholdSeconds = 300
	}
	return &cfg, nil
}

// SaveSentinelConfig persists the sentinel configuration singleton.
func (s *Store) SaveSentinelConfig(cfg *SentinelConfigRecord) error {
	if cfg == nil {
		return fmt.Errorf("configurazione sentinella nulla")
	}
	if cfg.Mode == "" {
		cfg.Mode = "push"
	}
	if cfg.VPSPort <= 0 {
		cfg.VPSPort = 8443
	}
	if cfg.PushIntervalSeconds <= 0 {
		cfg.PushIntervalSeconds = 60
	}
	if cfg.WeatherCity == "" {
		cfg.WeatherCity = "Roma"
	}
	if cfg.DigestTime == "" {
		cfg.DigestTime = "08:30"
	}
	if cfg.DownThresholdSeconds <= 0 {
		cfg.DownThresholdSeconds = 300
	}
	now := time.Now()
	cfg.UpdatedAt = now

	query := `
	INSERT INTO sentinel_config (id, mode, vps_host, vps_port, secret_token, push_interval_seconds, telegram_bot_token, telegram_chat_id, weather_city, digest_time, down_threshold_seconds, vps_setup_key, updated_at)
	VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		mode = excluded.mode,
		vps_host = excluded.vps_host,
		vps_port = excluded.vps_port,
		secret_token = excluded.secret_token,
		push_interval_seconds = excluded.push_interval_seconds,
		telegram_bot_token = excluded.telegram_bot_token,
		telegram_chat_id = excluded.telegram_chat_id,
		weather_city = excluded.weather_city,
		digest_time = excluded.digest_time,
		down_threshold_seconds = excluded.down_threshold_seconds,
		vps_setup_key = excluded.vps_setup_key,
		updated_at = excluded.updated_at;
	`
	_, err := s.db.Exec(query,
		cfg.Mode, cfg.VPSHost, cfg.VPSPort, cfg.SecretToken, cfg.PushIntervalSeconds,
		cfg.TelegramBotToken, cfg.TelegramChatID, cfg.WeatherCity, cfg.DigestTime,
		cfg.DownThresholdSeconds, cfg.VPSSetupKey, now.Format(time.RFC3339),
	)
	return err
}

// --- Wake-on-LAN (WoL) Methods ---

// ListWoLDevices returns all saved WoL devices ordered by ID ascending.
func (s *Store) ListWoLDevices() ([]WoLDevice, error) {
	query := `
	SELECT id, name, mac_address, broadcast_ip, port, created_at, last_wake_at
	FROM wol_devices ORDER BY id ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []WoLDevice
	for rows.Next() {
		var d WoLDevice
		var createdStr string
		var lastWakeStr sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &d.MACAddress, &d.BroadcastIP, &d.Port, &createdStr, &lastWakeStr); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		if lastWakeStr.Valid && lastWakeStr.String != "" {
			t, _ := time.Parse(time.RFC3339, lastWakeStr.String)
			d.LastWakeAt = &t
		}
		devices = append(devices, d)
	}
	return devices, nil
}

// GetWoLDevice fetches a single WoL device by its primary key ID.
func (s *Store) GetWoLDevice(id int64) (*WoLDevice, error) {
	query := `
	SELECT id, name, mac_address, broadcast_ip, port, created_at, last_wake_at
	FROM wol_devices WHERE id = ?
	`
	row := s.db.QueryRow(query, id)
	var d WoLDevice
	var createdStr string
	var lastWakeStr sql.NullString
	err := row.Scan(&d.ID, &d.Name, &d.MACAddress, &d.BroadcastIP, &d.Port, &createdStr, &lastWakeStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	if lastWakeStr.Valid && lastWakeStr.String != "" {
		t, _ := time.Parse(time.RFC3339, lastWakeStr.String)
		d.LastWakeAt = &t
	}
	return &d, nil
}

// SaveWoLDevice inserts a new WoL device (if ID == 0) or updates an existing device.
func (s *Store) SaveWoLDevice(d *WoLDevice) error {
	if d == nil {
		return fmt.Errorf("dispositivo WoL nullo")
	}
	d.Name = strings.TrimSpace(d.Name)
	d.MACAddress = strings.TrimSpace(d.MACAddress)
	if d.Name == "" {
		return fmt.Errorf("nome dispositivo obbligatorio")
	}
	if d.MACAddress == "" {
		return fmt.Errorf("indirizzo MAC obbligatorio")
	}
	if strings.TrimSpace(d.BroadcastIP) == "" {
		d.BroadcastIP = "255.255.255.255"
	}
	if d.Port <= 0 {
		d.Port = 9
	}

	now := time.Now()
	if d.ID == 0 {
		d.CreatedAt = now
		query := `
		INSERT INTO wol_devices (name, mac_address, broadcast_ip, port, created_at)
		VALUES (?, ?, ?, ?, ?)
		`
		res, err := s.db.Exec(query, d.Name, d.MACAddress, d.BroadcastIP, d.Port, now.Format(time.RFC3339))
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		d.ID = id
		return nil
	}

	query := `
	UPDATE wol_devices SET
		name = ?, mac_address = ?, broadcast_ip = ?, port = ?
	WHERE id = ?
	`
	_, err := s.db.Exec(query, d.Name, d.MACAddress, d.BroadcastIP, d.Port, d.ID)
	return err
}

// DeleteWoLDevice removes a WoL device by its ID.
func (s *Store) DeleteWoLDevice(id int64) error {
	_, err := s.db.Exec("DELETE FROM wol_devices WHERE id = ?", id)
	return err
}

// RecordWoLWake updates the last_wake_at timestamp to the current time.
func (s *Store) RecordWoLWake(id int64) error {
	now := time.Now().Format(time.RFC3339)
	query := `UPDATE wol_devices SET last_wake_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, now, id)
	return err
}

// RecordSystemMetric inserts a new telemetry data point into metrics_history.
func (s *Store) RecordSystemMetric(m *SystemMetricRecord) error {
	if m == nil {
		return fmt.Errorf("metric record cannot be nil")
	}
	ts := m.Timestamp
	if ts <= 0 {
		ts = time.Now().Unix()
	}
	_, err := s.db.Exec(`
		INSERT INTO metrics_history (timestamp, cpu_temp_c, cpu_usage_pct, ram_used_mb, ram_total_mb, storage_used_bytes, storage_total_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, ts, m.CPUTempC, m.CPUUsagePct, m.RAMUsedMB, m.RAMTotalMB, m.StorageUsedBytes, m.StorageTotalBytes)
	return err
}

// GetMetricsHistory retrieves historical telemetry aggregated over the specified duration.
// Supported ranges: "1h", "24h", "7d", "30d" (default: "24h").
func (s *Store) GetMetricsHistory(rangeStr string) ([]SystemMetricRecord, error) {
	now := time.Now().Unix()
	var since int64
	var step int64

	switch strings.ToLower(strings.TrimSpace(rangeStr)) {
	case "1h":
		since = now - 3600
		step = 0 // raw points (1 per minute)
	case "7d":
		since = now - (7 * 86400)
		step = 1800 // 30-minute buckets (~336 points)
	case "30d":
		since = now - (30 * 86400)
		step = 7200 // 2-hour buckets (~360 points)
	case "24h":
		fallthrough
	default:
		since = now - 86400
		step = 300 // 5-minute buckets (~288 points)
	}

	var rows *sql.Rows
	var err error

	if step <= 0 {
		rows, err = s.db.Query(`
			SELECT timestamp, cpu_temp_c, cpu_usage_pct, ram_used_mb, ram_total_mb, storage_used_bytes, storage_total_bytes
			FROM metrics_history
			WHERE timestamp >= ?
			ORDER BY timestamp ASC
		`, since)
	} else {
		rows, err = s.db.Query(`
			SELECT (timestamp / ?) * ? AS bucket,
			       ROUND(AVG(cpu_temp_c), 1),
			       ROUND(AVG(cpu_usage_pct), 1),
			       CAST(AVG(ram_used_mb) AS INTEGER),
			       CAST(MAX(ram_total_mb) AS INTEGER),
			       CAST(AVG(storage_used_bytes) AS INTEGER),
			       CAST(MAX(storage_total_bytes) AS INTEGER)
			FROM metrics_history
			WHERE timestamp >= ?
			GROUP BY bucket
			ORDER BY bucket ASC
		`, step, step, since)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SystemMetricRecord
	for rows.Next() {
		var r SystemMetricRecord
		if err := rows.Scan(&r.Timestamp, &r.CPUTempC, &r.CPUUsagePct, &r.RAMUsedMB, &r.RAMTotalMB, &r.StorageUsedBytes, &r.StorageTotalBytes); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// PruneMetricsHistory removes telemetry points older than the specified retention days.
func (s *Store) PruneMetricsHistory(retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().Unix() - int64(retentionDays*86400)
	_, err := s.db.Exec("DELETE FROM metrics_history WHERE timestamp < ?", cutoff)
	return err
}

