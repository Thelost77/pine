package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer store.Close()

	// Verify accounts table exists with expected columns
	rows, err := store.DB.Query(`SELECT id, server_url, username, token, is_default, created_at FROM accounts LIMIT 0`)
	if err != nil {
		t.Fatalf("accounts table not created: %v", err)
	}
	rows.Close()

	// Verify sessions table exists with expected columns
	rows, err = store.DB.Query(`SELECT id, item_id, session_id, current_time, duration, created_at FROM sessions LIMIT 0`)
	if err != nil {
		t.Fatalf("sessions table not created: %v", err)
	}
	rows.Close()
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	// Open twice — migrations should not fail on second run
	store1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() error: %v", err)
	}
	store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() error: %v", err)
	}
	defer store2.Close()

	// Insert a row, verify it survives re-open
	_, err = store2.DB.Exec(`INSERT INTO accounts (id, server_url, username, token, is_default, created_at) VALUES ('a1', 'http://localhost', 'user', 'tok', 1, datetime('now'))`)
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

func TestOpen_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}
