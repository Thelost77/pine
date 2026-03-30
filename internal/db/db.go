package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database connection.
type Store struct {
	DB *sql.DB
}

// Open opens (or creates) a SQLite database at path and runs migrations.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id         TEXT PRIMARY KEY,
			server_url TEXT NOT NULL,
			username   TEXT NOT NULL,
			token      TEXT NOT NULL,
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id           TEXT PRIMARY KEY,
			item_id      TEXT NOT NULL,
			session_id   TEXT NOT NULL,
			current_time REAL NOT NULL DEFAULT 0,
			duration     REAL NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL
		)`,
	}

	for _, m := range migrations {
		if _, err := s.DB.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Rename legacy column; ignore error if already renamed or column doesn't exist.
	s.DB.Exec(`ALTER TABLE accounts RENAME COLUMN token_encrypted TO token`)

	return nil
}
