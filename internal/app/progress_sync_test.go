package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/player"
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

// listenAndSync moves the position of the player to pos, and then runs one
// sync tick and its commands.
func listenAndSync(m Model, pos float64) Model {
	result, _ := m.Update(player.PositionMsg{Position: pos, Duration: m.player.Duration})
	result, cmd := result.(Model).Update(SyncTickMsg{})
	m, _ = feedCmd(result.(Model), cmd)
	return m
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
