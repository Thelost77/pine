package app

import (
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/spruce/config"
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Quit          key.Binding
	Back          key.Binding
	Help          key.Binding
	NextTrack     key.Binding
	PrevTrack     key.Binding
	Shuffle       key.Binding
	RepeatTrack   key.Binding
	RepeatQueue   key.Binding
	OpenPlaylists key.Binding
	GlobalPalette key.Binding
	SleepTimer    key.Binding
}

func DefaultKeyMap(cfg config.KeybindsConfig) KeyMap {
	def := config.Default().Keybinds
	quit := cfg.Quit
	if quit == "" {
		quit = def.Quit
	}
	back := cfg.Back
	if back == "" {
		back = def.Back
	}
	next := cfg.NextTrack
	if next == "" {
		next = def.NextTrack
	}
	prev := cfg.PrevTrack
	if prev == "" {
		prev = def.PrevTrack
	}
	sleep := cfg.SleepTimer
	if sleep == "" {
		sleep = def.SleepTimer
	}
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys(quit),
			key.WithHelp(quit, "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys(back, "left"),
			key.WithHelp(back, "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		NextTrack: key.NewBinding(
			key.WithKeys(next, "n", ">"),
			key.WithHelp(">/n", "next track"),
		),
		PrevTrack: key.NewBinding(
			key.WithKeys(prev, "p", "<"),
			key.WithHelp("</p", "prev track"),
		),
		Shuffle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "shuffle"),
		),
		RepeatTrack: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "repeat track"),
		),
		RepeatQueue: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "repeat queue"),
		),
		OpenPlaylists: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open playlists"),
		),
		GlobalPalette: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "command palette"),
		),
		SleepTimer: key.NewBinding(
			key.WithKeys(sleep),
			key.WithHelp(sleep, "sleep timer"),
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
