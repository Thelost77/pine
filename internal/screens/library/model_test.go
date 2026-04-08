package library

import (
	"fmt"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	return New(ui.DefaultStyles(), abs.NewClient("http://test", "tok"), "lib-001", nil)
}

func newTestModelNoClient() Model {
	return New(ui.DefaultStyles(), nil, "lib-001", nil)
}

func ptrString(s string) *string  { return &s }
func ptrFloat(f float64) *float64 { return &f }

func makeItems(n int) []abs.LibraryItem {
	items := make([]abs.LibraryItem, n)
	for i := range items {
		items[i] = abs.LibraryItem{
			ID:        fmt.Sprintf("item-%d", i),
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:      fmt.Sprintf("Book %d", i),
					AuthorName: ptrString(fmt.Sprintf("Author %d", i)),
					Duration:   ptrFloat(3600 * float64(i+1)),
				},
			},
		}
	}
	return items
}

func TestInitReturnsCommand(t *testing.T) {
	m := newTestModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil command")
	}
}

func TestInitNoClientReturnsError(t *testing.T) {
	m := newTestModelNoClient()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a command even without client")
	}
	msg := cmd()
	lm, ok := msg.(LibraryItemsMsg)
	if !ok {
		t.Fatalf("expected LibraryItemsMsg, got %T", msg)
	}
	if lm.Err == nil {
		t.Error("expected error for nil client")
	}
}

func TestLibraryItemsMsgPopulatesList(t *testing.T) {
	m := newTestModel()
	m.loading = true

	items := makeItems(3)
	msg := LibraryItemsMsg{Items: items, Total: 10, Page: 0}

	m, cmd := m.Update(msg)

	if m.Loading() {
		t.Error("loading should be false after receiving items")
	}
	if len(m.Items()) != 3 {
		t.Errorf("expected 3 items, got %d", len(m.Items()))
	}
	if m.TotalItems() != 10 {
		t.Errorf("expected total 10, got %d", m.TotalItems())
	}
	if m.Page() != 0 {
		t.Errorf("expected page 0, got %d", m.Page())
	}
	if cmd != nil {
		t.Fatal("expected no follow-up command after loading items")
	}
}

func TestLibraryItemsMsgAppendsOnNextPage(t *testing.T) {
	m := newTestModel()

	// First page
	first := makeItems(3)
	m, _ = m.Update(LibraryItemsMsg{Items: first, Total: 6, Page: 0})

	// Second page
	second := makeItems(3)
	m, _ = m.Update(LibraryItemsMsg{Items: second, Total: 6, Page: 1})

	if len(m.Items()) != 6 {
		t.Errorf("expected 6 items after two pages, got %d", len(m.Items()))
	}
	if m.Page() != 1 {
		t.Errorf("expected page 1, got %d", m.Page())
	}
}

func TestLibraryItemsMsgError(t *testing.T) {
	m := newTestModel()
	m.loading = true

	msg := LibraryItemsMsg{Err: fmt.Errorf("network error")}
	m, _ = m.Update(msg)

	if m.Loading() {
		t.Error("loading should be false after error")
	}
	if m.Error() == nil {
		t.Error("error should be set")
	}
}

func TestEnterEmitsNavigateDetailMsg(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)

	items := makeItems(3)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 3, Page: 0})

	// Press enter on the selected (first) item
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)

	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	nav, ok := msg.(NavigateDetailMsg)
	if !ok {
		t.Fatalf("expected NavigateDetailMsg, got %T", msg)
	}
	if nav.Item.ID != items[0].ID {
		t.Errorf("expected item ID %s, got %s", items[0].ID, nav.Item.ID)
	}
}

func TestSlashEmitsNavigateSearchMsg(t *testing.T) {
	libs := []abs.Library{{ID: "lib-001", Name: "Books", MediaType: "book"}}
	m := New(ui.DefaultStyles(), abs.NewClient("http://test", "tok"), "lib-001", libs)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if cmd == nil {
		t.Fatal("slash should produce a command")
	}
	msg := cmd()
	searchMsg, ok := msg.(NavigateSearchMsg)
	if !ok {
		t.Fatalf("expected NavigateSearchMsg, got %T", msg)
	}
	if searchMsg.LibraryID != "lib-001" {
		t.Fatalf("library ID = %q, want %q", searchMsg.LibraryID, "lib-001")
	}
	if searchMsg.LibraryMediaType != "book" {
		t.Fatalf("library media type = %q, want %q", searchMsg.LibraryMediaType, "book")
	}
}

