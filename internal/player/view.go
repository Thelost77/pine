package player

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View renders the player footer bar. Returns empty string if inactive.
func (m Model) View() string {
	if m.Title == "" {
		return ""
	}

	icon := "▶"
	if !m.Playing {
		icon = "⏸"
	}

	pos := formatTime(m.Position)
	dur := formatTime(m.Duration)
	timeStr := fmt.Sprintf("%s / %s", pos, dur)
	speedStr := fmt.Sprintf("%.1fx", m.Speed)

	content := fmt.Sprintf(" %s  %s  %s  %s", icon, m.Title, timeStr, speedStr)
	if m.Volume != 100 {
		content += fmt.Sprintf("  Vol:%d%%", m.Volume)
	}
	if m.SleepRemaining != "" {
		content += fmt.Sprintf("  Sleep:%s", m.SleepRemaining)
	}
	content += " "

	style := m.styles.PlayerBar
	if m.width > 0 {
		style = style.Width(m.width)
	}

	return lipgloss.PlaceVertical(1, lipgloss.Bottom, style.Render(content))
}
