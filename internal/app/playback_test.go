package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
)

// apiLog records HTTP requests made to the mock ABS server.
type apiLog struct {
	mu       sync.Mutex
	requests []apiRequest
}

type apiRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

func (l *apiLog) record(method, path string, body map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = append(l.requests, apiRequest{Method: method, Path: path, Body: body})
}

func (l *apiLog) get() []apiRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]apiRequest, len(l.requests))
	copy(cp, l.requests)
	return cp
}

// newMockABSServer creates an httptest.Server that handles playback API calls
// and logs them for verification.
func newMockABSServer(log *apiLog) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/items/item-001/play":
			resp := abs.PlaySession{
				ID: "sess-abc",
				AudioTracks: []abs.AudioTrack{
					{Index: 0, ContentURL: "/s/item/item-001/audio.mp3", Duration: 3600},
				},
				CurrentTime: 42.0,
			}
			json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && r.URL.Path == "/api/items/pod-001/play/ep-001":
			resp := abs.PlaySession{
				ID: "sess-ep-abc",
				AudioTracks: []abs.AudioTrack{
					{Index: 0, ContentURL: "/s/item/pod-001/ep-001.mp3", Duration: 1800},
				},
				CurrentTime: 340.0,
			}
			json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-abc/sync":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-abc/close":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-001":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestPlaybackLifecycleIntegration tests the full playback state machine:
