package search

import (
	"github.com/charmbracelet/lipgloss"
)

// View renders the search screen.
func (m Model) View() string {
	inputLine := m.input.View()

	if m.query == "" {
		hint := m.styles.Muted.Render("Type to search…")
		return lipgloss.JoinVertical(lipgloss.Left, inputLine, "", hint)
	}

	if m.loading && len(m.items) == 0 {
		loading := m.styles.Muted.Render("Searching…")
		return lipgloss.JoinVertical(lipgloss.Left, inputLine, "", loading)
	}

	if m.err != nil {
		errMsg := m.styles.Error.Render("Error: " + m.err.Error())
		return lipgloss.JoinVertical(lipgloss.Left, inputLine, "", errMsg)
	}

	if m.searched && len(m.items) == 0 {
		noResults := m.styles.Muted.Render("No results found.")
		return lipgloss.JoinVertical(lipgloss.Left, inputLine, "", noResults)
	}

	return lipgloss.JoinVertical(lipgloss.Left, inputLine, "", m.list.View())
}
