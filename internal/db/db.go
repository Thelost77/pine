package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/secrets"
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

	logger.Info("opening database", "path", path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	logger.Debug("database WAL mode enabled", "path", path)

	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	logger.Info("database ready", "path", path)
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	migrations := []struct {
		name  string
		query string
	}{
		{
			name: "create accounts table",
			query: `CREATE TABLE IF NOT EXISTS accounts (
			id         TEXT PRIMARY KEY,
			server_url TEXT NOT NULL,
			username   TEXT NOT NULL,
			token      TEXT NOT NULL DEFAULT '',
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		},
		{
			name: "create sessions table",
			query: `CREATE TABLE IF NOT EXISTS sessions (
			id           TEXT PRIMARY KEY,
			item_id      TEXT NOT NULL,
			session_id   TEXT NOT NULL,
			current_time REAL NOT NULL DEFAULT 0,
			duration     REAL NOT NULL DEFAULT 0,
			created_at   TEXT NOT NULL
		)`,
		},
		{
			name: "create api_cache table",
			query: `CREATE TABLE IF NOT EXISTS api_cache (
			cache_key   TEXT PRIMARY KEY,
			data        BLOB NOT NULL,
			cached_at   DATETIME NOT NULL,
			expires_at  DATETIME NOT NULL
		)`,
		},
		{
			name:  "create api_cache expires index",
			query: `CREATE INDEX IF NOT EXISTS idx_api_cache_expires ON api_cache(expires_at)`,
		},
	}

	for _, m := range migrations {
		if _, err := s.DB.Exec(m.query); err != nil {
			logger.Error("database migration failed", "migration", m.name, "err", err)
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Debug("database migration applied", "migration", m.name)
	}

	if err := s.ensureAccountTokenColumn(); err != nil {
		return err
	}
	if err := s.migrateTokenEncryptedColumn(); err != nil {
		return err
	}
	if err := s.normalizeAccountTokens(); err != nil {
		return err
	}

	// Add episode_id column if it doesn't exist (SQLite doesn't support IF NOT EXISTS for ALTER TABLE)
	var hasEpisodeID bool
	row := s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name = 'episode_id'`)
	if err := row.Scan(&hasEpisodeID); err == nil && !hasEpisodeID {
		if _, err := s.DB.Exec(`ALTER TABLE sessions ADD COLUMN episode_id TEXT NOT NULL DEFAULT ''`); err != nil {
			logger.Error("database migration failed", "migration", "add episode_id column", "err", err)
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Info("database migration applied", "migration", "add episode_id column")
	}

	return nil
}

func (s *Store) ensureAccountTokenColumn() error {
	if s.hasColumn("accounts", "token") {
		return nil
	}
	if _, err := s.DB.Exec(`ALTER TABLE accounts ADD COLUMN token TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("adding account token column: %w", err)
	}
	logger.Info("database migration applied", "migration", "add account token column")
	return nil
}

func (s *Store) migrateTokenEncryptedColumn() error {
	if !s.hasColumn("accounts", "token_encrypted") {
		return nil
	}
	if _, err := s.DB.Exec(`UPDATE accounts SET token = token_encrypted WHERE token = '' AND token_encrypted <> ''`); err != nil {
		return fmt.Errorf("copying legacy encrypted account tokens: %w", err)
	}
	logger.Info("database migration applied", "migration", "copy legacy token_encrypted column")
	return nil
}

func (s *Store) normalizeAccountTokens() error {
	rows, err := s.DB.Query(`SELECT id, server_url, username, token FROM accounts WHERE token <> ''`)
	if err != nil {
		return fmt.Errorf("querying account tokens: %w", err)
	}
	defer rows.Close()

	type tokenUpdate struct {
		id    string
		token string
	}
	var updates []tokenUpdate
	for rows.Next() {
		var id, serverURL, username, token string
		if err := rows.Scan(&id, &serverURL, &username, &token); err != nil {
			return fmt.Errorf("scanning account token: %w", err)
		}
		if secrets.IsObfuscatedToken(token) {
			continue
		}
		updates = append(updates, tokenUpdate{
			id:    id,
			token: secrets.EncodeToken(serverURL, username, token),
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating account tokens: %w", err)
	}
	for _, update := range updates {
		if _, err := s.DB.Exec(`UPDATE accounts SET token = ? WHERE id = ?`, update.token, update.id); err != nil {
			return fmt.Errorf("normalizing account token: %w", err)
		}
	}
	if len(updates) > 0 {
		logger.Info("database migration applied", "migration", "obfuscate account tokens", "count", len(updates))
	}
	return nil
}

func (s *Store) hasColumn(table, column string) bool {
	var count int
	row := s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column)
	return row.Scan(&count) == nil && count > 0
}
