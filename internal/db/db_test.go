package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
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

func TestOpen_ClearsObfuscatedAccountTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	_, err = store.DB.Exec(`INSERT INTO accounts (id, server_url, username, token, is_default, created_at) VALUES ('a1', 'http://localhost', 'user', 'pine:v2:bad', 1, datetime('now'))`)
	if err != nil {
		t.Fatalf("insert obfuscated token failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatalf("reopen Open() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	var token string
	if err := store.DB.QueryRow(`SELECT token FROM accounts WHERE id = 'a1'`).Scan(&token); err != nil {
		t.Fatalf("select token: %v", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

// TestOpen_MigratesLegacySchema verifies that Open updates the database of
// an old pine: its accounts table has the column token_encrypted, and its
// sessions table has no column episode_id.
func TestOpen_MigratesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	for _, query := range []string{
		`CREATE TABLE accounts (id TEXT PRIMARY KEY, server_url TEXT NOT NULL, username TEXT NOT NULL, token_encrypted TEXT NOT NULL, is_default INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL)`,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, item_id TEXT NOT NULL, session_id TEXT NOT NULL, current_time REAL NOT NULL DEFAULT 0, duration REAL NOT NULL DEFAULT 0, created_at TEXT NOT NULL)`,
		`INSERT INTO accounts VALUES ('a1', 'http://s', 'u', 'tok', 1, datetime('now'))`,
		`INSERT INTO sessions VALUES ('last', 'item-1', 'sess-1', 10, 100, datetime('now'))`,
	} {
		if _, err := legacy.Exec(query); err != nil {
			t.Fatalf("make legacy schema: %v", err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	account, err := store.GetDefaultAccount()
	if err != nil || account.ID != "a1" || account.Token != "tok" {
		t.Fatalf("GetDefaultAccount() = %+v, %v; want a1 with the token tok", account, err)
	}
	last, err := store.GetLastSession()
	if err != nil || last.ItemID != "item-1" || last.EpisodeID != "" {
		t.Fatalf("GetLastSession() = %+v, %v; want item-1 with no episode", last, err)
	}
	if err := store.SaveListeningSession(ListeningSession{ItemID: "pod-1", EpisodeID: "ep-1"}); err != nil {
		t.Fatalf("SaveListeningSession() error: %v", err)
	}
	last, err = store.GetLastSession()
	if err != nil || last.ItemID != "pod-1" || last.EpisodeID != "ep-1" {
		t.Fatalf("GetLastSession() after save = %+v, %v; want pod-1 and ep-1", last, err)
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

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
	if err != nil {
		t.Fatalf("os.Stat() error: %v", err)
	}
	// Windows has no Unix file modes.
	if mode := info.Mode().Perm(); runtime.GOOS != "windows" && mode != 0o600 {
		t.Fatalf("database file mode = %o, want 0600", mode)
	}
}

// TestOpen_ParallelWritesWait verifies that a write waits while a write of a
// different connection holds the lock, and does not fail with SQLITE_BUSY.
// pine saves the session while the cache writes.
func TestOpen_ParallelWritesWait(t *testing.T) {
	s := openTestStore(t)

	const n = 32
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- s.SaveListeningSession(ListeningSession{ItemID: "item-" + strconv.Itoa(i)})
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("parallel SaveListeningSession() error: %v", err)
		}
	}
}
