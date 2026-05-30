package metadataedit

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var metadataLabelStyle = lipgloss.NewStyle().Width(14).Align(lipgloss.Right).MarginRight(1)

// View renders the metadata editor screen.
func (m Model) View() string {
	title := m.styles.Title.Render("edit metadata")
	if m.episode != nil {
		title = m.styles.Title.Render("edit episode metadata")
	}

	fields := []struct {
		label string
		index int
	}{
		{"Title", fieldTitle},
		{"Author", fieldAuthor},
		{"Description", fieldDescription},
		{"Series", fieldSeries},
		{"Sequence", fieldSequence},
		{"Season", fieldSeason},
		{"Episode", fieldEpisode},
		{"Episode Type", fieldEpisodeType},
	}

	rows := make([]string, 0, len(fields)+6)
	for _, field := range fields {
		if !m.fieldVisible(field.index) {
			continue
		}
		label := metadataLabelStyle.Render(field.label)
		value := m.inputs[field.index].View()
		if !m.fieldEditable(field.index) {
			value = m.styles.Muted.Render(m.inputs[field.index].Value())
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, label, value))
	}

	if m.fieldVisible(fieldAuthor) && !m.authorEditable {
		rows = append(rows, m.styles.Muted.Render("Author editing is disabled for books with multiple ABS authors."))
	}
	if m.fieldVisible(fieldSeries) && !m.seriesEditable {
		rows = append(rows, m.styles.Muted.Render("Series editing is disabled for books with multiple ABS series."))
	}
	if m.validationErr != "" {
		rows = append(rows, m.styles.Error.Render(m.validationErr))
	}
	if m.saveErr != nil {
		rows = append(rows, m.styles.Error.Render(fmt.Sprintf("save failed: %v", m.saveErr)))
	}
	if m.saving {
		rows = append(rows, m.styles.Muted.Render("saving..."))
	}

	help := m.styles.Muted.Render("tab/up/down: fields • enter: save • esc: cancel")
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		help,
	)

	if m.width == 0 {
		return content
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
