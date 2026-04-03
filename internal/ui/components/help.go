package components

import (
	"strings"

	"github.com/Thelost77/pine/internal/ui"
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
				{Key: "esc / ←", Desc: "go back"},
				{Key: "enter / →", Desc: "open / select"},
				{Key: "?", Desc: "toggle help"},
			},
		},
		{
			Title: "Home",
			Bindings: []HelpBinding{
				{Key: "o", Desc: "open library"},
				{Key: "/", Desc: "search"},
				{Key: "tab", Desc: "switch library"},
			},
		},
		{
			Title: "Navigation",
			Bindings: []HelpBinding{
				{Key: "j / ↓", Desc: "move down"},
				{Key: "k / ↑", Desc: "move up"},
				{Key: "tab", Desc: "toggle focus"},
			},
		},
		{
			Title: "Player",
			Bindings: []HelpBinding{
				{Key: "p / space", Desc: "play / pause"},
				{Key: "l", Desc: "seek forward"},
				{Key: "h", Desc: "seek backward"},
				{Key: "+", Desc: "speed up"},
				{Key: "-", Desc: "speed down"},
				{Key: "] / [", Desc: "volume up / down"},
				{Key: "c", Desc: "open chapters"},
				{Key: "n / N", Desc: "next / prev chapter"},
				{Key: "s", Desc: "sleep timer"},
			},
		},
		{
			Title: "Detail",
			Bindings: []HelpBinding{
				{Key: "enter / p", Desc: "play"},
				{Key: "b", Desc: "add bookmark"},
				{Key: "a", Desc: "add to queue"},
				{Key: "A", Desc: "play next"},
				{Key: "d", Desc: "delete bookmark"},
				{Key: "f", Desc: "mark finished"},
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
