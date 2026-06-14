package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/Thelost77/pine/internal/db"
)

// Store provides generic get/put/delete/evict operations over a SQLite table.
type Store struct {
	db *db.Store
}

// NewStore creates a cache backed by the given database.
func NewStore(db *db.Store) *Store {
	return &Store{db: db}
}

// Get attempts to retrieve and decode the value for key into dest.
// Returns (true, nil) on hit, (false, nil) on miss.
func (s *Store) Get(key string, dest any) (bool, error) {
	var data []byte
	err := s.db.DB.QueryRow(
		`SELECT data FROM api_cache WHERE cache_key = ? AND expires_at > datetime('now')`,
		key,
	).Scan(&data)
	if err != nil {
		return false, nil // miss
	}

	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(dest); err != nil {
		return false, fmt.Errorf("decode cache value for key %q: %w", key, err)
	}
	return true, nil
}

// Put gob-encodes value and stores it with the given TTL.
func (s *Store) Put(key string, value any, ttl time.Duration) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("encode cache value for key %q: %w", key, err)
	}

	_, err := s.db.DB.Exec(
		`INSERT OR REPLACE INTO api_cache (cache_key, data, cached_at, expires_at)
		 VALUES (?, ?, datetime('now'), datetime('now', '+' || ? || ' seconds'))`,
		key, buf.Bytes(), int64(ttl.Seconds()),
	)
	if err != nil {
		return fmt.Errorf("write cache value for key %q: %w", key, err)
	}
	return nil
}

// Delete removes the entry for key.
func (s *Store) Delete(key string) error {
	_, err := s.db.DB.Exec(`DELETE FROM api_cache WHERE cache_key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete cache key %q: %w", key, err)
	}
	return nil
}

// EvictExpired removes all expired entries.
func (s *Store) EvictExpired() error {
	_, err := s.db.DB.Exec(`DELETE FROM api_cache WHERE expires_at <= datetime('now')`)
	if err != nil {
		return fmt.Errorf("evict expired cache entries: %w", err)
	}
	return nil
}

// ClearAll removes all cache entries.
func (s *Store) ClearAll() error {
	_, err := s.db.DB.Exec(`DELETE FROM api_cache`)
	if err != nil {
		return fmt.Errorf("clear all cache entries: %w", err)
	}
	return nil
}
