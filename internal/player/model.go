package player

import (
	"fmt"
	"math"

	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// StartPlayMsg signals that playback should begin.
type StartPlayMsg struct {
	Title string
}

// Model is the bubbletea sub-model for the player footer bar.
type Model struct {
	Playing         bool
	Title           string
	Position        float64
	Duration        float64
	Speed           float64
	Volume          int
	SleepRemaining  string
	RepeatStatus    string
	SubliminalCount int

	player Player
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

// Keys holds the key names for the player keybindings.
type Keys struct {
	PlayPause    string
	SeekForward  string
	SeekBackward string
	SpeedUp      string
	SpeedDown    string
	VolumeUp     string
	VolumeDown   string
}

// NewModel creates a new player sub-model with the initial speed and the key names.
func NewModel(p Player, speed float64, keys Keys, styles ui.Styles) Model {
	return Model{
		Speed:  speed,
		Volume: 100,
		player: p,
		styles: styles,
		keys: PlayerKeyMap{
			PlayPause: key.NewBinding(
				key.WithKeys(keys.PlayPause),
				key.WithHelp("space", "play/pause"),
			),
			SeekForward: key.NewBinding(
				key.WithKeys(keys.SeekForward),
				key.WithHelp("l", "seek forward"),
			),
			SeekBack: key.NewBinding(
				key.WithKeys(keys.SeekBackward),
				key.WithHelp("h", "seek backward"),
			),
			SpeedUp: key.NewBinding(
				key.WithKeys(keys.SpeedUp),
				key.WithHelp("+", "speed up"),
			),
			SpeedDown: key.NewBinding(
				key.WithKeys(keys.SpeedDown),
				key.WithHelp("-", "speed down"),
			),
			VolumeUp: key.NewBinding(
				key.WithKeys(keys.VolumeUp),
				key.WithHelp("]", "volume up"),
			),
			VolumeDown: key.NewBinding(
				key.WithKeys(keys.VolumeDown),
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

// PlayPauseKey returns the play/pause key binding.
func (m Model) PlayPauseKey() key.Binding { return m.keys.PlayPause }

// SeekForwardKey returns the seek forward key binding.
func (m Model) SeekForwardKey() key.Binding { return m.keys.SeekForward }

// SeekBackKey returns the seek backward key binding.
func (m Model) SeekBackKey() key.Binding { return m.keys.SeekBack }

// VolumeUpKey returns the volume-up key binding.
func (m Model) VolumeUpKey() key.Binding { return m.keys.VolumeUp }

// VolumeDownKey returns the volume-down key binding.
func (m Model) VolumeDownKey() key.Binding { return m.keys.VolumeDown }

// HandlesKey reports whether the player should consume the key while a session is active.
func (m Model) HandlesKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.PlayPause) ||
		key.Matches(msg, m.keys.SeekForward) ||
		key.Matches(msg, m.keys.SeekBack) ||
		key.Matches(msg, m.keys.SpeedUp) ||
		key.Matches(msg, m.keys.SpeedDown) ||
		key.Matches(msg, m.keys.VolumeUp) ||
		key.Matches(msg, m.keys.VolumeDown)
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
			return m, SetPauseCmd(m.player, !m.Playing)
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
