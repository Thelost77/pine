package queue

import (
	"slices"
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestQueueModel_ActionsAndView(t *testing.T) {
	tracks := []jellyfin.Track{
		{ID: "t-1", Name: "Track 1", RunTimeTicks: 1800000000},
		{ID: "t-2", Name: "Track 2", RunTimeTicks: 2400000000},
	}

	m := New(ui.DefaultStyles())
	if slices.Contains(m.list.KeyMap.NextPage.Keys(), "f") {
		t.Fatal("f remains bound to next page")
	}
	if slices.Contains(m.list.KeyMap.PrevPage.Keys(), "b") {
		t.Fatal("b remains bound to previous page")
	}
	m.SetSize(80, 24)

	// Test empty view
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view for empty queue")
	}

	// Populate queue
	m.SetQueue(tracks, 0)
	m.SetPlaybackState(true, false, 15.0, 180.0)

	if len(m.Tracks()) != 2 || m.CurrentIndex() != 0 {
		t.Fatalf("unexpected queue state: %d tracks, cur=%d", len(m.Tracks()), m.CurrentIndex())
	}

	// Test View with playing header
	v = m.View()
	if v == "" {
		t.Error("expected non-empty view with playing queue")
	}

	// Test Enter (JumpQueueMsg)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected jump cmd")
	}
	msg := cmd()
	if jqm, ok := msg.(JumpQueueMsg); !ok || jqm.Index != 0 {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test d (RemoveQueueMsg)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd == nil {
		t.Fatal("expected remove cmd for d")
	}
	msg = cmd()
	if rqm, ok := msg.(RemoveQueueMsg); !ok || rqm.Index != 0 {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test x (RemoveQueueMsg)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd == nil {
		t.Fatal("expected remove cmd for x")
	}
	msg = cmd()
	if rqm, ok := msg.(RemoveQueueMsg); !ok || rqm.Index != 0 {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test delete (RemoveQueueMsg)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if cmd == nil {
		t.Fatal("expected remove cmd for delete")
	}
	msg = cmd()
	if rqm, ok := msg.(RemoveQueueMsg); !ok || rqm.Index != 0 {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test backspace (RemoveQueueMsg)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd == nil {
		t.Fatal("expected remove cmd for backspace")
	}
	msg = cmd()
	if rqm, ok := msg.(RemoveQueueMsg); !ok || rqm.Index != 0 {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test c (Clear action)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("expected clear cmd")
	}
	msg = cmd()
	if qam, ok := msg.(QueueActionMsg); !ok || qam.Action != "clear" {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test n (Next action)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd == nil {
		t.Fatal("expected next cmd")
	}
	msg = cmd()
	if qam, ok := msg.(QueueActionMsg); !ok || qam.Action != "next" {
		t.Errorf("unexpected msg: %+v", msg)
	}

	// Test subliminal mode action.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if cmd == nil {
		t.Fatal("expected subliminal mode cmd")
	}
	msg = cmd()
	if qam, ok := msg.(QueueActionMsg); !ok || qam.Action != "subliminal" {
		t.Errorf("unexpected msg: %+v", msg)
	}

	m.SetSubliminalState(SubliminalPreparing, 1, 2, 20)
	if view := m.View(); !strings.Contains(view, "1/2 cached") || !strings.Contains(view, "20% per track") {
		t.Fatalf("unexpected subliminal view: %s", view)
	}

	// Test space (toggle pause)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected toggle pause cmd")
	}
	msg = cmd()
	if qam, ok := msg.(QueueActionMsg); !ok || qam.Action != "toggle_pause" {
		t.Errorf("unexpected msg: %+v", msg)
	}
}

func TestQueueModel_SetQueuePreservesCursorOnPlaybackProbe(t *testing.T) {
	tracks := []jellyfin.Track{
		{ID: "t-1", Name: "Track 1"},
		{ID: "t-2", Name: "Track 2"},
		{ID: "t-3", Name: "Track 3"},
	}

	m := New(ui.DefaultStyles())
	m.SetSize(80, 24)
	m.SetQueue(tracks, 0)

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		_ = cmd()
	}
	m.SetQueue(tracks, 0)

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected jump cmd")
	}
	msg := cmd()
	if jqm, ok := msg.(JumpQueueMsg); !ok || jqm.Index != 1 {
		t.Fatalf("expected cursor to stay on index 1, got %+v", msg)
	}
}

func TestQueueModel_SetQueueSelectsNewCurrentTrack(t *testing.T) {
	tracks := []jellyfin.Track{
		{ID: "t-1", Name: "Track 1"},
		{ID: "t-2", Name: "Track 2"},
		{ID: "t-3", Name: "Track 3"},
	}

	m := New(ui.DefaultStyles())
	m.SetSize(80, 24)
	m.SetQueue(tracks, 0)

	m.SetQueue(tracks, 2)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected jump cmd")
	}
	msg := cmd()
	if jqm, ok := msg.(JumpQueueMsg); !ok || jqm.Index != 2 {
		t.Fatalf("expected new current index selected, got %+v", msg)
	}
}