func TestSKeyEmitsNavigateSeriesListMsg(t *testing.T) {
	libs := []abs.Library{{ID: "lib-001", Name: "Books", MediaType: "book"}}
	m := New(ui.DefaultStyles(), abs.NewClient("http://test", "tok"), "lib-001", libs)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("s should produce a command")
	}
	msg := cmd()
	navMsg, ok := msg.(NavigateSeriesListMsg)
	if !ok {
		t.Fatalf("expected NavigateSeriesListMsg, got %T", msg)
	}
	if navMsg.LibraryID != "lib-001" {
		t.Fatalf("library ID = %q, want lib-001", navMsg.LibraryID)
	}
	if navMsg.LibraryName != "Books" {
		t.Fatalf("library name = %q, want Books", navMsg.LibraryName)
	}
}

func TestViewShowsLoadingWhenEmpty(t *testing.T) {
	m := newTestModel()
	m.loading = true

	view := m.View()
	if !containsString(view, "Loading") {
		t.Error("View() should show loading message when loading with no items")
	}
}

func TestViewShowsErrorMessage(t *testing.T) {
	m := newTestModel()
	m.err = fmt.Errorf("test error")

	view := m.View()
	if !containsString(view, "test error") {
		t.Error("View() should show error message")
	}
}

func TestViewShowsListAfterItems(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	items := makeItems(2)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 2, Page: 0})

	view := m.View()
	if !containsString(view, "Library") {
		t.Error("View() should contain 'Library' title")
	}
	if !containsString(view, "Book 0") {
		t.Error("View() should contain item title")
	}
}

func TestLibraryListItemDescription(t *testing.T) {
	item := libraryListItem{Item: abs.LibraryItem{
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:      "Test",
				AuthorName: ptrString("Jane Doe"),
				Duration:   ptrFloat(7200),
			},
		},
	}}

	desc := item.Description()
	if !containsString(desc, "Jane Doe") {
		t.Errorf("description should contain author, got %q", desc)
	}
	if !containsString(desc, "2h 0m") {
		t.Errorf("description should contain duration, got %q", desc)
	}
}

func TestLibraryListItemDescriptionOmitsSeriesInfo(t *testing.T) {
	item := libraryListItem{Item: abs.LibraryItem{
		MediaType: "book",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:      "Test",
				AuthorName: ptrString("Jane Doe"),
				Duration:   ptrFloat(7200),
				Series: &abs.SeriesSequence{
					ID:       "series-expanse",
					Name:     "The Expanse",
					Sequence: "2",
				},
			},
		},
	}}

	desc := item.Description()
	if containsString(desc, "The Expanse") {
		t.Errorf("description should omit series info, got %q", desc)
	}
}

func TestLibraryListItemDescriptionUnknownAuthor(t *testing.T) {
	item := libraryListItem{Item: abs.LibraryItem{
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Test"},
		},
	}}

	desc := item.Description()
	if !containsString(desc, "Unknown author") {
		t.Errorf("description should show 'Unknown author', got %q", desc)
	}
}

func TestLibraryListItemFilterValue(t *testing.T) {
	item := libraryListItem{Item: abs.LibraryItem{
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "My Book"},
		},
	}}
	if item.FilterValue() != "My Book" {
		t.Errorf("FilterValue() = %q, want 'My Book'", item.FilterValue())
	}
}

func TestSetSize(t *testing.T) {
	m := newTestModel()
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", m.width, m.height)
	}
}

func TestMaybePrefetchTriggersAtThreshold(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)

	// Load 10 items with total of 20
	items := makeItems(10)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 20, Page: 0})

	// Move cursor to 80% (index 8) by updating the list
	for i := 0; i < 8; i++ {
		m.list, _ = m.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// maybePrefetch should trigger since cursor >= 80% of loaded
	cmd := m.maybePrefetch()
	if cmd == nil {
		t.Error("maybePrefetch should return a command when cursor >= 80%")
	}
}

func TestMaybePrefetchDoesNotTriggerWhenAllLoaded(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)

	items := makeItems(5)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 5, Page: 0})

	// Move cursor near end
	for i := 0; i < 4; i++ {
		m.list, _ = m.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	cmd := m.maybePrefetch()
	if cmd != nil {
		t.Error("maybePrefetch should not trigger when all items are loaded")
	}
}

