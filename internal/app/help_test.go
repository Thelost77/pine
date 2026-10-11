package app

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
)

func TestHelpGroupsContainPineBindings(t *testing.T) {
	h := components.NewHelpOverlay(ui.DefaultStyles(), helpGroups())
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()

	wants := []string{
		"Global", "Navigation", "Player", "Detail",
		"toggle help", "quit", "go back",
		"move down", "move up",
		"play / pause", "seek forward", "seek backward",
		"play", "add bookmark", "delete bookmark",
		"open library", "switch library",
	}
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Errorf("help overlay should contain %q", want)
		}
	}
}
