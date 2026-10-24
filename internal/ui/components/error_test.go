package components

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewErrorBannerHasNoError(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	if b.HasError() {
		t.Error("new banner should have no error")
	}
	if b.View() != "" {
		t.Error("new banner view should be empty")
	}
}

func TestSetErrorAndView(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetWidth(80)
	cmd := b.SetError(errors.New("something broke"))

	if !b.HasError() {
		t.Error("expected HasError after SetError")
	}
	if cmd == nil {
		t.Error("expected auto-dismiss command")
	}

	view := b.View()
	if !contains(view, "something broke") {
		t.Errorf("view should contain error message, got: %q", view)
	}
	if !contains(view, "⚠") {
		t.Errorf("view should contain warning icon, got: %q", view)
	}
}

func TestErrorBannerSanitizesTerminalControls(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetWidth(120)
	b.SetError(errors.New("server said \x1b]52;c;payload\a\x1b[31mred\x1b[0m\nnext\tline\x00"))

	view := b.View()
	if strings.ContainsAny(view, "\x1b\a\x00\n\r\t") {
		t.Fatalf("view contains terminal controls: %q", view)
	}
	if !strings.Contains(view, "red next line") {
		t.Fatalf("view lost sanitized message text: %q", view)
	}
}

func TestErrorBannerStaysSingleLine(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetWidth(20)
	b.SetError(errors.New(strings.Repeat("connection failed ", 10)))

	view := b.View()
	if got := lipgloss.Height(view); got != 1 {
		t.Fatalf("view height = %d, want 1: %q", got, view)
	}
	if got := lipgloss.Width(view); got > 20 {
		t.Fatalf("view width = %d, want at most 20", got)
	}
}

func TestDismiss(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetError(errors.New("test"))
	b.Dismiss()

	if b.HasError() {
		t.Error("expected no error after Dismiss")
	}
	if b.View() != "" {
		t.Error("expected empty view after Dismiss")
	}
}

func TestHandleDismissIgnoresTheTickOfAnEarlierError(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetError(errors.New("first"))
	firstTick := ErrorDismissMsg{Generation: b.generation}
	b.SetError(errors.New("second"))

	b.HandleDismiss(firstTick)
	if !b.HasError() {
		t.Fatal("the tick of the first error dismissed the second error")
	}
	b.HandleDismiss(ErrorDismissMsg{Generation: b.generation})
	if b.HasError() {
		t.Error("the tick of the second error did not dismiss it")
	}
}

func TestSetErrorNil(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetError(nil)
	// nil error should still work (View returns empty)
	if b.View() != "" {
		t.Error("nil error should produce empty view")
	}
}

func TestErrorReturnsStoredError(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	err := errors.New("test error")
	b.SetError(err)
	if b.Error() != err {
		t.Error("Error() should return the stored error")
	}
}

// helpers

func defaultTestStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
