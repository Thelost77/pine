package components

import (
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmMsg is sent when the confirmation dialog is resolved.
type ConfirmMsg struct {
	Confirmed bool
	Data      any // optional context data
}

// ConfirmOverlay is a generic confirmation dialog.
type ConfirmOverlay struct {
	visible bool
	title   string
	prompt  string
	data    any
	styles  ui.Styles
}

// NewConfirmOverlay creates a new confirmation overlay.
func NewConfirmOverlay(styles ui.Styles) ConfirmOverlay {
	return ConfirmOverlay{
		styles: styles,
	}
}

// Show opens the confirmation dialog.
func (c *ConfirmOverlay) Show(title, prompt string, data any) {
	c.visible = true
	c.title = title
	c.prompt = prompt
	c.data = data
}

// Hide closes the confirmation dialog.
func (c *ConfirmOverlay) Hide() {
	c.visible = false
	c.data = nil
}

// Visible returns true if the dialog is currently visible.
func (c ConfirmOverlay) Visible() bool {
	return c.visible
}

// Update handles key events for the dialog.
func (c *ConfirmOverlay) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !c.visible {
		return nil, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			data := c.data
			c.Hide()
			return func() tea.Msg { return ConfirmMsg{Confirmed: true, Data: data} }, true
		case "n", "N", "esc", "q":
			data := c.data
			c.Hide()
			return func() tea.Msg { return ConfirmMsg{Confirmed: false, Data: data} }, true
		}
		// Absorb all other keys while dialog is open
		return nil, true
	}
	return nil, false
}

// View renders the confirmation dialog.
func (c ConfirmOverlay) View() string {
	if !c.visible {
		return ""
	}

	lines := []string{
		c.styles.Title.PaddingBottom(0).Render(c.title),
		"",
		c.styles.Accent.Render(c.prompt),
		"",
		c.styles.Muted.Render("y/enter confirm • n/esc cancel"),
	}

	return c.styles.Border.Render(lipgloss.JoinVertical(lipgloss.Center, lines...))
}
