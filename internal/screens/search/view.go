package search

import (
	"github.com/charmbracelet/lipgloss"
)

// View renders the search screen.
func (m Model) View() string {
	inputLine := m.input.View()
	body := m.styles.Muted.Render("Type to search…")
	normalizedQuery := normalizeQuery(m.query)

	if normalizedQuery == "" {
		body = m.styles.Muted.Render("Type to search…")
	} else if m.err != nil {
		body = m.styles.Error.Render("Error: " + m.err.Error())
	} else if m.searched && len(m.items) == 0 {
		body = m.styles.Muted.Render("No results found.")
	} else {
		body = m.list.View()
	}

	if m.width > 0 && m.height > 0 {
		bodyHeight := m.height - inputHeight
		if bodyHeight < 1 {
			bodyHeight = 1
		}
		body = lipgloss.Place(m.width, bodyHeight, lipgloss.Left, lipgloss.Top, body)
	}

	return lipgloss.JoinVertical(lipgloss.Left, inputLine, "", body)
}
