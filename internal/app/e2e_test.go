package app

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/screens/home"
	"github.com/Thelost77/pine/internal/screens/library"
	"github.com/Thelost77/pine/internal/screens/login"
	"github.com/Thelost77/pine/internal/screens/search"
	"github.com/Thelost77/pine/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// E2E: Login → Home transition
// ---------------------------------------------------------------------------

func TestE2E_LoginToHome(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModel(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Should start at login
	assertScreen(t, m, ScreenLogin)

	// Type credentials into the login form
	m = e2eTypeString(m, srv.URL)
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m = e2eTypeString(m, "alice")
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m = e2eTypeString(m, "correct")

	// Press enter to trigger login
	m, cmd := e2ePressSpecial(m, tea.KeyEnter)
	if !m.login.Loading() {
		t.Fatal("expected login loading state after enter")
	}

	// Execute the login command — calls POST /login on mock server
	m, cmd = feedCmd(m, cmd)

	// The login cmd returns LoginSuccessMsg which the login model processes.
	// But we need the root model to see it — the feedCmd fed it back into Update.
	// After LoginSuccessMsg, root model navigates to Home.
	// However, loginCmd is on the login screen, returning LoginSuccessMsg.
	// feedCmd executes cmd() → gets LoginSuccessMsg → feeds to m.Update()
	// Root model handles LoginSuccessMsg → navigates to Home + returns home.Init()
	assertScreen(t, m, ScreenHome)

	if m.client == nil {
		t.Fatal("client should be configured after login")
	}

	// Now execute the Home Init command (fetches personalized)
	m = feedCmdChain(m, cmd, 5)

	// Verify API calls
	assertAPICallMade(t, log, "POST", "/login")
	assertAPICallMade(t, log, "GET", "/api/libraries")
	assertAPICallMade(t, log, "GET", "/api/libraries/lib-001/personalized")

	// Verify items loaded into home
	if len(m.home.Items()) == 0 {
		t.Error("home screen should have items after personalized fetch")
	}
}

// ---------------------------------------------------------------------------
// E2E: Login failure + retry
// ---------------------------------------------------------------------------

func TestE2E_LoginFailure(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	// Use an unreachable server for the first attempt
	m := newE2EModel(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Type an invalid server URL
	m = e2eTypeString(m, "http://127.0.0.1:1")
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m = e2eTypeString(m, "alice")
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m = e2eTypeString(m, "pass")

	// Press enter
	m, cmd := e2ePressSpecial(m, tea.KeyEnter)

	// Execute login command — should fail (unreachable)
	m, _ = feedCmd(m, cmd)

	// Should still be on login with error
	assertScreen(t, m, ScreenLogin)
	if m.login.Error() == nil {
		t.Error("expected login error after failed attempt")
	}
}

// ---------------------------------------------------------------------------
// E2E: Home → Library navigation
// ---------------------------------------------------------------------------

func TestE2E_HomeToLibrary(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	assertScreen(t, m, ScreenHome)

	// Load home content first
	cmd := m.home.Init()
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 5)

	// Press 'o' to navigate to library
	// Home screen returns a cmd that produces NavigateLibraryMsg, which root model handles.
	m, cmd = e2ePressKey(m, 'o')
	m, cmd = feedCmd(m, cmd)

	assertScreen(t, m, ScreenLibrary)
	assertBackStack(t, m, []Screen{ScreenHome})

	// Execute library Init (fetches library items)
	m = feedCmdChain(m, cmd, 5)

	assertAPICallMade(t, log, "GET", "/api/libraries/lib-001/items")

	if len(m.library.Items()) == 0 {
		t.Error("library should have items after fetch")
	}
}

// ---------------------------------------------------------------------------
// E2E: Library pagination + item selection
// ---------------------------------------------------------------------------

func TestE2E_LibraryBrowseAndSelect(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Navigate to library via keypress
	m, cmd := e2ePressKey(m, 'o')
	m, cmd = feedCmd(m, cmd)
	assertScreen(t, m, ScreenLibrary)

	// Execute Init (fetches library items)
	m = feedCmdChain(m, cmd, 5)

	if len(m.library.Items()) == 0 {
		t.Fatal("library should have items after fetch")
	}

	// Scroll down with j
	m, _ = e2ePressKey(m, 'j')
	m, _ = e2ePressKey(m, 'j')

	// Press enter to select item → should emit library.NavigateDetailMsg
	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 5)

	// After NavigateDetailMsg, root model navigates to Detail
	assertScreen(t, m, ScreenDetail)
}

// ---------------------------------------------------------------------------
// E2E: Home → Search → select result
// ---------------------------------------------------------------------------

func TestE2E_HomeToSearchToDetail(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Press '/' to go to search — home returns cmd wrapping NavigateSearchMsg
	m, cmd := e2ePressKey(m, '/')
	m, cmd = feedCmd(m, cmd)

	assertScreen(t, m, ScreenSearch)
	assertBackStack(t, m, []Screen{ScreenHome})

	// Resolve any init cmds (like textinput.Blink)
	m = feedCmdChain(m, cmd, 3)

	// Type a search query — this triggers debounce internally
	m = e2eTypeString(m, "gatsby")

	// The debounceTickMsg is unexported from the search package, so we can't
	// trigger it directly. Instead, simulate the search result arriving by
	// feeding SearchResultsMsg through the root model (which dispatches to search).
	searchItems := []abs.LibraryItem{testLibraryItem("item-001", "The Great Gatsby")}
	res, _ := m.Update(search.SearchResultsMsg{Items: searchItems, Query: "gatsby"})
	m = res.(Model)

	if len(m.search.Items()) == 0 {
		t.Error("search should have results for 'gatsby'")
	}

	// Press enter to select the first result
	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)

	assertScreen(t, m, ScreenDetail)
	assertBackStack(t, m, []Screen{ScreenHome, ScreenSearch})
}

// ---------------------------------------------------------------------------
// E2E: Search → Esc returns to previous screen
// ---------------------------------------------------------------------------

func TestE2E_SearchBack(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Navigate to search
	m, cmd := e2ePressKey(m, '/')
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 3)
	assertScreen(t, m, ScreenSearch)

	// Press Esc — search screen emits search.BackMsg → root pops back
	m, cmd = e2ePressSpecial(m, tea.KeyEscape)
	m, cmd = feedCmd(m, cmd)

	assertScreen(t, m, ScreenHome)
	assertBackStack(t, m, []Screen{})
}

// ---------------------------------------------------------------------------
// E2E: Detail → Play lifecycle
// ---------------------------------------------------------------------------

