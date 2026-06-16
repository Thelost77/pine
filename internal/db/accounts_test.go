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
		IsDefault: true,
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
	if !got.IsDefault {
		t.Error("IsDefault = false, want true")
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
		IsDefault: true,
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

	// Save without explicitly setting IsDefault
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