// PlayCmd → StartPlaySessionCmd → PlaySessionMsg → LaunchCmd → PlayerReadyMsg → ticking → Back → close.
func TestPlaybackLifecycleIntegration(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	cfg := config.Default()
	m := NewWithPlayer(cfg, nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}

	// --- Step 1: Send PlayCmd (simulates user pressing 'p' on detail screen) ---
	dur := 3600.0
	playMsg := detail.PlayCmd{Item: abs.LibraryItem{
		ID: "item-001",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:    "Test Audiobook",
				Duration: &dur,
			},
		},
	}}

	result, cmd := m.Update(playMsg)
	m = result.(Model)

	if cmd == nil {
		t.Fatal("PlayCmd should return an async command to start play session")
	}

	// --- Step 2: Execute the command — it calls StartPlaySession on our mock server ---
	msg := cmd()
	psMsg, ok := msg.(PlaySessionMsg)
	if !ok {
		t.Fatalf("expected PlaySessionMsg, got %T: %+v", msg, msg)
	}

	if psMsg.Session.SessionID != "sess-abc" {
		t.Errorf("session ID = %q, want %q", psMsg.Session.SessionID, "sess-abc")
	}
	if psMsg.Session.ItemID != "item-001" {
		t.Errorf("item ID = %q, want %q", psMsg.Session.ItemID, "item-001")
	}
	if psMsg.Session.CurrentTime != 42.0 {
		t.Errorf("currentTime = %f, want 42.0", psMsg.Session.CurrentTime)
	}

	// Verify API was called
	reqs := log.get()
	found := false
	for _, r := range reqs {
		if r.Method == http.MethodPost && r.Path == "/api/items/item-001/play" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected POST /api/items/item-001/play to be called")
	}

	// --- Step 3: Inject PlaySessionMsg → verify state + LaunchCmd returned ---
	result, cmd = m.Update(psMsg)
	m = result.(Model)

	if m.sessionID != "sess-abc" {
		t.Errorf("sessionID = %q, want %q", m.sessionID, "sess-abc")
	}
	if m.itemID != "item-001" {
		t.Errorf("itemID = %q, want %q", m.itemID, "item-001")
	}
	if !m.isPlaying() {
		t.Error("model should be playing after PlaySessionMsg")
	}
	if m.player.Title != "Test Audiobook" {
		t.Errorf("player title = %q, want %q", m.player.Title, "Test Audiobook")
	}
	if m.player.Position != 42.0 {
		t.Errorf("player position = %f, want 42.0", m.player.Position)
	}
	if m.player.Duration != 3600.0 {
		t.Errorf("player duration = %f, want 3600.0", m.player.Duration)
	}
	if m.timeListened != 0 {
		t.Errorf("timeListened = %f, want 0", m.timeListened)
	}
	if m.lastSyncPos != 42.0 {
		t.Errorf("lastSyncPos = %f, want 42.0", m.lastSyncPos)
	}
	if cmd == nil {
		t.Fatal("PlaySessionMsg should return LaunchCmd")
	}

	// --- Step 4: Inject PlayerReadyMsg → verify TickCmd + syncTickCmd batch ---
	result, cmd = m.Update(player.PlayerReadyMsg{})
	m = result.(Model)

	if cmd == nil {
		t.Fatal("PlayerReadyMsg should return batch of TickCmd + syncTickCmd")
	}

	// --- Step 5: Inject PositionMsg → verify position tracking ---
	mp.position = 52.0
	mp.duration = 3600.0

	result, cmd = m.Update(player.PositionMsg{
		Position:   52.0,
		Duration:   3600.0,
		Paused:     false,
		Generation: 1, // matches playGeneration after PlaySessionMsg
	})
	m = result.(Model)

	if m.player.Position != 52.0 {
		t.Errorf("player position = %f, want 52.0", m.player.Position)
	}
	// Delta from 42.0 to 52.0 = 10.0 seconds listened
	if m.timeListened != 10.0 {
		t.Errorf("timeListened = %f, want 10.0", m.timeListened)
	}
	if cmd == nil {
		t.Error("PositionMsg should reschedule TickCmd")
	}

	// Another tick: advance to 57.0
	result, cmd = m.Update(player.PositionMsg{
		Position:   57.0,
		Duration:   3600.0,
		Paused:     false,
		Generation: 1,
	})
	m = result.(Model)

	if m.player.Position != 57.0 {
		t.Errorf("player position = %f, want 57.0", m.player.Position)
	}
	if m.timeListened != 15.0 {
		t.Errorf("timeListened = %f, want 15.0 (10+5)", m.timeListened)
	}

	// --- Step 6: Inject SyncTickMsg → verify sync fires ---
	result, cmd = m.Update(SyncTickMsg{})
	m = result.(Model)

	if m.lastSyncPos != 57.0 {
		t.Errorf("lastSyncPos = %f, want 57.0", m.lastSyncPos)
	}
	if cmd == nil {
		t.Fatal("SyncTickMsg should return batch of sync cmds")
	}

	// Execute sync commands to trigger API calls
	executeBatchCmds(cmd)

	// Verify SyncSession was called
	reqs = log.get()
	syncFound := false
	for _, r := range reqs {
		if r.Method == http.MethodPost && r.Path == "/api/session/sess-abc/sync" {
			syncFound = true
			break
		}
	}
	if !syncFound {
		t.Error("expected POST /api/session/sess-abc/sync to be called")
	}

	// --- Step 7: Send BackMsg → verify playback continues ---
	result, _ = m.Update(BackMsg{})
	m = result.(Model)

	// Verify playback state is preserved
	if m.sessionID != "sess-abc" {
		t.Errorf("sessionID should be preserved after back, got %q", m.sessionID)
	}
	if m.itemID != "item-001" {
		t.Errorf("itemID should be preserved after back, got %q", m.itemID)
	}
	if !m.player.Playing {
		t.Error("player should still be playing after back")
	}
	if m.player.Title != "Test Audiobook" {
		t.Errorf("player title should be preserved, got %q", m.player.Title)
	}
	if m.ActiveScreen() != ScreenHome {
		t.Errorf("screen = %v, want Home after back", m.ActiveScreen())
	}
}

func TestPlayEpisodeCmdUsesSessionDurationWhenEpisodeDurationMissing(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 1800}
	client := abs.NewClient(srv.URL, "tok")
	cfg := config.Default()
	m := NewWithPlayer(cfg, nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}

	result, cmd := m.Update(detail.PlayEpisodeCmd{
		Item: abs.LibraryItem{ID: "pod-001"},
		Episode: abs.PodcastEpisode{
			ID:       "ep-001",
			Title:    "Buggy Episode",
			Duration: 0,
		},
	})
	m = result.(Model)

	if cmd == nil {
		t.Fatal("PlayEpisodeCmd should return an async command to start play session")
	}

	msg := cmd()
	psMsg, ok := msg.(PlaySessionMsg)
	if !ok {
		t.Fatalf("expected PlaySessionMsg, got %T: %+v", msg, msg)
	}
	if psMsg.Session.Duration != 1800 {
		t.Fatalf("session duration = %v, want 1800", psMsg.Session.Duration)
	}
	if psMsg.Session.CurrentTime != 340.0 {
		t.Fatalf("currentTime = %v, want 340", psMsg.Session.CurrentTime)
	}
}

