package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Account represents a stored Audiobookshelf server account.
type Account struct {
	ID        string
	ServerURL string
	Username  string
	IsDefault bool
	CreatedAt time.Time
}

// SaveAccount inserts or updates an account. If it's the first account, it
// becomes the default automatically.
func (s *Store) SaveAccount(a Account) error {
	// If no accounts exist yet, make this one the default.
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		return fmt.Errorf("counting accounts: %w", err)
	}
	if count == 0 {
		a.IsDefault = true
	}

	isDefault := 0
	if a.IsDefault {
		isDefault = 1
	}

	_, err := s.DB.Exec(`
		INSERT INTO accounts (id, server_url, username, is_default, created_at)
		VALUES (?, ?, ?, ?, COALESCE((SELECT created_at FROM accounts WHERE id = ?), datetime('now')))
		ON CONFLICT(id) DO UPDATE SET
			server_url = excluded.server_url,
			username   = excluded.username,
			is_default = excluded.is_default
	`, a.ID, a.ServerURL, a.Username, isDefault, a.ID)
	if err != nil {
		return fmt.Errorf("saving account: %w", err)
	}
	return nil
}

// GetDefaultAccount returns the account marked as default.
// Returns an error if no default account is set.
func (s *Store) GetDefaultAccount() (Account, error) {
	row := s.DB.QueryRow(`
		SELECT id, server_url, username, is_default, created_at
		FROM accounts WHERE is_default = 1 LIMIT 1
	`)
	return scanAccount(row)
}

// scanAccount scans a single account from a *sql.Row.
func scanAccount(row *sql.Row) (Account, error) {
	var a Account
	var isDefault int
	var createdAt string
	err := row.Scan(&a.ID, &a.ServerURL, &a.Username, &isDefault, &createdAt)
	if err != nil {
		return Account{}, fmt.Errorf("scanning account: %w", err)
	}
	a.IsDefault = isDefault == 1
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return a, nil
}
