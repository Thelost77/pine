package components

import (
	"errors"
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

func TestSetErrorNil(t *testing.T) {
	b := NewErrorBanner(defaultTestStyle())
	b.SetError(nil)
	// nil error should still work (View returns empty)
	if b.View() != "" {
		t.Error("nil error should produce empty view")
	}
}

func TestEnrichMessageMpvNotFound(t *testing.T) {
	msg := enrichMessage("mpv not found")
	if !contains(msg, "Install mpv") {
		t.Errorf("expected install hint, got: %q", msg)
	}
}

func TestEnrichMessageNormalError(t *testing.T) {
	msg := enrichMessage("connection refused")
	if contains(msg, "Install") {
		t.Error("normal error should not get install hint")
	}
}

func TestIsUnauthorized(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("unexpected status 401: Unauthorized"), true},
		{errors.New("something failed"), false},
		{errors.New("unauthorized access"), true},
	}
	for _, tt := range tests {
		if got := IsUnauthorized(tt.err); got != tt.want {
			name := "<nil>"
			if tt.err != nil {
				name = tt.err.Error()
			}
			t.Errorf("IsUnauthorized(%q) = %v, want %v", name, got, tt.want)
		}
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
