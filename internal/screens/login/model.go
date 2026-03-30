package login

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
)

const (
	fieldServer   = 0
	fieldUsername  = 1
	fieldPassword  = 2
	numFields      = 3
)

// LoginSuccessMsg is sent when authentication succeeds.
type LoginSuccessMsg struct {
	Token     string
	ServerURL string
	Username  string
}

// LoginFailedMsg is sent when authentication fails.
type LoginFailedMsg struct {
	Err error
}

// Model is the bubbletea model for the login screen.
type Model struct {
	inputs  [numFields]textinput.Model
	focused int
	err     error
	loading bool
	width   int
	height  int
	styles  ui.Styles
}

// New creates a new login screen model.
func New(styles ui.Styles) Model {
	var inputs [numFields]textinput.Model

	server := textinput.New()
	server.Placeholder = "https://abs.example.com"
	server.CharLimit = 256
	server.Focus()
	inputs[fieldServer] = server

	username := textinput.New()
	username.Placeholder = "username"
	username.CharLimit = 128
	inputs[fieldUsername] = username

	password := textinput.New()
	password.Placeholder = "password"
	password.CharLimit = 128
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'
	inputs[fieldPassword] = password

	return Model{
		inputs:  inputs,
		focused: fieldServer,
		styles:  styles,
	}
}

// SetSize updates the terminal dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init returns the initial command (start blinking cursor).
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the login screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			if msg.String() == "shift+tab" {
				m.focused = (m.focused - 1 + numFields) % numFields
			} else {
				m.focused = (m.focused + 1) % numFields
			}
			return m, m.updateFocus()

		case "enter":
			if m.focused == fieldPassword {
				return m, m.loginCmd()
			}
			// Enter on other fields advances to next
			m.focused = (m.focused + 1) % numFields
			return m, m.updateFocus()
		}

	case LoginSuccessMsg:
		m.loading = false
		m.err = nil
		return m, nil

	case LoginFailedMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil
	}

	// Forward to active input
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

// updateFocus sets focus on the active field and blurs others.
func (m *Model) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, numFields)
	for i := range m.inputs {
		if i == m.focused {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

// loginCmd returns a tea.Cmd that calls abs.Client.Login().
func (m *Model) loginCmd() tea.Cmd {
	serverURL := m.inputs[fieldServer].Value()
	username := m.inputs[fieldUsername].Value()
	password := m.inputs[fieldPassword].Value()
	m.loading = true

	return func() tea.Msg {
		client := abs.NewClient(serverURL, "")
		token, err := client.Login(context.Background(), username, password)
		if err != nil {
			return LoginFailedMsg{Err: err}
		}
		return LoginSuccessMsg{Token: token, ServerURL: serverURL, Username: username}
	}
}

// Focused returns the index of the currently focused field.
func (m Model) Focused() int {
	return m.focused
}

// Loading returns whether a login is in progress.
func (m Model) Loading() bool {
	return m.loading
}

// Error returns the last login error, if any.
func (m Model) Error() error {
	return m.err
}

// ServerURL returns the current server URL input value.
func (m Model) ServerURL() string {
	return m.inputs[fieldServer].Value()
}

// Username returns the current username input value.
func (m Model) Username() string {
	return m.inputs[fieldUsername].Value()
}

// Password returns the current password input value.
func (m Model) Password() string {
	return m.inputs[fieldPassword].Value()
}
