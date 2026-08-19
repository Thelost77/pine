package app

import (
	"strings"

	"github.com/Thelost77/spruce/internal/screens/library"
	"github.com/Thelost77/spruce/internal/screens/playlists"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Heights reserved for the fixed layers of the render stack. Each is a single
// line so the available screen height is the terminal height minus whichever
// layers are currently visible.
const (
	headerHeight       = 1
	errorBannerHeight  = 1
	hintsHeight        = 1
	playerFooterHeight = 1
)

// View composes the header, error banner, active screen, hints, and player
// footer into a single string, then places it on the terminal canvas and
// overlays the command palette modal when visible.
func (m *Model) View() string {
	if m.help.Visible() {
		return m.help.View()
	}
	header := m.viewHeader()
	errBanner := m.err.View()
	body := m.viewScreen()
	hints := m.viewHints()
	footer := m.playerState.View()

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

	if m.width > 0 && m.height > 0 {
		w := normalizeViewWidth(m.width)
		content = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
		content = strings.Join(normalizeOverlayCanvas(content, w, m.height), "\n")
	}

	if m.palette.Visible() {
		content = m.overlayPaletteModal(content)
	}
	return content
}

// String returns a human-friendly name for the screen, used in the header.
func (s Screen) String() string {
	switch s {
	case ScreenLogin:
		return "login"
	case ScreenLibrary:
		return "library"
	case ScreenPlaylists:
		return "playlists"
	case ScreenQueue:
		return "queue"
	default:
		return ""
	}
}

// viewHeader renders the 1-line application header bar: `spruce › <screen>`.
func (m *Model) viewHeader() string {
	title := m.styles.Title.PaddingBottom(0).Render("spruce")
	breadcrumb := m.styles.Muted.Render(" › " + m.screen.String())
	return lipgloss.JoinHorizontal(lipgloss.Bottom, title, breadcrumb)
}

// viewScreen renders the currently active screen.
func (m *Model) viewScreen() string {
	switch m.screen {
	case ScreenLogin:
		return m.loginScreen.View()
	case ScreenLibrary:
		return m.libraryScreen.View()
	case ScreenPlaylists:
		return m.playlistsScreen.View()
	case ScreenQueue:
		return m.queueScreen.View()
	case ScreenMetadataEdit:
		return m.metadataEditScreen.View()
	default:
		return ""
	}
}

// viewHints renders a 1-line context-aware keybinding hint bar.
func (m *Model) viewHints() string {
	sep := m.styles.Muted.Render("  ")
	key := func(k, desc string) string {
		return m.styles.Accent.Render(k) + " " + m.styles.Muted.Render(desc)
	}

	var parts []string
	switch m.screen {
	case ScreenLibrary:
		if m.libraryScreen.CurrentLevel() == library.LevelAlbums {
			parts = append(parts,
				key("enter", "open"),
				key("f", "favorite"),
				key("a/A", "queue album"),
				key("S", "shuffle album"),
				key("m", "edit meta"),
				key("/", "search"),
				key("esc", "back"),
				key("tab", "queue"),
			)
		} else {
			parts = append(parts,
				key("enter", "play"),
				key("f", "favorite"),
				key("a", "add track"),
				key("A", "add album"),
				key("S", "shuffle album"),
				key("t", "sort"),
				key("m", "edit meta"),
				key("/", "search"),
				key("esc", "back"),
				key("tab", "queue"),
			)
		}
	case ScreenPlaylists:
		if m.playlistsScreen.CurrentLevel() == playlists.LevelPlaylists {
			parts = append(parts,
				key("enter", "open"),
				key("a/A", "queue playlist"),
				key("S", "shuffle playlist"),
				key("/", "search"),
				key("esc", "back"),
				key("tab", "queue"),
			)
		} else {
			parts = append(parts,
				key("enter", "play"),
				key("f", "favorite"),
				key("a", "add track"),
				key("A", "add playlist"),
				key("S", "shuffle playlist"),
				key("/", "search"),
				key("esc", "back"),
				key("tab", "queue"),
			)
		}
	case ScreenQueue:
		parts = append(parts,
			key("enter", "jump"),
			key("space", "pause"),
			key("</>", "prev/next"),
			key("s", "shuffle"),
			key("r/R", "repeat"),
			key("b", "subliminal"),
			key("m", "edit meta"),
			key("d", "remove"),
			key("/", "search"),
			key("esc", "back"),
			key("tab", "library"),
		)
	case ScreenMetadataEdit:
		parts = append(parts,
			key("enter", "save"),
			key("tab", "next field"),
			key("esc", "cancel"),
		)
	case ScreenLogin:
		parts = append(parts,
			key("enter", "login"),
			key("q", "quit"),
		)
	default:
		return ""
	}

	if m.screen != ScreenLogin && m.screen != ScreenMetadataEdit {
		if m.IsPlaying() || m.isSubliminalRunning() {
			if m.screen != ScreenQueue {
				parts = append(parts, key("</>", "prev/next"))
			}
			parts = append(parts, key("-/+", "speed"), key("[/]", "vol"))
		}
		parts = append(parts, key("?", "help"), key("q", "quit"))
	}

	style := m.styles.StatusBar
	barWidth := normalizeViewWidth(m.width)
	contentWidth := 1000
	if barWidth > 0 {
		contentWidth = max(1, barWidth-style.GetHorizontalFrameSize())
	}
	var finalParts []string
	for _, p := range parts {
		testStr := lipgloss.JoinHorizontal(lipgloss.Center, joinWith(append(finalParts, p), sep)...)
		if lipgloss.Width(testStr) > contentWidth {
			break
		}
		finalParts = append(finalParts, p)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Center, joinWith(finalParts, sep)...)
	if barWidth > 0 {
		content = ansi.Truncate(content, contentWidth, "")
		style = style.Width(barWidth)
	}
	return style.Render(content)
}

// overlayPaletteModal centers the command palette modal over the content.
func (m *Model) overlayPaletteModal(content string) string {
	overlay := m.palette.View()
	if overlay == "" {
		return content
	}
	if m.width <= 0 || m.height <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, content, "", overlay)
	}

	w := m.width
	if w > 120 {
		w = 120
	}
	baseLines := normalizeOverlayCanvas(content, w, m.height)
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
		right := ansi.TruncateLeft(baseLines[y+i], x+lineWidth, "")
		baseLines[y+i] = left + line + right
	}

	return strings.Join(baseLines, "\n")
}

// normalizeViewWidth subtracts 1 from the terminal width to prevent rendering
// bugs. In terminal emulators like iTerm2, Ghostty, and macOS Terminal (unlike
// Alacritty), if a line of text is padded exactly to the terminal's width and
// has a background color (like the selected list item, hints bar, or player
// footer), printing the final character in the bottom-right corner forces the
// cursor to automatically wrap to the next line. This causes the terminal to
// scroll down by 1 line, which pushes the header off-screen and throws
// Bubbletea's internal cursor tracking out of sync, leading to duplicated and
// interleaved text artifacts when switching tabs.
func normalizeViewWidth(width int) int {
	if width > 0 {
		return width - 1
	}
	return width
}

// normalizeOverlayCanvas pads/truncates content to a fixed width x height grid
// of lines for overlay composition.
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
