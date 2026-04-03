package detail

import (
	"errors"
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func sampleItem() abs.LibraryItem {
	author := "Jane Author"
	dur := 36000.0
	desc := "A thrilling adventure through time and space. " +
		"Follow our protagonist as they navigate the challenges of a world beyond imagination."
	return abs.LibraryItem{
		ID:        "li-001",
		MediaType: "book",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:       "The Great Adventure",
				AuthorName:  &author,
				Duration:    &dur,
				Description: &desc,
				Chapters: []abs.Chapter{
					{ID: 0, Start: 0, End: 3600, Title: "The Beginning"},
					{ID: 1, Start: 3600, End: 7200, Title: "The Journey"},
					{ID: 2, Start: 7200, End: 10800, Title: "The Return"},
				},
			},
		},
		UserMediaProgress: &abs.UserMediaProgress{
			CurrentTime: 16200,
			Progress:    0.45,
			IsFinished:  false,
		},
	}
}

func newTestModel() Model {
	styles := ui.DefaultStyles()
	item := sampleItem()
	m := New(styles, item)
	m.SetSize(80, 24)
	return m
}

func TestNew(t *testing.T) {
	styles := ui.DefaultStyles()
	item := sampleItem()
	m := New(styles, item)

	if m.Item().ID != "li-001" {
		t.Errorf("expected item ID li-001, got %s", m.Item().ID)
	}
	if m.Item().Media.Metadata.Title != "The Great Adventure" {
		t.Errorf("expected title 'The Great Adventure', got %s", m.Item().Media.Metadata.Title)
	}
}

func TestView_ShowsTitle(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if !strings.Contains(v, "The Great Adventure") {
		t.Error("expected view to contain book title")
	}
}

func TestView_ShowsAuthor(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if !strings.Contains(v, "Jane Author") {
		t.Error("expected view to contain author name")
	}
}

func TestView_ShowsDuration(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if !strings.Contains(v, "10h 0m") {
		t.Error("expected view to contain duration '10h 0m'")
	}
}

func TestView_ShowsProgressBar(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if !strings.Contains(v, "45%") {
		t.Error("expected view to contain '45%' progress")
	}
	if !strings.Contains(v, "█") {
		t.Error("expected view to contain filled progress bar characters")
	}
}

func TestView_ShowsDescription(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if !strings.Contains(v, "thrilling adventure") {
		t.Error("expected view to contain description text")
	}
}

func TestView_DoesNotShowInlineChapters(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if strings.Contains(v, "Chapters") {
		t.Error("expected view not to contain inline chapters section")
	}
	if strings.Contains(v, "The Beginning") || strings.Contains(v, "The Journey") || strings.Contains(v, "The Return") {
		t.Error("expected view not to contain inline chapter titles")
	}
	if strings.Contains(v, "Chapter 2/3") {
		t.Error("expected view not to contain chapter progress text derived from metadata chapters")
	}
}

func TestView_ShowsHelpHints(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if !strings.Contains(v, "p play") {
		t.Error("expected view to contain help hint for play")
	}
}

func TestPKey_FiresPlayCmd(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("expected a command from p key")
	}
	msg := cmd()
	playMsg, ok := msg.(PlayCmd)
	if !ok {
		t.Fatalf("expected PlayCmd, got %T", msg)
	}
	if playMsg.Item.ID != "li-001" {
		t.Errorf("expected item ID li-001, got %s", playMsg.Item.ID)
	}
}

func TestBKey_FiresBookmarkCmd(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatal("expected a command from b key")
	}
	msg := cmd()
	bmMsg, ok := msg.(AddBookmarkCmd)
	if !ok {
		t.Fatalf("expected AddBookmarkCmd, got %T", msg)
	}
	if bmMsg.Item.ID != "li-001" {
		t.Errorf("expected item ID li-001, got %s", bmMsg.Item.ID)
	}
}

func TestView_NoAuthor(t *testing.T) {
	styles := ui.DefaultStyles()
	item := abs.LibraryItem{
		ID: "li-002",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title: "Mystery Book",
			},
		},
	}
	m := New(styles, item)
	m.SetSize(80, 24)
	v := m.View()
	if !strings.Contains(v, "Unknown author") {
		t.Error("expected 'Unknown author' when no author set")
	}
}

