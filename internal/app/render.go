package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

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

	if m.width > 0 {
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	if !m.chapterOverlayVisible {
		return content
	}

	return m.overlayChapterModal(content)
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
	case ScreenSeriesList:
		return m.seriesList.View()
	case ScreenSeries:
		return m.series.View()
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
		parts = append(parts, key("a", "queue"))
		parts = append(parts, key("A", "next"))
		parts = append(parts, key("o", "library"))
		parts = append(parts, key("/", "search"))
		parts = append(parts, key("tab", "switch lib"))
	case ScreenLibrary:
		parts = append(parts, key("→/enter", "open"))
		parts = append(parts, key("s", "series"))
		parts = append(parts, key("/", "search"))
		parts = append(parts, key("←/esc", "back"))
	case ScreenDetail:
		parts = append(parts, key("enter/p", "play"))
		parts = append(parts, key("b", "bookmark"))
		parts = append(parts, key("a", "queue"))
		parts = append(parts, key("A", "next"))
		parts = append(parts, key("tab", "focus"))
		parts = append(parts, key("←/esc", "back"))
	case ScreenSearch:
		parts = append(parts, key("enter", "open"))
		parts = append(parts, key("esc", "back"))
	case ScreenSeriesList:
		parts = append(parts, key("enter", "open"))
		parts = append(parts, key("←/esc", "back"))
	case ScreenSeries:
		parts = append(parts, key("enter", "open"))
		parts = append(parts, key("←/esc", "back"))
	default:
		return ""
	}

	if m.isPlaying() {
		parts = append(parts, key("space", "pause"))
		parts = append(parts, key("h/l", "seek"))
		if len(m.queue) > 0 {
			parts = append(parts, key(">", "next queued"))
		}
		if len(m.chapters) > 0 {
			parts = append(parts, key("c", "chapters"))
		}
	}
	if len(m.queue) > 0 {
		parts = append(parts, m.styles.Muted.Render(fmt.Sprintf("%d queued", len(m.queue))))
	}
	parts = append(parts, key("?", "help"))

	style := m.styles.StatusBar
	if m.width > 0 {
		style = style.Width(m.width)
	}
	return style.Render(lipgloss.JoinHorizontal(lipgloss.Center, joinWith(parts, sep)...))
}

func (m Model) overlayChapterModal(content string) string {
	overlay := m.viewChapterOverlay()
	if overlay == "" {
		return content
	}
	if m.width <= 0 || m.height <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, content, "", overlay)
	}

	baseLines := normalizeOverlayCanvas(content, m.width, m.height)
	overlayLines := strings.Split(overlay, "\n")
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := len(overlayLines)
	if overlayWidth <= 0 || overlayHeight == 0 {
		return content
	}

	x := max(0, (m.width-overlayWidth)/2)
	y := max(0, (m.height-overlayHeight)/2)
	for i, line := range overlayLines {
		if y+i >= len(baseLines) {
			break
		}
		lineWidth := lipgloss.Width(line)
		left := ansi.Truncate(baseLines[y+i], x, "")
		rightWidth := max(0, m.width-(x+lineWidth))
		right := ansi.TruncateLeft(baseLines[y+i], rightWidth, "")
		baseLines[y+i] = left + line + right
	}

	return strings.Join(baseLines, "\n")
}

func (m Model) viewChapterOverlay() string {
	if !m.chapterOverlayVisible {
		return ""
	}

	titleWidth := 44
	if m.width > 0 {
		titleWidth = min(max(20, m.width-18), 56)
	}

	playbackTitle := m.player.Title
	if playbackTitle == "" {
		playbackTitle = "Current playback"
	}

	selected := min(max(m.chapterOverlayIndex, 0), max(len(m.chapters)-1, 0))
	maxItems := len(m.chapters)
	if m.height > 0 {
		maxItems = min(maxItems, max(1, m.height-11))
	}
	start, end := overlayWindow(len(m.chapters), selected, maxItems)

	lines := []string{
		m.styles.Title.PaddingBottom(0).Render("Chapter Navigation"),
		m.styles.Muted.Render("Current playback"),
		m.styles.Subtitle.Render(ansi.Truncate(playbackTitle, titleWidth, "…")),
		m.styles.Muted.Render(fmt.Sprintf("%d chapters • j/k navigate • H/L top/bottom • esc close", len(m.chapters))),
		"",
	}

	for i := start; i < end; i++ {
		line := ansi.Truncate(fmt.Sprintf("%d. %s", i+1, m.chapters[i].Title), titleWidth, "…")
		if i == selected {
			lines = append(lines, m.styles.Selected.Render("› "+line))
			continue
		}
		lines = append(lines, "  "+line)
	}

	if len(m.chapters) == 0 {
		lines = append(lines, m.styles.Muted.Render("No chapters available"))
	}

	if start > 0 || end < len(m.chapters) {
		lines = append(lines, "", m.styles.Muted.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.chapters))))
	}

	return m.styles.Border.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
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

func overlayWindow(total, selected, visible int) (int, int) {
	if visible <= 0 || visible >= total {
		return 0, total
	}

	start := selected - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > total {
		end = total
		start = end - visible
	}
	return start, end
}

func normalizeOverlayCanvas(content string, width, height int) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	canvas := make([]string, 0, height)
	for _, line := range lines {
		line = ansi.Truncate(line, width, "")
		if lipgloss.Width(line) < width {
			line += strings.Repeat(" ", width-lipgloss.Width(line))
		}
		canvas = append(canvas, line)
	}
	for len(canvas) < height {
		canvas = append(canvas, strings.Repeat(" ", width))
	}
	return canvas
}
