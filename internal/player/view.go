package player

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	extras := ""
	if m.Volume != 100 {
		extras += fmt.Sprintf("  Vol:%d%%", m.Volume)
	}
	if m.SleepRemaining != "" {
		extras += fmt.Sprintf("  Sleep:%s", m.SleepRemaining)
	}

	w := m.width
	if w > 0 {
		w-- // safe width
	}

	// Calculate how much space we have for the title
	fixedContent := fmt.Sprintf(" %s    %s  %s%s ", icon, timeStr, speedStr, extras)
	availTitle := w - lipgloss.Width(fixedContent)

	title := m.Title
	if availTitle > 0 && lipgloss.Width(title) > availTitle {
		title = ansi.Truncate(title, availTitle-1, "") + "…"
	} else if availTitle <= 0 {
		title = ""
	}

	content := fmt.Sprintf(" %s  %s  %s  %s%s ", icon, title, timeStr, speedStr, extras)

	style := m.styles.PlayerBar
	if w > 0 {
		style = style.Width(w)
	}

	return lipgloss.PlaceVertical(1, lipgloss.Bottom, style.Render(content))
}
