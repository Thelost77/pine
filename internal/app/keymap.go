package app

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/Thelost77/pine/internal/config"
)

// KeyMap defines the global keybindings for the root model.
type KeyMap struct {
	Quit           key.Binding
	Back           key.Binding
	Help           key.Binding
	ChapterOverlay key.Binding
	NextInQueue    key.Binding
	NextChapter    key.Binding
	PrevChapter    key.Binding
	SleepTimer     key.Binding
}

// DefaultKeyMap returns the default keybindings using the given config.
func DefaultKeyMap(cfg config.KeybindsConfig) KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "left"),
			key.WithHelp("esc/←", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		ChapterOverlay: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "chapters"),
		),
		NextInQueue: key.NewBinding(
			key.WithKeys(cfg.NextInQueue),
			key.WithHelp(">", "next queued"),
		),
		NextChapter: key.NewBinding(
			key.WithKeys(cfg.NextChapter),
			key.WithHelp("n", "next chapter"),
		),
		PrevChapter: key.NewBinding(
			key.WithKeys(cfg.PrevChapter),
			key.WithHelp("N", "prev chapter"),
		),
		SleepTimer: key.NewBinding(
			key.WithKeys(cfg.SleepTimer),
			key.WithHelp("s", "sleep timer"),
		),
	}
}
