package app

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/screens/home"
	"github.com/Thelost77/pine/internal/screens/library"
	"github.com/Thelost77/pine/internal/screens/login"
	"github.com/Thelost77/pine/internal/screens/search"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// mockPlayer implements player.Player for testing.
type mockPlayer struct {
	mu         sync.Mutex
	launched   bool
	connected  bool
	quit       bool
	position   float64
	duration   float64
	paused     bool
	launchErr  error
	connectErr error
}

func (p *mockPlayer) Launch(url, startTime, socketPath string, paused bool, httpHeaders []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.launched = true
	return p.launchErr
}
func (p *mockPlayer) Connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connected = true
	return p.connectErr
}
func (p *mockPlayer) GetPosition() (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.position, nil
}
func (p *mockPlayer) GetDuration() (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.duration, nil
}
func (p *mockPlayer) GetPaused() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused, nil
}
func (p *mockPlayer) SetPause(paused bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = paused
	return nil
}
func (p *mockPlayer) Seek(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.position = seconds
	return nil
}
func (p *mockPlayer) SetSpeed(speed float64) error { return nil }
func (p *mockPlayer) SetVolume(vol int) error      { return nil }
func (p *mockPlayer) GetVolume() (int, error)      { return 100, nil }
func (p *mockPlayer) Quit() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quit = true
	return nil
}

// newTestModel creates a model with no stored credentials (login screen).
func newTestModel() Model {
	return New(config.Default(), nil, nil)
}

// newTestModelAuthenticated creates a model with a client (home screen).
func newTestModelAuthenticated() Model {
	return New(config.Default(), nil, abs.NewClient("http://test", "tok"))
}

func TestScreenString(t *testing.T) {
	tests := []struct {
		screen Screen
		want   string
	}{
		{ScreenLogin, "Login"},
		{ScreenHome, "Home"},
		{ScreenLibrary, "Library"},
		{ScreenDetail, "Detail"},
		{ScreenSearch, "Search"},
		{ScreenSeriesList, "Series"},
		{ScreenSeries, "Series"},
		{Screen(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.screen.String(); got != tt.want {
			t.Errorf("Screen(%d).String() = %q, want %q", tt.screen, got, tt.want)
		}
	}
}

func TestNewStartsAtLogin(t *testing.T) {
	m := newTestModel()
	if m.ActiveScreen() != ScreenLogin {
		t.Errorf("New() screen = %v, want Login", m.ActiveScreen())
	}
	if len(m.BackStack()) != 0 {
		t.Errorf("New() back stack should be empty, got %v", m.BackStack())
	}
}

func TestNewStartsAtHomeWhenAuthenticated(t *testing.T) {
	m := newTestModelAuthenticated()
	if m.ActiveScreen() != ScreenHome {
		t.Errorf("New() with client screen = %v, want Home", m.ActiveScreen())
	}
	if len(m.BackStack()) != 0 {
		t.Errorf("New() back stack should be empty, got %v", m.BackStack())
	}
}

func TestNavigateMsg(t *testing.T) {
	m := newTestModel()

	// Navigate from Login → Home
	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	rm := result.(Model)

	if rm.ActiveScreen() != ScreenHome {
		t.Errorf("after navigate, screen = %v, want Home", rm.ActiveScreen())
	}
	stack := rm.BackStack()
	if len(stack) != 1 || stack[0] != ScreenLogin {
		t.Errorf("back stack = %v, want [Login]", stack)
	}
}

func TestNavigateChain(t *testing.T) {
	m := newTestModel()

	// Login → Home → Library → Detail
	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	m = result.(Model)
	result, _ = m.Update(NavigateMsg{Screen: ScreenLibrary})
	m = result.(Model)
	result, _ = m.Update(NavigateMsg{Screen: ScreenDetail})
	m = result.(Model)

	if m.ActiveScreen() != ScreenDetail {
		t.Errorf("screen = %v, want Detail", m.ActiveScreen())
	}
	stack := m.BackStack()
	if len(stack) != 3 {
		t.Fatalf("back stack length = %d, want 3", len(stack))
	}
	expected := []Screen{ScreenLogin, ScreenHome, ScreenLibrary}
	for i, s := range expected {
		if stack[i] != s {
			t.Errorf("back stack[%d] = %v, want %v", i, stack[i], s)
		}
	}
}

func TestBackMsg(t *testing.T) {
	m := newTestModel()

	// Navigate to Home, then back
	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	m = result.(Model)
	result, _ = m.Update(BackMsg{})
	m = result.(Model)

	if m.ActiveScreen() != ScreenLogin {
		t.Errorf("after back, screen = %v, want Login", m.ActiveScreen())
	}
	if len(m.BackStack()) != 0 {
		t.Errorf("after back, stack should be empty, got %v", m.BackStack())
	}
}

func TestBackEmptyStackIsNoOp(t *testing.T) {
	m := newTestModel()

	result, cmd := m.Update(BackMsg{})
	m = result.(Model)
	if cmd != nil {
		t.Fatal("back on empty stack should be no-op")
	}
}

func TestQKeyQuitsApp(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	m = result.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestEscKeyPopsBackStack(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)

	if m.ActiveScreen() != ScreenLogin {
		t.Errorf("esc on Home should go back to Login, got %v", m.ActiveScreen())
	}
}

func TestQOnLoginDoesNotQuit(t *testing.T) {
	m := newTestModel()

	// On login screen, q should be forwarded to login input, not quit
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	rm := result.(Model)

	if rm.ActiveScreen() != ScreenLogin {
		t.Errorf("q on login should stay on login, got %v", rm.ActiveScreen())
	}
	// cmd should NOT be tea.Quit
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("q on login screen should not quit")
		}
	}
}

