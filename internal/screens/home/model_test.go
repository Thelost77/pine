package home

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
)

func newTestModel() Model {
	styles := ui.DefaultStyles()
	return New(styles, nil)
}

func sampleItems() []abs.LibraryItem {
	author := "Jane Author"
	dur := 36000.0
	return []abs.LibraryItem{
		{
			ID:        "li-001",
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      "The Great Adventure",
					AuthorName: &author,
					Duration:   &dur,
				},
			},
			UserMediaProgress: &abs.UserMediaProgress{
				CurrentTime: 16200,
				Progress:    0.45,
				IsFinished:  false,
			},
		},
		{
			ID:        "li-002",
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      "New Horizons",
					AuthorName: &author,
				},
			},
		},
	}
}

func TestNew(t *testing.T) {
	m := newTestModel()
	if !m.Loading() {
		t.Error("expected loading to be true initially")
	}
	if m.Error() != nil {
		t.Error("expected no error initially")
	}
}

func TestPersonalizedMsg_Success(t *testing.T) {
	m := newTestModel()
	items := sampleItems()

	m, _ = m.Update(PersonalizedMsg{Items: items})

	if m.Loading() {
		t.Error("expected loading to be false after receiving items")
	}
	if m.Error() != nil {
		t.Errorf("expected no error, got %v", m.Error())
	}
	if len(m.Items()) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.Items()))
	}
}

func TestPersonalizedMsg_Error(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PersonalizedMsg{Err: fmt.Errorf("network error")})

	if m.Loading() {
		t.Error("expected loading to be false after error")
	}
	if m.Error() == nil {
		t.Error("expected an error")
	}
}

func TestEnterKey_NavigateDetail(t *testing.T) {
	m := newTestModel()
	items := sampleItems()
	m, _ = m.Update(PersonalizedMsg{Items: items})

	// Press enter — book with progress should navigate to detail (R1 change)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected a command from enter key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateDetailMsg); !ok {
		t.Errorf("expected NavigateDetailMsg for book with progress, got %T", msg)
	}
	_ = m // silence unused
}

func TestOKey_NavigateLibrary(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(PersonalizedMsg{Items: sampleItems()})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if cmd == nil {
		t.Fatal("expected a command from l key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateLibraryMsg); !ok {
		t.Errorf("expected NavigateLibraryMsg, got %T", msg)
	}
}

func TestSlashKey_NavigateSearch(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(PersonalizedMsg{Items: sampleItems()})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if cmd == nil {
		t.Fatal("expected a command from / key")
	}
	msg := cmd()
	if _, ok := msg.(NavigateSearchMsg); !ok {
		t.Errorf("expected NavigateSearchMsg, got %T", msg)
	}
}


func TestView_Loading(t *testing.T) {
	m := newTestModel()
	m.loading = true
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view when loading")
	}
}

func TestView_Error(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(PersonalizedMsg{Err: fmt.Errorf("test error")})
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view with error")
	}
}

func TestView_WithItems(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := sampleItems()
	m, _ = m.Update(PersonalizedMsg{Items: items})
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view with items")
	}
	// Verify View() contains the item titles
	for _, item := range items {
		if !strings.Contains(v, item.Media.Metadata.Title) {
			t.Errorf("expected view to contain title %q", item.Media.Metadata.Title)
		}
	}
}

func TestJKey_MovesSelection(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	m, _ = m.Update(PersonalizedMsg{Items: sampleItems()})

	// Initial selection should be index 0
	if m.list.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.list.Index())
	}

	// Press j to move selection down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.list.Index() != 1 {
		t.Errorf("expected index 1 after j, got %d", m.list.Index())
	}
}

func TestKKey_MovesSelectionUp(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	m, _ = m.Update(PersonalizedMsg{Items: sampleItems()})

	// Move down first, then up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.list.Index() != 0 {
		t.Errorf("expected index 0 after j then k, got %d", m.list.Index())
	}
}

func TestEnterKey_NavigateDetail_SelectedItem(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := sampleItems()
	m, _ = m.Update(PersonalizedMsg{Items: items})

	// Move to second item with j, then press enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from enter key")
	}
	msg := cmd()
	navMsg, ok := msg.(NavigateDetailMsg)
	if !ok {
		t.Fatalf("expected NavigateDetailMsg, got %T", msg)
	}
	if navMsg.Item.ID != items[1].ID {
		t.Errorf("expected item ID %q, got %q", items[1].ID, navMsg.Item.ID)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{3600, "1h 0m"},
		{7200, "2h 0m"},
		{5400, "1h 30m"},
		{1800, "30m"},
		{900, "15m"},
		{36000, "10h 0m"},
	}
	for _, tt := range tests {
		got := ui.FormatDuration(tt.seconds)
		if got != tt.expected {
			t.Errorf("ui.FormatDuration(%v) = %q, want %q", tt.seconds, got, tt.expected)
		}
	}
}

func TestListItem_Description(t *testing.T) {
	items := sampleItems()

	// Item with progress and duration
	li := listItem{item: items[0]}
	desc := li.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}

	// Item without progress/duration
	li2 := listItem{item: items[1]}
	desc2 := li2.Description()
	if desc2 == "" {
		t.Error("expected non-empty description for item without progress")
	}
}

func TestListItem_NoAuthor(t *testing.T) {
	item := abs.LibraryItem{
		ID: "li-003",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title: "No Author Book",
			},
		},
	}
	li := listItem{item: item}
	desc := li.Description()
	if desc == "" {
		t.Error("expected non-empty description for item without author")
	}
}

func TestInit_NilClient(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected a command from Init")
	}
	msg := cmd()
	pm, ok := msg.(PersonalizedMsg)
	if !ok {
		t.Fatalf("expected PersonalizedMsg, got %T", msg)
	}
	if pm.Err == nil {
		t.Error("expected error when client is nil")
	}
}