func TestMaybePrefetchDoesNotTriggerWhileLoading(t *testing.T) {
	m := newTestModel()
	m.loading = true
	m.items = makeItems(10)
	m.totalItems = 20

	cmd := m.maybePrefetch()
	if cmd != nil {
		t.Error("maybePrefetch should not trigger while already loading")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		secs float64
		want string
	}{
		{0, "0m"},
		{300, "5m"},
		{3600, "1h 0m"},
		{5400, "1h 30m"},
		{7261, "2h 1m"},
	}
	for _, tt := range tests {
		got := ui.FormatDuration(tt.secs)
		if got != tt.want {
			t.Errorf("ui.FormatDuration(%v) = %q, want %q", tt.secs, got, tt.want)
		}
	}
}

func TestViewShowsLoadingOnlyWhenNoItems(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)
	m.loading = true

	// With items, should NOT show loading text
	items := makeItems(2)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 10, Page: 0})
	m.loading = true // simulate loading next page

	view := m.View()
	if containsString(view, "Loading library") {
		t.Error("View() should not show loading message when items are already displayed")
	}
}

// Make list.Filtering accessible for test coverage
func TestUpdateDelegatesListKeys(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)

	items := makeItems(5)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 5, Page: 0})

	// j key should move cursor down
	before := m.list.Index()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	after := m.list.Index()

	if after != before+1 {
		t.Errorf("j key should move cursor from %d to %d, got %d", before, before+1, after)
	}
}

// TestScrollNearBottomTriggersPrefetchViaUpdate verifies that scrolling near the
// bottom of a partially-loaded list through Update() emits a fetch command.
func TestScrollNearBottomTriggersPrefetchViaUpdate(t *testing.T) {
	m := newTestModel() // real client so fetchLibraryItemsCmd sets loading=true
	m.SetSize(80, 24)

	// Load 10 items with 20 total — only half loaded
	items := makeItems(10)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 20, Page: 0})

	// Scroll down to index 8 (≥ 80% of 10) via Update()
	var cmd tea.Cmd
	for i := 0; i < 8; i++ {
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// The last Update that crosses the threshold should return a batch cmd
	if cmd == nil {
		t.Fatal("expected a command when scrolling past 80% threshold")
	}

	// Model should be in loading state after prefetch triggered
	if !m.Loading() {
		t.Error("expected loading=true after prefetch triggered")
	}
}

// TestScrollNearBottomFetchProducesLibraryItemsMsg verifies the fetch command
// produces a LibraryItemsMsg when executed (using nil client → error path).
func TestScrollNearBottomFetchProducesLibraryItemsMsg(t *testing.T) {
	m := newTestModelNoClient()
	m.SetSize(80, 24)

	items := makeItems(10)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 20, Page: 0})

	// Scroll to threshold
	for i := 0; i < 8; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Call maybePrefetch to get the fetch command
	cmd := m.maybePrefetch()
	if cmd == nil {
		// Already loading from the Update loop — reset and test directly
		m.loading = false
		cmd = m.maybePrefetch()
	}
	if cmd == nil {
		t.Fatal("expected a prefetch command")
	}

	// Execute the command — nil client returns LibraryItemsMsg with error
	msg := cmd()
	lm, ok := msg.(LibraryItemsMsg)
	if !ok {
		t.Fatalf("expected LibraryItemsMsg, got %T", msg)
	}
	if lm.Err == nil {
		t.Error("expected error from nil-client fetch")
	}
}

// TestEnterOnScrolledItemNavigatesCorrectItem verifies that pressing enter
// after scrolling to a specific item navigates to that item.
func TestEnterOnScrolledItemNavigatesCorrectItem(t *testing.T) {
	m := newTestModel()
	m.SetSize(80, 24)

	items := makeItems(5)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 5, Page: 0})

	// Scroll to the third item (index 2)
	for i := 0; i < 2; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Press enter
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from enter key")
	}

	msg := cmd()
	nav, ok := msg.(NavigateDetailMsg)
	if !ok {
		t.Fatalf("expected NavigateDetailMsg, got %T", msg)
	}
	if nav.Item.ID != items[2].ID {
		t.Errorf("expected item ID %s, got %s", items[2].ID, nav.Item.ID)
	}
}

