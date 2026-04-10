package home

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
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

func sampleRecentlyAddedItems() []abs.LibraryItem {
	author := "Jane Author"
	return []abs.LibraryItem{
		{
			ID:        "li-002",
			MediaType: "book",
			AddedAt:   1712222222222,
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      "New Horizons",
					AuthorName: &author,
				},
			},
		},
		{
			ID:        "li-003",
			MediaType: "book",
			AddedAt:   1713333333333,
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      "Hidden Empire",
					AuthorName: &author,
				},
			},
		},
		{
			ID:        "li-004",
			MediaType: "book",
			AddedAt:   1714444444444,
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      "Leviathan Falls",
					AuthorName: &author,
				},
			},
		},
		{
			ID:        "li-005",
			MediaType: "book",
			AddedAt:   1715555555555,
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      "Memory's Legion",
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

	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: sampleRecentlyAddedItems()})

	if m.Loading() {
		t.Error("expected loading to be false after receiving items")
	}
	if m.Error() != nil {
		t.Errorf("expected no error, got %v", m.Error())
	}
	if len(m.Items()) != 2 {
		t.Errorf("expected 2 items, got %d", len(m.Items()))
	}
	if len(m.RecentlyAdded()) != 3 {
		t.Errorf("expected 3 recently added items, got %d", len(m.RecentlyAdded()))
	}
}

func TestPersonalizedMsg_CapsContinueListeningAtFive(t *testing.T) {
	m := newTestModel()
	items := make([]abs.LibraryItem, 0, 7)
	for i := 0; i < 7; i++ {
		items = append(items, abs.LibraryItem{
			ID:        fmt.Sprintf("li-%03d", i),
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title: fmt.Sprintf("Book %d", i),
				},
			},
		})
	}

	m, _ = m.Update(PersonalizedMsg{Items: items})

	if len(m.Items()) != 5 {
		t.Fatalf("expected continue listening cap of 5, got %d", len(m.Items()))
	}
	if m.Items()[4].Media.Metadata.Title != "Book 4" {
		t.Fatalf("expected fifth visible item to be Book 4, got %q", m.Items()[4].Media.Metadata.Title)
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
	m, _ = m.Update(PersonalizedMsg{
		Items: sampleItems(),
		Libraries: []abs.Library{
			{ID: "lib-books", Name: "Books", MediaType: "book"},
		},
	})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if cmd == nil {
		t.Fatal("expected a command from / key")
	}
	msg := cmd()
	searchMsg, ok := msg.(NavigateSearchMsg)
	if !ok {
		t.Errorf("expected NavigateSearchMsg, got %T", msg)
	}
	if searchMsg.LibraryID != "lib-books" {
		t.Fatalf("libraryID = %q, want lib-books", searchMsg.LibraryID)
	}
	if searchMsg.LibraryMediaType != "book" {
		t.Fatalf("libraryMediaType = %q, want book", searchMsg.LibraryMediaType)
	}
}

func TestLKeyPagesDown(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 8)

	items := make([]abs.LibraryItem, 0, 5)
	recent := make([]abs.LibraryItem, 0, 5)
	for i := 0; i < 5; i++ {
		items = append(items, abs.LibraryItem{
			ID:        fmt.Sprintf("item-%d", i),
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: fmt.Sprintf("Book %d", i)}},
		})
		recent = append(recent, abs.LibraryItem{
			ID:        fmt.Sprintf("recent-%d", i),
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: fmt.Sprintf("Recent %d", i)}},
		})
	}

	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: recent})

	before := m.list.GlobalIndex()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	after := m.list.GlobalIndex()
	if after <= before {
		t.Fatalf("expected L to page down from %d, got %d", before, after)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	if m.list.GlobalIndex() >= after {
		t.Fatalf("expected H to page up from %d, got %d", after, m.list.GlobalIndex())
	}
}

