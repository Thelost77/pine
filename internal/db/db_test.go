package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/secrets"
	_ "modernc.org/sqlite"
)

func TestOpen_CreatesTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Verify accounts table exists with expected columns
	rows, err := store.DB.Query(`SELECT id, server_url, username, token, is_default, created_at FROM accounts LIMIT 0`)
	if err != nil {
		t.Fatalf("accounts table not created: %v", err)
	}
	_ = rows.Close()

	// Verify sessions table exists with expected columns
	rows, err = store.DB.Query(`SELECT id, item_id, session_id, current_time, duration, created_at FROM sessions LIMIT 0`)
	if err != nil {
		t.Fatalf("sessions table not created: %v", err)
	}
	_ = rows.Close()

	// Verify api_cache table exists with expected columns
	rows, err = store.DB.Query(`SELECT cache_key, data, cached_at, expires_at FROM api_cache LIMIT 0`)
	if err != nil {
		t.Fatalf("api_cache table not created: %v", err)
	}
	_ = rows.Close()
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	// Open twice — migrations should not fail on second run
	store1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error: %v", err)
	}
	_ = store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() error: %v", err)
	}
	defer func() { _ = store2.Close() }()

	// Insert a row, verify it survives re-open
	_, err = store2.DB.Exec(`INSERT INTO accounts (id, server_url, username, is_default, created_at) VALUES ('a1', 'http://localhost', 'user', 1, datetime('now'))`)
	if err != nil {
		t.Fatalf("insert after re-open failed: %v", err)
	}

	var count int
	err = store2.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestOpen_MigratesLegacyAccountTokenToObfuscatedColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}
	_, err = legacy.Exec(`CREATE TABLE accounts (
		id TEXT PRIMARY KEY,
		server_url TEXT NOT NULL,
		username TEXT NOT NULL,
		token TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create legacy accounts: %v", err)
	}
	_, err = legacy.Exec(`INSERT INTO accounts (id, server_url, username, token, is_default, created_at)
		VALUES ('a1', 'https://abs.example.com', 'alice', 'legacy-token', 1, datetime('now'))`)
	if err != nil {
		t.Fatalf("insert legacy account: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer store.Close()

	var storedToken string
	if err := store.DB.QueryRow(`SELECT token FROM accounts WHERE id = 'a1'`).Scan(&storedToken); err != nil {
		t.Fatalf("select token: %v", err)
	}
	if storedToken == "legacy-token" || strings.Contains(storedToken, "legacy-token") {
		t.Fatalf("expected obfuscated token, got %q", storedToken)
	}
	if !secrets.IsObfuscatedToken(storedToken) {
		t.Fatalf("expected obfuscated token prefix, got %q", storedToken)
	}
	if !secrets.IsCurrentToken(storedToken) {
		t.Fatalf("expected current token prefix, got %q", storedToken)
	}
	token, err := secrets.DecodeToken("https://abs.example.com", "alice", storedToken)
	if err != nil {
		t.Fatalf("DecodeToken() error: %v", err)
	}
	if token != "legacy-token" {
		t.Fatalf("token = %q, want legacy-token", token)
	}
}

func TestOpen_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}