func TestQOnSearchDoesNotQuit(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenSearch
	m.search = search.New(m.styles, m.searchCache, "lib-book", "book")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = result.(Model)

	if got := m.search.Query(); got != "q" {
		t.Fatalf("search query = %q, want %q", got, "q")
	}
	if cmd == nil {
		t.Fatal("expected typing command from search screen")
	}
	if msg := cmd(); msg != nil {
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("q on search screen should not quit")
		}
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rm := result.(Model)

	if rm.width != 120 || rm.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", rm.width, rm.height)
	}
}

func TestViewContainsHeader(t *testing.T) {
	m := newTestModel()
	view := m.View()

	if !containsString(view, "pine") {
		t.Error("View() should contain 'abs-cli' header")
	}
	if !containsString(view, "Login") {
		t.Error("View() should contain current screen name 'Login'")
	}
}

func TestViewAfterNavigate(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	rm := result.(Model)

	view := rm.View()
	if !containsString(view, "Home") {
		t.Error("View() should contain 'Home' after navigating")
	}
}

func TestDispatchToLogin(t *testing.T) {
	m := newTestModel()

	// Send a tab key which should be forwarded to login screen
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	rm := result.(Model)

	// Login model should have advanced focus from field 0 to field 1
	if rm.login.Focused() != 1 {
		t.Errorf("login focused = %d, want 1 (after tab)", rm.login.Focused())
	}
}

func TestLoginSuccessNavigatesToHome(t *testing.T) {
	m := newTestModel()

	// Simulate login success — root model intercepts and navigates to Home
	result, _ := m.Update(login.LoginSuccessMsg{Token: "tok", ServerURL: "http://test", Username: "alice"})
	rm := result.(Model)

	if rm.ActiveScreen() != ScreenHome {
		t.Errorf("after LoginSuccessMsg, screen = %v, want Home", rm.ActiveScreen())
	}
	if rm.client == nil {
		t.Error("after LoginSuccessMsg, client should be set")
	}
	if rm.login.Loading() {
		t.Error("login should not be loading after success msg")
	}
}

func TestBackStackIsCopy(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(NavigateMsg{Screen: ScreenHome})
	rm := result.(Model)

	stack := rm.BackStack()
	stack[0] = ScreenSearch // mutate the copy

	original := rm.BackStack()
	if original[0] != ScreenLogin {
		t.Error("BackStack() should return a copy, not a reference")
	}
}

func TestQFromHomeRootQuitsApp(t *testing.T) {
	// Simulate the "with stored creds" scenario where Home is the root screen.
	m := newTestModel()
	m.screen = ScreenHome
	m.backStack = nil

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q on Home with empty back stack should return tea.Quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}

func TestEscFromHomeRootIsNoOp(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenHome
	m.backStack = nil

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = result.(Model)
	if cmd != nil {
		t.Fatal("esc on Home with empty back stack should be no-op")
	}
	if m.ActiveScreen() != ScreenHome {
		t.Errorf("should stay on Home, got %v", m.ActiveScreen())
	}
}

func TestMultipleBackNavigation(t *testing.T) {
	m := newTestModel()

	// Login → Home → Library → Search
	for _, s := range []Screen{ScreenHome, ScreenLibrary, ScreenSearch} {
		result, _ := m.Update(NavigateMsg{Screen: s})
		m = result.(Model)
	}

	// Back: Search → Library → Home → Login
	expected := []Screen{ScreenLibrary, ScreenHome, ScreenLogin}
	for _, want := range expected {
		result, _ := m.Update(BackMsg{})
		m = result.(Model)
		if m.ActiveScreen() != want {
			t.Errorf("after back, screen = %v, want %v", m.ActiveScreen(), want)
		}
	}

	// One more back should be no-op (not quit)
	result, cmd := m.Update(BackMsg{})
	m = result.(Model)
	if cmd != nil {
		t.Fatal("back on empty stack should be no-op, got a command")
	}
	if m.ActiveScreen() != ScreenLogin {
		t.Errorf("should stay on Login after back on empty stack, got %v", m.ActiveScreen())
	}
}

// --- Playback lifecycle tests ---

func newPlaybackTestModel() Model {
	mp := &mockPlayer{position: 0, duration: 3600}
	cfg := config.Default()
	client := abs.NewClient("http://test", "tok")
	m := NewWithPlayer(cfg, nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}
	return m
}

func TestPlaySessionMsgSetsState(t *testing.T) {
	m := newPlaybackTestModel()

	result, cmd := m.Update(PlaySessionMsg{
		Session: PlaySessionData{
			SessionID:   "sess-123",
			ItemID:      "item-456",
			CurrentTime: 42.0,
			Duration:    3600.0,
			Title:       "Test Book",
		},
		StreamURL: "http://test/audio.mp3",
	})
	rm := result.(Model)

	if rm.sessionID != "sess-123" {
		t.Errorf("sessionID = %q, want %q", rm.sessionID, "sess-123")
	}
	if rm.itemID != "item-456" {
		t.Errorf("itemID = %q, want %q", rm.itemID, "item-456")
	}
	if rm.player.Title != "Test Book" {
		t.Errorf("player title = %q, want %q", rm.player.Title, "Test Book")
	}
	if rm.player.Position != 42.0 {
		t.Errorf("player position = %f, want 42.0", rm.player.Position)
	}
	if !rm.isPlaying() {
		t.Error("expected isPlaying to be true")
	}
	if cmd == nil {
		t.Error("expected LaunchCmd to be returned")
	}
}

