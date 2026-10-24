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
	Token     string
	CreatedAt time.Time
}

// SaveAccount inserts or updates an account and makes it the default. It
// deletes each other account in the same transaction, so that the database
// keeps no token of an earlier login.
func (s *Store) SaveAccount(a Account) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM accounts WHERE id != ?`, a.ID); err != nil {
		return fmt.Errorf("deleting other accounts: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO accounts (id, server_url, username, token, is_default, created_at)
		VALUES (?, ?, ?, ?, 1, COALESCE((SELECT created_at FROM accounts WHERE id = ?), datetime('now')))
		ON CONFLICT(id) DO UPDATE SET
			server_url = excluded.server_url,
			username   = excluded.username,
			token      = excluded.token,
			is_default = excluded.is_default
	`, a.ID, a.ServerURL, a.Username, a.Token, a.ID)
	if err != nil {
		return fmt.Errorf("saving account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing account save: %w", err)
	}
	return nil
}

// GetDefaultAccount returns the account marked as default.
// Returns an error if no default account is set. A database of an earlier
// pine can have more accounts, which are not the default.
func (s *Store) GetDefaultAccount() (Account, error) {
	row := s.DB.QueryRow(`
		SELECT id, server_url, username, token, created_at
		FROM accounts WHERE is_default = 1 LIMIT 1
	`)
	return scanAccount(row)
}

// scanAccount scans a single account from a *sql.Row.
func scanAccount(row *sql.Row) (Account, error) {
	var a Account
	var createdAt string
	err := row.Scan(&a.ID, &a.ServerURL, &a.Username, &a.Token, &createdAt)
	if err != nil {
		return Account{}, fmt.Errorf("scanning account: %w", err)
	}
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return a, nil
}