func TestPlayCmdPrefersLongerMediaMetadataChapters(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/api/items/item-001/play" {
			resp := abs.PlaySession{
				ID: "sess-abc",
				AudioTracks: []abs.AudioTrack{
					{Index: 0, ContentURL: "/s/item/item-001/audio.mp3", Duration: 3600},
				},
				CurrentTime: 42.0,
				MediaMetadata: abs.MediaMetadata{
					Title: "Test Audiobook",
					Chapters: []abs.Chapter{
						{ID: 0, Start: 0, End: 10, Title: "One"},
						{ID: 1, Start: 10, End: 20, Title: "Two"},
						{ID: 2, Start: 20, End: 30, Title: "Three"},
					},
				},
				Chapters: []abs.Chapter{
					{ID: 0, Start: 0, End: 10, Title: "One"},
					{ID: 1, Start: 10, End: 20, Title: "Two"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	cfg := config.Default()
	m := NewWithPlayer(cfg, nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}

	dur := 3600.0
	result, cmd := m.Update(detail.PlayCmd{Item: abs.LibraryItem{
		ID: "item-001",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:    "Test Audiobook",
				Duration: &dur,
			},
		},
	}})
	m = result.(Model)

	msg := cmd()
	psMsg, ok := msg.(PlaySessionMsg)
	if !ok {
		t.Fatalf("expected PlaySessionMsg, got %T", msg)
	}
	if len(psMsg.Session.Chapters) != 3 {
		t.Fatalf("chapters len = %d, want 3", len(psMsg.Session.Chapters))
	}
	if psMsg.Session.Chapters[2].Title != "Three" {
		t.Fatalf("last chapter title = %q, want Three", psMsg.Session.Chapters[2].Title)
	}
}

// TestPlaybackSeekBackwardThenForward tests that backward seeks don't inflate timeListened,
// and subsequent forward movement resumes tracking.
func TestPlaybackSeekBackwardThenForward(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}

	// Set up active session
	m.sessionID = "sess-abc"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0
	m.timeListened = 0

	// Forward: 100 → 110, adds 10s
	result, _ := m.Update(player.PositionMsg{Position: 110.0, Duration: 3600.0})
	m = result.(Model)
	if m.timeListened != 10.0 {
		t.Errorf("timeListened = %f, want 10.0", m.timeListened)
	}

	// Backward seek: 110 → 80, should NOT add time
	result, _ = m.Update(player.PositionMsg{Position: 80.0, Duration: 3600.0})
	m = result.(Model)
	if m.timeListened != 10.0 {
		t.Errorf("timeListened = %f, want 10.0 (no change after backward seek)", m.timeListened)
	}

	// Forward again: 80 → 90, adds 10s
	result, _ = m.Update(player.PositionMsg{Position: 90.0, Duration: 3600.0})
	m = result.(Model)
	if m.timeListened != 20.0 {
		t.Errorf("timeListened = %f, want 20.0", m.timeListened)
	}
}

// TestPlaybackNavigateWhilePlayingKeepsPlayback verifies that NavigateMsg
// during active playback preserves playback state and navigates.
func TestPlaybackNavigateWhilePlayingKeepsPlayback(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}

	// Set up active playback
	m.sessionID = "sess-abc"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Title = "Test Book"
	m.player.Position = 200.0
	m.player.Duration = 3600.0
	m.timeListened = 30.0

	// Navigate to Library while playing
	result, _ := m.Update(NavigateMsg{Screen: ScreenLibrary})
	m = result.(Model)

	// Playback should continue
	if m.sessionID != "sess-abc" {
		t.Errorf("sessionID should be preserved, got %q", m.sessionID)
	}
	if !m.player.Playing {
		t.Error("player should still be playing")
	}
	if m.ActiveScreen() != ScreenLibrary {
		t.Errorf("screen = %v, want Library", m.ActiveScreen())
	}
}