func TestE2E_DetailToPlayback(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Navigate to detail screen with an item
	item := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)

	// Press 'p' to trigger playback
	m, cmd := e2ePressKey(m, 'p')

	// First feedCmd: executes the cmd from 'p' press which wraps detail.PlayCmd →
	// root model's handlePlayCmd returns cmd to call StartPlaySession API
	m, cmd = feedCmd(m, cmd)

	// Second feedCmd: executes the StartPlaySession API cmd → gets PlaySessionMsg →
	// root model's handlePlaySessionMsg sets state + returns LaunchCmd
	m, cmd = feedCmd(m, cmd)

	if m.sessionID != "sess-e2e" {
		t.Errorf("sessionID = %q, want %q", m.sessionID, "sess-e2e")
	}
	if !m.isPlaying() {
		t.Error("should be playing after PlaySessionMsg")
	}

	assertAPICallMade(t, log, "POST", "/api/items/item-001/play")

	// Feed the LaunchCmd → PlayerReadyMsg
	m, cmd = feedCmd(m, cmd)

	mp.mu.Lock()
	launched := mp.launched
	connected := mp.connected
	mp.mu.Unlock()
	if !launched {
		t.Error("mock player should have been launched")
	}
	if !connected {
		t.Error("mock player should have been connected")
	}

	// Feed PlayerReadyMsg → returns TickCmd + syncTickCmd batch
	// The tick commands use time.After so they'll timeout in feedCmd — that's fine
	m = feedCmdChain(m, cmd, 3)

	// Simulate position updates
	mp.mu.Lock()
	mp.position = 52.0
	mp.mu.Unlock()
	res, cmd := m.Update(player.PositionMsg{Position: 52.0, Duration: 3600.0, Paused: false, Generation: 1})
	m = res.(Model)

	if m.player.Position != 52.0 {
		t.Errorf("position = %f, want 52.0", m.player.Position)
	}
	if m.timeListened != 10.0 {
		t.Errorf("timeListened = %f, want 10.0", m.timeListened)
	}

	// Test View contains player info
	assertViewContains(t, m, "The Great Gatsby")

	// Trigger sync
	res, cmd = m.Update(SyncTickMsg{})
	m = res.(Model)
	executeBatchCmds(cmd)

	assertAPICallMade(t, log, "POST", "/api/session/sess-e2e/sync")
}

func TestE2E_ChapterOverlayRequiresPlayback(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)

	m, _ = e2ePressKey(m, 'c')

	if m.chapterOverlayVisible {
		t.Fatal("chapter overlay should stay closed before playback starts")
	}
	if strings.Contains(m.View(), "Chapter Navigation") {
		t.Fatal("view should not render chapter overlay before playback starts")
	}
}

func TestE2E_ChapterOverlayOpenCloseAndSeek(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)

	m, cmd := e2ePressKey(m, 'p')
	m, cmd = feedCmd(m, cmd)
	m, cmd = feedCmd(m, cmd)

	if !m.isPlaying() {
		t.Fatal("expected playback to be active after play flow")
	}
	if len(m.chapters) != 2 {
		t.Fatalf("chapters len = %d, want 2", len(m.chapters))
	}

	m, _ = e2ePressKey(m, 'c')
	if !m.chapterOverlayVisible {
		t.Fatal("expected chapter overlay to open")
	}
	assertViewContains(t, m, "Chapter Navigation")
	assertViewContains(t, m, "Chapter 1")

	m, _ = e2ePressSpecial(m, tea.KeyEsc)
	if m.chapterOverlayVisible {
		t.Fatal("expected esc to close chapter overlay")
	}
	if m.ActiveScreen() != ScreenDetail {
		t.Fatalf("screen = %v, want Detail after closing overlay", m.ActiveScreen())
	}

	m, _ = e2ePressKey(m, 'c')
	m, _ = e2ePressKey(m, 'j')
	if m.chapterOverlayIndex != 1 {
		t.Fatalf("chapter overlay index = %d, want 1", m.chapterOverlayIndex)
	}

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	if m.chapterOverlayVisible {
		t.Fatal("expected enter to close chapter overlay")
	}
	if m.player.Position != 1800.0 {
		t.Fatalf("player position = %f, want 1800.0", m.player.Position)
	}

	m, _ = feedCmd(m, cmd)

	mp.mu.Lock()
	pos := mp.position
	mp.mu.Unlock()
	if pos != 1800.0 {
		t.Fatalf("mock player position = %f, want 1800.0", pos)
	}
}

// ---------------------------------------------------------------------------
// E2E: Playback cleanup on back
// ---------------------------------------------------------------------------

func TestE2E_PlaybackContinuesOnBack(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 100, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up playback state
	item := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Title = "The Great Gatsby"
	m.player.Position = 100.0
	m.player.Duration = 3600.0
	m.timeListened = 30.0

	// Press esc to go back (playback should continue)
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	// Verify playback continues
	if m.sessionID != "sess-e2e" {
		t.Errorf("sessionID should be preserved, got %q", m.sessionID)
	}
	if !m.isPlaying() {
		t.Error("should still be playing after back")
	}
	assertScreen(t, m, ScreenHome)

	// Verify player bar still visible
	m = e2eSetSize(m, 120, 40)
	assertViewContains(t, m, "The Great Gatsby")
}

// ---------------------------------------------------------------------------
// E2E: Navigate away during playback
// ---------------------------------------------------------------------------

func TestE2E_PlaybackNavigateWhilePlaying(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 50, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: on Detail with active playback
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Title = "Test Book"
	m.player.Position = 50.0
	m.player.Duration = 3600.0

	// Navigate to Library via NavigateMsg
	res, _ := m.Update(NavigateMsg{Screen: ScreenLibrary})
	m = res.(Model)

	// Playback should continue
	if m.sessionID != "sess-e2e" {
		t.Errorf("sessionID should be preserved, got %q", m.sessionID)
	}
	if !m.isPlaying() {
		t.Error("should still be playing after navigate")
	}
	assertScreen(t, m, ScreenLibrary)
}

// ---------------------------------------------------------------------------
// E2E: Bookmark CRUD
// ---------------------------------------------------------------------------

func TestE2E_BookmarkCRUD(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 120, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: on Detail with active playback
	item := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 120.0
	m.player.Duration = 3600.0

	// --- Create bookmark: press 'b' ---
	m, cmd := e2ePressKey(m, 'b')

	// cmd returns from detail.AddBookmarkCmd → root handleAddBookmark
	// which calls CreateBookmark + optimistic local update
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 5)

	assertAPICallMade(t, log, "POST", "/api/me/item/item-001/bookmark")

	// Verify bookmark was added to server state
	bms := state.getBookmarks("item-001")
	if len(bms) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(bms))
	}
	if bms[0].Time != 120.0 {
		t.Errorf("bookmark time = %f, want 120.0", bms[0].Time)
	}

	// Verify detail screen has bookmarks
	if len(m.detail.Bookmarks()) != 1 {
		t.Errorf("detail bookmarks = %d, want 1", len(m.detail.Bookmarks()))
	}

	// --- Seek to bookmark: Tab to focus bookmarks, Enter to seek ---
	m, _ = e2ePressSpecial(m, tea.KeyTab)

	if !m.detail.FocusBookmarks() {
		t.Error("expected bookmark focus after Tab")
	}

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)

	// cmd should be SeekToBookmarkCmd → handleSeekToBookmark → seeks mpv
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 3)

	// Verify mock player was seeked
	mp.mu.Lock()
	pos := mp.position
	mp.mu.Unlock()
	if pos != 120.0 {
		t.Errorf("mock player position = %f, want 120.0 (bookmark time)", pos)
	}

	// --- Delete bookmark: press 'd' (with bookmarks focused) ---
	m, cmd = e2ePressKey(m, 'd')
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 5)

	assertAPICallMade(t, log, "DELETE", "/api/me/item/item-001/bookmark/")

	// Verify bookmark removed
	bms = state.getBookmarks("item-001")
	if len(bms) != 0 {
		t.Errorf("expected 0 bookmarks after delete, got %d", len(bms))
	}
	if len(m.detail.Bookmarks()) != 0 {
		t.Errorf("detail bookmarks = %d, want 0", len(m.detail.Bookmarks()))
	}
	if !strings.Contains(m.detail.View(), "No bookmarks yet") {
		t.Error("expected detail view to show empty bookmark state after deleting the last bookmark")
	}
}

