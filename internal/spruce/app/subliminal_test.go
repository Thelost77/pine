package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/spruce/config"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/spruce/screens/queue"
	"github.com/Thelost77/pine/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
)

type mediaDownloadServer struct {
	*httptest.Server
	mu    sync.Mutex
	calls map[string]int
}

func newMediaDownloadServer(t *testing.T) *mediaDownloadServer {
	t.Helper()
	server := &mediaDownloadServer{calls: make(map[string]int)}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), `Token="token"`) {
			t.Errorf("missing download authorization: %q", r.Header.Get("Authorization"))
		}
		if !strings.HasPrefix(r.URL.Path, "/Items/") || !strings.HasSuffix(r.URL.Path, "/Download") {
			http.NotFound(w, r)
			return
		}
		itemID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Items/"), "/Download")
		server.mu.Lock()
		server.calls[itemID]++
		server.mu.Unlock()
		_, _ = fmt.Fprintf(w, "audio-%s", itemID)
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *mediaDownloadServer) count(itemID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[itemID]
}

func TestRepeatTrackDownloadsOnceThenStartsFromCache(t *testing.T) {
	server := newMediaDownloadServer(t)
	m := New(nil, nil)
	m.client = jellyfin.NewClient(server.URL, "token", "user", "Test", "device")
	m.tracks = []jellyfin.Track{{ID: "one", Name: "One"}}
	m.currentIndex = 0
	m.playerState.Title = "One"
	m.playerState.Playing = true
	m.repeatTrackID = "one"
	m.repeatGeneration = 1
	t.Cleanup(m.Cleanup)

	oldPlayGeneration := m.playGeneration
	m, cmd := m.startRepeatedPlaybackAt(0)
	if cmd == nil || m.pendingRepeatIndex != 0 {
		t.Fatal("repeat did not wait for an uncached track")
	}
	updated, _ := m.Update(player.PositionMsg{Generation: oldPlayGeneration, Err: errors.New("old mpv socket closed")})
	m = updated.(*Model)
	if m.pendingRepeatIndex != 0 {
		t.Fatal("stale mpv position event cancelled the pending repeat")
	}
	msg := cmd()
	cached, ok := msg.(repeatCacheMsg)
	if !ok || cached.Err != nil {
		t.Fatalf("cache command returned %T %+v", msg, msg)
	}
	updated, _ = m.Update(cached)
	m = updated.(*Model)
	if m.pendingRepeatIndex != -1 || m.currentIndex != 0 {
		t.Fatalf("repeat did not start after caching: pending=%d current=%d", m.pendingRepeatIndex, m.currentIndex)
	}
	if _, ok := m.mediaCache.Path("one"); !ok {
		t.Fatal("track is not present in the session cache")
	}

	for i := 0; i < 10; i++ {
		m, _ = m.startRepeatedPlaybackAt(0)
	}
	if got := server.count("one"); got != 1 {
		t.Fatalf("download count = %d, want 1", got)
	}
}

func TestRepeatCacheCompletionFollowsTargetAfterQueueReorder(t *testing.T) {
	m := New(nil, nil)
	m.tracks = []jellyfin.Track{{ID: "current"}, {ID: "remove"}, {ID: "other"}, {ID: "target"}}
	m.currentIndex = 0
	m.repeatQueue = true
	m.repeatGeneration = 4
	m.pendingRepeatIndex = 3
	m.pendingRepeatItemID = "target"

	updated, _ := m.Update(queue.RemoveQueueMsg{Index: 1})
	m = updated.(*Model)
	updated, _ = m.Update(repeatCacheMsg{Generation: 4, ItemID: "target", Index: 3, Start: true})
	m = updated.(*Model)
	if m.currentIndex != 2 || m.tracks[m.currentIndex].ID != "target" {
		t.Fatalf("repeat target after removal: index=%d tracks=%v", m.currentIndex, m.tracks)
	}

	m.pendingRepeatIndex = 2
	m.pendingRepeatItemID = "target"
	m.playerState.Playing = false
	m.tracks = []jellyfin.Track{{ID: "target"}, {ID: "current"}, {ID: "other"}}
	updated, _ = m.Update(repeatCacheMsg{Generation: 4, ItemID: "target", Index: 2, Start: true})
	m = updated.(*Model)
	if m.currentIndex != 0 || m.tracks[m.currentIndex].ID != "target" {
		t.Fatalf("repeat target after shuffle: index=%d tracks=%v", m.currentIndex, m.tracks)
	}
}

