package app

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	tea "github.com/charmbracelet/bubbletea"
)

// newFakeABS starts a fake ABS server and returns its request log and a client
// for it. The server answers a POST to a path of sessions with that session.
// For each other request, it answers with the status that status returns. If
// status is nil or returns 0, it answers 200.
func newFakeABS(t *testing.T, sessions map[string]abs.PlaySession, status func(method, path string) int) (*apiLog, *cache.Client) {
	t.Helper()
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		log.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")
		if session, ok := sessions[r.URL.Path]; ok && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(session)
			return
		}
		if status != nil {
			if code := status(r.Method, r.URL.Path); code != 0 {
				w.WriteHeader(code)
				return
			}
		}
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(srv.Close)
	return log, cache.NewClient(abs.NewClient(srv.URL, "tok"), nil)
}

// requestsTo returns the requests of the log with the method and the path.
func requestsTo(log *apiLog, method, path string) []apiRequest {
	var found []apiRequest
	for _, r := range log.get() {
		if r.Method == method && r.Path == path {
			found = append(found, r)
		}
	}
	return found
}

// timesListened returns the timeListened of each POST to the path.
func timesListened(log *apiLog, path string) []float64 {
	var times []float64
	for _, r := range requestsTo(log, http.MethodPost, path) {
		times = append(times, r.Body["timeListened"].(float64))
	}
	return times
}

// syncTick runs one sync tick and its commands.
func syncTick(m Model) Model {
	result, cmd := m.Update(SyncTickMsg{})
	m, _ = feedCmd(result.(Model), cmd)
	return m
}

// advanceClock moves the clock of the model by seconds. After the first call,
// the clock of the model moves only in a call of advanceClock.
func advanceClock(m Model, seconds float64) Model {
	now := m.clock().Add(time.Duration(seconds * float64(time.Second)))
	m.now = func() time.Time { return now }
	return m
}

// playTick moves the clock of the model by seconds, and then sends a position
// tick of a player that plays at the position pos.
func playTick(m Model, seconds, pos float64) Model {
	m = advanceClock(m, seconds)
	result, _ := m.Update(player.PositionMsg{
		Position:   pos - m.trackStartOffset,
		Duration:   m.player.Duration,
		Generation: m.playGeneration,
	})
	return result.(Model)
}

// playingNow stops the clock of the model, and records that the player played
// at the last position tick. The time until the next position tick of a
// player that plays is then listening time.
func playingNow(m Model) Model {
	m = advanceClock(m, 0)
	m.lastPlayingTick = m.clock()
	return m
}

// listen plays to the position pos at the speed 1.0, with one position tick
// for each second.
func listen(m Model, pos float64) Model {
	for m.player.Position < pos {
		step := math.Min(1, pos-m.player.Position)
		m = playTick(m, step, m.player.Position+step)
	}
	return m
}

// listenAndSync plays to the position pos at the speed 1.0, and then runs one
// sync tick and its commands.
func listenAndSync(m Model, pos float64) Model {
	return syncTick(listen(m, pos))
}

// newSyncTestModel returns a model that plays item-001 in the session sess-abc
// at the position 100. The server has that position.
func newSyncTestModel(client *cache.Client) Model {
	m := NewWithPlayer(config.Default(), nil, client, nil, &mockPlayer{})
	m.sessionID = "sess-abc"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100
	m.player.Duration = 3600
	m.lastSyncPos = 100
	return playingNow(m)
}

// TestSyncSendsListeningTimeSinceLastSync verifies that a sync sends the time
// listened since the last sync. The server adds the timeListened of each sync
// to the session, so the total time of the session must not be in a sync.
func TestSyncSendsListeningTimeSinceLastSync(t *testing.T) {
	log, client := newFakeABS(t, nil, nil)
	m := newSyncTestModel(client)

	m = listenAndSync(m, 110)
	m = listenAndSync(m, 115)

	want := []float64{10, 5}
	if got := timesListened(log, "/api/session/sess-abc/sync"); !reflect.DeepEqual(got, want) {
		t.Errorf("timeListened of the syncs = %v, want %v", got, want)
	}
	if m.timeListened != 0 {
		t.Errorf("timeListened = %v, want 0 after an accepted sync", m.timeListened)
	}
}

