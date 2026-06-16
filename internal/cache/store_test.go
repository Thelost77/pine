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
