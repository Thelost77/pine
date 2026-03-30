package components

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/ui"
)

func TestHelpOverlay_InitiallyHidden(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())
	if h.Visible() {
		t.Error("help overlay should be hidden initially")
	}
	if v := h.View(); v != "" {
		t.Error("View() should return empty string when hidden")
	}
}

func TestHelpOverlay_Toggle(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())

	h.Toggle()
	if !h.Visible() {
		t.Error("help should be visible after first toggle")
	}

	h.Toggle()
	if h.Visible() {
		t.Error("help should be hidden after second toggle")
	}
}

func TestHelpOverlay_Hide(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())
	h.Toggle() // show
	h.Hide()
	if h.Visible() {
		t.Error("help should be hidden after Hide()")
	}
}

func TestHelpOverlay_ViewContainsAllGroups(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()

	groups := []string{"Global", "Navigation", "Player", "Detail"}
	for _, g := range groups {
		if !strings.Contains(view, g) {
			t.Errorf("help overlay should contain group %q", g)
		}
	}
}

func TestHelpOverlay_ViewContainsKeyBindings(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()

	bindings := []string{
		"toggle help", "quit", "go back",
		"move down", "move up",
		"play / pause", "seek forward", "seek backward",
		"play", "add bookmark", "delete bookmark",
		"open library", "switch library",
	}
	for _, b := range bindings {
		if !strings.Contains(view, b) {
			t.Errorf("help overlay should contain binding %q", b)
		}
	}
}

func TestHelpOverlay_ViewContainsDismissHint(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()
	if !strings.Contains(view, "Esc") {
		t.Error("help overlay should contain Esc dismiss hint")
	}
}

func TestHelpOverlay_SetSize(t *testing.T) {
	h := NewHelpOverlay(ui.DefaultStyles())
	h.SetSize(120, 50)
	h.Toggle()

	// Should render without panic
	view := h.View()
	if view == "" {
		t.Error("view should not be empty when visible and sized")
	}
}