func TestPlaySessionMsgResetsChapterOverlayForNewSession(t *testing.T) {
	m := newPlaybackTestModel()
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 2
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "Old One"},
		{ID: 1, Start: 60, End: 120, Title: "Old Two"},
		{ID: 2, Start: 120, End: 180, Title: "Old Three"},
	}

	result, _ := m.Update(PlaySessionMsg{
		Session: PlaySessionData{
			SessionID:   "sess-456",
			ItemID:      "item-789",
			CurrentTime: 10.0,
			Duration:    1800.0,
			Title:       "New Book",
			Chapters: []abs.Chapter{
				{ID: 10, Start: 0, End: 30, Title: "New One"},
				{ID: 11, Start: 30, End: 60, Title: "New Two"},
			},
		},
		StreamURL: "http://test/new.mp3",
	})
	rm := result.(Model)

	if rm.chapterOverlayVisible {
		t.Fatal("overlay should close for a new play session")
	}
	if rm.chapterOverlayIndex != 0 {
		t.Fatalf("overlay index = %d, want 0", rm.chapterOverlayIndex)
	}
	if len(rm.chapters) != 2 {
		t.Fatalf("chapters len = %d, want 2", len(rm.chapters))
	}
	if rm.chapters[0].Title != "New One" {
		t.Fatalf("first chapter = %q, want %q", rm.chapters[0].Title, "New One")
	}
}

func TestPlayerReadyMsgStartsTicking(t *testing.T) {
	m := newPlaybackTestModel()

	// Set up session state
	m.sessionID = "sess-123"
	m.itemID = "item-456"

	result, cmd := m.Update(player.PlayerReadyMsg{})
	_ = result.(Model)

	if cmd == nil {
		t.Error("expected batch of TickCmd + syncTickCmd")
	}
}

func TestPositionMsgUpdatesPlayer(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.player.Playing = true
	m.player.Position = 10.0

	result, _ := m.Update(player.PositionMsg{
		Position: 20.0,
		Duration: 3600.0,
		Paused:   false,
	})
	rm := result.(Model)

	if rm.player.Position != 20.0 {
		t.Errorf("player position = %f, want 20.0", rm.player.Position)
	}
	if rm.timeListened != 10.0 {
		t.Errorf("timeListened = %f, want 10.0", rm.timeListened)
	}
}

func TestSyncTickWhenNotPlaying(t *testing.T) {
	m := newPlaybackTestModel()
	// No active session
	m.sessionID = ""

	result, cmd := m.Update(SyncTickMsg{})
	_ = result.(Model)

	if cmd != nil {
		t.Error("expected no cmd when not playing")
	}
}

func TestSyncTickWhenPlaying(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-456"
	m.player.Position = 100.0
	m.player.Duration = 3600.0
	m.timeListened = 50.0

	result, cmd := m.Update(SyncTickMsg{})
	rm := result.(Model)

	if rm.lastSyncPos != 100.0 {
		t.Errorf("lastSyncPos = %f, want 100.0", rm.lastSyncPos)
	}
	if cmd == nil {
		t.Error("expected batch cmd from sync tick")
	}
}

func TestStopPlaybackClearsState(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-456"
	m.chapters = []abs.Chapter{{ID: 0, Start: 0, End: 60, Title: "One"}}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 1
	m.player.Title = "Test Book"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0
	m.timeListened = 50.0

	m, cmd := m.stopPlayback()

	if m.sessionID != "" {
		t.Errorf("sessionID should be empty, got %q", m.sessionID)
	}
	if m.itemID != "" {
		t.Errorf("itemID should be empty, got %q", m.itemID)
	}
	if m.player.Title != "" {
		t.Errorf("player title should be empty, got %q", m.player.Title)
	}
	if m.player.Playing {
		t.Error("player should not be playing")
	}
	if m.timeListened != 0 {
		t.Errorf("timeListened should be 0, got %f", m.timeListened)
	}
	if m.chapterOverlayVisible {
		t.Error("chapter overlay should be closed")
	}
	if m.chapterOverlayIndex != 0 {
		t.Errorf("chapter overlay index = %d, want 0", m.chapterOverlayIndex)
	}
	if cmd == nil {
		t.Error("expected cleanup batch cmd")
	}
}

func TestBackWhilePlayingKeepsPlayback(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-456"
	m.player.Title = "Test Book"
	m.player.Playing = true

	result, _ := m.Update(BackMsg{})
	rm := result.(Model)

	if rm.sessionID != "sess-123" {
		t.Errorf("sessionID should be preserved, got %q", rm.sessionID)
	}
	if !rm.player.Playing {
		t.Error("player should still be playing after back")
	}
	if rm.ActiveScreen() != ScreenHome {
		t.Errorf("screen = %v, want Home", rm.ActiveScreen())
	}
}

func TestCKeyOpensChapterOverlayOnlyWhenPlayingWithChapters(t *testing.T) {
	m := newPlaybackTestModel()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = result.(Model)
	if m.chapterOverlayVisible {
		t.Fatal("overlay should stay closed when not playing")
	}

	m.sessionID = "sess-123"
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = result.(Model)
	if m.chapterOverlayVisible {
		t.Fatal("overlay should stay closed without chapters")
	}

	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "One"},
		{ID: 1, Start: 60, End: 120, Title: "Two"},
	}
	m.player.Position = 70.0
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = result.(Model)
	if !m.chapterOverlayVisible {
		t.Fatal("overlay should open during active playback when chapters exist")
	}
	if m.chapterOverlayIndex != 1 {
		t.Fatalf("overlay index = %d, want 1", m.chapterOverlayIndex)
	}
}

func TestSearchScreenTypingCDoesNotOpenChapterOverlay(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenSearch
	m.search = search.New(m.styles, m.searchCache, "lib-pod", "podcast")
	m.sessionID = "sess-123"
	m.chapters = []abs.Chapter{{ID: 0, Start: 0, End: 60, Title: "One"}}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = result.(Model)

	if m.chapterOverlayVisible {
		t.Fatal("overlay should stay closed while typing in search")
	}
	if got := m.search.Query(); got != "c" {
		t.Fatalf("search query = %q, want %q", got, "c")
	}
}

