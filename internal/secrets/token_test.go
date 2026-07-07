package secrets

import (
	"strings"
	"testing"
)

func TestTokenObfuscationRoundTrip(t *testing.T) {
	oldMachineIdentity := machineIdentity
	machineIdentity = func() string { return "machine-one" }
	t.Cleanup(func() { machineIdentity = oldMachineIdentity })

	stored := EncodeToken("https://abs.example.com/", "alice", "tok-1")
	if stored == "" {
		t.Fatal("expected stored token")
	}
	if !strings.HasPrefix(stored, tokenPrefixV2) {
		t.Fatalf("expected v2 token prefix, got %q", stored)
	}
	if stored == "tok-1" || strings.Contains(stored, "tok-1") {
		t.Fatalf("expected token to be obfuscated, got %q", stored)
	}
	if !IsObfuscatedToken(stored) {
		t.Fatalf("expected obfuscated token prefix, got %q", stored)
	}
	if !IsCurrentToken(stored) {
		t.Fatalf("expected current token prefix, got %q", stored)
	}

	token, err := DecodeToken("https://tailscale.example.com", "bob", stored)
	if err != nil {
		t.Fatalf("DecodeToken returned error: %v", err)
	}
	if token != "tok-1" {
		t.Fatalf("token = %q, want tok-1", token)
	}
}

func TestDecodeTokenV2RejectsDifferentMachine(t *testing.T) {
	oldMachineIdentity := machineIdentity
	machineIdentity = func() string { return "machine-one" }
	stored := EncodeToken("https://abs.example.com", "alice", "tok-1")
	machineIdentity = func() string { return "machine-two" }
	t.Cleanup(func() { machineIdentity = oldMachineIdentity })

	if _, err := DecodeToken("https://abs.example.com", "alice", stored); err == nil {
		t.Fatal("expected integrity error for different machine")
	}
}

func TestDecodeTokenRejectsUnsupportedObfuscatedToken(t *testing.T) {
	if _, err := DecodeToken("https://abs.example.com", "alice", "pine:v1:old-token"); err == nil {
		t.Fatal("expected unsupported token format error")
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