// TestSyncCarriesListeningTimeOfFailedSync verifies that the listening time of
// a sync that the server did not accept goes out again with the next sync.
func TestSyncCarriesListeningTimeOfFailedSync(t *testing.T) {
	var syncs atomic.Int32
	log, client := newFakeABS(t, nil, func(method, path string) int {
		if path == "/api/session/sess-abc/sync" && syncs.Add(1) == 1 {
			return http.StatusInternalServerError
		}
		return 0
	})
	m := newSyncTestModel(client)

	m = listenAndSync(m, 110)
	if m.timeListened != 10 {
		t.Fatalf("timeListened = %v, want 10 after a failed sync", m.timeListened)
	}
	m = listenAndSync(m, 115)

	want := []float64{10, 15}
	if got := timesListened(log, "/api/session/sess-abc/sync"); !reflect.DeepEqual(got, want) {
		t.Errorf("timeListened of the syncs = %v, want %v", got, want)
	}
	if m.timeListened != 0 {
		t.Errorf("timeListened = %v, want 0 after an accepted sync", m.timeListened)
	}
}

// TestCloseSendsListeningTimeSinceLastSync verifies that the close of a
// session sends only the time listened since the last sync.
func TestCloseSendsListeningTimeSinceLastSync(t *testing.T) {
	log, client := newFakeABS(t, nil, nil)
	m := newSyncTestModel(client)

	m = listenAndSync(m, 110)
	_, cmd := listen(m, 115).stopPlayback()
	executeBatchCmds(cmd)

	want := []float64{5}
	if got := timesListened(log, "/api/session/sess-abc/close"); !reflect.DeepEqual(got, want) {
		t.Errorf("timeListened of the close = %v, want %v", got, want)
	}
}

// TestSyncTickSendsNothingWhenNothingChanged verifies that a session sends no
// sync while it has no change since the last sync that the server accepted. A
// sync with an old position overwrites a newer position of a second device.
func TestSyncTickSendsNothingWhenNothingChanged(t *testing.T) {
	const syncPath = "/api/session/sess-abc/sync"
	log, client := newFakeABS(t, nil, nil)
	m := newSyncTestModel(client)

	// The player is paused at the position that the server has.
	result, _ := m.Update(player.PositionMsg{Position: 100, Duration: 3600, Paused: true})
	result, cmd := result.(Model).Update(SyncTickMsg{})
	if cmd == nil {
		t.Fatal("the sync tick must continue while a session exists")
	}
	m, _ = feedCmd(result.(Model), cmd)
	if n := len(requestsTo(log, http.MethodPost, syncPath)); n != 0 {
		t.Fatalf("%d syncs with no change, want 0", n)
	}

	// The user moves the position. One sync sends the new position.
	m, _ = m.handleSeek(200)
	m = syncTick(m)
	m = syncTick(m)
	syncs := requestsTo(log, http.MethodPost, syncPath)
	if len(syncs) != 1 {
		t.Fatalf("%d syncs after one seek, want 1", len(syncs))
	}
	if got := syncs[0].Body["currentTime"]; got != 300.0 {
		t.Errorf("currentTime of the sync = %v, want 300", got)
	}
	if m.lastSyncPos != 300 {
		t.Errorf("lastSyncPos = %v, want 300", m.lastSyncPos)
	}
}

// TestSyncSendsPositionOfSeekToOtherTrack verifies that a session that starts
// at a position that the server does not have sends that position.
func TestSyncSendsPositionOfSeekToOtherTrack(t *testing.T) {
	log, client := newFakeABS(t, map[string]abs.PlaySession{
		"/api/items/item-001/play": {
			ID: "sess-next",
			AudioTracks: []abs.AudioTrack{
				{Index: 0, StartOffset: 0, ContentURL: "/track0.mp3", Duration: 1800},
				{Index: 1, StartOffset: 1800, ContentURL: "/track1.mp3", Duration: 1800},
			},
			CurrentTime: 100,
		},
	}, nil)
	m := newSyncTestModel(client)
	m.trackDuration = 1800

	m, cmd := m.handleSeek(1800)
	m, _ = feedCmd(m, cmd)
	if m.sessionID != "sess-next" {
		t.Fatalf("sessionID = %q, want sess-next", m.sessionID)
	}
	m = syncTick(m)

	syncs := requestsTo(log, http.MethodPost, "/api/session/sess-next/sync")
	if len(syncs) != 1 {
		t.Fatalf("%d syncs after a seek to a different track, want 1", len(syncs))
	}
	if got := syncs[0].Body["currentTime"]; got != 1900.0 {
		t.Errorf("currentTime of the sync = %v, want 1900", got)
	}
	if m.lastSyncPos != 1900 {
		t.Errorf("lastSyncPos = %v, want 1900", m.lastSyncPos)
	}
}

