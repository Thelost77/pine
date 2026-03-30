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
	t.Cleanup(func() { store.Close() })
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
	}
	if err := s.SaveAccount(acc); err != nil {
		t.Fatalf("first SaveAccount() error: %v", err)
	}

	acc.ServerURL = "http://new.server"
	acc.Token = "tok-new"
	if err := s.SaveAccount(acc); err != nil {
		t.Fatalf("second SaveAccount() error: %v", err)
	}

	list, err := s.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0].ServerURL != "http://new.server" {
		t.Errorf("ServerURL = %q, want %q", list[0].ServerURL, "http://new.server")
	}
	if list[0].Token != "tok-new" {
		t.Errorf("Token = %q, want %q", list[0].Token, "tok-new")
	}
}

func TestGetDefaultAccount_NoDefault(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetDefaultAccount()
	if err == nil {
		t.Fatal("expected error when no default account, got nil")
	}
}

func TestSetDefaultAccount(t *testing.T) {
	s := openTestStore(t)

	// Insert two accounts, first is default
	a1 := Account{ID: "a1", ServerURL: "http://s1", Username: "u1", Token: "t1", IsDefault: true}
	a2 := Account{ID: "a2", ServerURL: "http://s2", Username: "u2", Token: "t2", IsDefault: false}

	if err := s.SaveAccount(a1); err != nil {
		t.Fatalf("SaveAccount(a1) error: %v", err)
	}
	if err := s.SaveAccount(a2); err != nil {
		t.Fatalf("SaveAccount(a2) error: %v", err)
	}

	// Switch default to a2
	if err := s.SetDefaultAccount("a2"); err != nil {
		t.Fatalf("SetDefaultAccount() error: %v", err)
	}

	got, err := s.GetDefaultAccount()
	if err != nil {
		t.Fatalf("GetDefaultAccount() error: %v", err)
	}
	if got.ID != "a2" {
		t.Errorf("default account ID = %q, want %q", got.ID, "a2")
	}

	// Verify a1 is no longer default
	list, _ := s.ListAccounts()
	for _, a := range list {
		if a.ID == "a1" && a.IsDefault {
			t.Error("a1 should no longer be default")
		}
	}
}

func TestSetDefaultAccount_NotFound(t *testing.T) {
	s := openTestStore(t)

	err := s.SetDefaultAccount("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent account, got nil")
	}
}

func TestListAccounts_Empty(t *testing.T) {
	s := openTestStore(t)

	list, err := s.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}

func TestListAccounts_Multiple(t *testing.T) {
	s := openTestStore(t)

	for _, id := range []string{"a1", "a2", "a3"} {
		acc := Account{ID: id, ServerURL: "http://s", Username: "u", Token: "t"}
		if err := s.SaveAccount(acc); err != nil {
			t.Fatalf("SaveAccount(%s) error: %v", id, err)
		}
	}

	list, err := s.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
}

func TestDeleteAccount(t *testing.T) {
	s := openTestStore(t)

	acc := Account{ID: "a1", ServerURL: "http://s", Username: "u", Token: "t"}
	if err := s.SaveAccount(acc); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteAccount("a1"); err != nil {
		t.Fatalf("DeleteAccount() error: %v", err)
	}

	list, _ := s.ListAccounts()
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0 after delete", len(list))
	}
}

func TestDeleteAccount_NotFound(t *testing.T) {
	s := openTestStore(t)

	err := s.DeleteAccount("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent account, got nil")
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
