package app

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
)

func TestHelpGroupsContainSpruceBindings(t *testing.T) {
	h := components.NewHelpOverlay(ui.DefaultStyles(), helpGroups())
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()

	wants := []string{
		"Global", "Library", "Queue", "Player",
		"toggle help", "quit", "go back",
		"filter list", "open album / play track",
		"jump to track", "clear queue",
		"play / pause", "seek forward", "seek backward",
		"shuffle", "sleep timer",
	}
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Errorf("help overlay should contain %q", want)
		}
	}
}