// sessionGone answers 404 to each sync of the session sess-abc, as a server
// does after a restart, or after 36 hours with no change of the session.
func sessionGone(_, path string) int {
	if path == "/api/session/sess-abc/sync" {
		return http.StatusNotFound
	}
	return 0
}

// TestSyncOpensNewSessionAfter404 verifies that pine opens a new session for
// the item if the server does not have the session of a sync, and that the
// new session gets the position and the listening time of that sync.
func TestSyncOpensNewSessionAfter404(t *testing.T) {
	tests := []struct {
		name      string
		episodeID string
		playPath  string
	}{
		{name: "book", playPath: "/api/items/item-001/play"},
		{name: "episode", episodeID: "ep-001", playPath: "/api/items/item-001/play/ep-001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, client := newFakeABS(t, map[string]abs.PlaySession{
				tt.playPath: {ID: "sess-new", AudioTracks: []abs.AudioTrack{{ContentURL: "/audio.mp3", Duration: 3600}}},
			}, sessionGone)
			m := newSyncTestModel(client)
			m.episodeID = tt.episodeID

			m = listenAndSync(m, 130)

			want := []string{
				"POST /api/session/sess-abc/sync",
				"POST " + tt.playPath,
				"POST /api/session/sess-new/sync",
			}
			if got := summarizeRequests(log.get()); !reflect.DeepEqual(got, want) {
				t.Fatalf("requests = %v, want %v", got, want)
			}
			body := log.get()[2].Body
			if body["currentTime"] != 130.0 || body["timeListened"] != 30.0 {
				t.Errorf("sync of the new session = %v, want currentTime 130 and timeListened 30", body)
			}
			if m.sessionID != "sess-new" {
				t.Errorf("sessionID = %q, want sess-new", m.sessionID)
			}
			if m.timeListened != 0 || m.lastSyncPos != 130 {
				t.Errorf("timeListened = %v, lastSyncPos = %v, want 0 and 130", m.timeListened, m.lastSyncPos)
			}

			listenAndSync(m, 140)
			if got := timesListened(log, "/api/session/sess-new/sync"); !reflect.DeepEqual(got, []float64{30, 10}) {
				t.Errorf("timeListened of the syncs of the new session = %v, want [30 10]", got)
			}
		})
	}
}

// TestSyncKeepsListeningTimeIfNewSessionFails verifies that the listening time
// stays with the session if pine cannot open a new session after a 404.
func TestSyncKeepsListeningTimeIfNewSessionFails(t *testing.T) {
	_, client := newFakeABS(t, nil, func(method, path string) int {
		if path == "/api/items/item-001/play" {
			return http.StatusInternalServerError
		}
		return sessionGone(method, path)
	})
	m := newSyncTestModel(client)

	m = listenAndSync(m, 130)

	if m.sessionID != "sess-abc" {
		t.Errorf("sessionID = %q, want sess-abc", m.sessionID)
	}
	if m.timeListened != 30 || m.lastSyncPos != 100 {
		t.Errorf("timeListened = %v, lastSyncPos = %v, want 30 and 100", m.timeListened, m.lastSyncPos)
	}
}

