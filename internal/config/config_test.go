package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load('') returned error: %v", err)
	}

	// Server defaults
	if cfg.Server.Address != "" {
		t.Errorf("expected empty server address, got %q", cfg.Server.Address)
	}

	// Player defaults
	if cfg.Player.Speed != 1.0 {
		t.Errorf("expected speed 1.0, got %f", cfg.Player.Speed)
	}
	if cfg.Player.SeekSeconds != 10 {
		t.Errorf("expected seek_seconds 10, got %d", cfg.Player.SeekSeconds)
	}

	// Theme: Everforest Dark defaults
	if cfg.Theme.Background != "#2b3339" {
		t.Errorf("expected background #2b3339, got %q", cfg.Theme.Background)
	}
	if cfg.Theme.Foreground != "#d3c6aa" {
		t.Errorf("expected foreground #d3c6aa, got %q", cfg.Theme.Foreground)
	}
	if cfg.Theme.Accent != "#a7c080" {
		t.Errorf("expected accent #a7c080, got %q", cfg.Theme.Accent)
	}
	if cfg.Theme.Error != "#e67e80" {
		t.Errorf("expected error #e67e80, got %q", cfg.Theme.Error)
	}
	if cfg.Theme.Muted != "#859289" {
		t.Errorf("expected muted #859289, got %q", cfg.Theme.Muted)
	}
	if cfg.Theme.Selected != "#475258" {
		t.Errorf("expected selected #475258, got %q", cfg.Theme.Selected)
	}
	if cfg.Theme.Border != "#4f585e" {
		t.Errorf("expected border #4f585e, got %q", cfg.Theme.Border)
	}
	if cfg.Theme.Warning != "#dbbc7f" {
		t.Errorf("expected warning #dbbc7f, got %q", cfg.Theme.Warning)
	}
	if cfg.Theme.Info != "#7fbbb3" {
		t.Errorf("expected info #7fbbb3, got %q", cfg.Theme.Info)
	}
}

func TestLoadNonexistentPathReturnsDefaults(t *testing.T) {
	cfg, err := Load("/tmp/abs-cli-test-nonexistent-config.toml")
	if err != nil {
		t.Fatalf("Load(nonexistent) returned error: %v", err)
	}
	if cfg.Player.Speed != 1.0 {
		t.Errorf("expected default speed, got %f", cfg.Player.Speed)
	}
}

func TestLoadPartialTOMLMergesWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	toml := `
[server]
address = "https://my-abs.example.com"

[player]
speed = 1.5
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Overridden values
	if cfg.Server.Address != "https://my-abs.example.com" {
		t.Errorf("expected overridden address, got %q", cfg.Server.Address)
	}
	if cfg.Player.Speed != 1.5 {
		t.Errorf("expected overridden speed 1.5, got %f", cfg.Player.Speed)
	}

	// Defaults preserved for unset fields
	if cfg.Player.SeekSeconds != 10 {
		t.Errorf("expected default seek_seconds 10, got %d", cfg.Player.SeekSeconds)
	}
	if cfg.Theme.Accent != "#a7c080" {
		t.Errorf("expected default accent color, got %q", cfg.Theme.Accent)
	}
	if cfg.Theme.Background != "#2b3339" {
		t.Errorf("expected default background, got %q", cfg.Theme.Background)
	}
}

func TestLoadFullTOMLOverridesAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	toml := `
[server]
address = "https://custom.example.com"

[player]
speed = 2.0
seek_seconds = 30

[theme]
background = "#000000"
foreground = "#ffffff"
accent = "#ff0000"
error = "#ff0000"
muted = "#888888"
selected = "#333333"
border = "#444444"
warning = "#ffff00"
info = "#00ffff"
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Address != "https://custom.example.com" {
		t.Errorf("expected custom address, got %q", cfg.Server.Address)
	}
	if cfg.Player.Speed != 2.0 {
		t.Errorf("expected speed 2.0, got %f", cfg.Player.Speed)
	}
	if cfg.Player.SeekSeconds != 30 {
		t.Errorf("expected seek_seconds 30, got %d", cfg.Player.SeekSeconds)
	}
	if cfg.Theme.Background != "#000000" {
		t.Errorf("expected custom background, got %q", cfg.Theme.Background)
	}
	if cfg.Theme.Foreground != "#ffffff" {
		t.Errorf("expected custom foreground, got %q", cfg.Theme.Foreground)
	}
}

func TestLoadInvalidTOMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(path, []byte("{{invalid toml"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()
	if dir == "" {
		t.Error("ConfigDir() returned empty string")
	}
	if filepath.Base(dir) != "pine" {
		t.Errorf("expected dir to end with 'abs-cli', got %q", dir)
	}
}

func TestLoadKeybinds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	toml := `
[keybinds]
quit = "ctrl+c"
play_pause = "space"
`
	if err := os.WriteFile(path, []byte(toml), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Keybinds.Quit != "ctrl+c" {
		t.Errorf("expected quit keybind 'ctrl+c', got %q", cfg.Keybinds.Quit)
	}
	if cfg.Keybinds.PlayPause != "space" {
		t.Errorf("expected play_pause keybind 'space', got %q", cfg.Keybinds.PlayPause)
	}
}

func TestDefaultKeybinds(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load('') returned error: %v", err)
	}

	if cfg.Keybinds.Quit != "q" {
		t.Errorf("expected default quit 'q', got %q", cfg.Keybinds.Quit)
	}
	if cfg.Keybinds.PlayPause != " " {
		t.Errorf("expected default play_pause ' ', got %q", cfg.Keybinds.PlayPause)
	}
	if cfg.Keybinds.SeekForward != "l" {
		t.Errorf("expected default seek_forward 'l', got %q", cfg.Keybinds.SeekForward)
	}
	if cfg.Keybinds.SeekBackward != "h" {
		t.Errorf("expected default seek_backward 'h', got %q", cfg.Keybinds.SeekBackward)
	}
}