func TestView_NoProgress(t *testing.T) {
	styles := ui.DefaultStyles()
	item := abs.LibraryItem{
		ID: "li-003",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title: "Fresh Book",
			},
		},
	}
	m := New(styles, item)
	m.SetSize(80, 24)
	v := m.View()
	// Should not contain progress bar characters
	if strings.Contains(v, "█") {
		t.Error("expected no progress bar when no progress data")
	}
}

func TestView_DescriptionScrolls(t *testing.T) {
	styles := ui.DefaultStyles()
	item := sampleItem()
	// Set very long description to ensure scrollability
	longDesc := strings.Repeat("This is a long description sentence. ", 100)
	item.Media.Metadata.Description = &longDesc
	m := New(styles, item)
	m.SetSize(80, 10) // Small height to trigger scrolling
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view with long description")
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
	}
	for _, tt := range tests {
		got := ui.FormatDuration(tt.seconds)
		if got != tt.expected {
			t.Errorf("ui.FormatDuration(%v) = %q, want %q", tt.seconds, got, tt.expected)
		}
	}
}

func TestWordWrap(t *testing.T) {
	text := "This is a test of the word wrapping function that should break lines"
	wrapped := wordWrap(text, 20)
	lines := strings.Split(wrapped, "\n")
	for _, line := range lines {
		if len(line) > 20 {
			t.Errorf("line exceeds width 20: %q (len=%d)", line, len(line))
		}
	}
}

func TestEscKey_EmitsBackMsg(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a command from esc key")
	}
	msg := cmd()
	_, ok := msg.(BackMsg)
	if !ok {
		t.Fatalf("expected BackMsg, got %T", msg)
	}
}

func TestJKey_ScrollsViewportDown(t *testing.T) {
	styles := ui.DefaultStyles()
	item := sampleItem()
	longDesc := strings.Repeat("This is a long description sentence. ", 100)
	item.Media.Metadata.Description = &longDesc
	m := New(styles, item)
	m.SetSize(80, 10) // Small height to force scrollable content

	initialOffset := m.viewport.YOffset

	// Send 'j' key to scroll down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if m.viewport.YOffset <= initialOffset {
		t.Errorf("expected viewport to scroll down: offset before=%d, after=%d", initialOffset, m.viewport.YOffset)
	}
}

func TestKKey_ScrollsViewportUp(t *testing.T) {
	styles := ui.DefaultStyles()
	item := sampleItem()
	longDesc := strings.Repeat("This is a long description sentence. ", 100)
	item.Media.Metadata.Description = &longDesc
	m := New(styles, item)
	m.SetSize(80, 10) // Small height to force scrollable content

	// Scroll down first
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	afterDown := m.viewport.YOffset
	if afterDown == 0 {
		t.Fatal("expected viewport to have scrolled down before testing k")
	}

	// Send 'k' key to scroll up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	if m.viewport.YOffset >= afterDown {
		t.Errorf("expected viewport to scroll up: offset before=%d, after=%d", afterDown, m.viewport.YOffset)
	}
}

func TestSetSize(t *testing.T) {
	styles := ui.DefaultStyles()
	item := sampleItem()
	m := New(styles, item)

	// Initially not ready
	if m.ready {
		t.Error("expected ready=false before SetSize")
	}

	m.SetSize(80, 24)
	if !m.ready {
		t.Error("expected ready=true after SetSize")
	}
	if m.width != 80 || m.height != 24 {
		t.Errorf("expected size 80x24, got %dx%d", m.width, m.height)
	}
}

// --- Bookmark tests ---

func sampleBookmarks() []abs.Bookmark {
	return []abs.Bookmark{
		{Title: "Start of journey", Time: 3600, CreatedAt: 1000000},
		{Title: "Key moment", Time: 7200, CreatedAt: 1000001},
		{Title: "Climax", Time: 9000, CreatedAt: 1000002},
	}
}

func newTestModelWithBookmarks() Model {
	m := newTestModel()
	m.SetBookmarks(sampleBookmarks())
	return m
}

func TestView_ShowsBookmarks(t *testing.T) {
	m := newTestModelWithBookmarks()
	v := m.View()
	if !strings.Contains(v, "Bookmarks") {
		t.Error("expected view to contain 'Bookmarks' section")
	}
	if !strings.Contains(v, "Start of journey") {
		t.Error("expected view to contain bookmark title 'Start of journey'")
	}
	if !strings.Contains(v, "Key moment") {
		t.Error("expected view to contain bookmark title 'Key moment'")
	}
	if !strings.Contains(v, "Climax") {
		t.Error("expected view to contain bookmark title 'Climax'")
	}
}

