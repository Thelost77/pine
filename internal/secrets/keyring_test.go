package secrets

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestTokenKeyring(t *testing.T) {
	keyring.MockInit()

	if err := SetToken("https://abs.example.com/", "alice", "tok-1"); err != nil {
		t.Fatalf("SetToken returned error: %v", err)
	}

	token, err := GetToken("https://abs.example.com", "alice")
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}
	if token != "tok-1" {
		t.Fatalf("expected token %q, got %q", "tok-1", token)
	}

	if err := DeleteToken("https://abs.example.com", "alice"); err != nil {
		t.Fatalf("DeleteToken returned error: %v", err)
	}

	_, err = GetToken("https://abs.example.com", "alice")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
