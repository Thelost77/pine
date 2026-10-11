package app

import (
	"github.com/Thelost77/pine/internal/ui"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestModelViewWidthNoScrollBug(t *testing.T) {
	m := Model{
		width:  80,
		height: 24,
		screen: ScreenHome,
		styles: ui.DefaultStyles(),
	}

	out := m.View()
	lines := strings.Split(out, "\n")

	if len(lines) != 24 {
		t.Errorf("expected 24 lines, got %d", len(lines))
	}

	for i, line := range lines {
		w := lipgloss.Width(line)
		if w >= 80 {
			t.Errorf("line %d is %d characters wide, expected < 80 to prevent terminal scrolling", i, w)
		}
	}

	if !strings.Contains(out, "pine") {
		t.Errorf("expected header 'pine' in view output")
	}
}

func TestModelViewShowsFullPlayerFooterAfterResize(t *testing.T) {
	// A terminal renders the footer with color codes. Without them, a footer
	// that is too wide still survives the line wrap in View.
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := newPlaybackTestModel()
	m.screen = ScreenHome
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)
	m.player.Title = "Mick Herron - Kulawe konie czyta K. Gosztyła 192kbps"
	m.player.Position = 3
	m.player.Duration = 44410

	lines := strings.Split(m.View(), "\n")
	footer := lines[len(lines)-1]

	text := ansi.Strip(footer)
	for _, want := range []string{m.player.Title, "0:03 / 740:10", "1.0x"} {
		if !strings.Contains(text, want) {
			t.Errorf("footer %q should contain %q", text, want)
		}
	}
	if w := lipgloss.Width(footer); w != 119 {
		t.Errorf("footer is %d cells wide, want 119 (one less than the terminal)", w)
	}
}