func TestE2E_BookmarkEnterStartsPlaybackWhenStopped(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	state.bookmarks["item-multitrack"] = []abs.Bookmark{
		testBookmark("Track Two", 2200.0),
	}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 1800}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-multitrack", "Multi Track Book")
	res, cmd := m.Update(home.NavigateDetailMsg{Item: item})
	m = res.(Model)
	assertScreen(t, m, ScreenDetail)
	m = feedCmdChain(m, cmd, 5)

	m, _ = e2ePressSpecial(m, tea.KeyTab)
	if !m.detail.FocusBookmarks() {
		t.Fatal("expected bookmark focus after Tab")
	}

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 2)

	assertAPICallMade(t, log, "POST", "/api/items/item-multitrack/play")
	if m.sessionID != "sess-mt-e2e" {
		t.Fatalf("sessionID = %q, want sess-mt-e2e", m.sessionID)
	}
	if m.player.Position != 2200.0 {
		t.Fatalf("player position = %f, want 2200.0", m.player.Position)
	}
	if m.trackStartOffset != 1800.0 {
		t.Fatalf("trackStartOffset = %f, want 1800.0", m.trackStartOffset)
	}
	if !mp.launched {
		t.Fatal("expected player launch after bookmark enter")
	}
}

func TestE2E_BookmarkTitleEdit(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	state.bookmarks["item-001"] = []abs.Bookmark{
		testBookmark("Old title", 300.0),
	}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-001", "The Great Gatsby")
	res, cmd := m.Update(home.NavigateDetailMsg{Item: item})
	m = res.(Model)
	assertScreen(t, m, ScreenDetail)
	m = feedCmdChain(m, cmd, 5)

	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m, _ = e2ePressKey(m, 'e')
	for i := 0; i < len("Old title"); i++ {
		m, _ = e2ePressSpecial(m, tea.KeyBackspace)
	}
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Renamed title")})
	m = res.(Model)

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 3)

	assertAPICallMade(t, log, "PATCH", "/api/me/item/item-001/bookmark")

	bms := state.getBookmarks("item-001")
	if len(bms) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(bms))
	}
	if bms[0].Title != "Renamed title" {
		t.Fatalf("server bookmark title = %q, want Renamed title", bms[0].Title)
	}
	if m.detail.Bookmarks()[0].Title != "Renamed title" {
		t.Fatalf("detail bookmark title = %q, want Renamed title", m.detail.Bookmarks()[0].Title)
	}
}

// ---------------------------------------------------------------------------
// E2E: Bookmarks fetched on detail entry
// ---------------------------------------------------------------------------

func TestE2E_BookmarkFetchOnDetailEntry(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	// Pre-populate bookmarks on server
	state.bookmarks["item-001"] = []abs.Bookmark{
		testBookmark("Chapter Start", 300.0),
		testBookmark("Good Part", 1500.0),
	}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Simulate navigating to detail via home.NavigateDetailMsg
	item := testLibraryItem("item-001", "The Great Gatsby")
	res, cmd := m.Update(home.NavigateDetailMsg{Item: item})
	m = res.(Model)

	// This should have navigated to Detail AND returned fetchBookmarksCmd
	assertScreen(t, m, ScreenDetail)

	// Execute the bookmark fetch
	m = feedCmdChain(m, cmd, 5)

	assertAPICallMade(t, log, "GET", "/api/me")

	// Verify bookmarks are on the detail screen
	if len(m.detail.Bookmarks()) != 2 {
		t.Errorf("detail bookmarks = %d, want 2", len(m.detail.Bookmarks()))
	}
}

func TestE2E_BookmarkFetchOnDetailEntryShowsEmptyStateForNoBookmarks(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-001", "The Great Gatsby")
	res, cmd := m.Update(home.NavigateDetailMsg{Item: item})
	m = res.(Model)
	assertScreen(t, m, ScreenDetail)

	m = feedCmdChain(m, cmd, 5)

	if !m.detail.BookmarksLoaded() {
		t.Fatal("expected bookmarks to be marked loaded")
	}
	if m.detail.BookmarkLoadError() != nil {
		t.Fatalf("expected no bookmark load error, got %v", m.detail.BookmarkLoadError())
	}
	if !strings.Contains(m.detail.View(), "No bookmarks yet") {
		t.Error("expected detail view to show empty bookmark state when user bookmark list is empty")
	}
}

func TestE2E_BookmarkFetchOnDetailEntryShowsBookmarkErrorForServerFailure(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{
		bookmarks:      make(map[string][]abs.Bookmark),
		progressStatus: map[string]int{"item-001": http.StatusInternalServerError},
	}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-001", "The Great Gatsby")
	res, cmd := m.Update(home.NavigateDetailMsg{Item: item})
	m = res.(Model)
	assertScreen(t, m, ScreenDetail)

	m = feedCmdChain(m, cmd, 5)

	if m.detail.BookmarkLoadError() == nil {
		t.Fatal("expected bookmark load error for server failure")
	}
	if !strings.Contains(m.detail.View(), "boom") {
		t.Error("expected detail view to show bookmark load error text")
	}
}

// ---------------------------------------------------------------------------
// E2E: 401 redirects to login
// ---------------------------------------------------------------------------

func TestE2E_401Redirect(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	assertScreen(t, m, ScreenHome)

	// Simulate a 401 error
	res, _ := m.Update(components.ErrMsg{Err: fmt.Errorf("unexpected status 401: unauthorized")})
	m = res.(Model)

	// Should redirect to login
	assertScreen(t, m, ScreenLogin)

	// Client should be nil
	if m.client != nil {
		t.Error("client should be nil after 401 redirect")
	}

	// Back stack should be cleared
	assertBackStack(t, m, []Screen{})
}

// ---------------------------------------------------------------------------
// E2E: Error banner display and dismiss
// ---------------------------------------------------------------------------