// TestPlaybackSyncTickNoOpWhenNotPlaying verifies SyncTickMsg is ignored
// when there's no active session.
func TestPlaybackSyncTickNoOpWhenNotPlaying(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "" // not playing

	_, cmd := m.Update(SyncTickMsg{})
	if cmd != nil {
		t.Error("SyncTickMsg with no active session should return nil cmd")
	}

	reqs := log.get()
	if len(reqs) > 0 {
		t.Errorf("expected no API calls, got %d", len(reqs))
	}
}

// TestPlaybackPositionErrorTriggersCleanup verifies that a PositionMsg with
// an error triggers stopPlayback.
func TestPlaybackPositionErrorTriggersCleanup(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-abc"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0

	// Send a PositionMsg with an error (mpv crashed)
	result, cmd := m.Update(player.PositionMsg{
		Err: fmt.Errorf("mpv exited"),
	})
	m = result.(Model)

	if m.sessionID != "" {
		t.Errorf("sessionID should be cleared after error, got %q", m.sessionID)
	}
	if m.player.Playing {
		t.Error("player should not be playing after error")
	}
	if cmd == nil {
		t.Error("expected cleanup commands after position error")
	}
}

func TestPlaybackPositionErrorAtTrackEndRestartsNextTrack(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/items/item-multitrack/play":
			_ = json.NewEncoder(w).Encode(abs.PlaySession{
				ID: "sess-next",
				AudioTracks: []abs.AudioTrack{
					{Index: 0, StartOffset: 0, ContentURL: "/s/item/item-multitrack/track0.mp3", Duration: 1800},
					{Index: 1, StartOffset: 1800, ContentURL: "/s/item/item-multitrack/track1.mp3", Duration: 1800},
				},
				CurrentTime: 500,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-mt/close":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-multitrack":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	mp := &mockPlayer{position: 1799, duration: 1800}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-mt"
	m.itemID = "item-multitrack"
	m.player.Playing = true
	m.player.Title = "Track Test"
	m.player.Position = 1800
	m.player.Duration = 3600
	m.trackStartOffset = 0
	m.trackDuration = 1800
	m.playGeneration = 1

	result, cmd := m.Update(player.PositionMsg{
		Err:        fmt.Errorf("get time-pos: trying to send command on closed mpv client"),
		Generation: 1,
	})
	m = result.(Model)
	m = feedCmdChain(m, cmd, 5)

	if m.sessionID != "sess-next" {
		t.Fatalf("sessionID = %q, want sess-next", m.sessionID)
	}
	if m.trackStartOffset != 1800 {
		t.Fatalf("trackStartOffset = %f, want 1800", m.trackStartOffset)
	}
	if m.player.Position != 1800 {
		t.Fatalf("player.Position = %f, want 1800", m.player.Position)
	}
	if !m.isPlaying() {
		t.Fatal("expected playback to continue after EOF rollover")
	}
	if !mp.quit {
		t.Fatal("expected previous mpv instance to be quit during rollover")
	}

	reqs := log.get()
	playCalls := 0
	closeCalls := 0
	for _, r := range reqs {
		if r.Method == http.MethodPost && r.Path == "/api/items/item-multitrack/play" {
			playCalls++
		}
		if r.Method == http.MethodPost && r.Path == "/api/session/sess-mt/close" {
			closeCalls++
		}
	}
	if playCalls != 1 {
		t.Fatalf("expected 1 restart play call, got %d", playCalls)
	}
	if closeCalls != 1 {
		t.Fatalf("expected 1 session close call, got %d", closeCalls)
	}
}

func TestPlaybackPositionErrorNearTrackEndWithinTwoSecondsRestartsNextTrack(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/items/item-multitrack/play":
			_ = json.NewEncoder(w).Encode(abs.PlaySession{
				ID: "sess-next-realish",
				AudioTracks: []abs.AudioTrack{
					{Index: 7, StartOffset: 1679.773766, ContentURL: "/s/item/item-multitrack/track7.mp3", Duration: 828.951689},
					{Index: 8, StartOffset: 2508.725455, ContentURL: "/s/item/item-multitrack/track8.mp3", Duration: 863.172067},
				},
				CurrentTime: 2209.0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-mt-realish/close":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-multitrack":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, &mockPlayer{})
	m.sessionID = "sess-mt-realish"
	m.itemID = "item-multitrack"
	m.player.Playing = true
	m.player.Title = "Track Test"
	m.player.Position = 2507.353147
	m.player.Duration = 17590.934698999998
	m.trackStartOffset = 1679.773766
	m.trackDuration = 828.951689
	m.playGeneration = 1

	result, cmd := m.Update(player.PositionMsg{
		Err:        fmt.Errorf("get time-pos: trying to send command on closed mpv client"),
		Generation: 1,
	})
	m = result.(Model)
	m = feedCmdChain(m, cmd, 5)

	if m.sessionID != "sess-next-realish" {
		t.Fatalf("sessionID = %q, want sess-next-realish", m.sessionID)
	}
	if m.trackStartOffset != 2508.725455 {
		t.Fatalf("trackStartOffset = %f, want 2508.725455", m.trackStartOffset)
	}
	if m.player.Position != 2508.725455 {
		t.Fatalf("player.Position = %f, want 2508.725455", m.player.Position)
	}
	if !m.isPlaying() {
		t.Fatal("expected playback to continue for near-end closed-client rollover")
	}
}

func TestPlaybackPositionErrorAtFinalTrackEndStopsPlayback(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-mt/close":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-multitrack":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	mp := &mockPlayer{position: 1799, duration: 1800}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-mt"
	m.itemID = "item-multitrack"
	m.player.Playing = true
	m.player.Title = "Track Test"
	m.player.Position = 3600
	m.player.Duration = 3600
	m.trackStartOffset = 1800
	m.trackDuration = 1800
	m.playGeneration = 1

	result, cmd := m.Update(player.PositionMsg{
		Err:        fmt.Errorf("get time-pos: trying to send command on closed mpv client"),
		Generation: 1,
	})
	m = result.(Model)

	if m.sessionID != "" {
		t.Fatalf("sessionID should be cleared at final EOF, got %q", m.sessionID)
	}
	if m.player.Playing {
		t.Fatal("expected playback to stop at final EOF")
	}

	executeBatchCmds(cmd)

	reqs := log.get()
	for _, r := range reqs {
		if r.Method == http.MethodPost && r.Path == "/api/items/item-multitrack/play" {
			t.Fatal("did not expect restart play call at final EOF")
		}
	}
}

func TestPlaybackCompletionStartsNextQueuedItem(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-current/close":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-current":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/items/item-next/play":
			_ = json.NewEncoder(w).Encode(abs.PlaySession{
				ID: "sess-next",
				AudioTracks: []abs.AudioTrack{
					{Index: 0, StartOffset: 0, ContentURL: "/s/item/item-next/audio.mp3", Duration: 5400},
				},
				CurrentTime: 0,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	nextDuration := 5400.0
	mp := &mockPlayer{}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	m.player.Title = "Current Book"
	m.player.Position = 3600
	m.player.Duration = 3600
	m.trackStartOffset = 1800
	m.trackDuration = 1800
	m.playGeneration = 1
	m.queue = []QueueEntry{{
		Item: abs.LibraryItem{
			ID:        "item-next",
			LibraryID: "lib-books-001",
			MediaType: "book",
			Media: abs.Media{
				Metadata: abs.MediaMetadata{
					Title:    "Next Book",
					Duration: &nextDuration,
				},
			},
		},
	}}

	result, cmd := m.Update(player.PositionMsg{
		Err:        fmt.Errorf("get time-pos: trying to send command on closed mpv client"),
		Generation: 1,
	})
	m = result.(Model)
	m = feedCmdChain(m, cmd, 5)

	if m.sessionID != "sess-next" {
		t.Fatalf("sessionID = %q, want sess-next", m.sessionID)
	}
	if m.itemID != "item-next" {
		t.Fatalf("itemID = %q, want item-next", m.itemID)
	}
	if len(m.queue) != 0 {
		t.Fatalf("queue len = %d, want 0 after consuming next item", len(m.queue))
	}
}

func TestManualStopPreservesQueue(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-current/close":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-current":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	mp := &mockPlayer{}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	m.player.Position = 120
	m.player.Duration = 3600
	m.queue = []QueueEntry{{Item: abs.LibraryItem{ID: "item-next"}}}

	m, cmd := m.stopPlayback()
	executeBatchCmds(cmd)

	if len(m.queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(m.queue))
	}
	if m.queue[0].Item.ID != "item-next" {
		t.Fatalf("queued item = %q, want item-next", m.queue[0].Item.ID)
	}
}

func TestPlaybackErrorDoesNotConsumeQueue(t *testing.T) {
	log := &apiLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/sess-current/close":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-current":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	mp := &mockPlayer{}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.sessionID = "sess-current"
	m.itemID = "item-current"
	m.player.Playing = true
	m.player.Position = 120
	m.player.Duration = 3600
	m.playGeneration = 1
	m.queue = []QueueEntry{{Item: abs.LibraryItem{ID: "item-next"}}}

	result, cmd := m.Update(player.PositionMsg{
		Err:        fmt.Errorf("socket disconnected"),
		Generation: 1,
	})
	m = result.(Model)
	executeBatchCmds(cmd)

	if len(m.queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(m.queue))
	}
	if m.itemID != "" {
		t.Fatalf("itemID = %q, want playback to stop", m.itemID)
	}
}

// TestPlaybackMultipleSyncCycles verifies that multiple sync ticks
// accumulate correctly and update lastSyncPos.
func TestPlaybackMultipleSyncCycles(t *testing.T) {
	log := &apiLog{}
	srv := newMockABSServer(log)
	defer srv.Close()

	mp := &mockPlayer{position: 0, duration: 3600}
	client := abs.NewClient(srv.URL, "tok")
	m := NewWithPlayer(config.Default(), nil, client, mp)
	m.screen = ScreenDetail
	m.backStack = []Screen{ScreenHome}
	m.sessionID = "sess-abc"
	m.itemID = "item-001"
	m.player.Playing = true
	m.player.Position = 100.0
	m.player.Duration = 3600.0
	m.lastSyncPos = 100.0
	m.timeListened = 10.0

	// First sync tick
	result, cmd := m.Update(SyncTickMsg{})
	m = result.(Model)
	if m.lastSyncPos != 100.0 {
		t.Errorf("lastSyncPos = %f, want 100.0", m.lastSyncPos)
	}
	executeBatchCmds(cmd)

	// Advance position
	result, _ = m.Update(player.PositionMsg{Position: 130.0, Duration: 3600.0})
	m = result.(Model)

	// Second sync tick
	result, cmd = m.Update(SyncTickMsg{})
	m = result.(Model)
	if m.lastSyncPos != 130.0 {
		t.Errorf("lastSyncPos = %f, want 130.0", m.lastSyncPos)
	}
	executeBatchCmds(cmd)

	// Verify two sync calls were made
	reqs := log.get()
	syncCount := 0
	for _, r := range reqs {
		if r.Method == http.MethodPost && r.Path == "/api/session/sess-abc/sync" {
			syncCount++
		}
	}
	if syncCount != 2 {
		t.Errorf("expected 2 sync API calls, got %d", syncCount)
	}
}

// TestPlaybackPlayCmdNoClientIsNoOp verifies PlayCmd with nil client returns nil cmd.
func TestPlaybackPlayCmdNoClientIsNoOp(t *testing.T) {
	mp := &mockPlayer{}
	m := NewWithPlayer(config.Default(), nil, nil, mp)
	m.screen = ScreenDetail

	dur := 3600.0
	_, cmd := m.Update(detail.PlayCmd{Item: abs.LibraryItem{
		ID: "item-1",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{Title: "X", Duration: &dur},
		},
	}})
	if cmd != nil {
		t.Error("PlayCmd with nil client should return nil")
	}
}

// executeBatchCmds recursively executes tea.Cmd including batch commands.
// It handles the internal batch type from bubbletea by calling the cmd
// and processing the result. Commands that block (e.g. tea.Tick) are
// executed with a short timeout so tests don't stall.
func executeBatchCmds(cmd tea.Cmd) {
	if cmd == nil {
		return
	}

	type result struct {
		msg tea.Msg
	}

	ch := make(chan result, 1)
	go func() {
		ch <- result{msg: cmd()}
	}()

	// Allow 50ms for immediate commands; skip tick-based commands that sleep.
	select {
	case r := <-ch:
		if r.msg == nil {
			return
		}
		if batchMsg, ok := r.msg.(tea.BatchMsg); ok {
			for _, c := range batchMsg {
				executeBatchCmds(c)
			}
		}
	case <-time.After(50 * time.Millisecond):
		// Command is a tea.Tick or similar blocking cmd — skip it.
		return
	}
}