func TestRemovedPendingRepeatQueueTargetAdvancesInsteadOfStalling(t *testing.T) {
	m := New(nil, nil)
	m.tracks = []jellyfin.Track{{ID: "current"}, {ID: "target"}, {ID: "next"}}
	m.currentIndex = 0
	m.repeatQueue = true
	m.repeatGeneration = 2
	m.pendingRepeatIndex = 1
	m.pendingRepeatItemID = "target"
	t.Cleanup(m.Cleanup)

	updated, _ := m.Update(queue.RemoveQueueMsg{Index: 1})
	m = updated.(*Model)
	updated, _ = m.Update(repeatCacheMsg{Generation: 2, ItemID: "target", Index: 1, Start: true})
	m = updated.(*Model)
	if m.pendingRepeatItemID != "next" {
		t.Fatalf("pending item after target removal = %q, want next", m.pendingRepeatItemID)
	}
}

func TestDisablingRepeatCancelsActiveDownload(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()

	m := New(nil, nil)
	m.client = jellyfin.NewClient(server.URL, "token", "user", "Test", "device")
	m.tracks = []jellyfin.Track{{ID: "one", Name: "One"}}
	m.currentIndex = 0
	m.repeatTrackID = "one"
	m.repeatGeneration = 1
	if err := m.ensureRepeatContext(); err != nil {
		t.Fatalf("ensureRepeatContext() error: %v", err)
	}
	t.Cleanup(m.Cleanup)

	result := make(chan tea.Msg, 1)
	cmd := m.cacheTrackCmd(m.repeatContext, m.tracks[0], 0, false, m.repeatGeneration)
	go func() { result <- cmd() }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	m.Update(queue.QueueActionMsg{Action: "repeat_track", Index: 0, TrackID: "one"})
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("download context was not cancelled")
	}
	select {
	case <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled download command did not return")
	}
}

func TestSubliminalModeCachesUniqueTracksAndStartsEveryQueueEntryAtTwentyPercent(t *testing.T) {
	server := newMediaDownloadServer(t)
	m := New(nil, nil)
	m.client = jellyfin.NewClient(server.URL, "token", "user", "Test", "device")
	m.subliminalGroup = player.NewGroup()
	m.tracks = []jellyfin.Track{
		{ID: "one", Name: "One"},
		{ID: "two", Name: "Two"},
		{ID: "one", Name: "One again"},
	}
	t.Cleanup(m.Cleanup)

	m, cmd := m.toggleSubliminalMode()
	if m.subliminalState != subliminalPreparing || cmd == nil {
		t.Fatalf("state=%v cmd=%v, want preparing command", m.subliminalState, cmd)
	}
	for m.subliminalState == subliminalPreparing {
		msg := cmd()
		updated, next := m.Update(msg)
		m = updated.(*Model)
		cmd = next
	}
	if m.subliminalState != subliminalStarting {
		t.Fatalf("state=%v, want starting", m.subliminalState)
	}
	if server.count("one") != 1 || server.count("two") != 1 {
		t.Fatalf("download calls: one=%d two=%d", server.count("one"), server.count("two"))
	}
	if m.playerState.Volume != 20 || m.playerState.SubliminalCount != 3 {
		t.Fatalf("volume=%d layers=%d", m.playerState.Volume, m.playerState.SubliminalCount)
	}
	if len(m.subliminalSnapshot) != 3 {
		t.Fatalf("snapshot has %d queue entries, want 3", len(m.subliminalSnapshot))
	}

	m.playerState.Speed = 1.5
	updated, watchCmd := m.Update(player.GroupReadyMsg{Generation: m.subliminalGeneration})
	m = updated.(*Model)
	if !m.isSubliminalRunning() || !m.playerState.Playing || watchCmd == nil {
		t.Fatal("group-ready message did not start subliminal mode")
	}
	if m.mprisState.CanSeek() || m.mprisState.CanGoNext() || m.mprisState.CurrentItemID() != "subliminal_mix" {
		t.Fatalf("unexpected MPRIS mix capabilities: seek=%v next=%v item=%q", m.mprisState.CanSeek(), m.mprisState.CanGoNext(), m.mprisState.CurrentItemID())
	}
	if m.mprisState.PlayerSpeed() != 1.0 || m.mprisState.PlayerMinimumRate() != 1.0 || m.mprisState.PlayerMaximumRate() != 1.0 {
		t.Fatalf("subliminal MPRIS rates = %v [%v, %v], want fixed 1.0", m.mprisState.PlayerSpeed(), m.mprisState.PlayerMinimumRate(), m.mprisState.PlayerMaximumRate())
	}
	m, _ = m.setSubliminalVolume(35)
	if view := m.queueScreen.View(); !strings.Contains(view, "35% per track") {
		t.Fatalf("queue view did not update group volume: %s", view)
	}

	updated, _ = m.Update(library.AddTrackToQueueMsg{Track: jellyfin.Track{ID: "three", Name: "Three"}})
	m = updated.(*Model)
	if len(m.tracks) != 3 || !m.err.HasError() {
		t.Fatal("active subliminal mode allowed a queue mutation")
	}

	m, stopCmd := m.toggleSubliminalMode()
	if m.isSubliminalActive() || m.playerState.Volume != 100 || len(m.tracks) != 3 {
		t.Fatalf("stop state active=%v volume=%d queue=%d", m.isSubliminalActive(), m.playerState.Volume, len(m.tracks))
	}
	if stopCmd != nil {
		_ = stopCmd()
	}
}