func TestE2E_ErrorBannerDismiss(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Send a non-401 error
	res, _ := m.Update(components.ErrMsg{Err: fmt.Errorf("network timeout")})
	m = res.(Model)

	// Error banner should be visible
	if !m.err.HasError() {
		t.Fatal("expected error banner to be visible")
	}
	assertViewContains(t, m, "network timeout")

	// Should still be on home (not redirected)
	assertScreen(t, m, ScreenHome)

	// Press any key to dismiss
	m, _ = e2ePressKey(m, 'x')

	if m.err.HasError() {
		t.Error("error banner should be dismissed after keypress")
	}
}

// ---------------------------------------------------------------------------
// E2E: Help overlay toggle
// ---------------------------------------------------------------------------

func TestE2E_HelpOverlay(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Press ? to toggle help
	m, _ = e2ePressKey(m, '?')

	if !m.help.Visible() {
		t.Fatal("help overlay should be visible after pressing ?")
	}

	// View should contain keybinding info
	v := m.View()
	if !strings.Contains(v, "Navigation") && !strings.Contains(v, "Help") && !strings.Contains(v, "?") {
		// Help overlay should show some keybinding-related content
		t.Log("Help overlay visible but content may vary — checking it renders without panic")
	}

	// Other keys should be swallowed while help is visible
	prevScreen := m.ActiveScreen()
	m, _ = e2ePressKey(m, 'o')
	assertScreen(t, m, prevScreen) // should not have navigated

	// Esc dismisses help
	m, _ = e2ePressSpecial(m, tea.KeyEscape)
	if m.help.Visible() {
		t.Error("help should be dismissed after Esc")
	}

	// Press ? again to verify toggle
	m, _ = e2ePressKey(m, '?')
	if !m.help.Visible() {
		t.Error("help should be visible again after second ?")
	}
}

// ---------------------------------------------------------------------------
// E2E: Deep navigation back traversal
// ---------------------------------------------------------------------------

func TestE2E_BackStackDeep(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Home → Library
	m, cmd := e2ePressKey(m, 'o')
	m = feedCmdChain(m, cmd, 5)
	assertScreen(t, m, ScreenLibrary)
	assertBackStack(t, m, []Screen{ScreenHome})

	// Library → Detail (via NavigateDetailMsg)
	item := testLibraryItem("item-001", "Test Book")
	res, cmd := m.Update(library.NavigateDetailMsg{Item: item})
	m = res.(Model)
	m = feedCmdChain(m, cmd, 5)
	assertScreen(t, m, ScreenDetail)
	assertBackStack(t, m, []Screen{ScreenHome, ScreenLibrary})

	// Back: Detail → Library (esc)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	assertScreen(t, m, ScreenLibrary)
	assertBackStack(t, m, []Screen{ScreenHome})

	// Back: Library → Home (esc)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	assertScreen(t, m, ScreenHome)
	assertBackStack(t, m, []Screen{})

	// esc on Home root = no-op
	res, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if cmd != nil {
		t.Fatal("esc on root screen should be no-op")
	}
	assertScreen(t, m, ScreenHome)

	// q always quits
	_, cmd = e2ePressKey(m, 'q')
	if cmd == nil {
		t.Fatal("q should return quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// E2E: Window resize propagation
// ---------------------------------------------------------------------------

func TestE2E_WindowResize(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)

	// Set initial size
	m = e2eSetSize(m, 80, 24)
	if m.width != 80 || m.height != 24 {
		t.Errorf("dimensions = %dx%d, want 80x24", m.width, m.height)
	}

	// Resize
	m = e2eSetSize(m, 120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", m.width, m.height)
	}

	// View should render without panic
	v := m.View()
	if v == "" {
		t.Error("View() should not be empty after resize")
	}

	// Test with playback active
	m.player.Playing = true
	m.player.Title = "Test"
	m.player.Duration = 3600
	m = e2eSetSize(m, 160, 50)

	v = m.View()
	if v == "" {
		t.Error("View() should not be empty with active player after resize")
	}
}

// ---------------------------------------------------------------------------
// E2E: Session persistence in DB
// ---------------------------------------------------------------------------

func TestE2E_SessionPersistence(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m, store := newE2EModelWithDB(t, srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Simulate login success — should save account to DB
	res, _ := m.Update(login.LoginSuccessMsg{
		Token:     "jwt-token-e2e",
		ServerURL: srv.URL,
		Username:  "alice",
	})
	m = res.(Model)

	// Verify account saved
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) == 0 {
		t.Error("expected account to be saved after login")
	}
}

// ---------------------------------------------------------------------------
// E2E: Complete user journey
// ---------------------------------------------------------------------------

func TestE2E_FullJourney(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m, store := newE2EModelUnauthWithDB(t, srv, mp)
	m = e2eSetSize(m, 120, 40)

	// === Phase 1: Login ===
	assertScreen(t, m, ScreenLogin)

	m = e2eTypeString(m, srv.URL)
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m = e2eTypeString(m, "alice")
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	m = e2eTypeString(m, "password")
	m, cmd := e2ePressSpecial(m, tea.KeyEnter)

	// Execute login
	m, cmd = feedCmd(m, cmd)
	assertScreen(t, m, ScreenHome)
	assertAPICallMade(t, log, "POST", "/login")

	// Verify account saved to DB
	accounts, _ := store.ListAccounts()
	if len(accounts) == 0 {
		t.Error("account should be saved after login")
	}

	// === Phase 2: Home screen loads ===
	m = feedCmdChain(m, cmd, 5)
	assertAPICallMade(t, log, "GET", "/api/libraries/lib-001/personalized")

	if len(m.home.Items()) == 0 {
		t.Fatal("home should have items")
	}
	assertViewContains(t, m, "Continue Listening")

	// === Phase 3: Navigate to Library ===
	m, cmd = e2ePressKey(m, 'o')
	m, cmd = feedCmd(m, cmd) // executes NavigateLibraryMsg wrapper
	m = feedCmdChain(m, cmd, 5)
	assertScreen(t, m, ScreenLibrary)
	assertAPICallMade(t, log, "GET", "/api/libraries/lib-001/items")

	if len(m.library.Items()) == 0 {
		t.Fatal("library should have items")
	}

	// === Phase 4: Select item → Detail ===
	// Scroll down and select
	m, _ = e2ePressKey(m, 'j')
	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd) // executes NavigateDetailMsg wrapper
	m, cmd = feedCmd(m, cmd) // NavigateDetailMsg → navigates to Detail + fetchBookmarks
	m = feedCmdChain(m, cmd, 5)
	assertScreen(t, m, ScreenDetail)
	// Login is NOT on back stack — it's the entry point, not a navigation target
	assertBackStack(t, m, []Screen{ScreenHome, ScreenLibrary})

	// === Phase 5: Start playback ===
	m, cmd = e2ePressKey(m, 'p')
	m, cmd = feedCmd(m, cmd) // executes PlayCmd wrapper from detail
	m, cmd = feedCmd(m, cmd) // executes StartPlaySession API call
	m, cmd = feedCmd(m, cmd) // PlaySessionMsg → handlePlaySessionMsg → LaunchCmd

	if m.sessionID != "sess-e2e" {
		t.Errorf("sessionID = %q, want sess-e2e", m.sessionID)
	}

	// Execute: LaunchCmd → PlayerReadyMsg → TickCmd+syncTickCmd
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 3)

	mp.mu.Lock()
	wasLaunched := mp.launched
	mp.mu.Unlock()
	if !wasLaunched {
		t.Error("player should be launched")
	}

	assertAPICallMade(t, log, "POST", "/api/items/")

	// === Phase 6: Position updates ===
	mp.mu.Lock()
	mp.position = 60.0
	mp.mu.Unlock()
	res, cmd := m.Update(player.PositionMsg{Position: 60.0, Duration: 3600.0, Paused: false, Generation: 1})
	m = res.(Model)

	if m.timeListened == 0 {
		t.Error("timeListened should be > 0 after position update")
	}

	// === Phase 7: Create bookmark ===
	m, cmd = e2ePressKey(m, 'b')
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 5)
	assertAPICallMade(t, log, "POST", "/api/me/item/")

	// === Phase 8: Sync tick ===
	res, cmd = m.Update(SyncTickMsg{})
	m = res.(Model)
	executeBatchCmds(cmd)
	assertAPICallMade(t, log, "POST", "/api/session/sess-e2e/sync")

	// === Phase 9: Back to Library (playback continues, esc) ===
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)

	if m.sessionID == "" {
		t.Error("playback should continue after back to Library")
	}
	assertScreen(t, m, ScreenLibrary)

	// === Phase 10: Back to Home (playback continues, esc) ===
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	assertScreen(t, m, ScreenHome)

	if m.sessionID == "" {
		t.Error("playback should continue on Home")
	}

	// === Phase 11: Quit from Home (stops playback and exits) ===
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("'q' on Home with empty back stack should quit")
	}

	// Should batch stop commands with quit
	executeBatchCmds(cmd)
	assertAPICallMade(t, log, "POST", "/api/session/sess-e2e/close")

	// === Verify API call sequence ===
	reqs := log.get()
	t.Logf("Total API calls: %d", len(reqs))
	for i, r := range reqs {
		t.Logf("  [%d] %s %s", i, r.Method, r.Path)
	}
}

