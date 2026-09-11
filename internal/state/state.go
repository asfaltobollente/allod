package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
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
	OnboardingToken   string     `json:"onboarding_token,omitempty"`
	OnboardingExpires *time.Time `json:"onboarding_expires,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
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
		onboarding_token TEXT DEFAULT '',
		onboarding_expires DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

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
	INSERT INTO family_members (username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, onboarding_token, onboarding_expires, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := s.db.Exec(query, m.Username, m.FirstName, m.LastName, m.Email, m.Role, m.AvatarColor, m.Notes, m.SmbActive, m.PhotosLinked, m.OnboardingToken, expiresStr, m.CreatedAt.Format(time.RFC3339), m.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	return nil
}

func (s *Store) GetFamilyMember(username string) (*FamilyMember, error) {
	query := `
	SELECT id, username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, onboarding_token, onboarding_expires, created_at, updated_at
	FROM family_members WHERE username = ?
	`
	row := s.db.QueryRow(query, username)
	var m FamilyMember
	var expiresStr sql.NullString
	var createdStr, updatedStr string
	err := row.Scan(&m.ID, &m.Username, &m.FirstName, &m.LastName, &m.Email, &m.Role, &m.AvatarColor, &m.Notes, &m.SmbActive, &m.PhotosLinked, &m.OnboardingToken, &expiresStr, &createdStr, &updatedStr)
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
	SELECT id, username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, onboarding_token, onboarding_expires, created_at, updated_at
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
		if err := rows.Scan(&m.ID, &m.Username, &m.FirstName, &m.LastName, &m.Email, &m.Role, &m.AvatarColor, &m.Notes, &m.SmbActive, &m.PhotosLinked, &m.OnboardingToken, &expiresStr, &createdStr, &updatedStr); err != nil {
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
		smb_active = ?, photos_linked = ?, onboarding_token = ?, onboarding_expires = ?, updated_at = ?
	WHERE username = ?
	`
	_, err := s.db.Exec(query, m.FirstName, m.LastName, m.Email, m.Role, m.AvatarColor, m.Notes, m.SmbActive, m.PhotosLinked, m.OnboardingToken, expiresStr, m.UpdatedAt.Format(time.RFC3339), m.Username)
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
	SELECT id, username, first_name, last_name, email, role, avatar_color, notes, smb_active, photos_linked, onboarding_token, onboarding_expires, created_at, updated_at
	FROM family_members WHERE onboarding_token = ?
	`
	row := s.db.QueryRow(query, token)
	var m FamilyMember
	var expiresStr sql.NullString
	var createdStr, updatedStr string
	err := row.Scan(&m.ID, &m.Username, &m.FirstName, &m.LastName, &m.Email, &m.Role, &m.AvatarColor, &m.Notes, &m.SmbActive, &m.PhotosLinked, &m.OnboardingToken, &expiresStr, &createdStr, &updatedStr)
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
