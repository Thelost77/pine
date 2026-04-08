package series

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	styles := ui.DefaultStyles()
	m := New(styles, nil, "lib-books-001", "series-expanse", "li-book-002")
	m.SetSize(80, 24)
	return m
}

func sampleSeriesContents() abs.SeriesContents {
	return abs.SeriesContents{
		Series: abs.Series{
			ID:   "series-expanse",
			Name: "The Expanse",
		},
		Items: []abs.LibraryItem{
			{
				ID:        "li-book-001",
				LibraryID: "lib-books-001",
				MediaType: "book",
				Media: abs.Media{
					Metadata: abs.MediaMetadata{
						Title: "Leviathan Wakes",
						Series: &abs.SeriesSequence{
							ID:       "series-expanse",
							Name:     "The Expanse",
							Sequence: "1",
						},
					},
				},
			},
			{
				ID:        "li-book-002",
				LibraryID: "lib-books-001",
				MediaType: "book",
				Media: abs.Media{
					Metadata: abs.MediaMetadata{
						Title: "Caliban's War",
						Series: &abs.SeriesSequence{
							ID:       "series-expanse",
							Name:     "The Expanse",
							Sequence: "2",
						},
					},
				},
			},
		},
	}
}

func TestSeriesLoadedMsgPopulatesListAndSelectsCurrentItem(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(LoadedMsg{Contents: sampleSeriesContents()})

	if m.Loading() {
		t.Fatal("expected loading to be false after series load")
	}
	if m.SelectedItemID() != "li-book-002" {
		t.Fatalf("selected item = %q, want li-book-002", m.SelectedItemID())
	}
}

func TestView_ShowsSeriesNameAndBooks(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{Contents: sampleSeriesContents()})

	v := m.View()
	for _, want := range []string{"The Expanse", "Leviathan Wakes", "Caliban's War"} {
		if !strings.Contains(v, want) {
			t.Fatalf("expected view to contain %q\n%s", want, v)
		}
	}
}

func TestEnterKey_NavigatesToSelectedBook(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{Contents: sampleSeriesContents()})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter key")
	}
	msg := cmd()
	navMsg, ok := msg.(NavigateDetailMsg)
	if !ok {
		t.Fatalf("expected NavigateDetailMsg, got %T", msg)
	}
	if navMsg.Item.ID != "li-book-002" {
		t.Fatalf("item ID = %q, want li-book-002", navMsg.Item.ID)
	}
}