// TestNoPrefetchWhenBelowThreshold verifies no fetch fires when cursor is
// below the 80% threshold during Update().
func TestNoPrefetchWhenBelowThreshold(t *testing.T) {
	m := newTestModelNoClient()
	m.SetSize(80, 24)

	items := makeItems(10)
	m, _ = m.Update(LibraryItemsMsg{Items: items, Total: 20, Page: 0})

	// Scroll to index 5 (50% < 80% threshold)
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	if m.Loading() {
		t.Error("should not be loading when cursor is below threshold")
	}
}

func TestTabSwitchUsesCachedLibraryItems(t *testing.T) {
	libs := []abs.Library{
		{ID: "lib-001", Name: "Books", MediaType: "book"},
		{ID: "lib-002", Name: "Podcasts", MediaType: "podcast"},
	}
	m := New(ui.DefaultStyles(), abs.NewClient("http://test", "tok"), "lib-001", libs)
	m.SetSize(80, 24)

	firstItems := makeItems(2)
	m, _ = m.Update(LibraryItemsMsg{
		Items:     firstItems,
		Total:     2,
		Page:      0,
		LibraryID: "lib-001",
	})

	secondItems := makeItems(2)
	secondItems[0].ID = "pod-1"
	secondItems[0].Media.Metadata.Title = "Podcast 1"
	secondItems[1].ID = "pod-2"
	secondItems[1].Media.Metadata.Title = "Podcast 2"

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("expected fetch command on first switch to uncached library")
	}
	if m.libraryID != "lib-002" {
		t.Fatalf("libraryID = %q, want lib-002", m.libraryID)
	}
	if !m.Loading() {
		t.Fatal("expected loading=true while fetching uncached library")
	}
	if len(m.Items()) != 0 {
		t.Fatalf("expected items to be cleared for uncached library loading, got %d", len(m.Items()))
	}
	if !containsString(m.View(), "Loading library") {
		t.Fatal("expected loading view when switching to uncached library")
	}

	m, _ = m.Update(LibraryItemsMsg{
		Items:     secondItems,
		Total:     2,
		Page:      0,
		LibraryID: "lib-002",
	})

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatal("expected no fetch command when switching to cached library")
	}
	if m.libraryID != "lib-001" {
		t.Fatalf("libraryID = %q, want lib-001", m.libraryID)
	}
	if len(m.Items()) != len(firstItems) {
		t.Fatalf("items length = %d, want %d", len(m.Items()), len(firstItems))
	}
	if m.Items()[0].ID != firstItems[0].ID {
		t.Fatalf("first cached item ID = %q, want %q", m.Items()[0].ID, firstItems[0].ID)
	}
}

func TestConfigureUsesCachedLibraryItems(t *testing.T) {
	libs := []abs.Library{
		{ID: "lib-001", Name: "Books", MediaType: "book"},
		{ID: "lib-002", Name: "Podcasts", MediaType: "podcast"},
	}
	m := New(ui.DefaultStyles(), abs.NewClient("http://test", "tok"), "lib-001", libs)
	firstItems := makeItems(2)
	secondItems := makeItems(2)
	secondItems[0].ID = "pod-1"
	secondItems[1].ID = "pod-2"

	m, _ = m.Update(LibraryItemsMsg{
		Items:     firstItems,
		Total:     2,
		Page:      0,
		LibraryID: "lib-001",
	})
	m.Configure("lib-002", libs)
	if !m.Loading() {
		t.Fatal("expected loading=true for uncached configured library")
	}

	m, _ = m.Update(LibraryItemsMsg{
		Items:     secondItems,
		Total:     2,
		Page:      0,
		LibraryID: "lib-002",
	})

	m.Configure("lib-001", libs)
	if m.Loading() {
		t.Fatal("expected loading=false when configuring cached library")
	}
	if len(m.Items()) != len(firstItems) {
		t.Fatalf("items length = %d, want %d", len(m.Items()), len(firstItems))
	}
	if m.Items()[0].ID != firstItems[0].ID {
		t.Fatalf("first cached item ID = %q, want %q", m.Items()[0].ID, firstItems[0].ID)
	}
}

func TestInitSkipsFetchWhenItemsAlreadyLoaded(t *testing.T) {
	m := newTestModel()
	m.items = makeItems(2)
	m.totalItems = 2

	if cmd := m.Init(); cmd != nil {
		t.Fatal("expected no init command when items are already loaded")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
