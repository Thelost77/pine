package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Thelost77/spruce/internal/jellyfin"
	"github.com/Thelost77/spruce/internal/player"
	"github.com/Thelost77/spruce/internal/screens/library"
	"github.com/Thelost77/spruce/internal/screens/login"
	tea "github.com/charmbracelet/bubbletea"
)

type pagedResponse[T any] struct {
	Items            []T `json:"Items"`
	TotalRecordCount int `json:"TotalRecordCount,omitempty"`
}

type recordedReq struct {
	Method string
	Path   string
	Query  string
	Body   string
}

type mockJellyfin struct {
	*httptest.Server

	mu             sync.Mutex
	requests       []recordedReq
	startedBodies  []string
	stoppedBodies  []string
	progressCount  int
	totalTracks    int
	libTrackChunks [][]jellyfin.Track
}

func newMockJellyfin(t *testing.T, totalTracks int) *mockJellyfin {
	t.Helper()
	mj := &mockJellyfin{totalTracks: totalTracks}

	track := func(i int) jellyfin.Track {
		return jellyfin.Track{
			ID:           "trk-" + strconv.Itoa(i),
			Name:         "Track " + strconv.Itoa(i),
			RunTimeTicks: int64(180) * 1e7,
			Artists:      []string{"Artist A"},
			Album:        "Album One",
			AlbumID:      "alb-1",
		}
	}

	all := make([]jellyfin.Track, totalTracks)
	for i := range all {
		all[i] = track(i)
	}
	pageSize := 200
	for start := 0; start < totalTracks; start += pageSize {
		end := start + pageSize
		if end > totalTracks {
			end = totalTracks
		}
		mj.libTrackChunks = append(mj.libTrackChunks, all[start:end])
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
		}
		mj.mu.Lock()
		mj.requests = append(mj.requests, recordedReq{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body,
		})
		mj.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/Users/usr-1/Views":
			_ = json.NewEncoder(w).Encode(pagedResponse[jellyfin.Library]{
				Items: []jellyfin.Library{{ID: "lib-1", Name: "Music", CollectionType: "music"}},
			})
			return

		case r.URL.Path == "/Users/usr-1/Items":
			itemTypes := r.URL.Query().Get("IncludeItemTypes")
			parentID := r.URL.Query().Get("ParentId")
			startIdx, _ := strconv.Atoi(r.URL.Query().Get("StartIndex"))

			switch {
			case itemTypes == "MusicAlbum" && parentID == "lib-1":
				_ = json.NewEncoder(w).Encode(pagedResponse[jellyfin.Album]{
					Items:            []jellyfin.Album{{ID: "alb-1", Name: "Album One", ProductionYear: 2020, Artists: []string{"Artist A"}}},
					TotalRecordCount: 1,
				})
			case itemTypes == "MusicAlbum" && parentID == "art-1":
				_ = json.NewEncoder(w).Encode(pagedResponse[jellyfin.Album]{
					Items:            []jellyfin.Album{{ID: "alb-1", Name: "Album One", ProductionYear: 2020}},
					TotalRecordCount: 1,
				})
			case itemTypes == "Audio" && parentID == "alb-1":
				_ = json.NewEncoder(w).Encode(pagedResponse[jellyfin.Track]{
					Items:            []jellyfin.Track{track(0), track(1)},
					TotalRecordCount: 2,
				})
			case itemTypes == "Audio" && parentID == "lib-1":
				chunkIdx := startIdx / 200
				if chunkIdx < 0 || chunkIdx >= len(mj.libTrackChunks) {
					http.Error(w, "out of range", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(pagedResponse[jellyfin.Track]{
					Items:            mj.libTrackChunks[chunkIdx],
					TotalRecordCount: mj.totalTracks,
				})
			default:
				http.NotFound(w, r)
			}
			return

		case strings.HasPrefix(r.URL.Path, "/Audio/") && strings.HasSuffix(r.URL.Path, "/stream"):
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("FAKEAUDIO"))
			return

		case r.URL.Path == "/Sessions/Playing" && r.Method == http.MethodPost:
			mj.mu.Lock()
			mj.startedBodies = append(mj.startedBodies, body)
			mj.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return

		case r.URL.Path == "/Sessions/Playing/Progress" && r.Method == http.MethodPost:
			mj.mu.Lock()
			mj.progressCount++
			mj.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return

		case r.URL.Path == "/Sessions/Playing/Stopped" && r.Method == http.MethodPost:
			mj.mu.Lock()
			mj.stoppedBodies = append(mj.stoppedBodies, body)
			mj.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return

		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	mj.Server = httptest.NewServer(mux)
	t.Cleanup(mj.Server.Close)
	return mj
}

func (mj *mockJellyfin) stoppedCalls() []string {
	mj.mu.Lock()
	defer mj.mu.Unlock()
	cp := make([]string, len(mj.stoppedBodies))
	copy(cp, mj.stoppedBodies)
	return cp
}

func (mj *mockJellyfin) startedCalls() []string {
	mj.mu.Lock()
	defer mj.mu.Unlock()
	cp := make([]string, len(mj.startedBodies))
	copy(cp, mj.startedBodies)
	return cp
}

func (mj *mockJellyfin) itemsRequestCount() int {
	mj.mu.Lock()
	defer mj.mu.Unlock()
	n := 0
	for _, r := range mj.requests {
		if r.Path == "/Users/usr-1/Items" {
			n++
		}
	}
	return n
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func keySpecial(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func update(m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	res, cmd := m.Update(msg)
	return res.(*Model), cmd
}

// execCmd runs a cmd, feeds non-nil messages back into Update, and recurses
// through BatchMsg. tea.QuitMsg and tea.Tick messages are not fed back.
func execCmd(m *Model, cmd tea.Cmd) (*Model, tea.Cmd) {
	if cmd == nil {
		return m, nil
	}
	msg := cmd()
	if msg == nil {
		return m, nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var last tea.Cmd
		for _, c := range batch {
			m, last = execCmd(m, c)
		}
		return m, last
	}
	if _, ok := msg.(tea.QuitMsg); ok {
		return m, nil
	}
	return update(m, msg)
}

// driveToPlayback logs in, loads the music library, and starts playback of the
// given tracks at startIndex, executing any HTTP side-effect cmds.
func driveToPlayback(mj *mockJellyfin, tracks []jellyfin.Track, startIndex int) func(*testing.T, *Model) (*Model, tea.Cmd) {
	return func(t *testing.T, m *Model) (*Model, tea.Cmd) {
		m, _ = update(m, login.LoginSuccessMsg{
			Token: "tok-1", ServerURL: mj.URL, Username: "alice", UserID: "usr-1",
		})
		m, _ = update(m, musicLibrariesLoadedMsg{
			libraries: []jellyfin.Library{{ID: "lib-1", Name: "Music", CollectionType: "music"}},
		})
		m, cmd := update(m, library.PlayTracksMsg{Tracks: tracks, StartIndex: startIndex})
		m, _ = execCmd(m, cmd)
		return m, nil
	}
}

func TestE2E(t *testing.T) {
	t.Run("G1_track_selection_starts_playback_and_renders_footer", func(t *testing.T) {
		mj := newMockJellyfin(t, 2)
		m := New(nil, nil)
		m.SetSize(80, 24)

		tracks := []jellyfin.Track{
			{ID: "trk-0", Name: "Track Zero", RunTimeTicks: 180 * 1e7, Artists: []string{"Artist A"}},
		}
		m, _ = driveToPlayback(mj, tracks, 0)(t, m)

		if m.playerState.Title != "Track Zero" {
			t.Errorf("playerState.Title = %q, want %q", m.playerState.Title, "Track Zero")
		}
		if m.currentIndex != 0 {
			t.Errorf("currentIndex = %d, want 0", m.currentIndex)
		}
		if !m.IsPlaying() {
			t.Error("expected IsPlaying true")
		}

		view := m.View()
		if !strings.Contains(view, "Track Zero") {
			t.Errorf("View() should contain track title in footer\n%s", view)
		}
		if !strings.Contains(view, "spruce") {
			t.Errorf("View() should contain 'spruce' header\n%s", view)
		}

		started := mj.startedCalls()
		if len(started) != 1 {
			t.Errorf("expected 1 POST /Sessions/Playing, got %d", len(started))
		}
	})

	t.Run("G2_stream_url_omits_token_headers_carry_auth", func(t *testing.T) {
		mj := newMockJellyfin(t, 1)
		c := jellyfin.NewClient(mj.URL, "tok-1", "usr-1", "Test device", "spruce-test-a41c29ef")
		url := c.StreamURL("trk-1", "sess-1")
		for _, want := range []string{"playSessionId=sess-1", "deviceId=spruce-test-a41c29ef", "static=true"} {
			if !strings.Contains(url, want) {
				t.Errorf("StreamURL %q missing %q", url, want)
			}
		}
		if strings.Contains(url, "tok-1") {
			t.Errorf("StreamURL must not embed auth token: %q", url)
		}
		headers := c.StreamHeaders()
		if len(headers) != 1 || !strings.Contains(headers[0], `Token="tok-1"`) {
			t.Errorf("StreamHeaders missing auth token: %v", headers)
		}
	})

	t.Run("G3_launch_failure_shows_error_banner_no_advance", func(t *testing.T) {
		mj := newMockJellyfin(t, 2)
		m := New(nil, nil)
		m.SetSize(80, 24)

		tracks := []jellyfin.Track{
			{ID: "trk-0", Name: "Track Zero", RunTimeTicks: 180 * 1e7, Artists: []string{"Artist A"}},
			{ID: "trk-1", Name: "Track One", RunTimeTicks: 200 * 1e7, Artists: []string{"Artist A"}},
		}
		m, _ = driveToPlayback(mj, tracks, 0)(t, m)
		gen := m.playGeneration

		m, _ = update(m, player.PlayerLaunchErrMsg{Generation: m.playGeneration, Err: errors.New("401 unauthorized")})

		if !m.err.HasError() {
			t.Fatal("expected error banner after launch failure")
		}
		if m.currentIndex != -1 {
			t.Errorf("currentIndex = %d, want -1 (stopped, no advance)", m.currentIndex)
		}
		if m.playGeneration == gen {
			t.Error("expected playGeneration bumped after stop")
		}
		view := m.View()
		if !strings.Contains(view, "401 unauthorized") {
			t.Errorf("View() should contain error text\n%s", view)
		}
	})

	t.Run("G4_back_stack_library_queue_back", func(t *testing.T) {
		m := New(nil, nil)
		m.SetSize(80, 24)
		m.screen = ScreenLibrary

		// Tab is a peer-level switch: does NOT push to backStack.
		m, _ = update(m, keySpecial(tea.KeyTab))
		if m.screen != ScreenQueue {
			t.Errorf("after tab: screen = %v, want ScreenQueue", m.screen)
		}
		if len(m.backStack) != 0 {
			t.Errorf("after tab: backStack = %v, want empty (tab is peer switch)", m.backStack)
		}

		// Esc on Queue screen when not filtering stays on Queue (no-op).
		m, _ = update(m, keySpecial(tea.KeyEsc))
		if m.screen != ScreenQueue {
			t.Errorf("after esc: screen = %v, want ScreenQueue", m.screen)
		}
		if len(m.backStack) != 0 {
			t.Errorf("after esc: backStack len = %d, want 0", len(m.backStack))
		}

		// Tab switches back to Library.
		m, _ = update(m, keySpecial(tea.KeyTab))
		if m.screen != ScreenLibrary {
			t.Errorf("after second tab: screen = %v, want ScreenLibrary", m.screen)
		}
	})

	t.Run("G5_quit_during_playback_reports_stopped", func(t *testing.T) {
		mj := newMockJellyfin(t, 2)
		m := New(nil, nil)
		m.SetSize(80, 24)

		tracks := []jellyfin.Track{
			{ID: "trk-0", Name: "Track Zero", RunTimeTicks: 180 * 1e7, Artists: []string{"Artist A"}},
		}
		m, _ = driveToPlayback(mj, tracks, 0)(t, m)

		if m.currentIndex < 0 || m.client == nil {
			t.Fatalf("precondition: playback not active, idx=%d client=%v", m.currentIndex, m.client)
		}
		m.playerState.Position = 42

		m, cmd := update(m, keyMsg("q"))
		m, _ = execCmd(m, cmd)

		stopped := mj.stoppedCalls()
		if len(stopped) != 1 {
			t.Fatalf("expected 1 POST /Sessions/Playing/Stopped, got %d", len(stopped))
		}
		var req jellyfin.PlaybackProgressRequest
		if err := json.Unmarshal([]byte(stopped[0]), &req); err != nil {
			t.Fatalf("unmarshal stopped body: %v", err)
		}
		if req.PlayMethod != "DirectPlay" {
			t.Errorf("PlayMethod = %q, want DirectPlay", req.PlayMethod)
		}
		if req.PlaySessionId == "" {
			t.Error("PlaySessionId empty in stopped report")
		}
		if req.ItemID != "trk-0" {
			t.Errorf("ItemID = %q, want trk-0", req.ItemID)
		}
		if req.PositionTicks != 42*1e7 {
			t.Errorf("PositionTicks = %d, want %d", req.PositionTicks, int64(42*1e7))
		}
	})

	t.Run("G6_eof_advances_error_does_not", func(t *testing.T) {
		mj := newMockJellyfin(t, 2)
		m := New(nil, nil)
		m.SetSize(80, 24)

		tracks := []jellyfin.Track{
			{ID: "trk-0", Name: "Track Zero", RunTimeTicks: 180 * 1e7, Artists: []string{"Artist A"}},
			{ID: "trk-1", Name: "Track One", RunTimeTicks: 200 * 1e7, Artists: []string{"Artist A"}},
		}
		m, _ = driveToPlayback(mj, tracks, 0)(t, m)

		m, cmd := update(m, player.PlayerEndMsg{Generation: m.playGeneration, Reason: "eof"})
		m, _ = execCmd(m, cmd)
		if m.currentIndex != 1 {
			t.Errorf("after eof: currentIndex = %d, want 1 (advance)", m.currentIndex)
		}
		if m.playerState.Title != "Track One" {
			t.Errorf("after eof: Title = %q, want Track One", m.playerState.Title)
		}

		var cmd2 tea.Cmd
		m, cmd2 = m.startPlaybackAt(0)
		m, _ = execCmd(m, cmd2)
		if m.currentIndex != 0 {
			t.Fatalf("reset: currentIndex = %d, want 0", m.currentIndex)
		}

		m, _ = update(m, player.PlayerEndMsg{
			Generation: m.playGeneration, Reason: "error", Err: errors.New("load failed"),
		})
		if m.currentIndex != -1 {
			t.Errorf("after error end: currentIndex = %d, want -1 (stopped, no advance)", m.currentIndex)
		}
		if !m.err.HasError() {
			t.Error("expected error banner after fatal PlayerEndMsg error")
		}
	})

	t.Run("G7_pagination_fetches_all_tracks_in_pages", func(t *testing.T) {
		mj := newMockJellyfin(t, 250)
		m := New(nil, nil)
		m.SetSize(80, 24)

		m, _ = update(m, login.LoginSuccessMsg{
			Token: "tok-1", ServerURL: mj.URL, Username: "alice", UserID: "usr-1",
		})
		m, cmd := update(m, musicLibrariesLoadedMsg{
			libraries: []jellyfin.Library{{ID: "lib-1", Name: "Music", CollectionType: "music"}},
		})
		m, _ = execCmd(m, cmd)

		all := m.libraryScreen.AllTracks()
		if len(all) != 250 {
			t.Errorf("AllTracks len = %d, want 250", len(all))
		}
		itemsReqs := mj.itemsRequestCount()
		if itemsReqs < 2 {
			t.Errorf("expected >= 2 GET /Users/usr-1/Items requests for pagination, got %d", itemsReqs)
		}
		for i := 1; i < len(all); i++ {
			if strings.ToLower(all[i-1].Name) > strings.ToLower(all[i].Name) {
				t.Errorf("tracks not sorted alphabetically at %d: %q before %q", i, all[i-1].Name, all[i].Name)
				break
			}
		}
	})

	t.Run("Help_overlay_toggle", func(t *testing.T) {
		m := New(nil, nil)
		m.SetSize(80, 24)
		m.screen = ScreenLibrary

		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		if !m.help.Visible() {
			t.Fatal("expected help overlay visible after '?'")
		}
		v := m.View()
		if !strings.Contains(v, "Keybindings") {
			t.Errorf("expected View to contain Keybindings, got %s", v)
		}

		m, _ = update(m, tea.KeyMsg{Type: tea.KeyEscape})
		if m.help.Visible() {
			t.Fatal("expected help overlay dismissed after Esc")
		}
	})
}
