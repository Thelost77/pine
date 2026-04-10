package library

// View renders the library screen.
func (m Model) View() string {
	if m.err != nil {
		return m.styles.Error.Render("Error: " + m.err.Error())
	}

	return m.list.View()
}
