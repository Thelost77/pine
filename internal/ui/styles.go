package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/Thelost77/pine/internal/config"
)

// Styles holds all lipgloss styles derived from the active theme.
type Styles struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Muted     lipgloss.Style
	Selected  lipgloss.Style
	Error     lipgloss.Style
	Accent    lipgloss.Style
	Border    lipgloss.Style
	PlayerBar lipgloss.Style
	StatusBar lipgloss.Style
}

// NewStyles builds a Styles set from a ThemeConfig, allowing user overrides.
func NewStyles(theme config.ThemeConfig) Styles {
	fg := lipgloss.Color(theme.Foreground)
	bg := lipgloss.Color(theme.Background)
	accent := lipgloss.Color(theme.Accent)
	muted := lipgloss.Color(theme.Muted)
	errColor := lipgloss.Color(theme.Error)
	selected := lipgloss.Color(theme.Selected)
	border := lipgloss.Color(theme.Border)
	info := lipgloss.Color(theme.Info)

	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			PaddingBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(fg).
			Bold(true),

		Muted: lipgloss.NewStyle().
			Foreground(muted),

		Selected: lipgloss.NewStyle().
			Background(selected).
			Foreground(fg).
			Bold(true).
			Padding(0, 1),

		Error: lipgloss.NewStyle().
			Foreground(errColor).
			Bold(true),

		Accent: lipgloss.NewStyle().
			Foreground(accent),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 2),

		PlayerBar: lipgloss.NewStyle().
			Background(selected).
			Foreground(info).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Background(bg).
			Foreground(muted).
			Padding(0, 1),
	}
}

// DefaultStyles returns styles using the default Everforest Dark palette.
func DefaultStyles() Styles {
	return NewStyles(config.Default().Theme)
}
