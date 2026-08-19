package components

import (
	"strings"

	"github.com/Thelost77/spruce/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// HelpBinding represents a single keybinding entry for the help overlay.
type HelpBinding struct {
	Key  string
	Desc string
}

// HelpGroup represents a named group of keybindings.
type HelpGroup struct {
	Title    string
	Bindings []HelpBinding
}

// HelpOverlay holds the state for the help overlay.
type HelpOverlay struct {
	visible bool
	width   int
	height  int
	styles  ui.Styles
	groups  []HelpGroup
}

// NewHelpOverlay creates a new help overlay with all keybinding groups.
func NewHelpOverlay(styles ui.Styles) HelpOverlay {
	return HelpOverlay{
		styles: styles,
		groups: defaultGroups(),
	}
}

// defaultGroups returns the keybinding groups for the help overlay.
func defaultGroups() []HelpGroup {
	return []HelpGroup{
		{
			Title: "Global",
			Bindings: []HelpBinding{
				{Key: "q", Desc: "quit"},
				{Key: "esc / ←", Desc: "go back / close filter"},
				{Key: "enter / →", Desc: "open / select"},
				{Key: "tab", Desc: "switch library/playlists and queue"},
				{Key: "o", Desc: "open playlists"},
				{Key: "ctrl+p", Desc: "command palette"},
				{Key: "?", Desc: "toggle help"},
			},
		},
		{
			Title: "Library",
			Bindings: []HelpBinding{
				{Key: "/", Desc: "filter list"},
				{Key: "enter", Desc: "open album / play track"},
				{Key: "a", Desc: "add track / album to queue"},
				{Key: "A", Desc: "queue album"},
				{Key: "S", Desc: "shuffle album to queue"},
				{Key: "t", Desc: "toggle track sort"},
				{Key: "m", Desc: "edit metadata"},
			},
		},
		{
			Title: "Playlists",
			Bindings: []HelpBinding{
				{Key: "/", Desc: "filter list"},
				{Key: "enter", Desc: "open playlist / play track"},
				{Key: "a", Desc: "add track / playlist to queue"},
				{Key: "A", Desc: "queue playlist"},
				{Key: "S", Desc: "shuffle playlist to queue"},
			},
		},
		{
			Title: "Queue",
			Bindings: []HelpBinding{
				{Key: "enter", Desc: "jump to track"},
				{Key: "space", Desc: "play / pause"},
				{Key: "< / >", Desc: "previous / next track"},
				{Key: "s", Desc: "shuffle queue"},
				{Key: "r / R", Desc: "repeat track / queue"},
				{Key: "b", Desc: "toggle subliminal mode"},
				{Key: "m", Desc: "edit metadata"},
				{Key: "d / x", Desc: "remove track"},
				{Key: "c", Desc: "clear queue"},
				{Key: "/", Desc: "filter queue"},
			},
		},
		{
			Title: "Player",
			Bindings: []HelpBinding{
				{Key: "space", Desc: "play / pause"},
				{Key: "l", Desc: "seek forward"},
				{Key: "h", Desc: "seek backward"},
				{Key: ">", Desc: "play next queued"},
				{Key: "<", Desc: "play previous"},
				{Key: "+", Desc: "speed up"},
				{Key: "-", Desc: "speed down"},
				{Key: "] / [", Desc: "volume up / down"},
				{Key: "s", Desc: "shuffle"},
				{Key: "S", Desc: "sleep timer"},
			},
		},
	}
}

// Toggle switches the help overlay on or off.
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

// Hide dismisses the help overlay.
func (h *HelpOverlay) Hide() {
	h.visible = false
}

// Visible returns whether the overlay is currently shown.
func (h HelpOverlay) Visible() bool {
	return h.visible
}

// SetSize updates the available dimensions for centering.
func (h *HelpOverlay) SetSize(w, h2 int) {
	h.width = w
	h.height = h2
}

// View renders the help overlay as a centered bordered box.
// Returns "" if not visible.
func (h HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	var sections []string

	title := h.styles.Title.PaddingBottom(0).Render("Keybindings")
	sections = append(sections, title, "")

	for i, g := range h.groups {
		header := h.styles.Accent.Bold(true).Render(g.Title)
		sections = append(sections, header)

		for _, b := range g.Bindings {
			keyCol := h.styles.Muted.Render(padRight(b.Key, 12))
			line := keyCol + b.Desc
			sections = append(sections, line)
		}

		if i < len(h.groups)-1 {
			sections = append(sections, "")
		}
	}

	dismiss := h.styles.Muted.Render("\nPress ? or Esc to close")
	sections = append(sections, dismiss)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	box := h.styles.Border.Render(content)

	if h.width > 0 && h.height > 0 {
		return lipgloss.Place(h.width, h.height,
			lipgloss.Center, lipgloss.Center, box)
	}

	return box
}

// padRight pads a string to the given width with spaces.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