// TestStopSendsNoFinishedFlag verifies that the progress update of a stop has
// no isFinished key. ABS reads "isFinished": false as "mark as not finished".
// At the end of a book, that update removed the mark that the close of the
// session set, and it set the position to 0.
func TestStopSendsNoFinishedFlag(t *testing.T) {
	stops := map[string]func(m Model){
		"end of the book": func(m Model) {
			_, cmd := m.Update(player.PositionMsg{Err: fmt.Errorf("get time-pos: %w", player.ErrClosed)})
			executeBatchCmds(cmd)
		},
		"cleanup": func(m Model) { m.Cleanup() },
	}
	for name, stop := range stops {
		t.Run(name, func(t *testing.T) {
			log, client := newFakeABS(t, nil, nil)
			m := newSyncTestModel(client)
			m.player.Position = 3600
			m.trackDuration = 3600

			stop(m)

			updates := requestsTo(log, http.MethodPatch, "/api/me/progress/item-001")
			if len(updates) != 1 {
				t.Fatalf("%d progress updates, want 1", len(updates))
			}
			want := map[string]interface{}{"currentTime": 3600.0, "progress": 1.0}
			if !reflect.DeepEqual(updates[0].Body, want) {
				t.Errorf("progress update = %v, want %v", updates[0].Body, want)
			}
		})
	}
}

// TestMarkFinishedSendsExplicitFlag verifies that the actions "mark as
// finished" and "mark as not finished" send the isFinished key.
func TestMarkFinishedSendsExplicitFlag(t *testing.T) {
	tests := []struct {
		name string
		undo bool
		want map[string]interface{}
	}{
		{name: "mark as finished", want: map[string]interface{}{"currentTime": 3600.0, "progress": 1.0, "isFinished": true}},
		{name: "mark as not finished", undo: true, want: map[string]interface{}{"currentTime": 0.0, "progress": 0.0, "isFinished": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, client := newFakeABS(t, nil, nil)
			m := newSyncTestModel(client)

			_, cmd := m.handleMarkFinished(detail.MarkFinishedCmd{Item: testLibraryItem("item-002", "1984"), Undo: tt.undo})
			cmd()

			updates := requestsTo(log, http.MethodPatch, "/api/me/progress/item-002")
			if len(updates) != 1 {
				t.Fatalf("%d progress updates, want 1", len(updates))
			}
			if !reflect.DeepEqual(updates[0].Body, tt.want) {
				t.Errorf("progress update = %v, want %v", updates[0].Body, tt.want)
			}
		})
	}
}

// TestStopOfUntouchedSessionSendsNoProgress verifies that the stop of a
// session sends no position if the user listened to nothing and did not move
// the position. pine restores the last session at each start. For a finished
// book, the server starts that session at the position 0, and it reads each
// position that is not at the end as "not finished".
func TestStopOfUntouchedSessionSendsNoProgress(t *testing.T) {
	stops := map[string]func(t *testing.T, m Model){
		"quit with q":      func(t *testing.T, m Model) { quitProgram(t, m, quitKeys["q"])() },
		"quit with ctrl+c": func(t *testing.T, m Model) { quitProgram(t, m, quitKeys["ctrl+c"])() },
		"cleanup":          func(_ *testing.T, m Model) { m.Cleanup() },
		"play of a different item": func(_ *testing.T, m Model) {
			_, cmd := m.handlePlayCmd(detail.PlayCmd{Item: testLibraryItem("item-002", "1984")})
			executeBatchCmds(cmd)
		},
	}
	for name, stop := range stops {
		t.Run(name, func(t *testing.T) {
			log, client := newFakeABS(t, map[string]abs.PlaySession{
				"/api/items/item-001/play": {ID: "sess-abc", AudioTracks: []abs.AudioTrack{{ContentURL: "/audio.mp3", Duration: 3600}}},
			}, nil)
			m := NewWithPlayer(config.Default(), nil, client, nil, &mockPlayer{paused: true})

			item := testLibraryItem("item-001", "The Great Gatsby")
			m, _ = feedCmd(m, m.startRestorePlaybackCmd(RestoreSessionMsg{Item: &item}))
			if m.sessionID != "sess-abc" {
				t.Fatalf("sessionID = %q, want sess-abc", m.sessionID)
			}
			result, _ := m.Update(player.PositionMsg{Duration: 3600, Paused: true, Generation: m.playGeneration})

			stop(t, result.(Model))

			if n := len(requestsTo(log, http.MethodPatch, "/api/me/progress/item-001")); n != 0 {
				t.Errorf("%d progress updates, want 0", n)
			}
			closes := requestsTo(log, http.MethodPost, "/api/session/sess-abc/close")
			if len(closes) != 1 {
				t.Fatalf("%d closes of the session, want 1", len(closes))
			}
			if len(closes[0].Body) != 0 {
				t.Errorf("close of the session = %v, want no position", closes[0].Body)
			}
		})
	}
}

