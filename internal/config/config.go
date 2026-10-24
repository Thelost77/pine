package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/Thelost77/pine/internal/ui"
)

// The limits of player.speed. mpv does not accept a speed above 100, and the
// speed-down key stops at 0.1.
const (
	minSpeed = 0.1
	maxSpeed = 100
)

type PlayerConfig struct {
	Speed       float64 `toml:"speed"`
	SeekSeconds int     `toml:"seek_seconds"`
}

type KeybindsConfig struct {
	Quit         string `toml:"quit"`
	PlayPause    string `toml:"play_pause"`
	SeekForward  string `toml:"seek_forward"`
	SeekBackward string `toml:"seek_backward"`
	NextInQueue  string `toml:"next_in_queue"`
	SpeedUp      string `toml:"speed_up"`
	SpeedDown    string `toml:"speed_down"`
	VolumeUp     string `toml:"volume_up"`
	VolumeDown   string `toml:"volume_down"`
	SleepTimer   string `toml:"sleep_timer"`
	Back         string `toml:"back"`
}

type Config struct {
	Player     PlayerConfig   `toml:"player"`
	Theme      ui.ThemeConfig `toml:"theme"`
	Keybinds   KeybindsConfig `toml:"keybinds"`
	DeviceName string         `toml:"device_name"`
	DeviceID   string         `toml:"device_id"`
}

func Default() Config {
	return Config{
		Player: PlayerConfig{
			Speed:       1.0,
			SeekSeconds: 10,
		},
		Theme: ui.DefaultTheme(),
		Keybinds: KeybindsConfig{
			Quit:         "q",
			PlayPause:    " ",
			SeekForward:  "l",
			SeekBackward: "h",
			NextInQueue:  ">",
			SpeedUp:      "+",
			SpeedDown:    "-",
			VolumeUp:     "]",
			VolumeDown:   "[",
			SleepTimer:   "S",
			Back:         "esc",
		},
		DeviceName: defaultDeviceName(),
	}
}

// Load reads a TOML config file at path, merging with defaults.
// If path is empty or the file doesn't exist, defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, err := ensurePlaybackIdentity(&cfg); err != nil {
				return Config{}, fmt.Errorf("generate playback device identity: %w", err)
			}
			return cfg, nil
		}
		return cfg, err
	}

	metadata, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, err
	}
	if s := cfg.Player.Speed; math.IsNaN(s) || s < minSpeed || s > maxSpeed {
		return Config{}, fmt.Errorf("player.speed is %v: it must be from %v to %v", s, minSpeed, maxSpeed)
	}
	if cfg.Player.SeekSeconds <= 0 {
		return Config{}, fmt.Errorf("player.seek_seconds is %d: it must be more than 0", cfg.Player.SeekSeconds)
	}
	changed, err := ensurePlaybackIdentity(&cfg)
	if err != nil {
		return Config{}, fmt.Errorf("generate playback device identity: %w", err)
	}
	if changed || !metadata.IsDefined("device_name") || !metadata.IsDefined("device_id") {
		if err := saveMigratedPlaybackIdentity(path, data, cfg); err != nil {
			return Config{}, fmt.Errorf("persist playback device identity: %w", err)
		}
	}

	return cfg, nil
}

func saveMigratedPlaybackIdentity(path string, data []byte, cfg Config) error {
	// TOML allows a byte order mark only at the start of the file, and the
	// identity comes before data.
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	var identity bytes.Buffer
	if err := toml.NewEncoder(&identity).Encode(struct {
		DeviceName string `toml:"device_name"`
		DeviceID   string `toml:"device_id"`
	}{cfg.DeviceName, cfg.DeviceID}); err != nil {
		return err
	}

	insertAt := len(data)
	for offset, line := 0, data; len(line) > 0; {
		next := bytes.IndexByte(line, '\n')
		lineEnd := len(line)
		if next >= 0 {
			lineEnd = next + 1
		}
		if bytes.HasPrefix(bytes.TrimSpace(line[:lineEnd]), []byte("[")) {
			insertAt = offset
			break
		}
		offset += lineEnd
		if next < 0 {
			break
		}
		line = line[lineEnd:]
	}

	prefix := withoutTopLevelIdentity(data[:insertAt])
	migrated := make([]byte, 0, len(data)+identity.Len())
	migrated = append(migrated, identity.Bytes()...)
	migrated = append(migrated, prefix...)
	migrated = append(migrated, data[insertAt:]...)
	return os.WriteFile(path, migrated, 0600)
}

func withoutTopLevelIdentity(data []byte) []byte {
	var result bytes.Buffer
	for rest := data; len(rest) > 0; {
		next := bytes.IndexByte(rest, '\n')
		lineEnd := len(rest)
		if next >= 0 {
			lineEnd = next
		}
		line := rest[:lineEnd]
		newline := []byte(nil)
		if next >= 0 {
			newline = []byte("\n")
		}
		if !isIdentityAssignment(line, "device_name") && !isIdentityAssignment(line, "device_id") {
			result.Write(line)
			result.Write(newline)
		}
		if next < 0 {
			break
		}
		rest = rest[next+1:]
	}
	return result.Bytes()
}

// isIdentityAssignment reports whether line sets key. The key can be bare or
// quoted.
func isIdentityAssignment(line []byte, key string) bool {
	line = bytes.TrimLeft(line, " \t")
	for _, form := range []string{key, `"` + key + `"`, "'" + key + "'"} {
		if rest, ok := bytes.CutPrefix(line, []byte(form)); ok {
			rest = bytes.TrimLeft(rest, " \t")
			return len(rest) > 0 && rest[0] == '='
		}
	}
	return false
}

func defaultDeviceName() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "Pine"
	}
	return hostname + " (Pine)"
}

func ensurePlaybackIdentity(cfg *Config) (bool, error) {
	changed := false
	if cfg.DeviceName == "" {
		cfg.DeviceName = defaultDeviceName()
		changed = true
	}
	if cfg.DeviceID != "" {
		return changed, nil
	}

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return false, err
	}
	cfg.DeviceID = "pine-" + normalizedHostname() + "-" + hex.EncodeToString(suffix)
	return true, nil
}

func normalizedHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "host"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(hostname) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	if normalized := strings.Trim(b.String(), "-"); normalized != "" {
		return normalized
	}
	return "host"
}

// Save writes a Config struct to path in TOML format.
func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0600)
}

// EnsureExists creates a default config file when path does not exist.
func EnsureExists(path string, cfg Config) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return Save(path, cfg)
}

// ConfigDir returns the abs-cli configuration directory path.
func ConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "pine")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pine")
}