// ---------------------------------------------------------------------------
// E2E: Library switching (Tab) and podcast library access
// ---------------------------------------------------------------------------

func TestE2E_LibrarySwitchToPodcasts(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Step 1: Initialize home (loads libraries + personalized)
	cmd := m.Init()
	m = feedCmdChain(m, cmd, 5)

	assertScreen(t, m, ScreenHome)
	if len(m.home.Libraries()) < 2 {
		t.Fatalf("expected 2+ libraries, got %d", len(m.home.Libraries()))
	}

	// Default library should be the first one (Audiobooks)
	if m.home.SelectedLibraryID() != "lib-001" {
		t.Errorf("default library = %q, want lib-001", m.home.SelectedLibraryID())
	}

	// Step 2: Press Tab to switch to podcast library
	m, cmd = e2ePressSpecial(m, tea.KeyTab)
	m = feedCmdChain(m, cmd, 5)

	if m.home.SelectedLibraryID() != "lib-pods" {
		t.Errorf("after tab, library = %q, want lib-pods", m.home.SelectedLibraryID())
	}
}

// ---------------------------------------------------------------------------
// E2E: Podcast detail shows episodes
// ---------------------------------------------------------------------------

func TestE2E_PodcastDetailShowsEpisodes(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: create detail screen directly with a podcast item
	podItem := testPodcastItem("pod-001", "Tech Talk Daily")
	m.detail = detail.New(m.styles, podItem)
	m.screen = ScreenDetail
	m.detail.SetSize(120, 35)

	// Episodes should be populated from the item
	if len(m.detail.Episodes()) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(m.detail.Episodes()))
	}

	// View should contain episode titles
	v := m.View()
	if !strings.Contains(v, "Episode 1 - Pilot") {
		t.Error("View should contain 'Episode 1 - Pilot'")
	}
	if !strings.Contains(v, "Episode 2 - Deep Dive") {
		t.Error("View should contain 'Episode 2 - Deep Dive'")
	}

	// Navigate episodes with j/k
	m, _ = e2ePressKey(m, 'j')
	if m.detail.SelectedEpisode() != 1 {
		t.Errorf("selected episode = %d, want 1", m.detail.SelectedEpisode())
	}
	m, _ = e2ePressKey(m, 'k')
	if m.detail.SelectedEpisode() != 0 {
		t.Errorf("selected episode = %d, want 0", m.detail.SelectedEpisode())
	}
}

// ---------------------------------------------------------------------------
// E2E: Podcast episode playback lifecycle
// ---------------------------------------------------------------------------

func TestE2E_PodcastEpisodePlayback(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 1800}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: detail screen with podcast
	podItem := testPodcastItem("pod-001", "Tech Talk Daily")
	m.detail = detail.New(m.styles, podItem)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}
	m.detail.SetSize(120, 35)

	// Step 1: Press Enter to play the first episode (focus is on episodes by default for podcasts)
	m, cmd := e2ePressSpecial(m, tea.KeyEnter)

	// Should emit PlayEpisodeCmd
	m, cmd = feedCmd(m, cmd)

	// Step 2: Feed the resulting PlaySessionMsg
	if cmd != nil {
		m, cmd = feedCmd(m, cmd)
	}

	// Verify session state
	if m.sessionID != "sess-ep-e2e" {
		t.Errorf("sessionID = %q, want sess-ep-e2e", m.sessionID)
	}
	if m.itemID != "pod-001" {
		t.Errorf("itemID = %q, want pod-001", m.itemID)
	}
	if m.episodeID != "pod-001-ep-001" {
		t.Errorf("episodeID = %q, want pod-001-ep-001", m.episodeID)
	}
	if !m.isPlaying() {
		t.Error("model should be playing after episode play")
	}

	// Verify API call was made to episode play endpoint
	assertAPICallMade(t, log, http.MethodPost, "/api/items/pod-001/play/pod-001-ep-001")

	// Step 3: Send position update
	result, _ := m.Update(player.PositionMsg{Position: 60.0, Duration: 1800.0, Generation: 1})
	m = result.(Model)
	if m.player.Position != 60.0 {
		t.Errorf("position = %f, want 60.0", m.player.Position)
	}

	// Step 4: Back (playback continues)
	result, _ = m.Update(BackMsg{})
	m = result.(Model)

	if m.sessionID == "" {
		t.Error("sessionID should be preserved after back")
	}
	if m.episodeID == "" {
		t.Error("episodeID should be preserved after back")
	}
}

// ---------------------------------------------------------------------------
// E2E: Book detail does NOT show episodes
// ---------------------------------------------------------------------------