// runUntilPlaySession runs cmd, and the commands that Update returns for
// their messages, as Bubble Tea does: each command of a batch runs in its own
// goroutine, and Update gets each message when it arrives. It returns at the
// first PlaySessionMsg.
func runUntilPlaySession(t *testing.T, m Model, cmd tea.Cmd) {
	t.Helper()
	msgs := make(chan tea.Msg)
	done := make(chan struct{})
	defer close(done)

	var run func(cmd tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		go func() {
			msg := cmd()
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					run(c)
				}
				return
			}
			select {
			case msgs <- msg:
			case <-done:
			}
		}()
	}
	run(cmd)

	timeout := time.After(5 * time.Second)
	for {
		select {
		case msg := <-msgs:
			if _, ok := msg.(PlaySessionMsg); ok {
				return
			}
			result, next := m.Update(msg)
			m = result.(Model)
			run(next)
		case <-timeout:
			t.Fatal("no PlaySessionMsg in 5 seconds")
		}
	}
}

// TestOldSessionClosesBeforeNextPlay verifies that the play request of the
// next item goes out after the close of the old session completes or fails.
// A server closes the open session of a device when that device starts a
// session, and it then answers 404 to the close: the listening time that the
// close carries is lost.
func TestOldSessionClosesBeforeNextPlay(t *testing.T) {
	current := testLibraryItem("item-001", "The Great Gatsby")
	current.LibraryID = "lib-001"
	current.Media.Metadata.Series = &abs.SeriesSequence{ID: "series-001", Name: "A Series", Sequence: "1"}
	next := testLibraryItem("item-002", "1984")
	next.LibraryID = "lib-001"
	next.Media.Metadata.Series = &abs.SeriesSequence{ID: "series-001", Name: "A Series", Sequence: "2"}
	podcast := testPodcastItem("pod-001", "Tech Talk Daily")

	endOfBook := func(m Model) (Model, tea.Cmd) {
		m.player.Position = 3600
		m.trackDuration = 3600
		result, cmd := m.Update(player.PositionMsg{Err: fmt.Errorf("get time-pos: %w", player.ErrClosed)})
		return result.(Model), cmd
	}
	tests := []struct {
		name        string
		closeStatus int
		start       func(m Model) (Model, tea.Cmd)
	}{
		{name: "play of a different book", start: func(m Model) (Model, tea.Cmd) {
			return m.handlePlayCmd(detail.PlayCmd{Item: next})
		}},
		{name: "play of an episode", start: func(m Model) (Model, tea.Cmd) {
			return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: podcast, Episode: podcast.Media.Episodes[0]})
		}},
		{name: "next item of the queue at the end of a book", start: func(m Model) (Model, tea.Cmd) {
			m.queue = []QueueEntry{{Item: next}}
			return endOfBook(m)
		}},
		{name: "skip to the next item of the queue", start: func(m Model) (Model, tea.Cmd) {
			m.queue = []QueueEntry{{Item: next}}
			return m.skipToNextQueued()
		}},
		{name: "next book of the series at the end of a book", start: func(m Model) (Model, tea.Cmd) {
			m.playbackSeriesID = "series-001"
			m.playbackLibraryID = "lib-001"
			return endOfBook(m)
		}},
		{name: "close fails", closeStatus: http.StatusInternalServerError, start: func(m Model) (Model, tea.Cmd) {
			return m.handlePlayCmd(detail.PlayCmd{Item: next})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			closed, played, playedBeforeClose := false, false, false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/api/session/sess-abc/close":
					time.Sleep(100 * time.Millisecond)
					mu.Lock()
					closed = true
					mu.Unlock()
					if tt.closeStatus != 0 {
						w.WriteHeader(tt.closeStatus)
					}
				case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/play"):
					mu.Lock()
					played = true
					playedBeforeClose = !closed
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(abs.PlaySession{
						ID:          "sess-next",
						AudioTracks: []abs.AudioTrack{{ContentURL: "/audio.mp3", Duration: 3600}},
					})
					return
				case r.URL.Path == "/api/libraries/lib-001/series/series-001":
					_ = json.NewEncoder(w).Encode(abs.Series{ID: "series-001", Name: "A Series"})
					return
				case strings.HasPrefix(r.URL.Path, "/api/libraries/lib-001/items"):
					_ = json.NewEncoder(w).Encode(abs.LibraryItemsResponse{Results: []abs.LibraryItem{current, next}, Total: 2})
					return
				}
				_, _ = w.Write([]byte("{}"))
			}))
			defer srv.Close()

			m := newSyncTestModel(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil))
			m, cmd := tt.start(m)
			runUntilPlaySession(t, m, cmd)

			mu.Lock()
			defer mu.Unlock()
			if !played {
				t.Fatal("no play request for the next item")
			}
			if playedBeforeClose {
				t.Error("the play request of the next item went out before the close of the old session completed")
			}
		})
	}
}

