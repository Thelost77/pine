package components

import (
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ErrorDismissTimeout is how long the error banner auto-dismisses after.
const ErrorDismissTimeout = 5 * time.Second

// ErrMsg signals an application error to be displayed in the banner.
type ErrMsg struct {
	Err error
}

// ErrorDismissMsg signals that the auto-dismiss time of an error ended.
// Generation identifies that error in the banner.
type ErrorDismissMsg struct {
	Generation uint64
}

// ErrorBanner holds the state for the error banner overlay.
type ErrorBanner struct {
	err   error
	style lipgloss.Style
	width int
	// generation counts the calls of SetError. It ties a dismiss tick to its
	// error.
	generation uint64
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
	b.generation++
	return scheduleDismiss(b.generation)
}

// Dismiss clears the error.
func (b *ErrorBanner) Dismiss() {
	b.err = nil
}

// HandleDismiss clears the error if msg is the dismiss tick of the current
// error. The tick of an earlier error does nothing.
func (b *ErrorBanner) HandleDismiss(msg ErrorDismissMsg) {
	if msg.Generation == b.generation {
		b.err = nil
	}
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

	msg := sanitizeTerminalText(b.err.Error())

	w := b.width
	if w < 4 {
		w = 40
	}

	style := b.style.Padding(0, 1)
	contentWidth := max(1, w-style.GetHorizontalFrameSize())
	content := ansi.Truncate("⚠ "+msg, contentWidth, "…")

	return style.Width(w).Render(content)
}

// sanitizeTerminalText removes escape sequences and controls before terminal rendering.
func sanitizeTerminalText(msg string) string {
	msg = ansi.Strip(msg)
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, msg)
}

// scheduleDismiss returns a command that sends ErrorDismissMsg for the error
// of the generation after the timeout.
func scheduleDismiss(generation uint64) tea.Cmd {
	return tea.Tick(ErrorDismissTimeout, func(_ time.Time) tea.Msg {
		return ErrorDismissMsg{Generation: generation}
	})
}