func TestSubliminalModeRejectsBlockedPaletteActionsWithErrors(t *testing.T) {
	for _, action := range []components.PaletteAction{
		components.ActionPlayDirect,
		components.ActionClearQueue,
		components.ActionShuffleQueue,
		components.ActionRepeatTrack,
		components.ActionRepeatQueue,
	} {
		t.Run(fmt.Sprint(action), func(t *testing.T) {
			m := New(nil, nil)
			m.subliminalState = subliminalPlaying
			m.tracks = []jellyfin.Track{{ID: "one"}}
			m.currentIndex = 0

			updated, cmd := m.handlePaletteAction(action, "two", jellyfin.Track{ID: "two"})
			m = updated.(*Model)
			if len(m.tracks) != 1 || cmd == nil || !m.err.HasError() {
				t.Fatalf("action=%v queue=%d cmd=%v error=%v", action, len(m.tracks), cmd, m.err.HasError())
			}
		})
	}
}

func TestConfiguredPlayPauseKeyControlsSubliminalMode(t *testing.T) {
	cfg := config.Default()
	cfg.Keybinds.PlayPause = "x"
	m := New(&cfg, nil)
	m.screen = ScreenQueue
	m.subliminalState = subliminalPlaying
	m.subliminalGroup = player.NewGroup()
	m.playerState.Title = "Subliminal mix"
	m.playerState.Playing = true
	m.tracks = []jellyfin.Track{{ID: "one"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(*Model)
	if m.playerState.Playing || cmd == nil {
		t.Fatalf("custom play/pause key: playing=%v cmd=%v", m.playerState.Playing, cmd)
	}
}

func TestStaleNormalLaunchMessagesDoNotAffectSubliminalMode(t *testing.T) {
	m := New(nil, player.NewMpv())
	m.playGeneration = 5
	m.subliminalState = subliminalPlaying
	m.playerState.Title = "Subliminal mix"
	m.playerState.Playing = true
	m.playerState.SubliminalCount = 1
	m.tracks = []jellyfin.Track{{ID: "one"}}

	updated, cleanupErrCmd := m.Update(player.PlayerLaunchErrMsg{Generation: 4, Err: errors.New("old launch failed")})
	m = updated.(*Model)
	updated, cleanupReadyCmd := m.Update(player.PlayerReadyMsg{Generation: 4})
	m = updated.(*Model)
	if cleanupErrCmd == nil || cleanupReadyCmd == nil {
		t.Fatal("stale normal launch messages did not schedule normal-player cleanup")
	}
	if !m.isSubliminalRunning() || m.playerState.Title != "Subliminal mix" || !m.playerState.Playing {
		t.Fatalf("stale normal launch changed subliminal state: active=%v title=%q playing=%v", m.isSubliminalRunning(), m.playerState.Title, m.playerState.Playing)
	}
}

func TestGroupExitStopsSubliminalModeAndUnlocksQueue(t *testing.T) {
	m := New(nil, nil)
	m.subliminalState = subliminalPlaying
	m.subliminalGeneration = 3
	m.subliminalGroup = player.NewGroup()
	m.subliminalContext, m.subliminalCancel = context.WithCancel(context.Background())
	m.playerState.Title = "Subliminal mix"
	m.playerState.Playing = true
	m.playerState.SubliminalCount = 1
	m.tracks = []jellyfin.Track{{ID: "one"}}
	m.syncQueueScreen()

	updated, cmd := m.Update(player.GroupEndedMsg{Generation: 3, Err: errors.New("mpv exited")})
	m = updated.(*Model)
	if m.isSubliminalActive() || !m.err.HasError() || m.mprisState.HasActiveItem() {
		t.Fatalf("group exit state: active=%v error=%v mpris=%v", m.isSubliminalActive(), m.err.HasError(), m.mprisState.HasActiveItem())
	}
	if cmd == nil {
		t.Fatal("group exit did not schedule cleanup")
	}
}

func TestEachSubliminalSessionOwnsItsPlayerGroup(t *testing.T) {
	server := newMediaDownloadServer(t)
	m := New(nil, nil)
	m.client = jellyfin.NewClient(server.URL, "token", "user", "Test", "device")
	m.tracks = []jellyfin.Track{{ID: "one"}}
	m.subliminalGroupFactory = player.NewGroup
	t.Cleanup(m.Cleanup)

	m, _ = m.toggleSubliminalMode()
	first := m.subliminalGroup
	m, _ = m.stopSubliminalMode()
	m, _ = m.toggleSubliminalMode()
	second := m.subliminalGroup
	if first == nil || second == nil || first == second {
		t.Fatalf("groups are not session-owned: first=%p second=%p", first, second)
	}
}

func TestSubliminalPreparationCanBeCancelledWithoutStoppingNormalPlayback(t *testing.T) {
	m := New(nil, nil)
	m.subliminalState = subliminalPreparing
	m.subliminalContext, m.subliminalCancel = context.WithCancel(context.Background())
	m.tracks = []jellyfin.Track{{ID: "one", Name: "One"}}
	m.currentIndex = 0
	m.playerState.Title = "One"
	m.playerState.Playing = true

	m, cmd := m.stopSubliminalMode()
	if cmd != nil || m.isSubliminalActive() || !m.IsPlaying() || m.playerState.Title != "One" {
		t.Fatalf("cancel changed normal playback: cmd=%v active=%v playing=%v title=%q", cmd, m.isSubliminalActive(), m.IsPlaying(), m.playerState.Title)
	}
}

func TestSubliminalModeRejectsOversizedQueue(t *testing.T) {
	m := New(nil, nil)
	m.tracks = make([]jellyfin.Track, player.MaxSubliminalTracks+1)
	for i := range m.tracks {
		m.tracks[i] = jellyfin.Track{ID: fmt.Sprintf("track-%d", i)}
	}

	m, cmd := m.toggleSubliminalMode()
	if cmd == nil || m.isSubliminalActive() {
		t.Fatalf("oversized queue cmd=%v active=%v", cmd, m.isSubliminalActive())
	}
	_ = cmd()
}

func TestQueueSubliminalActionTogglesMode(t *testing.T) {
	m := New(nil, nil)
	m.subliminalState = subliminalPlaying
	m.subliminalGroup = player.NewGroup()
	m.normalVolume = 100
	m.playerState.Volume = 20
	m.tracks = []jellyfin.Track{{ID: "one"}}

	updated, cmd := m.Update(queue.QueueActionMsg{Action: "subliminal"})
	m = updated.(*Model)
	if m.isSubliminalActive() {
		t.Fatal("queue action did not stop subliminal mode")
	}
	if cmd != nil {
		_ = cmd()
	}
}