func TestView_ShowsBookmarkTimestamps(t *testing.T) {
	m := newTestModelWithBookmarks()
	v := m.View()
	if !strings.Contains(v, "1:00:00") {
		t.Error("expected view to contain timestamp '1:00:00' for 3600s")
	}
	if !strings.Contains(v, "2:00:00") {
		t.Error("expected view to contain timestamp '2:00:00' for 7200s")
	}
}

func TestView_NoBookmarks(t *testing.T) {
	m := newTestModel()
	v := m.View()
	if strings.Contains(v, "Bookmarks") {
		t.Error("expected no 'Bookmarks' section before bookmarks finish loading")
	}
}

func TestView_ShowsEmptyBookmarkStateAfterLoadedEmpty(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(BookmarksUpdatedMsg{Bookmarks: []abs.Bookmark{}})

	v := m.View()
	if !strings.Contains(v, "Bookmarks") {
		t.Fatal("expected bookmark section when bookmarks load empty")
	}
	if !strings.Contains(v, "No bookmarks yet") {
		t.Error("expected empty bookmark state text")
	}
}

func TestView_ShowsBookmarkLoadError(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(BookmarksUpdatedMsg{Err: errors.New("bookmark load failed")})

	v := m.View()
	if !strings.Contains(v, "Bookmarks") {
		t.Fatal("expected bookmark section when bookmark load fails")
	}
	if !strings.Contains(v, "bookmark load failed") {
		t.Error("expected bookmark load error text")
	}
}

func TestTabKey_TogglesFocusBookmarks(t *testing.T) {
	m := newTestModelWithBookmarks()
	if m.FocusBookmarks() {
		t.Error("expected focusBookmarks=false initially")
	}

	// First tab focuses bookmarks for books.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.FocusBookmarks() {
		t.Error("expected focusBookmarks=true after first tab")
	}

	// Second tab unfocuses all
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusBookmarks() {
		t.Error("expected focusBookmarks=false after second tab")
	}
}

func TestTabKey_NoToggleWithoutBookmarks(t *testing.T) {
	m := newTestModel() // no bookmarks
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusBookmarks() {
		t.Error("expected focusBookmarks=false when no bookmarks")
	}
}

func TestJKKeys_NavigateBookmarks(t *testing.T) {
	m := newTestModelWithBookmarks()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // bookmarks
	if m.SelectedBookmark() != 0 {
		t.Errorf("expected selectedBookmark=0, got %d", m.SelectedBookmark())
	}

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.SelectedBookmark() != 1 {
		t.Errorf("expected selectedBookmark=1 after j, got %d", m.SelectedBookmark())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.SelectedBookmark() != 2 {
		t.Errorf("expected selectedBookmark=2 after second j, got %d", m.SelectedBookmark())
	}

	// Should not go past last bookmark
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.SelectedBookmark() != 2 {
		t.Errorf("expected selectedBookmark=2 (clamped), got %d", m.SelectedBookmark())
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.SelectedBookmark() != 1 {
		t.Errorf("expected selectedBookmark=1 after k, got %d", m.SelectedBookmark())
	}
}

func TestEnterKey_SeeksToBookmark(t *testing.T) {
	m := newTestModelWithBookmarks()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus bookmarks

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from enter key on focused bookmark")
	}
	msg := cmd()
	seekMsg, ok := msg.(SeekToBookmarkCmd)
	if !ok {
		t.Fatalf("expected SeekToBookmarkCmd, got %T", msg)
	}
	if seekMsg.Time != 3600 {
		t.Errorf("expected seek time 3600, got %f", seekMsg.Time)
	}
}

func TestEnterKey_NoActionWithoutFocus(t *testing.T) {
	m := newTestModelWithBookmarks()
	// focusBookmarks is false
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		// The viewport may produce a cmd from Enter; check it's not a SeekToBookmarkCmd
		msg := cmd()
		if _, ok := msg.(SeekToBookmarkCmd); ok {
			t.Error("expected no SeekToBookmarkCmd when bookmarks not focused")
		}
	}
	_ = m
}

