package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

type ServerConfig struct {
	Address    string `toml:"address"`
	Username   string `toml:"username"`
	Token      string `toml:"token"`
	UserID     string `toml:"user_id"`
	DeviceName string `toml:"device_name"`
	DeviceID   string `toml:"device_id"`
}

type PlayerConfig struct {
	Speed       float64 `toml:"speed"`
	SeekSeconds int     `toml:"seek_seconds"`
}

type ThemeConfig struct {
	Background string `toml:"background"`
	Foreground string `toml:"foreground"`
	Accent     string `toml:"accent"`
	Error      string `toml:"error"`
	Muted      string `toml:"muted"`
	Selected   string `toml:"selected"`
	Border     string `toml:"border"`
	Warning    string `toml:"warning"`
	Info       string `toml:"info"`
}

type KeybindsConfig struct {
	Quit         string `toml:"quit"`
	PlayPause    string `toml:"play_pause"`
	SeekForward  string `toml:"seek_forward"`
	SeekBackward string `toml:"seek_backward"`
	SpeedUp      string `toml:"speed_up"`
	SpeedDown    string `toml:"speed_down"`
	VolumeUp     string `toml:"volume_up"`
	VolumeDown   string `toml:"volume_down"`
	NextTrack    string `toml:"next_track"`
	PrevTrack    string `toml:"prev_track"`
	SleepTimer   string `toml:"sleep_timer"`
	Back         string `toml:"back"`
}

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Player   PlayerConfig   `toml:"player"`
	Theme    ThemeConfig    `toml:"theme"`
	Keybinds KeybindsConfig `toml:"keybinds"`
}

func Default() Config {
	return Config{
		Player: PlayerConfig{
			Speed:       1.0,
			SeekSeconds: 10,
		},
		Theme: ThemeConfig{
			Background: "#2b3339",
			Foreground: "#d3c6aa",
			Accent:     "#a7c080",
			Error:      "#e67e80",
			Muted:      "#859289",
			Selected:   "#475258",
			Border:     "#4f585e",
			Warning:    "#dbbc7f",
			Info:       "#7fbbb3",
		},
		Keybinds: KeybindsConfig{
			Quit:         "q",
			PlayPause:    " ",
			SeekForward:  "l",
			SeekBackward: "h",
			SpeedUp:      "+",
			SpeedDown:    "-",
			VolumeUp:     "]",
			VolumeDown:   "[",
			NextTrack:    "n",
			PrevTrack:    "N",
			SleepTimer:   "S",
			Back:         "esc",
		},
	}
}

// Load reads a TOML config file at path, merging with defaults.
// If path is empty or the file doesn't exist, defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		if _, err := ensureDevice(&cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, err := ensureDevice(&cfg); err != nil {
				return Config{}, err
			}
			return cfg, nil
		}
		return cfg, err
	}

	metadata, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, err
	}
	changed, err := migrateLegacyKeybinds(data, metadata, &cfg)
	if err != nil {
		return Config{}, err
	}
	deviceChanged, err := ensureDevice(&cfg)
	if err != nil {
		return Config{}, err
	}
	changed = changed || deviceChanged
	if changed {
		if err := Save(path, cfg); err != nil {
			return Config{}, fmt.Errorf("persist device identity migration: %w", err)
		}
	}

	return cfg, nil
}

func migrateLegacyKeybinds(data []byte, metadata toml.MetaData, cfg *Config) (bool, error) {
	hasLegacyNext := metadata.IsDefined("keybinds", "next_chapter")
	hasLegacyPrev := metadata.IsDefined("keybinds", "prev_chapter")
	hasDeadNext := metadata.IsDefined("keybinds", "next_in_queue")
	if !hasLegacyNext && !hasLegacyPrev && !hasDeadNext {
		return false, nil
	}

	var legacy struct {
		Keybinds struct {
			NextTrack string `toml:"next_chapter"`
			PrevTrack string `toml:"prev_chapter"`
		} `toml:"keybinds"`
	}
	if err := toml.Unmarshal(data, &legacy); err != nil {
		return false, fmt.Errorf("decode legacy keybinds: %w", err)
	}
	if hasLegacyNext && !metadata.IsDefined("keybinds", "next_track") && legacy.Keybinds.NextTrack != "" {
		cfg.Keybinds.NextTrack = legacy.Keybinds.NextTrack
	}
	if hasLegacyPrev && !metadata.IsDefined("keybinds", "prev_track") && legacy.Keybinds.PrevTrack != "" {
		cfg.Keybinds.PrevTrack = legacy.Keybinds.PrevTrack
	}
	return true, nil
}

func ensureDevice(cfg *Config) (bool, error) {
	changed := false
	if cfg.Server.DeviceName == "" {
		hostname, err := os.Hostname()
		if err != nil || hostname == "" {
			hostname = "This device"
		}
		cfg.Server.DeviceName = hostname + " (Spruce)"
		changed = true
	}
	if cfg.Server.DeviceID == "" {
		var suffix [4]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return false, fmt.Errorf("generate device ID: %w", err)
		}
		cfg.Server.DeviceID = "spruce-" + normalizeHostname() + "-" + hex.EncodeToString(suffix[:])
		changed = true
	}
	return changed, nil
}

func normalizeHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "device"
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
	return "device"
}

// ConfigDir returns the spruce configuration directory path.
func ConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "spruce")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "spruce")
}

// Save writes a Config struct to path in TOML format atomically with 0600 permissions.
func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
