package home

// View renders the home screen.
func (m Model) View() string {
	if m.loading {
		return m.styles.Muted.Render("Loading continue listening…")
	}

	if m.err != nil {
		return m.styles.Error.Render("Error: " + m.err.Error())
	}

	return m.list.View()
}