func TestE2E_BookDetailNoEpisodes(t *testing.T) {
	mp := &mockPlayer{}
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Create detail with a book item
	bookItem := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, bookItem)
	m.screen = ScreenDetail
	m.detail.SetSize(120, 35)

	// Should have NO episodes
	if len(m.detail.Episodes()) != 0 {
		t.Errorf("book should have 0 episodes, got %d", len(m.detail.Episodes()))
	}

	// View should contain neither inline chapters nor episodes
	v := m.View()
	if strings.Contains(v, "Chapters") {
		t.Error("book view should not contain inline chapters")
	}
	if strings.Contains(v, "Episodes") {
		t.Error("book view should NOT contain 'Episodes' section")
	}
}

// ---------------------------------------------------------------------------
// E2E: Seek keys (h/l) work on detail screen during playback
// ---------------------------------------------------------------------------

func TestE2E_SeekKeysWorkDuringPlayback(t *testing.T) {
	mp := &mockPlayer{position: 100, duration: 3600}
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: on Detail with active playback
	item := testLibraryItem("item-001", "Test Book")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)
	m.sessionID = "sess-test"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0

	// Press 'h' (seek backward) — should NOT navigate back
	m, cmd := e2ePressKey(m, 'h')
	if m.ActiveScreen() != ScreenDetail {
		t.Errorf("pressing 'h' during playback should seek, not go back; screen = %v", m.ActiveScreen())
	}
	if cmd == nil {
		t.Error("expected seek command from 'h' during playback")
	}
	// Position should be adjusted (100 - 10 = 90)
	if m.player.Position != 90.0 {
		t.Errorf("position after 'h' = %f, want 90.0", m.player.Position)
	}

	// Press 'l' (seek forward) — should NOT be swallowed by screen
	m, cmd = e2ePressKey(m, 'l')
	if cmd == nil {
		t.Error("expected seek command from 'l' during playback")
	}
	// Position should be adjusted (90 + 10 = 100)
	if m.player.Position != 100.0 {
		t.Errorf("position after 'l' = %f, want 100.0", m.player.Position)
	}
}

// ---------------------------------------------------------------------------
// E2E: q key navigates back when NOT playing on detail screen
// ---------------------------------------------------------------------------

func TestE2E_QKeyQuitsApp(t *testing.T) {
	mp := &mockPlayer{}
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	item := testLibraryItem("item-001", "Test Book")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)

	// 'q' should quit the app, not navigate back
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("pressing 'q' should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// E2E: Duration display uses media.duration (not metadata.duration)
// ---------------------------------------------------------------------------

func TestE2E_DurationFromMediaLevel(t *testing.T) {
	mp := &mockPlayer{}
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Create item with duration ONLY at media level (like real ABS API)
	mediaDur := 7200.0
	item := abs.LibraryItem{
		ID:        "item-media-dur",
		MediaType: "book",
		Media: abs.Media{
			Duration: &mediaDur,
			Metadata: abs.MediaMetadata{
				Title:      "Media Duration Book",
				AuthorName: &testAuthor,
			},
		},
	}

	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)

	v := m.View()
	if !strings.Contains(v, "2h 0m") {
		t.Errorf("expected duration '2h 0m' from media.duration, view:\n%.500s", v)
	}
}

// ---------------------------------------------------------------------------
// E2E: Sleep timer generation prevents stale expiration
// ---------------------------------------------------------------------------

func TestE2E_SleepTimerGeneration(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 100, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up active playback
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0

	// Cycle sleep timer to 15m (generation 1)
	m, _ = e2ePressKey(m, 'S')
	if m.sleepGeneration != 1 {
		t.Fatalf("sleepGeneration = %d, want 1", m.sleepGeneration)
	}

	// Cycle sleep timer to 30m (generation 2)
	m, _ = e2ePressKey(m, 'S')
	if m.sleepGeneration != 2 {
		t.Fatalf("sleepGeneration = %d, want 2", m.sleepGeneration)
	}

	// Fire stale generation 1 expiry — should NOT stop playback
	res, _ := m.Update(SleepTimerExpiredMsg{Generation: 1})
	m = res.(Model)
	if !m.isPlaying() {
		t.Error("stale sleep timer (gen 1) should NOT stop playback")
	}

	// Fire current generation 2 expiry — should stop playback
	res, _ = m.Update(SleepTimerExpiredMsg{Generation: 2})
	m = res.(Model)
	if m.isPlaying() {
		t.Error("current sleep timer (gen 2) should stop playback")
	}
}

// ---------------------------------------------------------------------------
// E2E: Chapter navigation during playback
// ---------------------------------------------------------------------------

func TestE2E_ChapterNavigation(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Navigate to detail and start playback via API
	item := testLibraryItem("item-001", "The Great Gatsby")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)

	// Start playback
	m, cmd := e2ePressKey(m, 'p')
	m, cmd = feedCmd(m, cmd) // PlayCmd → StartPlaySession API
	m, cmd = feedCmd(m, cmd) // PlaySessionMsg → LaunchCmd
	m, _ = feedCmd(m, cmd)   // LaunchCmd → PlayerReadyMsg

	// Chapters should come from the play session response
	if len(m.chapters) != 2 {
		t.Fatalf("expected 2 chapters from play session, got %d", len(m.chapters))
	}

	// Position is 42 (in chapter 1: 0-1800)
	// Press 'n' → next chapter start (1800)
	m, cmd = e2ePressKey(m, 'n')
	if m.player.Position != 1800.0 {
		t.Errorf("after 'n', position = %f, want 1800.0", m.player.Position)
	}
	if cmd == nil {
		t.Error("expected seek command from 'n'")
	}

	// Press 'N' → previous chapter start (0)
	m, cmd = e2ePressKey(m, 'N')
	if m.player.Position != 0.0 {
		t.Errorf("after 'N', position = %f, want 0.0", m.player.Position)
	}
	if cmd == nil {
		t.Error("expected seek command from 'N'")
	}
}

// ---------------------------------------------------------------------------
// E2E: Duration preservation from ABS (mpv reports 0 for streaming)
// ---------------------------------------------------------------------------

func TestE2E_DurationPreservation(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 50, duration: 0}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up playback with ABS-provided duration
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 50.0
	m.player.Duration = 3600.0
	m.playGeneration = 1

	// mpv reports Duration: 0 (streaming content)
	res, _ := m.Update(player.PositionMsg{Position: 55.0, Duration: 0, Paused: false, Generation: 1})
	m = res.(Model)

	if m.player.Duration != 3600.0 {
		t.Errorf("duration = %f, want 3600.0 (ABS value should be preserved)", m.player.Duration)
	}
	if m.player.Position != 55.0 {
		t.Errorf("position = %f, want 55.0", m.player.Position)
	}
}

// ---------------------------------------------------------------------------
// E2E: PlayerLaunchErrMsg cleans up all session state
// ---------------------------------------------------------------------------

