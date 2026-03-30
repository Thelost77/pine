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
		INSERT INTO accounts (id, server_url, username, token, is_default, created_at)
		VALUES (?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM accounts WHERE id = ?), datetime('now')))
		ON CONFLICT(id) DO UPDATE SET
			server_url      = excluded.server_url,
			username        = excluded.username,
			token = excluded.token,
			is_default      = excluded.is_default
	`, a.ID, a.ServerURL, a.Username, a.Token, isDefault, a.ID)
	if err != nil {
		return fmt.Errorf("saving account: %w", err)
	}
	return nil
}

// GetDefaultAccount returns the account marked as default.
// Returns an error if no default account is set.
func (s *Store) GetDefaultAccount() (Account, error) {
	row := s.DB.QueryRow(`
		SELECT id, server_url, username, token, is_default, created_at
		FROM accounts WHERE is_default = 1 LIMIT 1
	`)
	return scanAccount(row)
}

// SetDefaultAccount sets the given account as default and clears the flag on
// all others. Returns an error if the account does not exist.
func (s *Store) SetDefaultAccount(id string) error {
	var exists bool
	if err := s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = ?)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("checking account existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("account %q not found", id)
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE accounts SET is_default = 0`); err != nil {
		return fmt.Errorf("clearing defaults: %w", err)
	}
	if _, err := tx.Exec(`UPDATE accounts SET is_default = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("setting default: %w", err)
	}

	return tx.Commit()
}

// ListAccounts returns all stored accounts.
func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.DB.Query(`
		SELECT id, server_url, username, token, is_default, created_at
		FROM accounts ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		a, err := scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	if accounts == nil {
		accounts = []Account{}
	}
	return accounts, rows.Err()
}

// DeleteAccount removes an account by ID. Returns an error if it doesn't exist.
func (s *Store) DeleteAccount(id string) error {
	res, err := s.DB.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting account: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("account %q not found", id)
	}
	return nil
}

// scanAccount scans a single account from a *sql.Row.
func scanAccount(row *sql.Row) (Account, error) {
	var a Account
	var isDefault int
	var createdAt string
	err := row.Scan(&a.ID, &a.ServerURL, &a.Username, &a.Token, &isDefault, &createdAt)
	if err != nil {
		return Account{}, fmt.Errorf("scanning account: %w", err)
	}
	a.IsDefault = isDefault == 1
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return a, nil
}

// scanAccountRows scans a single account from *sql.Rows.
func scanAccountRows(rows *sql.Rows) (Account, error) {
	var a Account
	var isDefault int
	var createdAt string
	err := rows.Scan(&a.ID, &a.ServerURL, &a.Username, &a.Token, &isDefault, &createdAt)
	if err != nil {
		return Account{}, fmt.Errorf("scanning account: %w", err)
	}
	a.IsDefault = isDefault == 1
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return a, nil
}