func TestDKey_DeletesBookmark(t *testing.T) {
	m := newTestModelWithBookmarks()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus bookmarks
	// Move to second bookmark
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("expected a command from d key on focused bookmark")
	}
	msg := cmd()
	delMsg, ok := msg.(DeleteBookmarkCmd)
	if !ok {
		t.Fatalf("expected DeleteBookmarkCmd, got %T", msg)
	}
	if delMsg.ItemID != "li-001" {
		t.Errorf("expected item ID li-001, got %s", delMsg.ItemID)
	}
	if delMsg.Bookmark.Time != 7200 {
		t.Errorf("expected bookmark time 7200, got %f", delMsg.Bookmark.Time)
	}
	if delMsg.Bookmark.Title != "Key moment" {
		t.Errorf("expected bookmark title 'Key moment', got %s", delMsg.Bookmark.Title)
	}
}

func TestDKey_NoActionWithoutFocus(t *testing.T) {
	m := newTestModelWithBookmarks()
	// focusBookmarks is false
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(DeleteBookmarkCmd); ok {
			t.Error("expected no DeleteBookmarkCmd when bookmarks not focused")
		}
	}
}

func TestBookmarksUpdatedMsg_UpdatesList(t *testing.T) {
	m := newTestModel()
	newBookmarks := []abs.Bookmark{
		{Title: "New bookmark", Time: 500, CreatedAt: 2000000},
	}
	m, _ = m.Update(BookmarksUpdatedMsg{Bookmarks: newBookmarks})

	if len(m.Bookmarks()) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(m.Bookmarks()))
	}
	if m.Bookmarks()[0].Title != "New bookmark" {
		t.Errorf("expected 'New bookmark', got %s", m.Bookmarks()[0].Title)
	}
}

func TestBookmarksUpdatedMsg_ClampsSelection(t *testing.T) {
	m := newTestModelWithBookmarks()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus bookmarks
	// Select last bookmark (index 2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Remove all but one bookmark
	m, _ = m.Update(BookmarksUpdatedMsg{Bookmarks: []abs.Bookmark{
		{Title: "Only one", Time: 100},
	}})
	if m.SelectedBookmark() != 0 {
		t.Errorf("expected selectedBookmark clamped to 0, got %d", m.SelectedBookmark())
	}
}

func TestBookmarksUpdatedMsg_EmptyDisablesFocus(t *testing.T) {
	m := newTestModelWithBookmarks()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus bookmarks
	if !m.FocusBookmarks() {
		t.Fatal("expected focusBookmarks=true")
	}

	m, _ = m.Update(BookmarksUpdatedMsg{Bookmarks: nil})
	if m.FocusBookmarks() {
		t.Error("expected focusBookmarks=false after empty bookmarks update")
	}
}

func TestBookmarksUpdatedMsg_EmptyMarksBookmarksLoaded(t *testing.T) {
	m := newTestModel()

	if m.BookmarksLoaded() {
		t.Fatal("expected bookmarks to start unloaded")
	}

	m, _ = m.Update(BookmarksUpdatedMsg{Bookmarks: []abs.Bookmark{}})

	if !m.BookmarksLoaded() {
		t.Error("expected bookmarks to be marked loaded after empty update")
	}
	if m.BookmarkLoadError() != nil {
		t.Errorf("expected no bookmark load error, got %v", m.BookmarkLoadError())
	}
}

func TestBookmarksUpdatedMsg_ErrorStoresBookmarkLoadError(t *testing.T) {
	m := newTestModel()
	wantErr := errors.New("bookmark load failed")

	m, _ = m.Update(BookmarksUpdatedMsg{Err: wantErr})

	if !m.BookmarksLoaded() {
		t.Error("expected bookmarks to be marked loaded after error")
	}
	if m.BookmarkLoadError() == nil {
		t.Fatal("expected bookmark load error to be stored")
	}
	if m.BookmarkLoadError().Error() != wantErr.Error() {
		t.Errorf("bookmark load error = %v, want %v", m.BookmarkLoadError(), wantErr)
	}
}

func TestSetBookmarks_ClearsBookmarkLoadError(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(BookmarksUpdatedMsg{Err: errors.New("bookmark load failed")})

	m.SetBookmarks(sampleBookmarks())

	if !m.BookmarksLoaded() {
		t.Error("expected bookmarks to stay marked loaded after SetBookmarks")
	}
	if m.BookmarkLoadError() != nil {
		t.Errorf("expected bookmark load error to be cleared, got %v", m.BookmarkLoadError())
	}
}