func TestE2E_PlayerLaunchErrCleanup(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up session state as if play session was received but mpv failed to launch
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Title = "Test Book"
	m.player.Position = 42.0
	m.player.Duration = 3600.0

	// Send PlayerLaunchErrMsg
	res, _ := m.Update(player.PlayerLaunchErrMsg{Err: fmt.Errorf("mpv not found")})
	m = res.(Model)

	if m.sessionID != "" {
		t.Errorf("sessionID should be cleared, got %q", m.sessionID)
	}
	if m.player.Title != "" {
		t.Errorf("player.Title should be empty, got %q", m.player.Title)
	}
	if m.player.Playing {
		t.Error("player.Playing should be false")
	}
	if m.player.Position != 0 {
		t.Errorf("player.Position should be 0, got %f", m.player.Position)
	}
	if m.player.Duration != 0 {
		t.Errorf("player.Duration should be 0, got %f", m.player.Duration)
	}
	if m.err.HasError() == false {
		t.Error("error banner should be visible")
	}
}

// ---------------------------------------------------------------------------
// E2E: Arrow keys navigate during playback (don't seek)
// ---------------------------------------------------------------------------

func TestE2E_ArrowKeysNavigateDuringPlayback(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 100, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: on Detail with active playback
	item := testLibraryItem("item-001", "Test Book")
	m.detail = detail.New(m.styles, item)
	m, _ = m.navigate(ScreenDetail)
	m = e2eSetSize(m, 120, 40)
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0

	// Press left arrow — should go back, NOT seek
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = res.(Model)

	assertScreen(t, m, ScreenHome)
	if !m.isPlaying() {
		t.Error("playback should continue after left arrow navigation")
	}
	if m.player.Position != 100.0 {
		t.Errorf("position should be unchanged at 100.0, got %f", m.player.Position)
	}
}

// ---------------------------------------------------------------------------
// E2E: Esc on root screen is no-op with active playback
// ---------------------------------------------------------------------------

func TestE2E_EscOnRootNoOpWithPlayback(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 100, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: on Home with active playback, empty back stack
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0

	assertScreen(t, m, ScreenHome)
	assertBackStack(t, m, []Screen{})

	// Press esc — should be no-op
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)

	assertScreen(t, m, ScreenHome)
	if !m.isPlaying() {
		t.Error("playback should continue after esc on root")
	}
	if cmd != nil {
		t.Error("esc on root with playback should return no cmd")
	}
}

// ---------------------------------------------------------------------------
// E2E: q quits and stops playback
// ---------------------------------------------------------------------------

func TestE2E_QQuitsAndStopsPlayback(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 100, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: on Home with active playback
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0

	// Press q — should quit and stop playback
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should return a command")
	}

	// Execute the batch — should include stop + quit
	executeBatchCmds(cmd)

	// Verify close session API was called
	assertAPICallMade(t, log, "POST", "/api/session/sess-e2e/close")
}

// ---------------------------------------------------------------------------
// E2E: Within-track seek uses mpv.Seek directly (no session restart)
// ---------------------------------------------------------------------------

func TestE2E_WithinTrackSeek(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 500, duration: 1800}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up playback with explicit track boundaries
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 500.0  // book-global
	m.player.Duration = 3600.0 // book total
	m.trackStartOffset = 0
	m.trackDuration = 1800.0 // track covers 0-1800
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 900, Title: "Chapter 1"},
		{ID: 1, Start: 900, End: 1800, Title: "Chapter 2"},
	}

	apiCallsBefore := len(log.get())

	// Press 'l' (seek forward 10s) — should NOT trigger session restart
	m, cmd := e2ePressKey(m, 'l')
	if cmd != nil {
		m = feedCmdChain(m, cmd, 3)
	}

	if m.player.Position != 510.0 {
		t.Errorf("position after 'l' = %f, want 510.0", m.player.Position)
	}

	// Verify mock player was seeked to track-relative position
	mp.mu.Lock()
	seekPos := mp.position
	mp.mu.Unlock()
	if seekPos != 510.0 {
		t.Errorf("mpv seek position = %f, want 510.0 (track-relative)", seekPos)
	}

	// No new API calls should have been made (no session restart)
	apiCallsAfter := len(log.get())
	if apiCallsAfter != apiCallsBefore {
		t.Errorf("expected no new API calls for within-track seek, got %d new calls", apiCallsAfter-apiCallsBefore)
	}

	// Press 'n' (next chapter) — chapter 2 starts at 900, within same track
	m, cmd = e2ePressKey(m, 'n')
	if cmd != nil {
		m = feedCmdChain(m, cmd, 3)
	}

	if m.player.Position != 900.0 {
		t.Errorf("position after 'n' = %f, want 900.0", m.player.Position)
	}

	mp.mu.Lock()
	seekPos = mp.position
	mp.mu.Unlock()
	if seekPos != 900.0 {
		t.Errorf("mpv seek position after chapter = %f, want 900.0", seekPos)
	}

	// Still no new API calls
	apiCallsAfter = len(log.get())
	if apiCallsAfter != apiCallsBefore {
		t.Errorf("expected no new API calls for within-track chapter seek, got %d new calls", apiCallsAfter-apiCallsBefore)
	}
}

// ---------------------------------------------------------------------------
// E2E: Within-track seek with trackDuration==0 (fallback)
// ---------------------------------------------------------------------------

func TestE2E_WithinTrackSeekZeroDuration(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 100, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up playback WITHOUT trackDuration (zero value — single-file or missing)
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0
	m.trackStartOffset = 0
	m.trackDuration = 0 // not set

	apiCallsBefore := len(log.get())

	// Press 'l' — should still seek directly, not restart
	m, cmd := e2ePressKey(m, 'l')
	if cmd != nil {
		m = feedCmdChain(m, cmd, 3)
	}

	if m.player.Position != 110.0 {
		t.Errorf("position = %f, want 110.0", m.player.Position)
	}

	apiCallsAfter := len(log.get())
	if apiCallsAfter != apiCallsBefore {
		t.Errorf("expected no API calls for seek with trackDuration==0, got %d new calls", apiCallsAfter-apiCallsBefore)
	}
}

// ---------------------------------------------------------------------------
// E2E: Cross-track chapter seek lands on correct position
// ---------------------------------------------------------------------------

func TestE2E_CrossTrackChapterSeek(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 500, duration: 1800}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up playback: position 500 in track 1 (0-1800)
	m.sessionID = "sess-e2e"
	m.itemID = "item-multitrack"
	m.player.Playing = true
	m.player.Position = 500.0
	m.player.Duration = 3600.0
	m.trackStartOffset = 0
	m.trackDuration = 1800.0
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 1800, Title: "Chapter 1"},
		{ID: 1, Start: 1800, End: 3600, Title: "Chapter 2"},
	}

	// Press 'n' → chapter 2 at position 2000 is beyond current track (0-1800)
	// This should trigger a cross-track restart
	m, cmd := e2ePressKey(m, 'n')

	// Feed the cross-track restart command chain
	m = feedCmdChain(m, cmd, 10)

	// Verify: new session started on the correct track
	if m.trackStartOffset != 1800.0 {
		t.Errorf("trackStartOffset = %f, want 1800.0", m.trackStartOffset)
	}
	if m.player.Position != 1800.0 {
		t.Errorf("player.Position = %f, want 1800.0 (chapter 2 start)", m.player.Position)
	}
	if !m.isPlaying() {
		t.Error("should still be playing after cross-track seek")
	}

	// Verify: API call was made for new play session
	assertAPICallMade(t, log, "POST", "/api/items/item-multitrack/play")
}

