package series

import (
	"fmt"
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

func sampleLongSeriesContents(n int) abs.SeriesContents {
	items := make([]abs.LibraryItem, 0, n)
	for i := 0; i < n; i++ {
		seq := fmt.Sprintf("%d", i+1)
		items = append(items, abs.LibraryItem{
			ID:        fmt.Sprintf("li-book-%03d", i),
			LibraryID: "lib-books-001",
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title: fmt.Sprintf("Book %d", i+1),
					Series: &abs.SeriesSequence{
						ID:       "series-expanse",
						Name:     "The Expanse",
						Sequence: seq,
					},
				},
			},
		})
	}
	return abs.SeriesContents{
		Series: abs.Series{ID: "series-expanse", Name: "The Expanse"},
		Items:  items,
	}
}

func TestNewStartsWithSkeletonRows(t *testing.T) {
	m := newTestModel()

	rows := m.list.Items()
	if len(rows) == 0 {
		t.Fatal("expected initial skeleton rows")
	}
	if _, ok := rows[0].(seriesSkeletonItem); !ok {
		t.Fatalf("expected first row to be seriesSkeletonItem, got %T", rows[0])
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

func TestViewDoesNotShowLoadingMessage(t *testing.T) {
	m := newTestModel()

	if strings.Contains(m.View(), "Loading series") {
		t.Fatal("view should not show loading text")
	}
}

func TestLoadingSkeletonsDoNotRenderSelectedHighlight(t *testing.T) {
	m := newTestModel()

	if strings.Contains(m.View(), "│") {
		t.Fatalf("loading skeletons should not render selected highlight\n%s", m.View())
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

func TestHLPageAcrossSeries(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{Contents: sampleLongSeriesContents(30)})

	before := m.list.GlobalIndex()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	after := m.list.GlobalIndex()
	if after <= before {
		t.Fatalf("expected L to page down from %d, got %d", before, after)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	if got := m.list.GlobalIndex(); got >= after {
		t.Fatalf("expected H to page up from %d, got %d", after, got)
	}
}

func TestHLFallsBackToExtremes(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{Contents: sampleLongSeriesContents(30)})

	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	}
	if got, want := m.list.GlobalIndex(), len(m.list.Items())-1; got != want {
		t.Fatalf("L at end should jump to last row: got %d want %d", got, want)
	}

	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	}
	if got := m.list.GlobalIndex(); got != 0 {
		t.Fatalf("H at start should jump to first row: got %d want 0", got)
	}
}
