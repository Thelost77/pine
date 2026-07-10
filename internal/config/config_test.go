package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load('') returned error: %v", err)
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

func TestLoadMigratesPlaybackIdentityAndPreservesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := []byte("[server]\naddress = \"https://abs.example\"\n\n[player]\nspeed = 1.5\n\n[theme]\naccent = \"#123456\"\n\n[keybinds]\nquit = \"ctrl+c\"\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DeviceID == "" || cfg.DeviceName == "" {
		t.Fatalf("identity not generated: %#v", cfg)
	}
	if cfg.Player.Speed != 1.5 || cfg.Theme.Accent != "#123456" || cfg.Keybinds.Quit != "ctrl+c" {
		t.Fatalf("existing config changed: %#v", cfg)
	}

	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if !strings.Contains(string(persisted), "device_id") || !strings.Contains(string(persisted), "device_name") {
		t.Fatalf("identity not persisted:\n%s", persisted)
	}
	if !strings.Contains(string(persisted), "[server]") || !strings.Contains(string(persisted), "address = \"https://abs.example\"") {
		t.Fatalf("legacy server config lost:\n%s", persisted)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.DeviceID != cfg.DeviceID || reloaded.DeviceName != cfg.DeviceName {
		t.Fatalf("identity changed after reload: got %#v, want %#v", reloaded, cfg)
	}
}

func TestLoadGeneratesDistinctDeviceIDs(t *testing.T) {
	first, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("first Load() error: %v", err)
	}
	second, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("second Load() error: %v", err)
	}
	if first.DeviceID == second.DeviceID {
		t.Fatalf("generated duplicate device ID %q", first.DeviceID)
	}
}

func TestLoadPreservesManualPlaybackIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte("device_name = \"Desk Pine\"\ndevice_id = \"pine-custom-id\"\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DeviceName != "Desk Pine" || cfg.DeviceID != "pine-custom-id" {
		t.Fatalf("manual identity changed: %#v", cfg)
	}

	cfg.DeviceName = "Laptop Pine"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save renamed device: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload renamed device: %v", err)
	}
	if reloaded.DeviceID != "pine-custom-id" {
		t.Fatalf("device ID changed after name edit: %q", reloaded.DeviceID)
	}
}

func TestLoadUpsertsPartialPlaybackIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{"name only", "# keep me\ndevice_name = \"Desk Pine\" # keep this\n[player]\nspeed = 1.5\n"},
		{"ID only", "device_id = \"pine-manual\"\n[server]\naddress = \"https://abs.example\"\n"},
		{"empty values", "device_name = \"\"\ndevice_id = \"\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(test.data), 0600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.DeviceName == "" || cfg.DeviceID == "" {
				t.Fatalf("identity not populated: %#v", cfg)
			}
			persisted, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			if strings.Count(string(persisted), "device_name =") != 1 || strings.Count(string(persisted), "device_id =") != 1 {
				t.Fatalf("identity keys duplicated:\n%s", persisted)
			}
			if test.name == "name only" && !strings.Contains(string(persisted), "# keep me") {
				t.Fatalf("unrelated comment lost:\n%s", persisted)
			}
			reloaded, err := Load(path)
			if err != nil {
				t.Fatalf("reload config: %v", err)
			}
			if reloaded.DeviceName != cfg.DeviceName || reloaded.DeviceID != cfg.DeviceID {
				t.Fatalf("identity changed after reload: got %#v, want %#v", reloaded, cfg)
			}
		})
	}
}

func TestLoadFullTOMLOverridesAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	toml := `
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

func TestEnsureExistsCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Default()
	if err := EnsureExists(path, cfg); err != nil {
		t.Fatalf("EnsureExists returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected config file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Theme.Accent != cfg.Theme.Accent {
		t.Fatalf("expected default accent %q, got %q", cfg.Theme.Accent, loaded.Theme.Accent)
	}
}

func TestEnsureExistsDoesNotOverwriteExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := []byte("[player]\nspeed = 1.5\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	if err := EnsureExists(path, Default()); err != nil {
		t.Fatalf("EnsureExists returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("existing config was overwritten: %q", string(got))
	}
}

func TestSaveOmitsLegacyServerConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(got) == "" || filepath.Base(path) != "config.toml" {
		t.Fatalf("unexpected empty config output")
	}
	if strings.Contains(string(got), "[server]") || strings.Contains(string(got), "address") {
		t.Fatalf("expected no legacy server config, got:\n%s", string(got))
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
