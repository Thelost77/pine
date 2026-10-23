package cache

import (
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/db"
)

func openTestDB(t *testing.T) *db.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(dir + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStore_RoundTrip(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	type record struct {
		A string
		B int
	}

	want := record{A: "hello", B: 42}
	if err := s.Put("key1", want, 5*time.Minute); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	var got record
	hit, err := s.Get("key1", &got)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit, got miss")
	}
	if got.A != want.A || got.B != want.B {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStore_Miss(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	var v string
	hit, err := s.Get("missing", &v)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if hit {
		t.Fatal("expected cache miss")
	}
}

// TestStore_GetReportsDatabaseError verifies that Get gives a database error
// as an error, not as a miss.
func TestStore_GetReportsDatabaseError(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)
	if err := dbStore.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	var v string
	hit, err := s.Get("key", &v)
	if hit || err == nil {
		t.Fatalf("Get() = %v, %v; want a miss and an error", hit, err)
	}
}

// TestStore_KeysHaveVersion verifies that the store writes and deletes each key
// with the key version, and that it does not read a row with no version.
func TestStore_KeysHaveVersion(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)
	rows := func(key string) int {
		t.Helper()
		var n int
		if err := dbStore.DB.QueryRow(`SELECT COUNT(*) FROM api_cache WHERE cache_key = ?`, key).Scan(&n); err != nil {
			t.Fatalf("count rows: %v", err)
		}
		return n
	}

	if err := s.Put("k", "value", time.Minute); err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if n := rows("v1-k"); n != 1 {
		t.Fatalf("rows with key v1-k = %d, want 1", n)
	}
	if err := s.Delete("k"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if n := rows("v1-k"); n != 0 {
		t.Fatalf("rows with key v1-k after Delete = %d, want 0", n)
	}

	// A row of an older pine, with no version and data that does not decode.
	if _, err := dbStore.DB.Exec(`INSERT INTO api_cache (cache_key, data, cached_at, expires_at)
		VALUES ('old', x'00', datetime('now'), datetime('now', '+60 seconds'))`); err != nil {
		t.Fatalf("insert old row: %v", err)
	}
	var v string
	if hit, err := s.Get("old", &v); hit || err != nil {
		t.Fatalf("Get(old) = %v, %v; want a miss with no error", hit, err)
	}
}

func TestStore_Expiry(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	if err := s.Put("key2", "value", 1*time.Millisecond); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	var v string
	hit, err := s.Get("key2", &v)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if hit {
		t.Fatal("expected expired entry to be a miss")
	}
}

func TestStore_Delete(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	if err := s.Put("key3", 123, 5*time.Minute); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	if err := s.Delete("key3"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var v int
	hit, err := s.Get("key3", &v)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if hit {
		t.Fatal("expected deleted entry to be a miss")
	}
}

func TestStore_EvictExpired(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	if err := s.Put("exp1", "a", 1*time.Millisecond); err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if err := s.Put("exp2", "b", 1*time.Millisecond); err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if err := s.Put("live", "c", 5*time.Minute); err != nil {
		t.Fatalf("Put error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := s.EvictExpired(); err != nil {
		t.Fatalf("EvictExpired error: %v", err)
	}

	var v string
	if hit, _ := s.Get("exp1", &v); hit {
		t.Fatal("expected exp1 to be evicted")
	}
	if hit, _ := s.Get("exp2", &v); hit {
		t.Fatal("expected exp2 to be evicted")
	}
	hit, err := s.Get("live", &v)
	if err != nil {
		t.Fatalf("Get live error: %v", err)
	}
	if !hit {
		t.Fatal("expected live entry to survive eviction")
	}
	if v != "c" {
		t.Fatalf("got %q, want %q", v, "c")
	}
}