func TestSearchScreenSpaceKeyDoesNotControlPlayback(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenSearch
	m.search = search.New(m.styles, m.searchCache, "lib-pod", "podcast")
	m.sessionID = "sess-123"
	m.player.Title = "Episode"
	m.player.Playing = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = result.(Model)

	if !m.player.Playing {
		t.Fatal("player should not pause while typing in search")
	}
	if cmd != nil {
		t.Fatal("space-only query should not schedule a search")
	}
	if got := m.search.Query(); got != "" {
		t.Fatalf("search query = %q, want empty", got)
	}
}

func TestSearchScreenNextInQueueKeyUpdatesQuery(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenSearch
	m.search = search.New(m.styles, m.searchCache, "lib-pod", "podcast")
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	m.queue = []QueueEntry{{
		Item: abs.LibraryItem{
			ID: "queued-book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{Title: "Queued Book"},
			},
		},
	}}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	m = result.(Model)

	if cmd == nil {
		t.Fatal("expected typing command from search screen")
	}
	if len(m.Queue()) != 1 {
		t.Fatalf("queue = %#v, want unchanged queue", m.Queue())
	}
	if got := m.search.Query(); got != ">" {
		t.Fatalf("search query = %q, want %q", got, ">")
	}
}

func TestSearchScreenSleepTimerKeyUpdatesQuery(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenSearch
	m.search = search.New(m.styles, m.searchCache, "lib-pod", "podcast")
	m.sessionID = "sess-123"
	m.player.Playing = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(Model)

	if m.sleepDuration != 0 {
		t.Fatalf("sleep duration = %v, want 0", m.sleepDuration)
	}
	if cmd == nil {
		t.Fatal("expected typing command from search screen")
	}
	if got := m.search.Query(); got != "s" {
		t.Fatalf("search query = %q, want %q", got, "s")
	}
}

func TestLibraryScreenSeriesKeyWinsDuringPlayback(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenLibrary
	m.sessionID = "sess-123"
	m.player.Playing = true
	m.library = library.New(m.styles, m.client, "lib-books", []abs.Library{{ID: "lib-books", Name: "Books", MediaType: "book"}})

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	rm := result.(Model)

	if rm.sleepDuration != 0 {
		t.Fatalf("sleep duration = %v, want 0", rm.sleepDuration)
	}
	if cmd == nil {
		t.Fatal("expected series navigation command from library key")
	}
	msg := cmd()
	nav, ok := msg.(library.NavigateSeriesListMsg)
	if !ok {
		t.Fatalf("expected NavigateSeriesListMsg, got %T", msg)
	}
	if nav.LibraryID != "lib-books" {
		t.Fatalf("library ID = %q, want lib-books", nav.LibraryID)
	}
}

func TestEscClosesChapterOverlayBeforeBackNavigation(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.chapters = []abs.Chapter{{ID: 0, Start: 0, End: 60, Title: "One"}}
	m.chapterOverlayVisible = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.chapterOverlayVisible {
		t.Fatal("overlay should close on esc")
	}
	if m.ActiveScreen() != ScreenDetail {
		t.Fatalf("screen = %v, want Detail when overlay closes", m.ActiveScreen())
	}
}

func TestJKMovesChapterOverlaySelection(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "One"},
		{ID: 1, Start: 60, End: 120, Title: "Two"},
		{ID: 2, Start: 120, End: 180, Title: "Three"},
	}
	m.chapterOverlayVisible = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = result.(Model)

	if m.chapterOverlayIndex != 1 {
		t.Fatalf("overlay index = %d, want 1", m.chapterOverlayIndex)
	}
}

func TestHLJumpToOverlayExtremes(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "One"},
		{ID: 1, Start: 60, End: 120, Title: "Two"},
		{ID: 2, Start: 120, End: 180, Title: "Three"},
	}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 1

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m = result.(Model)
	if m.chapterOverlayIndex != 2 {
		t.Fatalf("overlay index after L = %d, want 2", m.chapterOverlayIndex)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	m = result.(Model)
	if m.chapterOverlayIndex != 0 {
		t.Fatalf("overlay index after H = %d, want 0", m.chapterOverlayIndex)
	}
}

func TestEnterSeeksSelectedChapterAndClosesOverlay(t *testing.T) {
	m := newPlaybackTestModel()
	mp := m.mpv.(*mockPlayer)
	m.sessionID = "sess-123"
	m.itemID = "item-456"
	m.player.Position = 10.0
	m.player.Duration = 3600.0
	m.trackStartOffset = 0
	m.trackDuration = 3600.0
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "One"},
		{ID: 1, Start: 120, End: 180, Title: "Two"},
	}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 1

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	if m.chapterOverlayVisible {
		t.Fatal("overlay should close after selecting a chapter")
	}
	if m.player.Position != 120.0 {
		t.Fatalf("player position = %f, want 120.0", m.player.Position)
	}
	if cmd == nil {
		t.Fatal("expected seek command after selecting chapter")
	}

	_ = cmd()

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.position != 120.0 {
		t.Fatalf("mock player position = %f, want 120.0", mp.position)
	}
}

func TestPausedPositionMsgKeepsChapterOverlayOpen(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-456"
	m.playGeneration = 1
	m.player.Position = 42.0
	m.player.Duration = 3600.0
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "One"},
		{ID: 1, Start: 60, End: 120, Title: "Two"},
	}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 1

	result, _ := m.Update(player.PositionMsg{
		Position:   42.0,
		Duration:   3600.0,
		Paused:     true,
		Generation: 1,
	})
	m = result.(Model)

	if !m.chapterOverlayVisible {
		t.Fatal("overlay should stay open on pause-only updates")
	}
	if m.chapterOverlayIndex != 1 {
		t.Fatalf("overlay index = %d, want 1", m.chapterOverlayIndex)
	}
	if m.player.Playing {
		t.Fatal("player should reflect paused state")
	}
}

