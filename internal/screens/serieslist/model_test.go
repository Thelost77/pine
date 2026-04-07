package serieslist

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	m := New(ui.DefaultStyles(), nil, "lib-books-001", "Books")
	m.SetSize(80, 24)
	return m
}

func sampleSeries() []abs.Series {
	return []abs.Series{
		{ID: "series-expanse", Name: "The Expanse"},
		{ID: "series-dune", Name: "Dune"},
	}
}

func TestLoadedMsgPopulatesList(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(LoadedMsg{Results: sampleSeries(), Total: 2, Page: 0})

	if m.Loading() {
		t.Fatal("expected loading to be false after load")
	}
	if len(m.series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(m.series))
	}
}

func TestViewShowsSeriesNames(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{Results: sampleSeries(), Total: 2, Page: 0})

	v := m.View()
	for _, want := range []string{"Series", "The Expanse", "Dune"} {
		if !strings.Contains(v, want) {
			t.Fatalf("expected view to contain %q\n%s", want, v)
		}
	}
}

func TestEnterNavigatesToSeries(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{Results: sampleSeries(), Total: 2, Page: 0})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter key")
	}
	msg := cmd()
	navMsg, ok := msg.(NavigateSeriesMsg)
	if !ok {
		t.Fatalf("expected NavigateSeriesMsg, got %T", msg)
	}
	if navMsg.LibraryID != "lib-books-001" {
		t.Fatalf("library ID = %q, want lib-books-001", navMsg.LibraryID)
	}
	if navMsg.SeriesID != "series-expanse" {
		t.Fatalf("series ID = %q, want series-expanse", navMsg.SeriesID)
	}
}