func TestSetBookmarks(t *testing.T) {
	m := newTestModel()
	bms := sampleBookmarks()
	m.SetBookmarks(bms)
	if len(m.Bookmarks()) != 3 {
		t.Errorf("expected 3 bookmarks, got %d", len(m.Bookmarks()))
	}
	if !m.BookmarksLoaded() {
		t.Error("expected bookmarks to be marked loaded after SetBookmarks")
	}
}

func TestView_BookmarkFocusHighlights(t *testing.T) {
	m := newTestModelWithBookmarks()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus bookmarks
	v := m.View()
	// When focused, the header should show the focus indicator
	if !strings.Contains(v, "▸ Bookmarks") {
		t.Error("expected focused bookmark header with '▸' indicator")
	}
}

func TestView_HelpTextChangesWithFocus(t *testing.T) {
	m := newTestModelWithBookmarks()
	v := m.View()
	if !strings.Contains(v, "tab focus bookmarks") {
		t.Error("expected help to mention 'tab focus bookmarks' when bookmarks are present")
	}
	if strings.Contains(v, "chapters") {
		t.Error("expected help not to mention chapters for books")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus bookmarks
	v = m.View()
	if !strings.Contains(v, "enter seek") {
		t.Error("expected help to mention 'enter seek' when bookmarks focused")
	}
	if !strings.Contains(v, "d delete") {
		t.Error("expected help to mention 'd delete' when bookmarks focused")
	}
}

func TestPodcastHelp_DoesNotAdvertiseBookmarkFocusWhenBookmarksEmpty(t *testing.T) {
	styles := ui.DefaultStyles()
	desc := "Podcast description"
	item := abs.LibraryItem{
		ID:        "pod-help-001",
		MediaType: "podcast",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:       "Podcast Help",
				Description: &desc,
			},
			Episodes: []abs.PodcastEpisode{{ID: "ep-001", Index: 1, Title: "Episode 1", Duration: 1800}},
		},
	}

	m := New(styles, item)
	m.SetSize(80, 24)
	m, _ = m.Update(BookmarksUpdatedMsg{Bookmarks: []abs.Bookmark{}})

	v := m.View()
	if strings.Contains(v, "episodes/bookmarks") {
		t.Error("expected podcast help not to advertise bookmark focus when bookmark list is empty")
	}
}

func TestPodcastTabKey_CyclesEpisodesAndBookmarks(t *testing.T) {
	styles := ui.DefaultStyles()
	desc := "Podcast description"
	item := abs.LibraryItem{
		ID:        "pod-001",
		MediaType: "podcast",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:       "Podcast Show",
				Description: &desc,
			},
			Episodes: []abs.PodcastEpisode{
				{ID: "ep-1", Title: "Episode One", Duration: 1800},
				{ID: "ep-2", Title: "Episode Two", Duration: 2400},
			},
		},
	}
	m := New(styles, item)
	m.SetSize(80, 24)
	m.SetBookmarks(sampleBookmarks())

	if !m.focusEpisodes {
		t.Fatal("expected podcast episodes to start focused")
	}
	if m.FocusBookmarks() {
		t.Fatal("expected podcast bookmarks to start unfocused")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusEpisodes {
		t.Fatal("expected first tab to leave episode focus")
	}
	if !m.FocusBookmarks() {
		t.Fatal("expected first tab to focus bookmarks")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusEpisodes || m.FocusBookmarks() {
		t.Fatal("expected second tab to clear podcast focus")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.focusEpisodes {
		t.Fatal("expected third tab to return focus to episodes")
	}
	if m.FocusBookmarks() {
		t.Fatal("expected bookmarks to be unfocused when episode focus returns")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusEpisodes {
		t.Fatal("expected fourth tab to leave episode focus again")
	}
	if !m.FocusBookmarks() {
		t.Fatal("expected fourth tab to focus bookmarks again")
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00"},
		{65, "1:05"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{7200, "2:00:00"},
	}
	for _, tt := range tests {
		got := ui.FormatTimestamp(tt.seconds)
		if got != tt.expected {
			t.Errorf("ui.FormatTimestamp(%v) = %q, want %q", tt.seconds, got, tt.expected)
		}
	}
}