func TestLKeyFallsBackToEndAndHToStart(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)

	items := make([]abs.LibraryItem, 0, 10)
	recent := make([]abs.LibraryItem, 0, 10)
	for i := 0; i < 10; i++ {
		items = append(items, abs.LibraryItem{
			ID:        fmt.Sprintf("item-%d", i),
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: fmt.Sprintf("Book %d", i)}},
		})
		recent = append(recent, abs.LibraryItem{
			ID:        fmt.Sprintf("recent-%d", i),
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: fmt.Sprintf("Recent %d", i)}},
		})
	}

	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: recent})

	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	}
	lastIndex := len(m.list.Items()) - 1
	if got := m.list.GlobalIndex(); got != lastIndex {
		t.Fatalf("L at end should jump to last row: got %d want %d", got, lastIndex)
	}

	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	}
	if got := m.list.GlobalIndex(); got != 0 {
		t.Fatalf("H at start should jump to first row: got %d want 0", got)
	}
}

func TestView_Loading(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	m.loading = true
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view when loading")
	}
	if !strings.Contains(v, "Continue Listening") {
		t.Fatalf("expected title to remain visible while loading, got %q", v)
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
	recent := sampleRecentlyAddedItems()
	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: recent})
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
	if !strings.Contains(v, "Recently Added") {
		t.Error("expected view to contain Recently Added subsection")
	}
	for _, item := range m.RecentlyAdded() {
		if !strings.Contains(v, item.Media.Metadata.Title) {
			t.Errorf("expected view to contain recently added title %q", item.Media.Metadata.Title)
		}
	}
}

func TestView_LoadingWithItemsKeepsListVisible(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := sampleItems()
	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: sampleRecentlyAddedItems()})
	m.loading = true

	view := m.View()
	if !strings.Contains(view, items[0].Media.Metadata.Title) {
		t.Fatalf("expected cached content to stay visible during refresh\n%s", view)
	}
}

func TestLoadingRevealShowsSkeletonRows(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	m.loading = true
	m.refreshListRows()

	view := m.View()
	if !strings.Contains(view, "Recently Added") {
		t.Fatalf("expected skeleton layout to include Recently Added section\n%s", view)
	}
	if !strings.Contains(view, "----------------------") {
		t.Fatalf("expected skeleton rows in loading view\n%s", view)
	}
}

func TestTabSwitchToUncachedLibraryDelaysSkeletons(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	libs := []abs.Library{
		{ID: "lib-1", Name: "Books", MediaType: "book"},
		{ID: "lib-2", Name: "Podcasts", MediaType: "podcast"},
	}
	m, _ = m.Update(PersonalizedMsg{
		Items:         sampleItems(),
		RecentlyAdded: sampleRecentlyAddedItems(),
		Libraries:     libs,
		LibraryID:     "lib-1",
	})

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if !m.Loading() {
		t.Fatal("expected loading to be true after switching to uncached library")
	}
	if cmd == nil {
		t.Fatal("expected refresh command after switching libraries")
	}
	view := m.View()
	if !strings.Contains(view, sampleItems()[0].Media.Metadata.Title) {
		t.Fatalf("expected previous content to remain visible before reveal delay\n%s", view)
	}
	if strings.Contains(view, "No items in continue listening") {
		t.Fatalf("expected previous content instead of empty state\n%s", view)
	}
	if strings.Contains(view, "----------------------") {
		t.Fatalf("expected skeletons to stay hidden before reveal delay\n%s", view)
	}
}

func TestTabSwitchToUncachedLibraryShowsLoadingHintAfterRevealDelay(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	libs := []abs.Library{
		{ID: "lib-1", Name: "Books", MediaType: "book"},
		{ID: "lib-2", Name: "Podcasts", MediaType: "podcast"},
	}
	m, _ = m.Update(PersonalizedMsg{
		Items:         sampleItems(),
		RecentlyAdded: sampleRecentlyAddedItems(),
		Libraries:     libs,
		LibraryID:     "lib-1",
	})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(loadingRevealMsg{generation: m.loadingGen})

	view := m.View()
	if !strings.Contains(view, "(loading...)") {
		t.Fatalf("expected loading hint in title after reveal delay\n%s", view)
	}
	if !strings.Contains(view, sampleItems()[0].Media.Metadata.Title) {
		t.Fatalf("expected stale content to remain visible while loading\n%s", view)
	}
}

