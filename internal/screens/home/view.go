package home

import "github.com/charmbracelet/lipgloss"

// View renders the home screen.
func (m Model) View() string {
	if m.loading {
		return m.styles.Muted.Render("Loading home…")
	}

	if m.err != nil {
		return m.styles.Error.Render("Error: " + m.err.Error())
	}

	main := m.list.View()
	if len(m.items) == 0 {
		main = m.styles.Muted.Render("No items in continue listening")
	}

	if len(m.recentlyAdded) == 0 {
		return main
	}

	lines := []string{m.styles.Subtitle.Render("Recently Added")}
	for _, item := range m.recentlyAdded {
		row := item.Media.Metadata.Title
		if item.Media.Metadata.AuthorName != nil {
			row += "  " + m.styles.Muted.Render(*item.Media.Metadata.AuthorName)
		}
		lines = append(lines, "  "+row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, "", lipgloss.JoinVertical(lipgloss.Left, lines...))
}
