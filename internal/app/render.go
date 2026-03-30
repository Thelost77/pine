package app

import "github.com/charmbracelet/lipgloss"

// View composes the header, error banner, active screen, and player footer.
func (m Model) View() string {
	if m.help.Visible() {
		return m.help.View()
	}

	header := m.viewHeader()
	errBanner := m.err.View()
	body := m.viewScreen()
	hints := m.viewHints()
	footer := m.player.View()

	parts := []string{header}
	if errBanner != "" {
		parts = append(parts, errBanner)
	}
	parts = append(parts, body)
	if hints != "" {
		parts = append(parts, hints)
	}
	if footer != "" {
		parts = append(parts, footer)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if m.width == 0 {
		return content
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewHeader renders the application header bar.
func (m Model) viewHeader() string {
	title := m.styles.Title.PaddingBottom(0).Render("pine")
	breadcrumb := m.styles.Muted.Render(" › " + m.screen.String())
	return lipgloss.JoinHorizontal(lipgloss.Bottom, title, breadcrumb) + "\n"
}

// viewScreen renders the currently active screen.
func (m Model) viewScreen() string {
	switch m.screen {
	case ScreenLogin:
		return m.login.View()
	case ScreenHome:
		return m.home.View()
	case ScreenLibrary:
		return m.library.View()
	case ScreenDetail:
		return m.detail.View()
	case ScreenSearch:
		return m.search.View()
	default:
		return ""
	}
}

// viewHints renders context-aware keybinding hints for the status bar.
func (m Model) viewHints() string {
	sep := m.styles.Muted.Render(" • ")
	key := func(k, desc string) string {
		return m.styles.Accent.Render(k) + " " + m.styles.Muted.Render(desc)
	}

	var parts []string

	switch m.screen {
	case ScreenHome:
		parts = append(parts, key("→/enter", "open"))
		parts = append(parts, key("o", "library"))
		parts = append(parts, key("/", "search"))
		parts = append(parts, key("tab", "switch lib"))
	case ScreenLibrary:
		parts = append(parts, key("→/enter", "open"))
		parts = append(parts, key("←/esc", "back"))
	case ScreenDetail:
		parts = append(parts, key("enter/p", "play"))
		parts = append(parts, key("b", "bookmark"))
		parts = append(parts, key("tab", "focus"))
		parts = append(parts, key("←/esc", "back"))
	case ScreenSearch:
		parts = append(parts, key("enter", "open"))
		parts = append(parts, key("esc", "back"))
	default:
		return ""
	}

	if m.isPlaying() {
		parts = append(parts, key("space", "pause"))
		parts = append(parts, key("h/l", "seek"))
	}
	parts = append(parts, key("?", "help"))

	style := m.styles.StatusBar
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(lipgloss.JoinHorizontal(lipgloss.Center, joinWith(parts, sep)...))
}

// joinWith interleaves items with a separator for lipgloss joining.
func joinWith(items []string, sep string) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items)*2-1)
	for i, item := range items {
		if i > 0 {
			result = append(result, sep)
		}
		result = append(result, item)
	}
	return result
}