// ---------------------------------------------------------------------------
// E2E: Position tick converts track-relative to book-global
// ---------------------------------------------------------------------------

func TestE2E_MultiTrackPositionTick(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 200, duration: 1800}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: playing track 2 (startOffset=1800)
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 2000.0 // book-global
	m.player.Duration = 3600.0
	m.trackStartOffset = 1800.0
	m.trackDuration = 1800.0
	m.playGeneration = 1

	// mpv reports position 250 (track-relative, i.e. 250s into track 2)
	res, _ := m.Update(player.PositionMsg{Position: 250.0, Duration: 1800.0, Paused: false, Generation: 1})
	m = res.(Model)

	// Player position should be book-global: 250 + 1800 = 2050
	if m.player.Position != 2050.0 {
		t.Errorf("position = %f, want 2050.0 (book-global)", m.player.Position)
	}

	// Duration should NOT be overwritten by mpv's track duration
	if m.player.Duration != 3600.0 {
		t.Errorf("duration = %f, want 3600.0 (book-global, not track duration)", m.player.Duration)
	}
}

// ---------------------------------------------------------------------------
// E2E: Sync sends book-global position for multi-track
// ---------------------------------------------------------------------------

func TestE2E_MultiTrackSyncPosition(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 300, duration: 1800}
	m := newE2EModelAuthenticated(srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Set up: playing track 2 (startOffset=1800), book-global position 2100
	m.sessionID = "sess-e2e"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 2100.0
	m.player.Duration = 3600.0
	m.trackStartOffset = 1800.0
	m.trackDuration = 1800.0

	// Trigger sync
	res, cmd := m.Update(SyncTickMsg{})
	m = res.(Model)
	executeBatchCmds(cmd)

	// Verify sync API was called
	assertAPICallMade(t, log, "POST", "/api/session/sess-e2e/sync")

	// Verify the synced position is book-global (2100, not track-relative 300)
	reqs := log.get()
	for _, r := range reqs {
		if r.Method == "POST" && strings.Contains(r.Path, "/sync") {
			if ct, ok := r.Body["currentTime"].(float64); ok {
				if ct != 2100.0 {
					t.Errorf("synced currentTime = %f, want 2100.0 (book-global)", ct)
				}
			}
			break
		}
	}
}

// ---------------------------------------------------------------------------
// E2E: Session restore on startup
// ---------------------------------------------------------------------------

func TestE2E_SessionRestore(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m, store := newE2EModelWithDB(t, srv, mp)
	m = e2eSetSize(m, 120, 40)

	// Save a session to the DB simulating a previous listening session
	if err := store.SaveListeningSession(db.ListeningSession{
		ItemID:      "item-001",
		EpisodeID:   "",
		SessionID:   "sess-old",
		CurrentTime: 100.0,
		Duration:    3600.0,
	}); err != nil {
		t.Fatalf("SaveListeningSession: %v", err)
	}

	// Simulate login so client is available
	res, _ := m.Update(login.LoginSuccessMsg{
		Token:     "jwt-token-e2e",
		ServerURL: srv.URL,
		Username:  "alice",
	})
	m = res.(Model)

	// Call Init which triggers restoreSessionCmd
	cmd := m.Init()

	// Execute the restore command chain through playback start.
	m = feedCmdChain(m, cmd, 3)

	// After restore, should stay on Home screen while restoring paused playback
	assertScreen(t, m, ScreenHome)

	// Session state should be populated once restore playback starts.
	if m.sessionID == "" {
		t.Fatal("expected restored session to start playback")
	}
	if m.itemID != "item-001" {
		t.Errorf("itemID = %q, want %q", m.itemID, "item-001")
	}
}

func TestE2E_SessionRestorePodcastMissingEpisodeSkipsSilently(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m, store := newE2EModelWithDB(t, srv, mp)
	m = e2eSetSize(m, 120, 40)

	if err := store.SaveListeningSession(db.ListeningSession{
		ItemID:      "pod-001",
		EpisodeID:   "pod-001-ep-missing",
		SessionID:   "sess-old-pod",
		CurrentTime: 100.0,
		Duration:    1800.0,
	}); err != nil {
		t.Fatalf("SaveListeningSession: %v", err)
	}

	res, _ := m.Update(login.LoginSuccessMsg{
		Token:     "jwt-token-e2e",
		ServerURL: srv.URL,
		Username:  "alice",
	})
	m = res.(Model)

	m = feedCmdChain(m, m.Init(), 3)

	assertScreen(t, m, ScreenHome)
	if m.sessionID != "" {
		t.Fatalf("expected restore to skip playback, got session %q", m.sessionID)
	}
	if m.err.HasError() {
		t.Fatal("expected missing restore episode to skip without error banner")
	}
	if _, err := store.GetLastSession(); err == nil {
		t.Fatal("expected stale restore session to be cleared")
	}
	assertNoAPICall(t, log, http.MethodPost, "/api/items/pod-001/play")
}

func TestE2E_SessionRestorePodcastNoTracksSkipsSilently(t *testing.T) {
	log := &apiLog{}
	state := &e2eServerState{
		bookmarks:      make(map[string][]abs.Bookmark),
		noTrackEpisode: map[string]bool{"pod-001|pod-001-ep-001": true},
	}
	srv := newFullMockABSServer(log, state)
	defer srv.Close()

	mp := &mockPlayer{position: 42, duration: 3600}
	m, store := newE2EModelWithDB(t, srv, mp)
	m = e2eSetSize(m, 120, 40)

	if err := store.SaveListeningSession(db.ListeningSession{
		ItemID:      "pod-001",
		EpisodeID:   "pod-001-ep-001",
		SessionID:   "sess-old-pod",
		CurrentTime: 100.0,
		Duration:    1800.0,
	}); err != nil {
		t.Fatalf("SaveListeningSession: %v", err)
	}

	res, _ := m.Update(login.LoginSuccessMsg{
		Token:     "jwt-token-e2e",
		ServerURL: srv.URL,
		Username:  "alice",
	})
	m = res.(Model)

	m = feedCmdChain(m, m.Init(), 4)

	assertScreen(t, m, ScreenHome)
	if m.sessionID != "" {
		t.Fatalf("expected restore to skip playback, got session %q", m.sessionID)
	}
	if m.err.HasError() {
		t.Fatal("expected no-track restore to skip without error banner")
	}
	if _, err := store.GetLastSession(); err == nil {
		t.Fatal("expected stale restore session to be cleared")
	}
	assertAPICallMade(t, log, http.MethodPost, "/api/items/pod-001/play/pod-001-ep-001")
	for _, req := range log.get() {
		if req.Method == http.MethodPost && req.Path == "/api/items/pod-001/play" {
			t.Fatalf("unexpected fallback book playback call: %s %s", req.Method, req.Path)
		}
	}
}
