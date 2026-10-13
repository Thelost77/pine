package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
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

// listenAndSync moves the position of the player to pos, and then runs one
// sync tick and its commands.
func listenAndSync(m Model, pos float64) Model {
	result, _ := m.Update(player.PositionMsg{Position: pos, Duration: m.player.Duration})
	return syncTick(result.(Model))
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
	return m
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
	result, _ := m.Update(player.PositionMsg{Position: 115, Duration: 3600})
	_, cmd := result.(Model).stopPlayback()
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