// quitProgram sends the key of a quit to the model, and then does what the
// program does at a quit. Bubble Tea runs each command of a batch in its own
// goroutine, and it returns at the QuitMsg: it does not wait for the other
// commands. main then calls Cleanup on the final model, and the process ends.
// quitProgram returns at that moment. The function that it returns waits for
// the commands that still run.
func quitProgram(t *testing.T, m Model, key tea.KeyMsg) (wait func()) {
	t.Helper()
	result, cmd := m.Update(key)

	var running sync.WaitGroup
	var once sync.Once
	quit := make(chan struct{})
	var run func(cmd tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		running.Add(1)
		go func() {
			defer running.Done()
			switch msg := cmd().(type) {
			case tea.BatchMsg:
				for _, c := range msg {
					run(c)
				}
			case tea.QuitMsg:
				once.Do(func() { close(quit) })
			}
		}()
	}
	run(cmd)

	select {
	case <-quit:
	case <-time.After(5 * time.Second):
		t.Fatal("no QuitMsg in 5 seconds")
	}
	result.(Model).Cleanup()
	return running.Wait
}

// quitKeys are the keys that quit pine.
var quitKeys = map[string]tea.KeyMsg{
	"q":      {Type: tea.KeyRunes, Runes: []rune{'q'}},
	"ctrl+c": {Type: tea.KeyCtrlC},
}

// TestQuitWaitsForLastPosition verifies that the server has the last position
// and the listening time of a session when the process of a quit ends. A
// request that a command of the quit sends is lost if the process ends before
// the server gets it.
func TestQuitWaitsForLastPosition(t *testing.T) {
	for name, key := range quitKeys {
		t.Run(name, func(t *testing.T) {
			// The server needs 100 ms for each request.
			answered := &apiLog{}
			log, client := newFakeABS(t, nil, func(method, path string) int {
				time.Sleep(100 * time.Millisecond)
				answered.record(method, path, nil)
				return 0
			})
			m := newSyncTestModel(client)
			m.player.Position = 130
			m.timeListened = 30

			wait := quitProgram(t, m, key)
			atEnd := summarizeRequests(answered.get())
			wait()

			for _, want := range []string{"POST /api/session/sess-abc/close", "PATCH /api/me/progress/item-001"} {
				found := false
				for _, got := range atEnd {
					found = found || got == want
				}
				if !found {
					t.Errorf("requests that the server answered before the end of the process = %v, want %s", atEnd, want)
				}
			}
			closes := requestsTo(log, http.MethodPost, "/api/session/sess-abc/close")
			if len(closes) == 0 {
				t.Fatal("no close of the session")
			}
			want := map[string]interface{}{"currentTime": 130.0, "timeListened": 30.0}
			if !reflect.DeepEqual(closes[0].Body, want) {
				t.Errorf("close of the session = %v, want %v", closes[0].Body, want)
			}
		})
	}
}

// TestQuitSendsLastPositionOnce verifies that a quit sends the close of the
// session and the progress update one time. The server adds the timeListened
// of each close to the session.
func TestQuitSendsLastPositionOnce(t *testing.T) {
	for name, key := range quitKeys {
		t.Run(name, func(t *testing.T) {
			log, client := newFakeABS(t, nil, nil)
			m := newSyncTestModel(client)
			m.player.Position = 130
			m.timeListened = 30

			quitProgram(t, m, key)()

			if got := timesListened(log, "/api/session/sess-abc/close"); !reflect.DeepEqual(got, []float64{30}) {
				t.Errorf("timeListened of the closes = %v, want [30]", got)
			}
			if n := len(requestsTo(log, http.MethodPatch, "/api/me/progress/item-001")); n != 1 {
				t.Errorf("%d progress updates, want 1", n)
			}
		})
	}
}

