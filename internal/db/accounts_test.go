package db

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSaveAccount_Insert(t *testing.T) {
	s := openTestStore(t)

	acc := Account{
		ID:        "acc-1",
		ServerURL: "http://localhost:13378",
		Username:  "admin",
		Token:     "tok-abc",
	}

	if err := s.SaveAccount(acc); err != nil {
		t.Fatalf("SaveAccount() error: %v", err)
	}

	got, err := s.GetDefaultAccount()
	if err != nil {
		t.Fatalf("GetDefaultAccount() error: %v", err)
	}
	if got.ID != acc.ID {
		t.Errorf("ID = %q, want %q", got.ID, acc.ID)
	}
	if got.ServerURL != acc.ServerURL {
		t.Errorf("ServerURL = %q, want %q", got.ServerURL, acc.ServerURL)
	}
	if got.Username != acc.Username {
		t.Errorf("Username = %q, want %q", got.Username, acc.Username)
	}
	if got.Token != acc.Token {
		t.Errorf("Token = %q, want %q", got.Token, acc.Token)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestSaveAccount_Upsert(t *testing.T) {
	s := openTestStore(t)

	acc := Account{
		ID:        "acc-1",
		ServerURL: "http://old.server",
		Username:  "admin",
		Token:     "tok-old",
	}
	if err := s.SaveAccount(acc); err != nil {
		t.Fatalf("first SaveAccount() error: %v", err)
	}

	acc.ServerURL = "http://new.server"
	acc.Token = "tok-new"
	if err := s.SaveAccount(acc); err != nil {
		t.Fatalf("second SaveAccount() error: %v", err)
	}

	got, err := s.GetDefaultAccount()
	if err != nil {
		t.Fatalf("GetDefaultAccount() error: %v", err)
	}
	if got.ServerURL != "http://new.server" {
		t.Errorf("ServerURL = %q, want %q", got.ServerURL, "http://new.server")
	}
	if got.Token != "tok-new" {
		t.Errorf("Token = %q, want %q", got.Token, "tok-new")
	}
}

func TestGetDefaultAccount_NoDefault(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetDefaultAccount()
	if err == nil {
		t.Fatal("expected error when no default account, got nil")
	}
}

func TestSaveAccount_FirstAccountBecomesDefault(t *testing.T) {
	s := openTestStore(t)

	acc := Account{ID: "a1", ServerURL: "http://s", Username: "u", Token: "t"}
	if err := s.SaveAccount(acc); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetDefaultAccount()
	if err != nil {
		t.Fatalf("first account should be default: %v", err)
	}
	if got.ID != "a1" {
		t.Errorf("default ID = %q, want %q", got.ID, "a1")
	}
}

// TestSaveAccount_DeletesOtherAccounts verifies that the database keeps no
// token of an earlier login.
func TestSaveAccount_DeletesOtherAccounts(t *testing.T) {
	s := openTestStore(t)

	if err := s.SaveAccount(Account{ID: "a1", ServerURL: "http://s1", Username: "u1", Token: "t1"}); err != nil {
		t.Fatalf("first SaveAccount() error: %v", err)
	}
	if err := s.SaveAccount(Account{ID: "a2", ServerURL: "http://s2", Username: "u2", Token: "t2"}); err != nil {
		t.Fatalf("second SaveAccount() error: %v", err)
	}

	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if count != 1 {
		t.Errorf("accounts table has %d rows, want 1", count)
	}
	got, err := s.GetDefaultAccount()
	if err != nil || got.ID != "a2" {
		t.Fatalf("GetDefaultAccount() = %+v, %v; want a2", got, err)
	}
}
