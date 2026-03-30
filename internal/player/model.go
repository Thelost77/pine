package player

import (
	"fmt"
	"math"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/ui"
)

// StartPlayMsg signals that playback should begin.
type StartPlayMsg struct {
	Title string
}

// Model is the bubbletea sub-model for the player footer bar.
type Model struct {
	Playing        bool
	Title          string
	Position       float64
	Duration       float64
	Speed          float64
	Volume         int
	SleepRemaining string

	player Player
	config config.PlayerConfig
	styles ui.Styles
	width  int
	keys   PlayerKeyMap
}

// PlayerKeyMap defines keybindings active when the player is playing.
type PlayerKeyMap struct {
	PlayPause   key.Binding
	SeekForward key.Binding
	SeekBack    key.Binding
	SpeedUp     key.Binding
	SpeedDown   key.Binding
	VolumeUp    key.Binding
	VolumeDown  key.Binding
}

// NewModel creates a new player sub-model.
func NewModel(p Player, cfg config.Config, styles ui.Styles) Model {
	return Model{
		Speed:  cfg.Player.Speed,
		Volume: 100,
		player: p,
		config: cfg.Player,
		styles: styles,
		keys: PlayerKeyMap{
			PlayPause: key.NewBinding(
				key.WithKeys(cfg.Keybinds.PlayPause),
				key.WithHelp("space", "play/pause"),
			),
			SeekForward: key.NewBinding(
				key.WithKeys(cfg.Keybinds.SeekForward),
				key.WithHelp("l", "seek forward"),
			),
			SeekBack: key.NewBinding(
				key.WithKeys(cfg.Keybinds.SeekBackward),
				key.WithHelp("h", "seek backward"),
			),
			SpeedUp: key.NewBinding(
				key.WithKeys(cfg.Keybinds.SpeedUp),
				key.WithHelp("+", "speed up"),
			),
			SpeedDown: key.NewBinding(
				key.WithKeys(cfg.Keybinds.SpeedDown),
				key.WithHelp("-", "speed down"),
			),
			VolumeUp: key.NewBinding(
				key.WithKeys(cfg.Keybinds.VolumeUp),
				key.WithHelp("]", "volume up"),
			),
			VolumeDown: key.NewBinding(
				key.WithKeys(cfg.Keybinds.VolumeDown),
				key.WithHelp("[", "volume down"),
			),
		},
	}
}

// Init returns nil; the player is inactive until playback starts.
func (m Model) Init() tea.Cmd {
	return nil
}

// SetWidth sets the available width for rendering.
func (m *Model) SetWidth(w int) {
	m.width = w
}

// Update handles messages for the player sub-model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StartPlayMsg:
		m.Playing = true
		m.Title = msg.Title
		m.Position = 0
		m.Duration = 0
		return m, nil

	case PositionMsg:
		if msg.Err != nil {
			return m, nil
		}
		m.Position = msg.Position
		if msg.Duration > 0 {
			m.Duration = msg.Duration
		}
		m.Playing = !msg.Paused
		return m, nil

	case tea.KeyMsg:
		if !m.Playing && !m.isPaused() {
			return m, nil
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) isPaused() bool {
	// If we have a title set, the player has been started (may be paused)
	return m.Title != "" && !m.Playing
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.PlayPause):
		m.Playing = !m.Playing
		if m.player != nil {
			return m, TogglePauseCmd(m.player, m.Playing)
		}
		return m, nil

	case key.Matches(msg, m.keys.SeekForward):
		seekSec := float64(m.config.SeekSeconds)
		currentPos := m.Position
		m.Position = math.Min(m.Position+seekSec, m.Duration)
		if m.player != nil {
			return m, SeekCmd(m.player, currentPos, seekSec, m.Duration)
		}
		return m, nil

	case key.Matches(msg, m.keys.SeekBack):
		seekSec := float64(m.config.SeekSeconds)
		currentPos := m.Position
		m.Position = math.Max(m.Position-seekSec, 0)
		if m.player != nil {
			return m, SeekCmd(m.player, currentPos, -seekSec, m.Duration)
		}
		return m, nil

	case key.Matches(msg, m.keys.SpeedUp):
		m.Speed = math.Round((m.Speed+0.1)*10) / 10
		if m.player != nil {
			return m, SetSpeedCmd(m.player, m.Speed)
		}
		return m, nil

	case key.Matches(msg, m.keys.SpeedDown):
		newSpeed := math.Round((m.Speed-0.1)*10) / 10
		if newSpeed >= 0.1 {
			m.Speed = newSpeed
		}
		if m.player != nil {
			return m, SetSpeedCmd(m.player, m.Speed)
		}
		return m, nil

	case key.Matches(msg, m.keys.VolumeUp):
		if m.Volume < 150 {
			m.Volume += 5
		}
		if m.player != nil {
			return m, SetVolumeCmd(m.player, m.Volume)
		}
		return m, nil

	case key.Matches(msg, m.keys.VolumeDown):
		if m.Volume > 0 {
			m.Volume -= 5
		}
		if m.player != nil {
			return m, SetVolumeCmd(m.player, m.Volume)
		}
		return m, nil
	}

	return m, nil
}

// formatTime formats seconds as MM:SS.
func formatTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