func TestNavigateWhilePlayingKeepsPlayback(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-456"
	m.player.Title = "Test Book"
	m.player.Playing = true

	result, _ := m.Update(NavigateMsg{Screen: ScreenLibrary})
	rm := result.(Model)

	if rm.sessionID != "sess-123" {
		t.Errorf("sessionID should be preserved, got %q", rm.sessionID)
	}
	if !rm.player.Playing {
		t.Error("player should still be playing after navigate")
	}
	if rm.ActiveScreen() != ScreenLibrary {
		t.Errorf("screen = %v, want Library", rm.ActiveScreen())
	}
}

func TestPlayCmdWithNoClient(t *testing.T) {
	m := newPlaybackTestModel()
	m.client = nil

	dur := 3600.0
	result, cmd := m.Update(detail.PlayCmd{Item: abs.LibraryItem{
		ID: "item-1",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Test", Duration: &dur},
		},
	}})
	_ = result.(Model)

	if cmd != nil {
		t.Error("expected no cmd when client is nil")
	}
}

func TestPlayCmdTogglesPauseWhilePlaying(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-old"
	m.player.Playing = true

	dur := 3600.0
	result, cmd := m.Update(detail.PlayCmd{Item: abs.LibraryItem{
		ID: "item-old", // Same item ID triggers toggle
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "New Book", Duration: &dur},
		},
	}})
	rm := result.(Model)

	if rm.sessionID != "sess-123" {
		t.Errorf("sessionID should be unchanged, got %q", rm.sessionID)
	}
	if rm.itemID != "item-old" {
		t.Errorf("itemID should be unchanged, got %q", rm.itemID)
	}
	if rm.player.Playing {
		t.Error("player should be paused after toggle")
	}
	if cmd == nil {
		t.Error("expected pause toggle command")
	}
}

func TestPlayEpisodeCmdTogglesPauseWhilePlaying(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-old"
	m.episodeID = "ep-old"
	m.player.Playing = true

	result, cmd := m.Update(detail.PlayEpisodeCmd{
		Item:    abs.LibraryItem{ID: "item-old"}, // Same item ID triggers toggle
		Episode: abs.PodcastEpisode{ID: "ep-old", Title: "New Episode", Duration: 1800},
	})
	rm := result.(Model)

	if rm.sessionID != "sess-123" {
		t.Errorf("sessionID should be unchanged, got %q", rm.sessionID)
	}
	if rm.episodeID != "ep-old" {
		t.Errorf("episodeID should be unchanged, got %q", rm.episodeID)
	}
	if rm.player.Playing {
		t.Error("player should be paused after toggle")
	}
	if cmd == nil {
		t.Error("expected pause toggle command")
	}
}

func TestAddToQueueCmdAppendsEntry(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	item := abs.LibraryItem{
		ID:        "item-queue",
		MediaType: "book",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Queued Book"},
		},
	}

	result, cmd := m.Update(detail.AddToQueueCmd{Item: item})
	rm := result.(Model)

	if cmd != nil {
		t.Fatal("expected no command when adding to queue")
	}
	if len(rm.Queue()) != 1 {
		t.Fatalf("expected queue length 1, got %d", len(rm.Queue()))
	}
	if rm.Queue()[0].Item.ID != "item-queue" {
		t.Fatalf("expected queued item item-queue, got %s", rm.Queue()[0].Item.ID)
	}
	if rm.Queue()[0].Episode != nil {
		t.Fatal("expected queued book entry to have no episode")
	}
}

func TestPlayNextCmdPrependsEntry(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	first := QueueEntry{
		Item: abs.LibraryItem{
			ID:        "item-old",
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Old Queue"}},
		},
	}
	m.queue = []QueueEntry{first}
	nextEpisode := abs.PodcastEpisode{ID: "ep-1", Title: "Episode 1", Duration: 1800}
	item := abs.LibraryItem{
		ID:        "pod-1",
		MediaType: "podcast",
		Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Podcast"}},
	}

	result, cmd := m.Update(detail.PlayNextCmd{Item: item, Episode: &nextEpisode})
	rm := result.(Model)

	if cmd != nil {
		t.Fatal("expected no command when queueing play next")
	}
	if len(rm.Queue()) != 2 {
		t.Fatalf("expected queue length 2, got %d", len(rm.Queue()))
	}
	if rm.Queue()[0].Item.ID != "pod-1" {
		t.Fatalf("expected play-next item at head, got %s", rm.Queue()[0].Item.ID)
	}
	if rm.Queue()[0].Episode == nil || rm.Queue()[0].Episode.ID != "ep-1" {
		t.Fatal("expected episode payload at queue head")
	}
	if rm.Queue()[1].Item.ID != "item-old" {
		t.Fatalf("expected original queue entry to remain second, got %s", rm.Queue()[1].Item.ID)
	}
}

func TestHomeAddToQueueMsgAppendsEntry(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	item := abs.LibraryItem{
		ID: "book-home",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Home Queue"},
		},
	}

	result, cmd := m.Update(home.AddToQueueMsg{Item: item})
	rm := result.(Model)

	if cmd != nil {
		t.Fatal("expected no command when enqueueing from home")
	}
	if len(rm.Queue()) != 1 || rm.Queue()[0].Item.ID != "book-home" {
		t.Fatalf("queue = %#v, want appended home item", rm.Queue())
	}
}

