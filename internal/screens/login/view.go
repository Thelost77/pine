package login

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle = lipgloss.NewStyle().Width(12).Align(lipgloss.Right).MarginRight(1)
	formStyle  = lipgloss.NewStyle().Padding(1, 2)
)

// View renders the login screen.
func (m Model) View() string {
	title := m.styles.Title.Render("pine login")

	fields := []struct {
		label string
		index int
	}{
		{"Server URL", fieldServer},
		{"Username", fieldUsername},
		{"Password", fieldPassword},
	}

	var rows []string
	for _, f := range fields {
		label := labelStyle.Render(f.label)
		input := m.inputs[f.index].View()
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, label, input))
	}

	form := lipgloss.JoinVertical(lipgloss.Left, rows...)

	var status string
	if m.loading {
		status = m.styles.Muted.Render("logging in...")
	} else if m.err != nil {
		status = m.styles.Error.Render(fmt.Sprintf("error: %v", m.err))
	}

	help := m.styles.Muted.Render("tab: next field • enter: submit • ctrl+c: quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		formStyle.Render(form),
		status,
		help,
	)

	if m.width == 0 {
		return content
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
