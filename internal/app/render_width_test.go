package app

import (
	"github.com/Thelost77/pine/internal/ui"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
