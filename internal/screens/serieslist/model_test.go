package serieslist

import (
	"errors"
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
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

	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Results: sampleSeries(), Total: 2, Page: 0})

	if m.Loading() {
		t.Fatal("expected loading to be false after load")
	}
	if len(m.series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(m.series))
	}
}

func TestLoadedMsgClearsEarlierError(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Results: sampleSeries(), Total: 4, Page: 0})
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Err: errors.New("network error")})
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Results: []abs.Series{{ID: "series-foundation", Name: "Foundation"}}, Total: 4, Page: 1})

	if m.err != nil {
		t.Fatalf("err = %v after a successful load, want nil", m.err)
	}
	if v := m.View(); !strings.Contains(v, "Foundation") {
		t.Fatalf("expected view to show the loaded series\n%s", v)
	}
}

func TestViewShowsSeriesNames(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Results: sampleSeries(), Total: 2, Page: 0})

	v := m.View()
	for _, want := range []string{"Series", "The Expanse", "Dune"} {
		if !strings.Contains(v, want) {
			t.Fatalf("expected view to contain %q\n%s", want, v)
		}
	}
}

func TestEnterNavigatesToSeries(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Results: sampleSeries(), Total: 2, Page: 0})

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

// TestViewShowsNumberOfBooks verifies that a row shows the number of books of
// the series, and the description after it.
func TestViewShowsNumberOfBooks(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-books-001", Results: []abs.Series{
		{ID: "series-dune", Name: "Dune", Books: []abs.SeriesBook{{ID: "b1"}, {ID: "b2"}}},
		{ID: "series-expanse", Name: "The Expanse", Description: "Space opera", Books: []abs.SeriesBook{{ID: "b3"}}},
	}, Total: 2, Page: 0})

	v := m.View()
	for _, want := range []string{"2 books", "1 book · Space opera"} {
		if !strings.Contains(v, want) {
			t.Errorf("expected view to contain %q\n%s", want, v)
		}
	}
}

func seriesNames(m Model) string {
	names := make([]string, len(m.series))
	for i, s := range m.series {
		names[i] = s.Name
	}
	return strings.Join(names, ",")
}

// TestReloadDropsAnswerOfEarlierLoad verifies that a reload replaces the list
// with the new first page. The answer of a load of a next page that started
// before the reload must not add an old page to the new list.
func TestReloadDropsAnswerOfEarlierLoad(t *testing.T) {
	const lib = "lib-books-001"
	m := New(ui.DefaultStyles(), cache.NewClient(abs.NewClient("http://test", "tok"), nil), lib, "Books")
	m.SetSize(80, 24)
	page := func(names ...string) []abs.Series {
		series := make([]abs.Series, len(names))
		for i, name := range names {
			series[i] = abs.Series{ID: "series-" + name, Name: name}
		}
		return series
	}
	m, _ = m.Update(LoadedMsg{LibraryID: lib, Results: page("A", "B"), Total: 6, Page: 0})
	m, _ = m.Update(LoadedMsg{LibraryID: lib, Results: page("C", "D"), Total: 6, Page: 1})
	late := LoadedMsg{LibraryID: lib, Results: page("E", "F"), Total: 6, Page: 2}

	_ = m.ReloadCmd()
	if !m.Loading() {
		t.Error("the list does not load during a reload, so it can start a load of a next page")
	}
	m, _ = m.Update(LoadedMsg{LibraryID: lib, Generation: m.generation, Results: page("A", "B2"), Total: 6, Page: 0})
	m, _ = m.Update(late)

	if got := seriesNames(m); got != "A,B2" {
		t.Errorf("series = %s, want A,B2", got)
	}
}

// TestLoadedMsgOfOtherLibraryIsDropped verifies that the list drops the answer
// of a load for a different library.
func TestLoadedMsgOfOtherLibraryIsDropped(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(LoadedMsg{LibraryID: "lib-other", Results: sampleSeries(), Total: 2, Page: 0})
	if got := seriesNames(m); got != "" {
		t.Errorf("series = %s, want none", got)
	}
}
