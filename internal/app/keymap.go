package app

import (
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/player"
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines the global keybindings for the root model.
type KeyMap struct {
	Quit           key.Binding
	Back           key.Binding
	Help           key.Binding
	ChapterOverlay key.Binding
	Captions       key.Binding
	NextInQueue    key.Binding
	SleepTimer     key.Binding
	GlobalPalette  key.Binding
	Sessions       key.Binding
}

// DefaultKeyMap returns the default keybindings using the given config.
func DefaultKeyMap(cfg config.KeybindsConfig) KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys(cfg.Quit),
			key.WithHelp(cfg.Quit, "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys(cfg.Back, "left"),
			key.WithHelp(cfg.Back, "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		ChapterOverlay: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "chapters"),
		),
		Captions: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "captions"),
		),
		NextInQueue: key.NewBinding(
			key.WithKeys(cfg.NextInQueue),
			key.WithHelp(">", "next queued"),
		),
		SleepTimer: key.NewBinding(
			key.WithKeys(cfg.SleepTimer),
			key.WithHelp(cfg.SleepTimer, "sleep timer"),
		),
		GlobalPalette: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "command palette"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "listening sessions"),
		),
	}
}

// playerKeys returns the key names that the player model uses.
func playerKeys(cfg config.KeybindsConfig) player.Keys {
	return player.Keys{
		PlayPause:    cfg.PlayPause,
		SeekForward:  cfg.SeekForward,
		SeekBackward: cfg.SeekBackward,
		SpeedUp:      cfg.SpeedUp,
		SpeedDown:    cfg.SpeedDown,
		VolumeUp:     cfg.VolumeUp,
		VolumeDown:   cfg.VolumeDown,
	}
}