func TestStalePersonalizedMsgDoesNotClearLoadingForCurrentLibrary(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	libs := []abs.Library{
		{ID: "lib-1", Name: "Books", MediaType: "book"},
		{ID: "lib-2", Name: "Podcasts", MediaType: "podcast"},
	}
	m, _ = m.Update(PersonalizedMsg{
		Items:         sampleItems(),
		RecentlyAdded: sampleRecentlyAddedItems(),
		Libraries:     libs,
		LibraryID:     "lib-1",
	})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	if !m.Loading() {
		t.Fatal("expected loading to be true after switching libraries")
	}

	m, _ = m.Update(PersonalizedMsg{LibraryID: "lib-1", Items: sampleItems()})

	if !m.Loading() {
		t.Fatal("stale message should not clear loading for the active library")
	}
}

func TestTabSwitchToUncachedLibraryDisablesStaleSelection(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	libs := []abs.Library{
		{ID: "lib-1", Name: "Books", MediaType: "book"},
		{ID: "lib-2", Name: "Podcasts", MediaType: "podcast"},
	}
	m, _ = m.Update(PersonalizedMsg{
		Items:         sampleItems(),
		RecentlyAdded: sampleRecentlyAddedItems(),
		Libraries:     libs,
		LibraryID:     "lib-1",
	})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected selection to be disabled while stale content is shown for another library")
	}
}

func TestView_EmptyContinueListeningKeepsHeaderVisible(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	recent := sampleRecentlyAddedItems()

	m, _ = m.Update(PersonalizedMsg{RecentlyAdded: recent})

	view := m.View()
	if !strings.Contains(view, "Continue Listening") {
		t.Fatalf("expected Continue Listening header to stay visible\n%s", view)
	}
	if !strings.Contains(view, "Recently Added") {
		t.Fatalf("expected Recently Added section to render\n%s", view)
	}
	if !strings.Contains(view, recent[0].Media.Metadata.Title) {
		t.Fatalf("expected recently added item to render when continue listening is empty\n%s", view)
	}
}

func TestSelection_SkipsRecentlyAddedHeader(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := sampleItems()[:1]
	recent := sampleRecentlyAddedItems()[:1]

	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: recent})
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
	if navMsg.Item.ID != recent[0].ID {
		t.Fatalf("expected selection to skip section header and open recent item %q, got %q", recent[0].ID, navMsg.Item.ID)
	}
}

func TestAKey_AddsSelectedHomeItemToQueue(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := sampleItems()[:1]

	m, _ = m.Update(PersonalizedMsg{Items: items})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected queue command from a")
	}
	msg := cmd()
	queueMsg, ok := msg.(AddToQueueMsg)
	if !ok {
		t.Fatalf("expected AddToQueueMsg, got %T", msg)
	}
	if queueMsg.Item.ID != items[0].ID {
		t.Fatalf("expected queued home item %q, got %q", items[0].ID, queueMsg.Item.ID)
	}
}

func TestShiftAKey_AddsSelectedHomeItemAsPlayNext(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := sampleItems()[:1]

	m, _ = m.Update(PersonalizedMsg{Items: items})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if cmd == nil {
		t.Fatal("expected queue command from A")
	}
	msg := cmd()
	queueMsg, ok := msg.(PlayNextMsg)
	if !ok {
		t.Fatalf("expected PlayNextMsg, got %T", msg)
	}
	if queueMsg.Item.ID != items[0].ID {
		t.Fatalf("expected play-next home item %q, got %q", items[0].ID, queueMsg.Item.ID)
	}
}

func TestView_DoesNotCountRecentlyAddedInListStatus(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := make([]abs.LibraryItem, 0, 5)
	for i := 0; i < 5; i++ {
		items = append(items, abs.LibraryItem{
			ID:        fmt.Sprintf("book-%d", i),
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: fmt.Sprintf("Book %d", i)}},
		})
	}
	recent := sampleRecentlyAddedItems()[:2]

	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: recent})
	view := m.View()
	if strings.Contains(view, "8 items") {
		t.Fatalf("expected synthetic rows to stay out of item count\n%s", view)
	}
}

