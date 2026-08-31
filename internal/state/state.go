package state

import (
	"database/sql"
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
