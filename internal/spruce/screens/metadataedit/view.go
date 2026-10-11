package metadataedit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the metadata edit screen.
func (m Model) View() string {
	var b stringsBuilder

	title := m.styles.Title.Render(fmt.Sprintf("Edit %s Metadata", m.itemType))
	b.WriteString(title + "\n\n")

	if m.errText != "" {
		b.WriteString(m.styles.Error.Render(m.errText) + "\n\n")
	}

	for i, input := range m.inputs {
		label := m.labels[i]
		if i == m.focused {
			label = m.styles.Accent.Render("▸ " + label)
		} else {
			label = "  " + m.styles.Muted.Render(label)
		}
		b.WriteString(label + "\n")
		b.WriteString("  " + input.View() + "\n\n")
	}

	help := m.styles.Muted.Render("[Enter] Save • [Esc] Cancel • [Tab/Up/Down] Navigate")
	if m.saving {
		help = m.styles.Accent.Render("Saving changes to Jellyfin...")
	}
	b.WriteString("\n" + help)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

type stringsBuilder struct {
	strings.Builder
}