func TestJKey_MovesSelection(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	m, _ = m.Update(PersonalizedMsg{Items: sampleItems(), RecentlyAdded: sampleRecentlyAddedItems()})

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
	m, _ = m.Update(PersonalizedMsg{Items: sampleItems(), RecentlyAdded: sampleRecentlyAddedItems()})

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
	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: sampleRecentlyAddedItems()})

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
	li := listItem{kind: rowKindItem, item: items[0]}
	desc := li.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}

	// Item without progress/duration
	li2 := listItem{kind: rowKindItem, item: items[1]}
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
	li := listItem{kind: rowKindItem, item: item}
	desc := li.Description()
	if desc == "" {
		t.Error("expected non-empty description for item without author")
	}
}

func TestItemDescription_OmitsSeriesInfoWhenPresent(t *testing.T) {
	item := sampleItems()[0]
	item.Media.Metadata.Series = &abs.SeriesSequence{
		ID:       "series-expanse",
		Name:     "The Expanse",
		Sequence: "2",
	}

	got := itemDescription(item)
	if strings.Contains(got, "The Expanse") {
		t.Fatalf("expected series info to be omitted, got %q", got)
	}
}

func TestInit_NilClient(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected a command from Init")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected BatchMsg, got %T", msg)
	}
	var sawPersonalized bool
	for _, cmd := range batch {
		if cmd == nil {
			continue
		}
		if pm, ok := cmd().(PersonalizedMsg); ok {
			sawPersonalized = true
			if pm.Err == nil {
				t.Error("expected error when client is nil")
			}
		}
	}
	if !sawPersonalized {
		t.Fatal("expected batch to contain PersonalizedMsg command")
	}
}

func TestPersonalizedMsg_DedupesRecentlyAddedByTitle(t *testing.T) {
	m := newTestModel()
	items := sampleItems()
	recent := append([]abs.LibraryItem{
		{
			ID:        "li-999",
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title: "The Great Adventure",
				},
			},
		},
	}, sampleRecentlyAddedItems()...)

	m, _ = m.Update(PersonalizedMsg{Items: items, RecentlyAdded: recent})

	if len(m.RecentlyAdded()) != 3 {
		t.Fatalf("expected 3 recently added items after dedupe/cap, got %d", len(m.RecentlyAdded()))
	}
	for _, item := range m.RecentlyAdded() {
		if item.Media.Metadata.Title == "The Great Adventure" {
			t.Fatal("expected duplicate title to be excluded from recently added")
		}
	}
}

func TestHydrateRecentlyAddedPodcastsUsesLatestEpisode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/pod-1" || r.URL.Query().Get("expanded") != "1" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(abs.LibraryItem{
			ID:        "pod-1",
			LibraryID: "lib-pod",
			MediaType: "podcast",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{Title: "Podcast Show"},
				Episodes: []abs.PodcastEpisode{
					{ID: "ep-old", Title: "Older", AddedAt: 10, PublishedAt: 10, Index: 1, Duration: 1200},
					{ID: "ep-new", Title: "Newer", AddedAt: 20, PublishedAt: 20, Index: 2, Duration: 1800},
				},
			},
		})
	}))
	defer srv.Close()

	client := abs.NewClient(srv.URL, "tok")
	items := []abs.LibraryItem{{
		ID:        "pod-1",
		MediaType: "podcast",
		Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Podcast Show"}},
	}}

	hydrated := hydrateRecentlyAddedPodcasts(context.Background(), client, items)
	if hydrated[0].RecentEpisode == nil {
		t.Fatal("expected recent episode to be hydrated")
	}
	if hydrated[0].RecentEpisode.ID != "ep-new" {
		t.Fatalf("expected newest episode to be selected, got %q", hydrated[0].RecentEpisode.ID)
	}
	if itemTitle(hydrated[0]) != "Newer" {
		t.Fatalf("expected item title to use hydrated episode title, got %q", itemTitle(hydrated[0]))
	}
}
