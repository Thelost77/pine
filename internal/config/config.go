package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ServerConfig struct {
	Address string `toml:"address"`
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
	NextChapter  string `toml:"next_chapter"`
	PrevChapter  string `toml:"prev_chapter"`
	SleepTimer   string `toml:"sleep_timer"`
	Select       string `toml:"select"`
	Back         string `toml:"back"`
	Up           string `toml:"up"`
	Down         string `toml:"down"`
	Search       string `toml:"search"`
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
			NextChapter:  "n",
			PrevChapter:  "N",
			SleepTimer:   "s",
			Select:       "enter",
			Back:         "esc",
			Up:           "k",
			Down:         "j",
			Search:       "/",
		},
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
			return cfg, nil
		}
		return cfg, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// ConfigDir returns the abs-cli configuration directory path.
func ConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "pine")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pine")
}
