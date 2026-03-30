package components

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrorDismissTimeout is how long the error banner auto-dismisses after.
const ErrorDismissTimeout = 5 * time.Second

// ErrMsg signals an application error to be displayed in the banner.
type ErrMsg struct {
	Err error
}

// ErrorDismissMsg signals that the error banner should be dismissed.
type ErrorDismissMsg struct{}

// ErrorBanner holds the state for the error banner overlay.
type ErrorBanner struct {
	err   error
	style lipgloss.Style
	width int
}

// NewErrorBanner creates a new error banner with the given error style.
func NewErrorBanner(errStyle lipgloss.Style) ErrorBanner {
	return ErrorBanner{
		style: errStyle,
	}
}

// SetWidth updates the banner width.
func (b *ErrorBanner) SetWidth(w int) {
	b.width = w
}

// SetError sets the current error and returns a command to auto-dismiss.
func (b *ErrorBanner) SetError(err error) tea.Cmd {
	b.err = err
	return scheduleDismiss()
}

// Dismiss clears the error.
func (b *ErrorBanner) Dismiss() {
	b.err = nil
}

// HasError returns true if there is an active error.
func (b ErrorBanner) HasError() bool {
	return b.err != nil
}

// Error returns the current error.
func (b ErrorBanner) Error() error {
	return b.err
}

// View renders the error banner. Returns "" if no error.
func (b ErrorBanner) View() string {
	if b.err == nil {
		return ""
	}

	msg := enrichMessage(b.err.Error())

	w := b.width
	if w < 4 {
		w = 40
	}

	banner := b.style.
		Width(w).
		Padding(0, 1).
		Render("⚠ " + msg)

	return banner
}

// enrichMessage adds contextual hints for known error patterns.
func enrichMessage(msg string) string {
	lower := strings.ToLower(msg)

	if strings.Contains(lower, "mpv not found") || strings.Contains(lower, "mpv: not found") ||
		strings.Contains(lower, "executable file not found") && strings.Contains(lower, "mpv") {
		return msg + " — Install mpv: https://mpv.io/installation/"
	}

	return msg
}

// IsUnauthorized returns true if the error message indicates HTTP 401.
func IsUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status 401") || strings.Contains(msg, "unauthorized")
}

// scheduleDismiss returns a command that sends ErrorDismissMsg after the timeout.
func scheduleDismiss() tea.Cmd {
	return tea.Tick(ErrorDismissTimeout, func(_ time.Time) tea.Msg {
		return ErrorDismissMsg{}
	})
}
