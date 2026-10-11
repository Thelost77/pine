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
	if m.SubliminalCount > 0 {
		content := fmt.Sprintf(" %s  %s  ∞  Vol:%d%% per track ", icon, m.Title, m.Volume)
		style := m.styles.PlayerBar
		if m.width > 0 {
			contentWidth := max(1, m.width-style.GetHorizontalFrameSize())
			content = ansi.Truncate(content, contentWidth, "")
			style = style.Width(m.width)
		}
		return lipgloss.PlaceVertical(1, lipgloss.Bottom, style.Render(content))
	}

	pos := formatTime(m.Position)
	dur := formatTime(m.Duration)
	timeStr := fmt.Sprintf("%s / %s", pos, dur)
	speedStr := fmt.Sprintf("%.1fx", m.Speed)

	extras := ""
	if m.Volume != 100 {
		extras += fmt.Sprintf("  Vol:%d%%", m.Volume)
	}
	if m.RepeatStatus != "" {
		extras += fmt.Sprintf("  %s", m.RepeatStatus)
	}
	if m.SleepRemaining != "" {
		extras += fmt.Sprintf("  Sleep:%s", m.SleepRemaining)
	}

	style := m.styles.PlayerBar
	barWidth := m.width
	contentWidth := barWidth
	if barWidth > 0 {
		contentWidth = max(1, barWidth-style.GetHorizontalFrameSize())
	}

	fixedContent := fmt.Sprintf(" %s    %s  %s%s ", icon, timeStr, speedStr, extras)
	availTitle := contentWidth - lipgloss.Width(fixedContent)

	title := m.Title
	if availTitle > 0 && lipgloss.Width(title) > availTitle {
		title = ansi.Truncate(title, availTitle-1, "") + "…"
	} else if availTitle <= 0 {
		title = ""
	}

	content := fmt.Sprintf(" %s  %s  %s  %s%s ", icon, title, timeStr, speedStr, extras)
	if barWidth > 0 {
		content = ansi.Truncate(content, contentWidth, "")
		style = style.Width(barWidth)
	}

	return lipgloss.PlaceVertical(1, lipgloss.Bottom, style.Render(content))
}
