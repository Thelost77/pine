package ui

import "github.com/charmbracelet/lipgloss"

// ThemeConfig holds the theme colors of the [theme] config section.
type ThemeConfig struct {
	Background string `toml:"background"`
	Foreground string `toml:"foreground"`
	Accent     string `toml:"accent"`
	Error      string `toml:"error"`
	Muted      string `toml:"muted"`
	Selected   string `toml:"selected"`
	Border     string `toml:"border"`
	Info       string `toml:"info"`
}

// DefaultTheme returns the default Everforest Dark palette.
func DefaultTheme() ThemeConfig {
	return ThemeConfig{
		Background: "#2b3339",
		Foreground: "#d3c6aa",
		Accent:     "#a7c080",
		Error:      "#e67e80",
		Muted:      "#859289",
		Selected:   "#475258",
		Border:     "#4f585e",
		Info:       "#7fbbb3",
	}
}

// Styles holds all lipgloss styles derived from the active theme.
type Styles struct {
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Muted     lipgloss.Style
	Selected  lipgloss.Style
	Error     lipgloss.Style
	Accent    lipgloss.Style
	Info      lipgloss.Style
	Border    lipgloss.Style
	PlayerBar lipgloss.Style
	StatusBar lipgloss.Style
	Caption   lipgloss.Style
}

// NewStyles builds a Styles set from a ThemeConfig, allowing user overrides.
func NewStyles(theme ThemeConfig) Styles {
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

		Info: lipgloss.NewStyle().
			Foreground(info),

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

		Caption: lipgloss.NewStyle().
			Background(selected).
			Foreground(fg).
			Padding(0, 1),
	}
}

// DefaultStyles returns styles using the default Everforest Dark palette.
func DefaultStyles() Styles {
	return NewStyles(DefaultTheme())
}
