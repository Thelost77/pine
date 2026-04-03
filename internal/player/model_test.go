package player

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/ui"
)

func newTestModel() Model {
	cfg := config.Default()
	styles := ui.DefaultStyles()
	return NewModel(nil, cfg, styles)
}

func TestInitReturnsNil(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestNewModelDefaults(t *testing.T) {
	m := newTestModel()
	if m.Playing {
		t.Error("expected Playing to be false")
	}
	if m.Title != "" {
		t.Error("expected Title to be empty")
	}
	if m.Speed != 1.0 {
		t.Errorf("expected Speed 1.0, got %f", m.Speed)
	}
}

func TestStartPlayMsg(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test Book"})
	if !m.Playing {
		t.Error("expected Playing to be true after StartPlayMsg")
	}
	if m.Title != "Test Book" {
		t.Errorf("expected Title 'Test Book', got '%s'", m.Title)
	}
}

func TestPositionMsg(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	m, _ = m.Update(PositionMsg{Position: 120, Duration: 600, Paused: false})
	if m.Position != 120 {
		t.Errorf("expected Position 120, got %f", m.Position)
	}
	if m.Duration != 600 {
		t.Errorf("expected Duration 600, got %f", m.Duration)
	}
	if !m.Playing {
		t.Error("expected Playing to be true")
	}
}

func TestPositionMsgPaused(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	m, _ = m.Update(PositionMsg{Position: 60, Duration: 300, Paused: true})
	if m.Playing {
		t.Error("expected Playing to be false when paused")
	}
}

func TestPositionMsgError(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	m.Position = 42
	m, _ = m.Update(PositionMsg{Err: errTest})
	// State should not change on error
	if m.Position != 42 {
		t.Errorf("expected Position unchanged at 42, got %f", m.Position)
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestSpaceTogglesPause(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	if !m.Playing {
		t.Fatal("expected Playing true after start")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.Playing {
		t.Error("expected Playing false after space")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !m.Playing {
		t.Error("expected Playing true after second space")
	}
}

func TestSpeedUp(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	m.Speed = 1.0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if m.Speed != 1.1 {
		t.Errorf("expected Speed 1.1, got %f", m.Speed)
	}
}

func TestSpeedDown(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	m.Speed = 1.0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if m.Speed != 0.9 {
		t.Errorf("expected Speed 0.9, got %f", m.Speed)
	}
}

func TestSpeedDownFloor(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	m.Speed = 0.1

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if m.Speed != 0.1 {
		t.Errorf("expected Speed 0.1 (floor), got %f", m.Speed)
	}
}

func TestViewInactive(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if v != "" {
		t.Errorf("expected empty view when inactive, got '%s'", v)
	}
}

func TestViewPlaying(t *testing.T) {
	m := newTestModel()
	m.Title = "My Audiobook"
	m.Playing = true
	m.Position = 754 // 12:34
	m.Duration = 1725 // 28:45
	m.Speed = 1.0
	m.width = 80

	v := m.View()
	if !strings.Contains(v, "▶") {
		t.Error("expected ▶ icon when playing")
	}
	if !strings.Contains(v, "My Audiobook") {
		t.Error("expected title in view")
	}
	if !strings.Contains(v, "12:34") {
		t.Errorf("expected '12:34' in view, got: %s", v)
	}
	if !strings.Contains(v, "28:45") {
		t.Errorf("expected '28:45' in view, got: %s", v)
	}
	if !strings.Contains(v, "1.0x") {
		t.Error("expected '1.0x' in view")
	}
}

func TestViewPaused(t *testing.T) {
	m := newTestModel()
	m.Title = "Test"
	m.Playing = false
	m.Position = 60
	m.Duration = 120
	m.Speed = 1.5

	v := m.View()
	if !strings.Contains(v, "⏸") {
		t.Error("expected ⏸ icon when paused")
	}
	if !strings.Contains(v, "1.5x") {
		t.Error("expected '1.5x' in view")
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00"},
		{59, "0:59"},
		{60, "1:00"},
		{754, "12:34"},
		{1725, "28:45"},
		{3600, "60:00"},
		{-1, "0:00"},
	}
	for _, tc := range tests {
		got := formatTime(tc.seconds)
		if got != tc.expected {
			t.Errorf("formatTime(%f) = %s, want %s", tc.seconds, got, tc.expected)
		}
	}
}

func TestKeysIgnoredWhenInactive(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.Playing {
		t.Error("space should be ignored when player is inactive")
	}
}

func TestKeysWorkWhenPaused(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(StartPlayMsg{Title: "Test"})
	// Pause it
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.Playing {
		t.Fatal("expected paused")
	}
	// Space should still work when paused
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !m.Playing {
		t.Error("space should resume when paused")
	}
}

func TestPlayerKeyMapBindings(t *testing.T) {
	m := newTestModel()
	// Verify key bindings are set
	if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}, m.keys.SpeedUp) {
		t.Error("expected '+' to match SpeedUp")
	}
	if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}, m.keys.SpeedDown) {
		t.Error("expected '-' to match SpeedDown")
	}
}