func TestHomePlayNextMsgPrependsEntry(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	m.queue = []QueueEntry{{Item: abs.LibraryItem{ID: "existing"}}}
	item := abs.LibraryItem{
		ID: "book-home-next",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Home Next"},
		},
	}

	result, cmd := m.Update(home.PlayNextMsg{Item: item})
	rm := result.(Model)

	if cmd != nil {
		t.Fatal("expected no command when prepending from home")
	}
	queue := rm.Queue()
	if len(queue) != 2 || queue[0].Item.ID != "book-home-next" {
		t.Fatalf("queue = %#v, want home-next at front", queue)
	}
}

func TestAddToQueueCmdStartsPlaybackWhenIdle(t *testing.T) {
	m := newPlaybackTestModel()
	item := abs.LibraryItem{
		ID: "idle-detail-queue",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Idle Detail Queue"},
		},
	}

	result, cmd := m.Update(detail.AddToQueueCmd{Item: item})
	rm := result.(Model)

	if cmd == nil {
		t.Fatal("expected play command when idle")
	}
	if len(rm.Queue()) != 0 {
		t.Fatalf("queue = %#v, want empty when add-to-queue starts immediately", rm.Queue())
	}
}

func TestHomeAddToQueueMsgStartsPlaybackWhenIdle(t *testing.T) {
	m := newPlaybackTestModel()
	item := abs.LibraryItem{
		ID: "idle-home-queue",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Idle Home Queue"},
		},
	}

	result, cmd := m.Update(home.AddToQueueMsg{Item: item})
	rm := result.(Model)

	if cmd == nil {
		t.Fatal("expected play command when idle")
	}
	if len(rm.Queue()) != 0 {
		t.Fatalf("queue = %#v, want empty when home add-to-queue starts immediately", rm.Queue())
	}
}

func TestNextInQueueKeyStartsQueuedItem(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	m.queue = []QueueEntry{
		{
			Item: abs.LibraryItem{
				ID: "queued-book",
				Media: abs.Media{
					Metadata: abs.MediaMetadata{Title: "Queued Book"},
				},
			},
		},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	rm := result.(Model)

	if cmd == nil {
		t.Fatal("expected play command when skipping to queued item")
	}
	if len(rm.Queue()) != 0 {
		t.Fatalf("queue = %#v, want consumed head after skip", rm.Queue())
	}
	if rm.sessionID != "" {
		t.Fatalf("sessionID = %q, want cleared before next playback starts", rm.sessionID)
	}
}

func TestNextInQueueKeyDoesNothingWithoutQueue(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	rm := result.(Model)

	if cmd != nil {
		t.Fatal("expected no command when queue is empty")
	}
	if !rm.player.Playing {
		t.Fatal("expected current playback to remain active")
	}
}

func TestNavigateSeriesMsgOpensSeriesScreen(t *testing.T) {
	m := newPlaybackTestModel()

	result, cmd := m.Update(detail.NavigateSeriesMsg{
		LibraryID:     "lib-books-001",
		SeriesID:      "series-expanse",
		CurrentItemID: "item-old",
	})
	rm := result.(Model)

	if rm.ActiveScreen() != ScreenSeries {
		t.Fatalf("screen = %v, want Series", rm.ActiveScreen())
	}
	if cmd == nil {
		t.Fatal("expected init command when navigating to series screen")
	}
}

func TestNavigateSeriesListMsgOpensSeriesListScreen(t *testing.T) {
	m := newPlaybackTestModel()

	result, cmd := m.Update(library.NavigateSeriesListMsg{
		LibraryID:   "lib-books-001",
		LibraryName: "Books",
	})
	rm := result.(Model)

	if rm.ActiveScreen() != ScreenSeriesList {
		t.Fatalf("screen = %v, want Series", rm.ActiveScreen())
	}
	if cmd == nil {
		t.Fatal("expected init command when navigating to series list screen")
	}
}

func TestBookDetailLoadedMsgUpdatesDetailItem(t *testing.T) {
	m := newPlaybackTestModel()
	m.detail = detail.New(m.styles, abs.LibraryItem{
		ID:        "li-001",
		MediaType: "book",
		Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Caliban's War"}},
	})

	result, cmd := m.Update(BookDetailLoadedMsg{
		Item: &abs.LibraryItem{
			ID:        "li-001",
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
	})
	rm := result.(Model)

	if cmd != nil {
		t.Fatal("expected no command from BookDetailLoadedMsg")
	}
	if rm.detail.Item().Media.Metadata.Series == nil {
		t.Fatal("expected detail item to be enriched with series data")
	}
	if rm.detail.Item().Media.Metadata.Series.Name != "The Expanse" {
		t.Fatalf("series name = %q, want The Expanse", rm.detail.Item().Media.Metadata.Series.Name)
	}
}

func TestManualPlayPreservesQueue(t *testing.T) {
	m := newPlaybackTestModel()
	m.queue = []QueueEntry{
		{
			Item: abs.LibraryItem{
				ID:        "queued-item",
				MediaType: "book",
				Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Queued Book"}},
			},
		},
	}

	dur := 3600.0
	result, _ := m.Update(detail.PlayCmd{Item: abs.LibraryItem{
		ID: "item-now",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "Play Now", Duration: &dur},
		},
	}})
	rm := result.(Model)

	if len(rm.Queue()) != 1 {
		t.Fatalf("expected queue length 1 after manual play, got %d", len(rm.Queue()))
	}
	if rm.Queue()[0].Item.ID != "queued-item" {
		t.Fatalf("expected queued-item to remain queued, got %s", rm.Queue()[0].Item.ID)
	}
}

func TestViewIncludesPlayerWhenActive(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.player.Title = "My Book"
	m.player.Playing = true
	m.player.Position = 60
	m.player.Duration = 3600
	m.player.Speed = 1.0
	m.width = 80
	m.height = 24

	view := m.View()
	if !containsString(view, "My Book") {
		t.Error("expected player bar with title in view")
	}
}