// TestListeningTimeIsTimeOfPlay verifies that the listening time is the time
// during which the player played, and not the change of the position. At the
// speed 2.0, the position moves two seconds in each second. The server adds
// the timeListened of each sync to the listening time of the user.
func TestListeningTimeIsTimeOfPlay(t *testing.T) {
	// idleTick moves the clock by seconds, and then sends a position tick of
	// a player that does not play: it is paused, or it waits for data.
	idleTick := func(m Model, seconds float64, paused bool) Model {
		m = advanceClock(m, seconds)
		result, _ := m.Update(player.PositionMsg{Position: m.player.Position, Duration: m.player.Duration, Paused: paused, Idle: true})
		return result.(Model)
	}
	tests := []struct {
		name string
		play func(m Model) Model
		want map[string]interface{}
	}{
		{
			name: "one minute at the speed 2.0",
			play: func(m Model) Model {
				for i := 0; i < 120; i++ {
					m = playTick(m, 0.5, m.player.Position+1)
				}
				return m
			},
			want: map[string]interface{}{"currentTime": 220.0, "timeListened": 60.0},
		},
		{
			name: "a pause of one minute",
			play: func(m Model) Model {
				m = listen(m, 110)
				for i := 0; i < 120; i++ {
					m = idleTick(m, 0.5, true)
				}
				// The tick after the pause starts the count again.
				return listen(playTick(m, 0.5, 110), 120)
			},
			want: map[string]interface{}{"currentTime": 120.0, "timeListened": 20.0},
		},
		{
			name: "one minute of a player that waits for data",
			play: func(m Model) Model {
				m = listen(m, 110)
				for i := 0; i < 120; i++ {
					m = idleTick(m, 0.5, false)
				}
				// The tick after the wait starts the count again.
				return listen(playTick(m, 0.5, 110), 120)
			},
			want: map[string]interface{}{"currentTime": 120.0, "timeListened": 20.0},
		},
		{
			name: "a tick after a suspend of the machine",
			play: func(m Model) Model {
				m = listen(m, 110)
				m = playTick(m, 8*3600, 110.5)
				return listen(m, 120.5)
			},
			want: map[string]interface{}{"currentTime": 120.5, "timeListened": 20.0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, client := newFakeABS(t, nil, nil)
			m := syncTick(tt.play(newSyncTestModel(client)))

			syncs := requestsTo(log, http.MethodPost, "/api/session/sess-abc/sync")
			if len(syncs) != 1 {
				t.Fatalf("%d syncs, want 1", len(syncs))
			}
			if !reflect.DeepEqual(syncs[0].Body, tt.want) {
				t.Errorf("sync = %v, want %v", syncs[0].Body, tt.want)
			}
			if m.timeListened != 0 {
				t.Errorf("timeListened = %v, want 0 after an accepted sync", m.timeListened)
			}
		})
	}
}

// TestStalledPlayerAddsNoListeningTime verifies that the time during which
// the player waits for data is not listening time. mpv reports no pause while
// it fills its cache: its position does not move, and it is idle.
func TestStalledPlayerAddsNoListeningTime(t *testing.T) {
	mp := &mockPlayer{position: 100, duration: 3600, idle: true}
	ticks := map[string]func(generation uint64) tea.Cmd{
		"position tick": func(generation uint64) tea.Cmd {
			return player.TickCmd(mp, generation)
		},
		"position tick after a property that was not ready": func(generation uint64) tea.Cmd {
			return tickCmd(mp, generation, time.Millisecond)
		},
	}
	for name, tick := range ticks {
		t.Run(name, func(t *testing.T) {
			_, client := newFakeABS(t, nil, nil)
			m := advanceClock(newSyncTestModel(client), 1)

			result, _ := m.Update(tick(m.playGeneration)())
			m = result.(Model)

			if m.timeListened != 0 {
				t.Errorf("timeListened = %v after one second of a player that waits for data, want 0", m.timeListened)
			}
			if !m.player.Playing {
				t.Error("the model shows a pause for a player that waits for data")
			}
		})
	}
}
