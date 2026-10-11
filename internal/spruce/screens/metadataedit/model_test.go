package metadataedit

import (
	"testing"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMetadataEditTrack(t *testing.T) {
	styles := ui.DefaultStyles()
	track := &jellyfin.Track{
		ID:                "track1",
		Name:              "My Song",
		Artists:           []string{"Artist A", "Artist B"},
		Album:             "My Album",
		IndexNumber:       5,
		ParentIndexNumber: 1,
	}

	m := New(styles, nil, track.ID, "Track", track, nil)
	m.SetSize(80, 24)

	// Check inputs initialized properly
	if len(m.inputs) != 5 {
		t.Fatalf("expected 5 inputs for track, got %d", len(m.inputs))
	}
	if m.inputs[0].Value() != "My Song" {
		t.Errorf("expected Title input 'My Song', got '%s'", m.inputs[0].Value())
	}
	if m.inputs[1].Value() != "Artist A, Artist B" {
		t.Errorf("expected Artists input 'Artist A, Artist B', got '%s'", m.inputs[1].Value())
	}
	if m.inputs[3].Value() != "5" {
		t.Errorf("expected Track Number '5', got '%s'", m.inputs[3].Value())
	}

	// Test tab key moves focus
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if newM.focused != 1 {
		t.Errorf("expected focus 1 after tab, got %d", newM.focused)
	}

	// Test Esc key returns BackMsg
	_, cmd := newM.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected cmd on esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg on esc, got %T", msg)
	}

	// Test Enter triggers save (client nil -> SavedMsg with err)
	_, cmd = newM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected cmd on enter, got nil")
	}
	saved := cmd()
	sMsg, ok := saved.(SavedMsg)
	if !ok {
		t.Fatalf("expected SavedMsg on enter, got %T", saved)
	}
	if sMsg.Err == nil {
		t.Errorf("expected client not configured error, got nil")
	}
}