func TestViewNoPlayerWhenInactive(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24

	view := m.View()
	// Player bar should be empty when no title
	if containsString(view, "▶") {
		t.Error("expected no player icon when inactive")
	}
}

func TestPositionMsgTracksTimeListened(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.player.Position = 100.0
	m.timeListened = 0

	// Position advances by 5 seconds
	result, _ := m.Update(player.PositionMsg{Position: 105.0, Duration: 3600.0, Paused: false})
	rm := result.(Model)
	if rm.timeListened != 5.0 {
		t.Errorf("timeListened = %f, want 5.0", rm.timeListened)
	}

	// Position advances by 3 more seconds
	result, _ = rm.Update(player.PositionMsg{Position: 108.0, Duration: 3600.0, Paused: false})
	rm = result.(Model)
	if rm.timeListened != 8.0 {
		t.Errorf("timeListened = %f, want 8.0", rm.timeListened)
	}
}

func TestPositionMsgBackwardSeekDoesNotAddTime(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.player.Position = 100.0
	m.timeListened = 5.0

	// Seek backward should not add time
	result, _ := m.Update(player.PositionMsg{Position: 80.0, Duration: 3600.0, Paused: false})
	rm := result.(Model)
	if rm.timeListened != 5.0 {
		t.Errorf("timeListened = %f, want 5.0 (unchanged after backward)", rm.timeListened)
	}
}

func TestPlaybackErrorMsgIsHandled(t *testing.T) {
	m := newPlaybackTestModel()
	result, cmd := m.Update(PlaybackErrorMsg{Err: nil})
	_ = result.(Model)
	if cmd != nil {
		t.Error("expected no cmd from PlaybackErrorMsg")
	}
}

func TestPlayerLaunchErrClearsSession(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"

	result, _ := m.Update(player.PlayerLaunchErrMsg{Err: nil})
	rm := result.(Model)
	if rm.sessionID != "" {
		t.Errorf("sessionID should be cleared on launch error, got %q", rm.sessionID)
	}
}

// --- Cleanup tests ---

func TestCleanupWhenPlaying(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-abc"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 200.0
	m.player.Duration = 3600.0
	m.timeListened = 30.0

	m.Cleanup()

	if !mp.quit {
		t.Error("expected mpv to be quit during Cleanup")
	}

	reqs := log.get()
	var closeCalled, progressCalled bool
	for _, r := range reqs {
		if r.Method == "POST" && r.Path == "/api/session/sess-abc/close" {
			closeCalled = true
		}
		if r.Method == "PATCH" && r.Path == "/api/me/progress/item-001" {
			progressCalled = true
		}
	}
	if !closeCalled {
		t.Error("expected CloseSession API call during Cleanup")
	}
	if !progressCalled {
		t.Error("expected UpdateProgress API call during Cleanup")
	}
}

