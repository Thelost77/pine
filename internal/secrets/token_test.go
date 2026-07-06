package secrets

import (
	"strings"
	"testing"
)

func TestTokenObfuscationRoundTrip(t *testing.T) {
	stored := EncodeToken("https://abs.example.com/", "alice", "tok-1")
	if stored == "" {
		t.Fatal("expected stored token")
	}
	if stored == "tok-1" || strings.Contains(stored, "tok-1") {
		t.Fatalf("expected token to be obfuscated, got %q", stored)
	}
	if !IsObfuscatedToken(stored) {
		t.Fatalf("expected obfuscated token prefix, got %q", stored)
	}

	token, err := DecodeToken("https://abs.example.com", "alice", stored)
	if err != nil {
		t.Fatalf("DecodeToken returned error: %v", err)
	}
	if token != "tok-1" {
		t.Fatalf("token = %q, want tok-1", token)
	}
}

func TestDecodeTokenAllowsLegacyPlaintext(t *testing.T) {
	token, err := DecodeToken("https://abs.example.com", "alice", "plain-token")
	if err != nil {
		t.Fatalf("DecodeToken returned error: %v", err)
	}
	if token != "plain-token" {
		t.Fatalf("token = %q, want plain-token", token)
	}
}
