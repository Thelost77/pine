package app

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/screens/home"
	tea "github.com/charmbracelet/bubbletea"
)

func newSessionsTestModel(t *testing.T) (Model, *apiLog, *mockPlayer) {
	t.Helper()
	log := &apiLog{}
	srv := newFullMockABSServer(log, &e2eServerState{bookmarks: make(map[string][]abs.Bookmark)})
	t.Cleanup(srv.Close)

	mp := &mockPlayer{position: 50, duration: 3600}
	m := newE2EModelAuthenticated(srv, mp)
	return e2eSetSize(m, 120, 40), log, mp
}

// playBook sets up the model as if the book item-001 plays.
func playBook(m Model) Model {
	m.sessionID = "sess-other"
	m.itemID = "item-001"
	m.player.Title = "The Great Gatsby"
	m.player.Playing = true
	m.player.Position = 50.0
	m.player.Duration = 3600.0
	return m
}

// TestE2E_SessionsKeySeeksItemThatPlays verifies that R shows the sessions of
// the book that plays, and that enter on a session of that book seeks to the
// end of the session with no new play session.
func TestE2E_SessionsKeySeeksItemThatPlays(t *testing.T) {
	m, log, mp := newSessionsTestModel(t)
	m = playBook(m)
	start := m.screen

	m, cmd := e2ePressKey(m, 'R')
	assertScreen(t, m, ScreenSessions)
	m, _ = feedCmd(m, cmd)
	assertAPICallMade(t, log, "GET", "/api/me/item/listening-sessions/item-001")
	if view := m.View(); !strings.Contains(view, "15:00  from 10:00 · Pine desktop") {
		t.Fatalf("view has no session of item-001:\n%s", view)
	}

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 2)

	assertScreen(t, m, start)
	assertNoAPICall(t, log, "POST", "/api/items/item-001/play")
	if m.sessionID != "sess-other" {
		t.Fatalf("sessionID = %q, want sess-other", m.sessionID)
	}
	mp.mu.Lock()
	pos := mp.position
	mp.mu.Unlock()
	if pos != 900.0 {
		t.Fatalf("mock player position = %f, want 900.0 (end of the session)", pos)
	}
}

// TestE2E_SessionsKeyOnPodcastDetailPlaysEpisode verifies that R on the detail
// screen of a podcast shows the sessions of the episode under the cursor, and
// that enter stops the book that plays and plays that episode from the end of
// the session.
func TestE2E_SessionsKeyOnPodcastDetailPlaysEpisode(t *testing.T) {
	m, log, mp := newSessionsTestModel(t)
	m = playBook(m)

	res, cmd := m.Update(home.NavigateDetailMsg{Item: testPodcastItem("pod-001", "Tech Talk Daily")})
	m = feedCmdChain(res.(Model), cmd, 5)
	assertScreen(t, m, ScreenDetail)

	m, cmd = e2ePressKey(m, 'R')
	assertScreen(t, m, ScreenSessions)
	m, _ = feedCmd(m, cmd)
	assertAPICallMade(t, log, "GET", "/api/me/item/listening-sessions/pod-001/pod-001-ep-001")

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 2)

	assertScreen(t, m, ScreenDetail)
	assertAPICallMade(t, log, "POST", "/api/session/sess-other/close")
	assertAPICallMade(t, log, "POST", "/api/items/pod-001/play/pod-001-ep-001")
	if m.itemID != "pod-001" || m.episodeID != "pod-001-ep-001" {
		t.Fatalf("playing %q/%q, want pod-001/pod-001-ep-001", m.itemID, m.episodeID)
	}
	if m.player.Position != 420.0 {
		t.Fatalf("player position = %f, want 420.0 (end of the session)", m.player.Position)
	}
	if !mp.launched {
		t.Fatal("expected player launch after enter on a session")
	}
}

// TestE2E_SessionsKeyWithNothingPlayingShowsAllItems verifies that R with no
// item that plays shows the sessions of all items, and that enter plays the
// book of the selected session from the end of the session.
func TestE2E_SessionsKeyWithNothingPlayingShowsAllItems(t *testing.T) {
	m, log, mp := newSessionsTestModel(t)

	m, cmd := e2ePressKey(m, 'R')
	assertScreen(t, m, ScreenSessions)
	m, _ = feedCmd(m, cmd)
	assertAPICallMade(t, log, "GET", "/api/me/listening-sessions")
	view := m.View()
	for _, want := range []string{"Listening History", "1984", "Episode 1 - Pilot", "The Great Gatsby"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view has no %q:\n%s", want, view)
		}
	}

	m, cmd = e2ePressSpecial(m, tea.KeyEnter)
	m, cmd = feedCmd(m, cmd)
	m, cmd = feedCmd(m, cmd)
	m = feedCmdChain(m, cmd, 2)

	assertAPICallMade(t, log, "GET", "/api/items/item-002")
	assertAPICallMade(t, log, "POST", "/api/items/item-002/play")
	if m.itemID != "item-002" {
		t.Fatalf("itemID = %q, want item-002", m.itemID)
	}
	if m.player.Position != 300.0 {
		t.Fatalf("player position = %f, want 300.0 (end of the session)", m.player.Position)
	}
	if !mp.launched {
		t.Fatal("expected player launch after enter on a session")
	}
}

// TestE2E_SessionsKeyOnPodcastDetailWithNoEpisodeShowsError verifies that R
// on the detail screen of a podcast with no episode under the cursor stays on
// the detail screen and tells the user to select an episode. The server has
// no sessions for a whole podcast.
func TestE2E_SessionsKeyOnPodcastDetailWithNoEpisodeShowsError(t *testing.T) {
	m, log, _ := newSessionsTestModel(t)

	res, cmd := m.Update(home.NavigateDetailMsg{Item: testPodcastItem("pod-001", "Tech Talk Daily")})
	m = feedCmdChain(res.(Model), cmd, 5)
	m, _ = e2ePressSpecial(m, tea.KeyTab)
	if _, _, ok := m.detail.PlayTarget(); ok {
		t.Fatal("expected no episode under the cursor after Tab")
	}

	m, _ = e2ePressKey(m, 'R')

	assertScreen(t, m, ScreenDetail)
	assertNoAPICall(t, log, "GET", "/api/me/item/listening-sessions/")
	if view := m.View(); !strings.Contains(view, "select an episode to see its listening sessions") {
		t.Fatalf("view has no error:\n%s", view)
	}
}

// TestPaletteListeningSessionsOfDetailTarget verifies that the context entry
// of the detail screen opens the sessions of the episode under the cursor.
func TestPaletteListeningSessionsOfDetailTarget(t *testing.T) {
	m, log, _ := newSessionsTestModel(t)

	res, cmd := m.Update(home.NavigateDetailMsg{Item: testPodcastItem("pod-001", "Tech Talk Daily")})
	m = feedCmdChain(res.(Model), cmd, 5)

	var found bool
	for _, item := range m.buildContextPaletteItems() {
		if item.Label != "Listening Sessions" {
			continue
		}
		found = true
		m, cmd = m.handlePaletteAction(item.Action, item.Payload, item.LibraryID, item.ItemID, item.Data)
	}
	if !found {
		t.Fatal("detail palette has no Listening Sessions entry")
	}
	assertScreen(t, m, ScreenSessions)
	_, _ = feedCmd(m, cmd)
	assertAPICallMade(t, log, "GET", "/api/me/item/listening-sessions/pod-001/pod-001-ep-001")
}