func TestCleanupWhenNotPlaying(t *testing.T) {
	mp := &mockPlayer{}
	m := NewWithPlayer(config.Default(), nil, nil, mp)
	m.sessionID = "" // not playing

	// Should still quit mpv (safety net for orphaned processes),
	// but skip session/progress cleanup.
	m.Cleanup()

	if !mp.quit {
		t.Error("mpv should be quit even when session is cleared (safety net)")
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

func TestStaleEpisodesLoadedMsgDiscarded(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenDetail
	m.detail = detail.New(ui.NewStyles(config.Default().Theme), abs.LibraryItem{
		ID:        "item-current",
		MediaType: "podcast",
		Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Current Podcast"}},
	})

	staleMsg := EpisodesLoadedMsg{
		ItemID:   "item-stale",
		Episodes: []abs.PodcastEpisode{{ID: "ep-stale", Title: "Stale Episode"}},
	}
	result, _ := m.Update(staleMsg)
	rm := result.(Model)
	if len(rm.detail.Episodes()) != 0 {
		t.Error("stale EpisodesLoadedMsg should be discarded")
	}

	freshMsg := EpisodesLoadedMsg{
		ItemID:   "item-current",
		Episodes: []abs.PodcastEpisode{{ID: "ep-fresh", Title: "Fresh Episode"}},
	}
	result, _ = rm.Update(freshMsg)
	rm = result.(Model)
	if len(rm.detail.Episodes()) != 1 {
		t.Error("fresh EpisodesLoadedMsg should be applied")
	}
	if rm.detail.Episodes()[0].ID != "ep-fresh" {
		t.Error("wrong episode applied")
	}
}

func TestStaleBookDetailLoadedMsgDiscarded(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenDetail
	m.detail = detail.New(ui.NewStyles(config.Default().Theme), abs.LibraryItem{
		ID:        "item-current",
		MediaType: "book",
		Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Current Book"}},
	})

	staleMsg := BookDetailLoadedMsg{
		ItemID: "item-stale",
		Item: &abs.LibraryItem{
			ID:        "item-stale",
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Stale Book"}},
		},
	}
	result, _ := m.Update(staleMsg)
	rm := result.(Model)
	if rm.detail.Item().ID != "item-current" {
		t.Error("stale BookDetailLoadedMsg should be discarded")
	}

	freshMsg := BookDetailLoadedMsg{
		ItemID: "item-current",
		Item: &abs.LibraryItem{
			ID:        "item-current",
			MediaType: "book",
			Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Enriched Book"}},
		},
	}
	result, _ = rm.Update(freshMsg)
	rm = result.(Model)
	if rm.detail.Item().Media.Metadata.Title != "Enriched Book" {
		t.Error("fresh BookDetailLoadedMsg should update detail item")
	}
}

func TestUnauthorizedPlaybackErrorRedirectsToLogin(t *testing.T) {
	m := newTestModelAuthenticated()
	m.screen = ScreenHome

	result, cmd := m.Update(PlaybackErrorMsg{
		Err: fmt.Errorf("unexpected status 401: unauthorized"),
	})
	rm := result.(Model)
	if rm.screen != ScreenLogin {
		t.Errorf("screen = %v, want ScreenLogin after 401 playback error", rm.screen)
	}
	if rm.client != nil {
		t.Error("client should be nil after 401 redirect")
	}
	if cmd == nil {
		t.Error("expected login init command after 401 redirect")
	}
}

func TestUnauthorizedBookDetailErrorRedirectsToLogin(t *testing.T) {
	m := newTestModelAuthenticated()
	m.screen = ScreenDetail

	result, cmd := m.Update(BookDetailLoadedMsg{
		ItemID: "item-1",
		Err:    fmt.Errorf("unauthorized status 401"),
	})
	rm := result.(Model)
	if rm.screen != ScreenLogin {
		t.Errorf("screen = %v, want ScreenLogin after 401 book detail error", rm.screen)
	}
	if rm.client != nil {
		t.Error("client should be nil after 401 redirect")
	}
	if cmd == nil {
		t.Error("expected login init command after 401 redirect")
	}
}

func TestNormalPlaybackErrorShowsBanner(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenHome

	result, cmd := m.Update(PlaybackErrorMsg{
		Err: fmt.Errorf("network error"),
	})
	rm := result.(Model)
	if rm.screen != ScreenHome {
		t.Errorf("screen = %v, want ScreenHome for non-401 error", rm.screen)
	}
	if !rm.err.HasError() {
		t.Error("non-401 playback error should show error banner")
	}
	if cmd == nil {
		t.Error("expected auto-dismiss cmd for error banner")
	}
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Error banner tests ---

func TestErrMsgShowsBanner(t *testing.T) {
	m := newTestModelAuthenticated()
	m.width = 80
	m.height = 24

	result, cmd := m.Update(components.ErrMsg{Err: fmt.Errorf("API connection failed")})
	rm := result.(Model)

	if !rm.err.HasError() {
		t.Error("expected error banner to be visible")
	}
	view := rm.View()
	if !containsString(view, "API connection failed") {
		t.Error("view should contain error message")
	}
	if cmd == nil {
		t.Error("expected auto-dismiss command")
	}
}

func TestErrMsgNilIsNoOp(t *testing.T) {
	m := newTestModelAuthenticated()
	result, cmd := m.Update(components.ErrMsg{Err: nil})
	rm := result.(Model)

	if rm.err.HasError() {
		t.Error("nil error should not show banner")
	}
	if cmd != nil {
		t.Error("nil error should return nil cmd")
	}
}

func TestErrorDismissOnKeypress(t *testing.T) {
	m := newTestModelAuthenticated()
	result, _ := m.Update(components.ErrMsg{Err: fmt.Errorf("test error")})
	m = result.(Model)

	if !m.err.HasError() {
		t.Fatal("error should be set before keypress test")
	}

	// Any keypress should dismiss
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = result.(Model)

	if m.err.HasError() {
		t.Error("error should be dismissed after keypress")
	}
}

func TestErrorDismissMsg(t *testing.T) {
	m := newTestModelAuthenticated()
	result, _ := m.Update(components.ErrMsg{Err: fmt.Errorf("test error")})
	m = result.(Model)

	result, _ = m.Update(components.ErrorDismissMsg{})
	m = result.(Model)

	if m.err.HasError() {
		t.Error("error should be dismissed after ErrorDismissMsg")
	}
}

func TestHTTP401RedirectsToLogin(t *testing.T) {
	m := newTestModelAuthenticated()
	if m.ActiveScreen() != ScreenHome {
		t.Fatal("should start at Home")
	}

	result, _ := m.Update(components.ErrMsg{Err: fmt.Errorf("unexpected status 401: Unauthorized")})
	rm := result.(Model)

	if rm.ActiveScreen() != ScreenLogin {
		t.Errorf("401 should redirect to login, got %v", rm.ActiveScreen())
	}
	if rm.client != nil {
		t.Error("client should be nil after 401")
	}
}

func TestPlaybackErrorMsgShowsBanner(t *testing.T) {
	m := newPlaybackTestModel()
	result, cmd := m.Update(PlaybackErrorMsg{Err: fmt.Errorf("no audio tracks")})
	rm := result.(Model)

	if !rm.err.HasError() {
		t.Error("PlaybackErrorMsg should show error banner")
	}
	if cmd == nil {
		t.Error("expected auto-dismiss cmd")
	}
}

func TestPlayerLaunchErrShowsBanner(t *testing.T) {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.chapters = []abs.Chapter{{ID: 0, Start: 0, End: 60, Title: "One"}}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 1

	result, cmd := m.Update(player.PlayerLaunchErrMsg{Err: fmt.Errorf("mpv not found")})
	rm := result.(Model)

	if rm.sessionID != "" {
		t.Error("session should be cleared")
	}
	if rm.chapterOverlayVisible {
		t.Error("chapter overlay should be closed")
	}
	if rm.chapterOverlayIndex != 0 {
		t.Errorf("chapter overlay index = %d, want 0", rm.chapterOverlayIndex)
	}
	if len(rm.chapters) != 0 {
		t.Errorf("chapters len = %d, want 0", len(rm.chapters))
	}
	if !rm.err.HasError() {
		t.Error("launch error should show banner")
	}
	view := rm.View()
	if !containsString(view, "mpv not found") {
		t.Error("view should contain mpv error")
	}
	if !containsString(view, "Install mpv") {
		t.Error("view should contain install hint for mpv errors")
	}
	if cmd == nil {
		t.Error("expected auto-dismiss cmd")
	}
}
