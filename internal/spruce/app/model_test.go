package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/mpris"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/spruce/config"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/spruce/screens/login"
	"github.com/Thelost77/pine/internal/spruce/screens/metadataedit"
	"github.com/Thelost77/pine/internal/spruce/screens/playlists"
	"github.com/Thelost77/pine/internal/spruce/screens/queue"
	"github.com/Thelost77/pine/internal/ui/components"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestAppModel_LifecycleAndMPRIS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users/AuthenticateByName" {
			_ = json.NewEncoder(w).Encode(jellyfin.AuthResponse{
				User:        jellyfin.User{ID: "usr-1", Name: "alice"},
				AccessToken: "tok-1",
			})
			return
		}
		if r.URL.Path == "/Users/usr-1/Views" {
			_ = json.NewEncoder(w).Encode(struct{ Items []jellyfin.Library }{
				Items: []jellyfin.Library{{ID: "lib-1", Name: "Music", CollectionType: "music"}},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := New(nil, nil)
	m.SetSize(80, 24)

	// Test LoginSuccessMsg -> triggers music libraries fetch
	loginMsg := login.LoginSuccessMsg{Token: "tok-1", ServerURL: server.URL, Username: "alice", UserID: "usr-1"}
	newM, cmd := m.Update(loginMsg)
	m = newM.(*Model)
	if m.client == nil {
		t.Fatal("expected client set after login")
	}
	if cmd == nil {
		t.Fatal("expected fetch libs cmd")
	}

	// Wait, tea.Batch returns a batch cmd, let's test musicLibrariesLoadedMsg directly
	libsMsg := musicLibrariesLoadedMsg{libraries: []jellyfin.Library{{ID: "lib-1", Name: "Music"}}}
	newM, _ = m.Update(libsMsg)
	m = newM.(*Model)
	if m.screen != ScreenLibrary {
		t.Errorf("expected ScreenLibrary after libs loaded, got %v", m.screen)
	}

	// Test PlayTracksMsg
	tracks := []jellyfin.Track{
		{ID: "t-1", Name: "Track 1", RunTimeTicks: 1800000000, Artists: []string{"Artist A"}},
		{ID: "t-2", Name: "Track 2", RunTimeTicks: 2400000000, Artists: []string{"Artist A"}},
	}
	playMsg := library.PlayTracksMsg{Tracks: tracks, StartIndex: 0}
	newM, _ = m.Update(playMsg)
	m = newM.(*Model)

	if m.screen != ScreenLibrary {
		t.Errorf("expected ScreenLibrary during playback, got %v", m.screen)
	}
	if !m.IsPlaying() || m.CurrentItemID() != "t-1" {
		t.Errorf("unexpected playing state: playing=%v, id=%q", m.IsPlaying(), m.CurrentItemID())
	}

	// Test MPRIS NextMsg
	newM, _ = m.Update(mpris.NextMsg{})
	m = newM.(*Model)
	if m.currentIndex != 1 || m.CurrentItemID() != "t-2" {
		t.Errorf("expected advance to track 2, got idx=%d, id=%q", m.currentIndex, m.CurrentItemID())
	}

	// Test MPRIS PreviousMsg (position <= 3.0 -> goes back to track 0)
	m.playerState.Position = 1.0
	newM, _ = m.Update(mpris.PreviousMsg{})
	m = newM.(*Model)
	if m.currentIndex != 0 || m.CurrentItemID() != "t-1" {
		t.Errorf("expected prev track 0, got idx=%d, id=%q", m.currentIndex, m.CurrentItemID())
	}

	// Test PositionMsg.Err -> stops playback, does NOT auto-advance (fatal).
	posMsg := player.PositionMsg{Generation: m.playGeneration, Err: errors.New("socket dead")}
	newM, _ = m.Update(posMsg)
	m = newM.(*Model)
	if m.IsPlaying() {
		t.Errorf("expected playback stopped on PositionMsg.Err, no advance")
	}

	// Restart and verify mpv's brief EOF-window "property unavailable" does not kill repeat playback.
	newM, _ = m.startPlaybackAt(0)
	m = newM.(*Model)
	posMsg = player.PositionMsg{Generation: m.playGeneration, Err: fmt.Errorf("get time-pos: %w", player.ErrPropertyUnavailable)}
	newM, _ = m.Update(posMsg)
	m = newM.(*Model)
	if !m.IsPlaying() || m.currentIndex != 0 {
		t.Errorf("expected transient mpv property error to keep playback alive, playing=%v idx=%d", m.IsPlaying(), m.currentIndex)
	}

	// Restart at track 0 for PlayerEndMsg tests.
	newM, _ = m.startPlaybackAt(0)
	m = newM.(*Model)

	// Test PlayerEndMsg{eof} -> advances queue.
	endMsg := player.PlayerEndMsg{Generation: m.playGeneration, Reason: "eof"}
	newM, _ = m.Update(endMsg)
	m = newM.(*Model)
	if m.currentIndex != 1 || m.CurrentItemID() != "t-2" {
		t.Errorf("expected eof to advance to track 2, got idx=%d", m.currentIndex)
	}

	// Test PlayerEndMsg{eof} on final track -> stops playback.
	endMsg = player.PlayerEndMsg{Generation: m.playGeneration, Reason: "eof"}
	newM, _ = m.Update(endMsg)
	m = newM.(*Model)
	if m.IsPlaying() {
		t.Errorf("expected playback stopped after final eof")
	}

	// Test PlayerEndMsg{error} -> stops playback, no advance.
	newM, _ = m.startPlaybackAt(0)
	m = newM.(*Model)
	endMsg = player.PlayerEndMsg{Generation: m.playGeneration, Reason: "error", Err: errors.New("load failed")}
	newM, _ = m.Update(endMsg)
	m = newM.(*Model)
	if m.IsPlaying() {
		t.Errorf("expected playback stopped on error reason, no advance")
	}

	// Test QueueActionMsg clear
	m.tracks = tracks
	m.currentIndex = 0
	newM, _ = m.Update(queue.QueueActionMsg{Action: "clear"})
	m = newM.(*Model)
	if len(m.tracks) != 0 || m.IsPlaying() {
		t.Errorf("expected queue cleared")
	}

	// Test View rendering
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}

func TestAppModel_FavoriteChangedUpdatesTrackCopies(t *testing.T) {
	m := New(nil, nil)
	tracks := []jellyfin.Track{{ID: "t-1", Name: "One"}, {ID: "t-2", Name: "Two"}}
	m.tracks = append([]jellyfin.Track(nil), tracks...)
	m.libraryScreen.SetAllTracks(append([]jellyfin.Track(nil), tracks...))
	m.libraryScreen, _ = m.libraryScreen.Update(library.TracksLoadedMsg{Tracks: tracks})
	m.playlistsScreen, _ = m.playlistsScreen.Update(playlists.PlaylistTracksLoadedMsg{Tracks: tracks})

	newM, _ := m.Update(library.FavoriteChangedMsg{TrackID: "t-2", IsFavorite: true})
	m = newM.(*Model)

	if !m.tracks[1].UserData.IsFavorite {
		t.Fatal("queue source favorite state was not updated")
	}
	if got := m.libraryScreen.Tracks(); len(got) != 2 || got[0].ID != "t-2" || !got[0].UserData.IsFavorite {
		t.Fatalf("library tracks = %+v", got)
	}
	if got := m.libraryScreen.AllTracks(); !got[1].UserData.IsFavorite {
		t.Fatalf("all-tracks cache = %+v", got)
	}
	if got := m.playlistsScreen.Tracks(); len(got) != 2 || got[0].ID != "t-2" || !got[0].UserData.IsFavorite {
		t.Fatalf("playlist tracks = %+v", got)
	}
	if got := m.queueScreen.Tracks(); !got[1].UserData.IsFavorite {
		t.Fatalf("queue screen tracks = %+v", got)
	}
}

func TestAppModel_AlbumFavoriteChangedUpdatesLibrary(t *testing.T) {
	m := New(nil, nil)
	m.libraryScreen, _ = m.libraryScreen.Update(library.AlbumsLoadedMsg{Albums: []jellyfin.Album{
		{ID: "a-1", Name: "Alpha"},
		{ID: "b-1", Name: "Bravo"},
	}})

	newM, _ := m.Update(library.AlbumFavoriteChangedMsg{AlbumID: "b-1", IsFavorite: true})
	m = newM.(*Model)
	got := m.libraryScreen.Albums()
	if len(got) != 2 || got[0].ID != "b-1" || !got[0].UserData.IsFavorite {
		t.Fatalf("albums = %+v", got)
	}
}

func TestMPRISPlayAndPauseSetExplicitPlaybackState(t *testing.T) {
	m := New(nil, nil)
	m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}}
	m.currentIndex = 0
	m.playerState.Playing = true

	for _, tt := range []struct {
		name    string
		msg     tea.Msg
		playing bool
	}{
		{name: "pause", msg: mpris.PauseMsg{}, playing: false},
		{name: "pause remains paused", msg: mpris.PauseMsg{}, playing: false},
		{name: "play", msg: mpris.PlayMsg{}, playing: true},
		{name: "play remains playing", msg: mpris.PlayMsg{}, playing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			updated, _ := m.Update(tt.msg)
			m = updated.(*Model)
			if m.playerState.Playing != tt.playing {
				t.Fatalf("Playing = %v; want %v", m.playerState.Playing, tt.playing)
			}
		})
	}
}

// The player reports ErrClosed from the instant of the end-file event. A
// position tick can thus arrive before the PlayerEndMsg of the same playback.
func TestClosedPlayerTickDoesNotStopBeforeTheEndEvent(t *testing.T) {
	m := New(nil, nil)
	m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}, {ID: "t-2", Name: "Track 2"}}
	m, _ = m.startPlaybackAt(0)

	tick := player.PositionMsg{Generation: m.playGeneration, Err: fmt.Errorf("get time-pos: %w", player.ErrClosed)}
	updated, cmd := m.Update(tick)
	m = updated.(*Model)
	if !m.IsPlaying() || m.currentIndex != 0 {
		t.Fatalf("a closed player tick stopped playback: playing=%v idx=%d", m.IsPlaying(), m.currentIndex)
	}
	if cmd != nil {
		t.Error("a closed player tick must not schedule a command")
	}

	updated, _ = m.Update(player.PlayerEndMsg{Generation: m.playGeneration, Reason: "eof"})
	m = updated.(*Model)
	if m.currentIndex != 1 || m.CurrentItemID() != "t-2" {
		t.Fatalf("eof after a closed player tick did not advance: idx=%d id=%q", m.currentIndex, m.CurrentItemID())
	}
}

// TestPlayerCommandErrorShowsErrorBanner verifies that the model shows the
// error of a player that did not accept a new speed. A player that is closed
// has no state, so its error has no banner.
func TestPlayerCommandErrorShowsErrorBanner(t *testing.T) {
	m := New(nil, nil)

	updated, _ := m.Update(player.CommandErrMsg{Err: fmt.Errorf("set speed: %w", player.ErrClosed)})
	m = updated.(*Model)
	if m.err.HasError() {
		t.Fatal("the error banner is visible for a player that is closed")
	}

	updated, _ = m.Update(player.CommandErrMsg{Err: errors.New("set speed: not accepted")})
	m = updated.(*Model)
	if !m.err.HasError() {
		t.Fatal("the error banner is not visible")
	}
}

func TestMPRISSeekAbsoluteSeeksTheCurrentTrack(t *testing.T) {
	m := New(nil, player.NewMpv())
	m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}}
	m.currentIndex = 0
	m.playerState.Duration = 300

	updated, cmd := m.Update(mpris.SeekAbsoluteMsg{PositionSeconds: 30})

	if got := updated.(*Model).playerState.Position; got != 30 {
		t.Fatalf("position = %v; want 30", got)
	}
	if cmd == nil {
		t.Fatal("expected a seek command")
	}
}

func TestNewAppliesConfiguredTheme(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Foreground = "#111111"
	cfg.Theme.Accent = "#222222"
	cfg.Theme.Selected = "#333333"

	m := New(&cfg, nil)

	if got := m.styles.Accent.GetForeground(); got != lipgloss.Color("#222222") {
		t.Fatalf("expected configured accent, got %v", got)
	}
	if got := m.styles.Selected.GetForeground(); got != lipgloss.Color("#111111") {
		t.Fatalf("expected configured selected foreground, got %v", got)
	}
	if got := m.styles.Selected.GetBackground(); got != lipgloss.Color("#333333") {
		t.Fatalf("expected configured selected background, got %v", got)
	}
}

func TestNewWithSavedLoginStartsAtLibrary(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Address = "https://jellyfin.example.com"
	cfg.Server.Username = "alice"
	cfg.Server.Token = "tok-1"
	cfg.Server.UserID = "usr-1"

	m := New(&cfg, nil)

	if m.screen != ScreenLibrary {
		t.Fatalf("expected saved login to start at library, got %v", m.screen)
	}
	if m.client == nil {
		t.Fatal("expected client from saved login")
	}
	if m.client.BaseURL() != "https://jellyfin.example.com" {
		t.Fatalf("expected saved server URL, got %q", m.client.BaseURL())
	}
	if m.client.Token() != "tok-1" || m.client.UserID() != "usr-1" {
		t.Fatalf("unexpected saved credentials: token=%q user=%q", m.client.Token(), m.client.UserID())
	}
}

func TestInitWithSavedLoginFetchesLibrariesWithoutLoginScreen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/usr-1/Views" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(struct{ Items []jellyfin.Library }{
			Items: []jellyfin.Library{{ID: "lib-1", Name: "Music", CollectionType: "music"}},
		})
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.Server.Address = server.URL
	cfg.Server.Token = "tok-1"
	cfg.Server.UserID = "usr-1"

	m := New(&cfg, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected saved login init command")
	}

	msg := cmd()
	libsMsg, ok := msg.(musicLibrariesLoadedMsg)
	if !ok {
		t.Fatalf("expected musicLibrariesLoadedMsg, got %T", msg)
	}
	if libsMsg.err != nil {
		t.Fatalf("expected libraries without error: %v", libsMsg.err)
	}
	if len(libsMsg.libraries) != 1 || libsMsg.libraries[0].ID != "lib-1" {
		t.Fatalf("unexpected libraries: %+v", libsMsg.libraries)
	}
}

func TestInitWithSavedLoginFallbackOn401(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := config.Default()
	cfg.Server.Address = "https://jellyfin.example.com"
	cfg.Server.Token = "tok-expired"
	cfg.Server.UserID = "usr-1"
	cfg.Server.DeviceName = "Test device"
	cfg.Server.DeviceID = "test-device"
	path := filepath.Join(config.ConfigDir(), "config.toml")
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	m := New(&cfg, nil)
	if m.screen != ScreenLibrary {
		t.Fatalf("expected initial screen ScreenLibrary, got %v", m.screen)
	}

	authErr := fmt.Errorf("load libraries: %w", &jellyfin.HTTPStatusError{StatusCode: http.StatusUnauthorized})
	msg := musicLibrariesLoadedMsg{err: authErr}

	newM, _ := m.Update(msg)
	updated := newM.(*Model)

	if updated.screen != ScreenLogin {
		t.Fatalf("expected transition to ScreenLogin on 401, got %v", updated.screen)
	}
	if updated.client != nil {
		t.Fatal("expected client to be nil after 401 reset")
	}
	if !updated.err.HasError() {
		t.Fatal("expected error banner to be set after 401 reset")
	}
	if updated.cfg.Server.Token != "tok-expired" || updated.cfg.Server.UserID != "usr-1" {
		t.Fatalf("expected credentials preserved in config on 401, got token=%q user=%q", updated.cfg.Server.Token, updated.cfg.Server.UserID)
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Server.Token != "tok-expired" || reloaded.Server.UserID != "usr-1" {
		t.Fatalf("expected credentials preserved on disk, got token=%q user=%q", reloaded.Server.Token, reloaded.Server.UserID)
	}
}

func TestMusicLibrariesLoadErrorKeepsSessionUnlessUnauthorized(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Address = "https://jellyfin.example.com"
	cfg.Server.Token = "tok-valid"
	cfg.Server.UserID = "usr-1"

	m := New(&cfg, nil)
	client := m.client

	updated, _ := m.Update(musicLibrariesLoadedMsg{err: errors.New("request timeout")})
	m = updated.(*Model)

	if m.screen != ScreenLibrary {
		t.Fatalf("expected session screen to remain ScreenLibrary, got %v", m.screen)
	}
	if m.client != client {
		t.Fatal("expected client to remain after non-auth error")
	}
	if m.cfg.Server.Token != "tok-valid" || m.cfg.Server.UserID != "usr-1" {
		t.Fatalf("expected credentials to remain after non-auth error, got token=%q user=%q", m.cfg.Server.Token, m.cfg.Server.UserID)
	}
	if !m.err.HasError() {
		t.Fatal("expected non-auth error banner")
	}
}

func TestMusicLibrariesUnauthorizedTextDoesNotResetSession(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Address = "https://jellyfin.example.com"
	cfg.Server.Token = "tok-valid"
	cfg.Server.UserID = "usr-1"
	m := New(&cfg, nil)
	client := m.client

	updated, _ := m.Update(musicLibrariesLoadedMsg{err: errors.New("request says unauthorized but has no HTTP status")})
	m = updated.(*Model)
	if m.screen != ScreenLibrary || m.client != client {
		t.Fatalf("text-only unauthorized error reset session: screen=%v client=%v", m.screen, m.client)
	}
}

func TestLibraryLoadUnauthorizedResetsPlaybackAndQueue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	authErr := fmt.Errorf("load library: %w", &jellyfin.HTTPStatusError{StatusCode: http.StatusUnauthorized})
	for name, msg := range map[string]tea.Msg{
		"albums": library.AlbumsLoadedMsg{Err: authErr},
		"tracks": library.TracksLoadedMsg{Err: authErr},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Server.Address = "https://jellyfin.example.com"
			cfg.Server.Token = "tok-expired"
			cfg.Server.UserID = "usr-1"

			m := New(&cfg, nil)
			tracks := []jellyfin.Track{
				{ID: "t-1", Name: "Track 1"},
				{ID: "t-2", Name: "Track 2"},
			}
			updated, _ := m.Update(library.PlayTracksMsg{Tracks: tracks, StartIndex: 0})
			m = updated.(*Model)
			m.repeatTrackID = "t-1"
			m.repeatQueue = true
			m.playerState.RepeatStatus = "🔁 Queue"

			updated, _ = m.Update(msg)
			m = updated.(*Model)

			if m.screen != ScreenLogin || m.client != nil {
				t.Fatalf("expected login screen with nil client, got screen=%v client=%v", m.screen, m.client)
			}
			if m.IsPlaying() || len(m.tracks) != 0 || m.currentIndex != -1 {
				t.Fatalf("expected stopped empty playback state, playing=%v tracks=%d index=%d", m.IsPlaying(), len(m.tracks), m.currentIndex)
			}
			if len(m.queueScreen.Tracks()) != 0 || m.mprisState.CanGoNext() {
				t.Fatalf("expected cleared queue projections, screen=%d mpris next=%v", len(m.queueScreen.Tracks()), m.mprisState.CanGoNext())
			}
			if m.repeatTrackID != "" || m.repeatQueue || m.playerState.RepeatStatus != "" {
				t.Fatalf("expected cleared repeat state, track=%q queue=%v status=%q", m.repeatTrackID, m.repeatQueue, m.playerState.RepeatStatus)
			}
			if m.cfg.Server.Token != "tok-expired" || m.cfg.Server.UserID != "usr-1" {
				t.Fatalf("expected preserved credentials, token=%q user=%q", m.cfg.Server.Token, m.cfg.Server.UserID)
			}
		})
	}
}

func TestAppModel_CommandPaletteAndGlobalKeys(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary

	// Test open palette with ctrl+p
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = newM.(*Model)
	if !m.palette.Visible() {
		t.Fatal("expected palette to be visible after ctrl+p")
	}

	// Test View overlay when palette is open
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view when palette is overlaid")
	}

	// Test executing a static palette action directly via handlePaletteAction
	newM, _ = m.handlePaletteAction(components.ActionShowQueue, "", nil)
	m = newM.(*Model)
	if m.screen != ScreenQueue {
		t.Errorf("expected screen to switch to queue, got %v", m.screen)
	}
	m.palette.Close()

	// Test repeat queue action
	if m.repeatQueue {
		t.Error("expected repeatQueue false initially")
	}
	newM, _ = m.Update(queue.QueueActionMsg{Action: "repeat_queue"})
	m = newM.(*Model)
	if !m.repeatQueue {
		t.Error("expected repeatQueue true after repeat_queue action")
	}

	// Setup tracks to test global n / p
	tracks := []jellyfin.Track{
		{ID: "t-1", Name: "Track 1", RunTimeTicks: 1800000000},
		{ID: "t-2", Name: "Track 2", RunTimeTicks: 1800000000},
	}
	m.tracks = tracks
	m.currentIndex = 0

	// Test global n (next)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newM.(*Model)
	if m.currentIndex != 1 {
		t.Errorf("expected currentIndex 1 after n, got %d", m.currentIndex)
	}

	// Test global p (previous)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = newM.(*Model)
	if m.currentIndex != 0 {
		t.Errorf("expected currentIndex 0 after p, got %d", m.currentIndex)
	}

	vol := m.playerState.Volume
	newM, _ = m.handlePaletteAction(components.ActionVolumeUp, "", nil)
	m = newM.(*Model)
	if m.playerState.Volume <= vol {
		t.Fatalf("expected palette volume up to increase volume")
	}

	m.repeatQueue = false
	newM, _ = m.handlePaletteAction(components.ActionRepeatQueue, "", nil)
	m = newM.(*Model)
	if !m.repeatQueue {
		t.Fatalf("expected palette repeat queue to enable repeatQueue")
	}
}

func TestAppModel_PlaylistsNavigationAndPalette(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = newM.(*Model)
	if m.screen != ScreenPlaylists {
		t.Fatalf("expected o to open playlists, got %v", m.screen)
	}

	m.playlistsScreen, _ = m.playlistsScreen.Update(playlists.PlaylistsLoadedMsg{
		Playlists: []jellyfin.Playlist{{ID: "pl-1", Name: "Road Trip"}},
	})
	m.openCommandPalette()
	found := false
	for _, item := range m.contentSearchFunc()("road") {
		if item.Label == "Playlist: Road Trip" && item.Action == components.ActionOpenSelected {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected playlist search result in command palette")
	}

	newM, _ = m.handlePaletteAction(components.ActionGoPlaylists, "", nil)
	m = newM.(*Model)
	if m.screen != ScreenPlaylists {
		t.Fatalf("expected palette Go Playlists to open playlists, got %v", m.screen)
	}
}

func TestAppModel_ContextPaletteActions(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary
	m.libraryScreen, _ = m.libraryScreen.Update(library.AlbumsLoadedMsg{
		Albums: []jellyfin.Album{{ID: "alb-1", Name: "Album One"}},
	})

	items := m.contextPaletteItems()
	hasQueueAlbum := false
	hasEdit := false
	for _, item := range items {
		hasQueueAlbum = hasQueueAlbum || item.Action == components.ActionQueueItem
		hasEdit = hasEdit || item.Action == components.ActionEditMetadata
	}
	if !hasQueueAlbum || !hasEdit {
		t.Fatalf("expected album context queue/edit actions, got %+v", items)
	}

	m.screen = ScreenPlaylists
	m.playlistsScreen, _ = m.playlistsScreen.Update(playlists.PlaylistsLoadedMsg{
		Playlists: []jellyfin.Playlist{{ID: "pl-1", Name: "Road Trip"}},
	})
	items = m.contextPaletteItems()
	hasShuffle := false
	for _, item := range items {
		hasShuffle = hasShuffle || item.Action == components.ActionShuffleItem
	}
	if !hasShuffle {
		t.Fatalf("expected playlist context shuffle action, got %+v", items)
	}
}

func TestAppModel_GlobalRepeatKeysWhenPlayerActive(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary
	m.tracks = []jellyfin.Track{
		{ID: "t-1", Name: "Track 1", RunTimeTicks: 1800000000},
		{ID: "t-2", Name: "Track 2", RunTimeTicks: 1800000000},
	}
	m.currentIndex = 1

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = newM.(*Model)
	if m.repeatTrackID != "t-2" || m.repeatQueue {
		t.Fatalf("expected current track repeat, got track=%q queue=%v", m.repeatTrackID, m.repeatQueue)
	}
	if m.playerState.RepeatStatus == "" {
		t.Fatal("expected repeat status after r")
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = newM.(*Model)
	if m.repeatTrackID != "" || m.playerState.RepeatStatus != "" {
		t.Fatalf("expected repeat track cleared, got track=%q status=%q", m.repeatTrackID, m.playerState.RepeatStatus)
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = newM.(*Model)
	if !m.repeatQueue || m.repeatTrackID != "" {
		t.Fatalf("expected repeat queue enabled, got track=%q queue=%v", m.repeatTrackID, m.repeatQueue)
	}
}

func TestAppModel_KeyboardVolumeSpeedMPRISSync(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary

	tracks := []jellyfin.Track{
		{ID: "t-1", Name: "Track 1", RunTimeTicks: 1800000000},
	}
	newM, _ := m.Update(library.PlayTracksMsg{Tracks: tracks, StartIndex: 0})
	m = newM.(*Model)

	if !m.IsPlaying() {
		t.Fatal("expected playing initially")
	}

	initialVol := m.PlayerVolume()
	// Press ']' (VolumeUp)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = newM.(*Model)

	if m.PlayerVolume() == initialVol {
		t.Errorf("expected volume to change after VolumeUp, got %d", m.PlayerVolume())
	}
	if m.mprisState == nil || m.mprisState.PlayerVolume() != m.PlayerVolume() {
		t.Errorf("expected mprisState volume to be synced to %d, got %v", m.PlayerVolume(), m.mprisState.PlayerVolume())
	}

	initialSpeed := m.PlayerSpeed()
	// Press '+' (SpeedUp)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m = newM.(*Model)

	if m.PlayerSpeed() == initialSpeed {
		t.Errorf("expected speed to change after SpeedUp, got %f", m.PlayerSpeed())
	}
	if m.mprisState == nil || m.mprisState.PlayerSpeed() != m.PlayerSpeed() {
		t.Errorf("expected mprisState speed to be synced to %f, got %v", m.PlayerSpeed(), m.mprisState.PlayerSpeed())
	}
}

func TestAppModel_MPRISCmdHelpers(t *testing.T) {
	m := New(nil, nil)

	if cmd := m.mprisPlaybackCmd(); cmd != nil {
		t.Errorf("expected nil cmd when bridge is nil, got %v", cmd)
	}
	if cmd := m.mprisPlayPauseCmd(); cmd != nil {
		t.Errorf("expected nil cmd when bridge is nil, got %v", cmd)
	}
	if cmd := m.mprisEndedCmd(); cmd != nil {
		t.Errorf("expected nil cmd when bridge is nil, got %v", cmd)
	}
	if cmd := m.mprisTitleCmd(); cmd != nil {
		t.Errorf("expected nil cmd when bridge is nil, got %v", cmd)
	}
	if cmd := m.mprisVolumeCmd(); cmd != nil {
		t.Errorf("expected nil cmd when bridge is nil, got %v", cmd)
	}

	m.mprisBridge = mpris.NewBridge(nil, "spruce")
	if cmd := m.mprisPlaybackCmd(); cmd != nil {
		t.Errorf("expected nil cmd when server is nil, got %v", cmd)
	}
}

func TestAppModel_SleepTimer(t *testing.T) {
	m := New(nil, nil)
	m.screen = ScreenQueue
	tracks := []jellyfin.Track{{ID: "t-1", Name: "Track One"}}
	newM, _ := m.Update(library.PlayTracksMsg{Tracks: tracks, StartIndex: 0})
	m = newM.(*Model)

	if !m.IsPlaying() {
		t.Fatal("expected IsPlaying true")
	}

	// Press 'S' to cycle to 15m
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m = newM.(*Model)
	if m.sleepDuration != 15*time.Minute || m.sleepGeneration != 1 {
		t.Fatalf("expected 15m and gen 1, got %v gen %d", m.sleepDuration, m.sleepGeneration)
	}

	// Press 'S' to cycle to 30m
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m = newM.(*Model)
	if m.sleepDuration != 30*time.Minute || m.sleepGeneration != 2 {
		t.Fatalf("expected 30m and gen 2, got %v gen %d", m.sleepDuration, m.sleepGeneration)
	}

	// Stale expiry (gen 1) should not stop playback
	newM, _ = m.Update(SleepTimerExpiredMsg{Generation: 1})
	m = newM.(*Model)
	if !m.IsPlaying() {
		t.Fatal("stale sleep timer should not stop playback")
	}

	// Active expiry (gen 2) should stop playback
	newM, _ = m.Update(SleepTimerExpiredMsg{Generation: 2})
	m = newM.(*Model)
	if m.IsPlaying() {
		t.Fatal("active sleep timer should stop playback")
	}

	// Test palette action setting
	newM, _ = m.handlePaletteAction(components.ActionSleep45, "", nil)
	m = newM.(*Model)
	if m.sleepDuration != 45*time.Minute {
		t.Fatalf("expected 45m from palette action, got %v", m.sleepDuration)
	}
}

func TestAppModel_HelpOverlay(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 40)
	m.screen = ScreenLibrary

	if m.help.Visible() {
		t.Fatal("expected help overlay initially hidden")
	}

	// Press '?' to open help overlay
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(*Model)
	if !m.help.Visible() {
		t.Fatal("expected help overlay visible after pressing '?'")
	}

	v := m.View()
	if !strings.Contains(v, "Keybindings") || !strings.Contains(v, "Library") {
		t.Errorf("expected view to contain keybindings overlay, got:\n%s", v)
	}

	// Swallowing keys while help is open
	prevScreen := m.screen
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = newM.(*Model)
	if m.screen != prevScreen || !m.help.Visible() {
		t.Errorf("expected key to be swallowed while help overlay open")
	}

	// Press Esc to close help overlay
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(*Model)
	if m.help.Visible() {
		t.Fatal("expected help overlay hidden after Esc")
	}

	// Press '?' again to open
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(*Model)
	if !m.help.Visible() {
		t.Fatal("expected help overlay visible after second '?'")
	}
}

func TestAppModel_LibraryNavigationAndHints(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary

	if m.libraryScreen.CurrentLevel() != library.LevelAlbums {
		t.Fatalf("expected LevelAlbums initially, got %v", m.libraryScreen.CurrentLevel())
	}
	hints := m.viewHints()
	if !strings.Contains(hints, "queue album") {
		t.Errorf("expected hints to contain 'queue album' at LevelAlbums, got: %s", hints)
	}

	// Move libraryScreen to LevelTracks via update
	libsMsg := library.TracksLoadedMsg{Tracks: []jellyfin.Track{{ID: "t-1", Name: "Track 1"}}}
	newLib, _ := m.libraryScreen.Update(libsMsg)
	m.libraryScreen = newLib

	if m.libraryScreen.CurrentLevel() != library.LevelTracks {
		t.Fatalf("expected LevelTracks after TracksLoadedMsg, got %v", m.libraryScreen.CurrentLevel())
	}
	hints = m.viewHints()
	if !strings.Contains(hints, "add track") {
		t.Errorf("expected hints to contain 'add track' at LevelTracks, got: %s", hints)
	}

	// Press Esc while at LevelTracks on ScreenLibrary
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newM.(*Model)

	if m.screen != ScreenLibrary {
		t.Errorf("expected screen to remain ScreenLibrary, got %v", m.screen)
	}
	if m.libraryScreen.CurrentLevel() != library.LevelAlbums {
		t.Errorf("expected libraryScreen level to pop back to LevelAlbums on Esc, got %v", m.libraryScreen.CurrentLevel())
	}
}

func TestAppModel_HintsStaySingleLineAndPlayerRemainsVisible(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(77, 24)
	m.screen = ScreenLibrary
	m.libraryScreen, _ = m.libraryScreen.Update(library.TracksLoadedMsg{
		Tracks: []jellyfin.Track{{ID: "t-1", Name: "Track One"}},
	})

	newM, _ := m.Update(library.PlayTracksMsg{
		Tracks:     []jellyfin.Track{{ID: "t-1", Name: "Track One"}},
		StartIndex: 0,
	})
	m = newM.(*Model)

	if got := lipgloss.Height(m.viewHints()); got != hintsHeight {
		t.Fatalf("hints height = %d, want %d", got, hintsHeight)
	}

	view := m.View()
	if !strings.Contains(view, "1.0x") {
		t.Fatalf("player footer is not visible:\n%s", view)
	}
	if got := len(strings.Split(view, "\n")); got != m.height {
		t.Fatalf("view height = %d, want %d", got, m.height)
	}
	for i, line := range strings.Split(view, "\n") {
		if got, want := lipgloss.Width(line), normalizeViewWidth(m.width); got > want {
			t.Fatalf("line %d width = %d, want at most %d", i+1, got, want)
		}
	}
}

func TestAppModel_QueueHintsDoNotDuplicatePlayerControls(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(220, 24)
	m.screen = ScreenQueue
	m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}}
	m.currentIndex = 0

	hints := m.viewHints()
	if got := strings.Count(hints, "prev/next"); got != 1 {
		t.Fatalf("expected one prev/next hint, got %d in %q", got, hints)
	}
	if !strings.Contains(hints, "speed") || !strings.Contains(hints, "vol") {
		t.Fatalf("expected global player speed/vol hints, got %q", hints)
	}
}

func TestAppModel_PlayFromLibraryFlashesSourceItem(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary
	m.libraryScreen, _ = m.libraryScreen.Update(library.TracksLoadedMsg{
		Tracks: []jellyfin.Track{{ID: "t-1", Name: "Airbag"}},
	})

	newM, cmd := m.Update(library.PlayTracksMsg{
		Tracks:     []jellyfin.Track{{ID: "t-1", Name: "Airbag"}},
		StartIndex: 0,
		Confirmation: library.QueueConfirmation{
			Origin:  library.QueueOriginLibrary,
			ItemIDs: []string{"t-1"},
		},
	})
	m = newM.(*Model)
	if !m.IsPlaying() || m.CurrentItemID() != "t-1" {
		t.Fatalf("expected entered track to play, playing=%v item=%q", m.IsPlaying(), m.CurrentItemID())
	}
	if !m.libraryScreen.IsFlashing("t-1") {
		t.Fatal("expected entered library track to flash")
	}
	if cmd == nil {
		t.Fatal("expected flash expiry command")
	}
}

func TestAppModel_QueueAddFlashesOnlyOriginScreen(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenLibrary
	m.libraryScreen, _ = m.libraryScreen.Update(library.TracksLoadedMsg{
		Tracks: []jellyfin.Track{{ID: "t-1", Name: "Airbag"}},
	})
	m.playlistsScreen, _ = m.playlistsScreen.Update(playlists.PlaylistTracksLoadedMsg{
		Tracks: []jellyfin.Track{{ID: "t-1", Name: "Airbag"}},
	})

	// Keep playback already active so add-to-queue does not start a new launch path.
	m.tracks = []jellyfin.Track{{ID: "playing", Name: "Now"}}
	m.currentIndex = 0
	m.playerState.Title = "Now"
	m.playerState.Playing = true

	newM, cmd := m.Update(library.AddTrackToQueueMsg{
		Track: jellyfin.Track{ID: "t-1", Name: "Airbag"},
		Confirmation: library.QueueConfirmation{
			Origin:  library.QueueOriginLibrary,
			ItemIDs: []string{"t-1"},
		},
	})
	m = newM.(*Model)
	if len(m.tracks) != 2 {
		t.Fatalf("queue length = %d, want 2", len(m.tracks))
	}
	if !m.libraryScreen.IsFlashing("t-1") {
		t.Fatal("expected queued library track to flash")
	}
	if m.playlistsScreen.IsFlashing("t-1") {
		t.Fatal("playlist track must not flash for a library action")
	}
	if cmd == nil {
		t.Fatal("expected flash expiry command")
	}
}

func TestAppModel_FlashExpiresWhileCommandPaletteIsOpen(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.tracks = []jellyfin.Track{{ID: "playing", Name: "Now"}}
	m.currentIndex = 0
	m.playerState.Playing = true

	newM, expiryCmd := m.Update(library.AddTrackToQueueMsg{
		Track: jellyfin.Track{ID: "t-1", Name: "Airbag"},
		Confirmation: library.QueueConfirmation{
			Origin:  library.QueueOriginLibrary,
			ItemIDs: []string{"t-1"},
		},
	})
	m = newM.(*Model)
	if !m.libraryScreen.IsFlashing("t-1") || expiryCmd == nil {
		t.Fatal("expected active flash and expiry command")
	}

	m.openCommandPalette()
	if !m.palette.Visible() {
		t.Fatal("expected command palette to be visible")
	}
	newM, _ = m.Update(expiryCmd())
	m = newM.(*Model)
	if m.libraryScreen.IsFlashing("t-1") {
		t.Fatal("command palette consumed flash expiration")
	}
	if !m.palette.Visible() {
		t.Fatal("flash expiration should not close the command palette")
	}
}

func TestAppModel_MultiTrackQueueAddFlashesAllConfirmedItems(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.libraryScreen, _ = m.libraryScreen.Update(library.TracksLoadedMsg{
		Tracks: []jellyfin.Track{
			{ID: "t-1", Name: "Airbag"},
			{ID: "t-2", Name: "Paranoid Android"},
		},
	})
	m.tracks = []jellyfin.Track{{ID: "playing", Name: "Now"}}
	m.currentIndex = 0
	m.playerState.Playing = true

	newM, cmd := m.Update(library.AddTracksToQueueMsg{
		Tracks: []jellyfin.Track{
			{ID: "t-1", Name: "Airbag"},
			{ID: "t-2", Name: "Paranoid Android"},
		},
		Confirmation: library.QueueConfirmation{
			Origin:  library.QueueOriginLibrary,
			ItemIDs: []string{"t-1", "t-2"},
		},
	})
	m = newM.(*Model)
	if !m.libraryScreen.IsFlashing("t-1") || !m.libraryScreen.IsFlashing("t-2") {
		t.Fatal("expected every confirmed track to flash")
	}
	if cmd == nil {
		t.Fatal("expected flash expiry command")
	}
}

func TestAppModel_EmptyAndFailedQueueAddsDoNotConfirmSuccess(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		m := New(nil, nil)
		newM, cmd := m.Update(library.AddTracksToQueueMsg{
			Confirmation: library.QueueConfirmation{
				Origin:  library.QueueOriginLibrary,
				ItemIDs: []string{"t-1"},
			},
		})
		m = newM.(*Model)
		if len(m.tracks) != 0 || m.libraryScreen.IsFlashing("t-1") {
			t.Fatal("empty add must not mutate the queue or flash success")
		}
		if cmd != nil {
			t.Fatal("empty add should not return a command")
		}
	})

	t.Run("error", func(t *testing.T) {
		m := New(nil, nil)
		newM, cmd := m.Update(library.AddTracksToQueueMsg{
			Err: errors.New("queue album: request failed"),
			Confirmation: library.QueueConfirmation{
				Origin:  library.QueueOriginLibrary,
				ItemIDs: []string{"alb-1"},
			},
		})
		m = newM.(*Model)
		if len(m.tracks) != 0 || m.libraryScreen.IsFlashing("alb-1") {
			t.Fatal("failed add must not mutate the queue or flash success")
		}
		if !m.err.HasError() || cmd == nil {
			t.Fatal("failed add must display the error banner")
		}
	})
}

func TestScreenKeysWorkDuringPlayback(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}, {ID: "t-2", Name: "Track 2"}}
	m.currentIndex = 0
	m.syncQueueScreen()
	m.libraryScreen, _ = m.libraryScreen.Update(library.AlbumsLoadedMsg{
		Albums: []jellyfin.Album{{ID: "alb-1", Name: "Album One"}},
	})
	m.playlistsScreen, _ = m.playlistsScreen.Update(playlists.PlaylistsLoadedMsg{
		Playlists: []jellyfin.Playlist{{ID: "pl-1", Name: "Road Trip"}},
	})

	for _, screen := range []Screen{ScreenLibrary, ScreenPlaylists} {
		m.screen = screen
		var cmd tea.Cmd
		m, cmd = update(m, keyMsg("S"))
		if m.sleepDuration != 0 {
			t.Fatalf("S on screen %v set the sleep timer to %v", screen, m.sleepDuration)
		}
		if cmd == nil {
			t.Fatalf("S on screen %v returned no command", screen)
		}
		if msg, ok := cmd().(library.AddShuffledTracksToQueueMsg); !ok {
			t.Fatalf("S on screen %v returned %T, want AddShuffledTracksToQueueMsg", screen, msg)
		}
	}

	m.screen = ScreenQueue
	m, _ = update(m, keySpecial(tea.KeyDown))
	m, cmd := update(m, keyMsg("r"))
	m, _ = execCmd(m, cmd)
	if m.repeatTrackID != "t-2" {
		t.Fatalf("r on the selected queue row repeats %q, want t-2", m.repeatTrackID)
	}
}

func TestQueueRowsKeepTheirIndexesWithActiveFilter(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenQueue
	m.tracks = []jellyfin.Track{
		{ID: "a-1", Name: "Alpha One"},
		{ID: "b", Name: "Bravo"},
		{ID: "a-2", Name: "Alpha Two"},
	}
	m.currentIndex = 1
	m.syncQueueScreen()

	m = applyKeyWithFilter(m, keyMsg("/"))
	m = applyKeyWithFilter(m, keyMsg("Alpha"))
	m = applyKeyWithFilter(m, keySpecial(tea.KeyEnter))
	if !m.queueScreen.HasActiveFilter() {
		t.Fatal("expected an active queue filter")
	}

	for range 2 {
		var cmd tea.Cmd
		m, cmd = update(m, keyMsg("d"))
		m, _ = execCmd(m, cmd)
	}
	if len(m.tracks) != 1 || m.tracks[0].ID != "b" || m.CurrentItemID() != "b" {
		t.Fatalf("queue after removing both filtered rows = %+v, current %q; want only b", m.tracks, m.CurrentItemID())
	}
}

func TestPositionTickKeepsTheSelectedRowOfAFilteredQueue(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.screen = ScreenQueue
	for i := range 60 {
		m.tracks = append(m.tracks, jellyfin.Track{ID: fmt.Sprintf("a-%d", i), Name: fmt.Sprintf("Alpha %02d", i)})
	}
	m.tracks = append(m.tracks, jellyfin.Track{ID: "b", Name: "Bravo"})
	m, _ = m.startPlaybackAt(60)

	m = applyKeyWithFilter(m, keyMsg("/"))
	m = applyKeyWithFilter(m, keyMsg("Alpha"))
	m = applyKeyWithFilter(m, keySpecial(tea.KeyEnter))
	// The filtered rows fill more than one page, so the selection leaves the
	// first page.
	for range 25 {
		m, _ = update(m, keySpecial(tea.KeyDown))
	}
	before := selectedQueueIndex(t, m)

	m, _ = update(m, player.PositionMsg{Generation: m.playGeneration, Position: 5, Duration: 100})
	if after := selectedQueueIndex(t, m); after != before {
		t.Fatalf("selected queue index after a position tick = %d; want %d", after, before)
	}
}

// selectedQueueIndex returns the queue index that 'd' removes.
func selectedQueueIndex(t *testing.T, m *Model) int {
	t.Helper()
	_, cmd := m.queueScreen.Update(keyMsg("d"))
	if cmd == nil {
		t.Fatal("the queue has no selected row")
	}
	return cmd().(queue.RemoveQueueMsg).Index
}

// applyKeyWithFilter sends key to m and feeds the filter results back.
func applyKeyWithFilter(m *Model, key tea.KeyMsg) *Model {
	m, cmd := update(m, key)
	return applyFilterMatches(m, cmd)
}

func applyFilterMatches(m *Model, cmd tea.Cmd) *Model {
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, batchCmd := range msg {
			m = applyFilterMatches(m, batchCmd)
		}
	case list.FilterMatchesMsg:
		m, _ = update(m, msg)
	}
	return m
}

func TestLoginAfterAuthResetDropsTheLibraryOfTheOldSession(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m, _ = update(m, login.LoginSuccessMsg{Token: "tok-old", ServerURL: "http://old.invalid", Username: "old", UserID: "usr-old"})
	m.libraryScreen, _ = m.libraryScreen.Update(library.AlbumsLoadedMsg{
		Albums: []jellyfin.Album{{ID: "alb-old", Name: "Old Album"}},
	})
	m.playlistsScreen, _ = m.playlistsScreen.Update(playlists.PlaylistsLoadedMsg{
		Playlists: []jellyfin.Playlist{{ID: "pl-old", Name: "Old Playlist"}},
	})

	m, _ = update(m, library.TracksLoadedMsg{Err: &jellyfin.HTTPStatusError{StatusCode: http.StatusUnauthorized}})
	if m.screen != ScreenLogin {
		t.Fatalf("expected the login screen after 401, got %v", m.screen)
	}
	m, _ = update(m, login.LoginSuccessMsg{Token: "tok-new", ServerURL: "http://new.invalid", Username: "new", UserID: "usr-new"})

	if albums := m.libraryScreen.Albums(); len(albums) != 0 {
		t.Fatalf("albums of the old session remain after a new login: %+v", albums)
	}
	if lists := m.playlistsScreen.Playlists(); len(lists) != 0 {
		t.Fatalf("playlists of the old session remain after a new login: %+v", lists)
	}
}

func TestPaletteQueueResultJumpsToTheQueuedTrack(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.tracks = []jellyfin.Track{
		{ID: "t-1", Name: "Airbag"},
		{ID: "t-2", Name: "Lucky"},
		{ID: "t-3", Name: "Karma Police"},
	}
	m.currentIndex = 0

	var result *components.PaletteItem
	for _, item := range m.contentSearchFunc()("Karma") {
		if strings.HasPrefix(item.Label, "Queue: ") {
			result = &item
		}
	}
	if result == nil {
		t.Fatal("expected a queue result for Karma")
	}
	newM, _ := m.handlePaletteAction(result.Action, result.ItemID, result.Data)
	m = newM.(*Model)
	if len(m.tracks) != 3 || m.currentIndex != 2 {
		t.Fatalf("queue length = %d, current index = %d; want 3 tracks at index 2", len(m.tracks), m.currentIndex)
	}
}

func TestAlbumMetadataSaveUpdatesTheAlbumList(t *testing.T) {
	m := New(nil, nil)
	m.SetSize(80, 24)
	m.libraryScreen, _ = m.libraryScreen.Update(library.AlbumsLoadedMsg{
		Albums: []jellyfin.Album{{ID: "alb-1", Name: "Old Name", Artists: []string{"Old Artist"}, ProductionYear: 1999}},
	})

	year := 2001
	m, _ = update(m, metadataedit.SavedMsg{ItemID: "alb-1", Req: jellyfin.UpdateItemRequest{
		ID:             "alb-1",
		Name:           "New Name",
		Artists:        []string{"New Artist"},
		ProductionYear: &year,
	}})

	albums := m.libraryScreen.Albums()
	if len(albums) != 1 || albums[0].Name != "New Name" || !slices.Equal(albums[0].Artists, []string{"New Artist"}) || albums[0].ProductionYear != 2001 {
		t.Fatalf("albums after save = %+v", albums)
	}
	if album, ok := m.libraryScreen.SelectedAlbum(); !ok || album.Name != "New Name" {
		t.Fatalf("album row after save = %+v", album)
	}
}

func TestPreviousAtTheFirstTrackKeepsPlaying(t *testing.T) {
	for name, msg := range map[string]tea.Msg{
		"key":     keyMsg("p"),
		"mpris":   mpris.PreviousMsg{},
		"queue":   queue.QueueActionMsg{Action: "prev"},
		"palette": nil,
	} {
		t.Run(name, func(t *testing.T) {
			m := New(nil, nil)
			m.screen = ScreenLibrary
			m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}, {ID: "t-2", Name: "Track 2"}}
			m.currentIndex = 0
			m.playerState.Position = 1
			if msg == nil {
				newM, _ := m.handlePaletteAction(components.ActionPrevTrack, "", nil)
				m = newM.(*Model)
			} else {
				m, _ = update(m, msg)
			}
			if !m.IsPlaying() || m.currentIndex != 0 {
				t.Fatalf("previous at the first track: playing=%v index=%d, want index 0", m.IsPlaying(), m.currentIndex)
			}
		})
	}
}

func TestNextAtTheLastTrack(t *testing.T) {
	for name, msg := range map[string]tea.Msg{
		"key":     keyMsg("n"),
		"mpris":   mpris.NextMsg{},
		"queue":   queue.QueueActionMsg{Action: "next"},
		"palette": nil,
	} {
		for _, repeatQueue := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/repeat=%v", name, repeatQueue), func(t *testing.T) {
				m := New(nil, nil)
				m.screen = ScreenLibrary
				m.tracks = []jellyfin.Track{{ID: "t-1", Name: "Track 1"}, {ID: "t-2", Name: "Track 2"}}
				m.currentIndex = 1
				m.repeatQueue = repeatQueue
				if msg == nil {
					newM, _ := m.handlePaletteAction(components.ActionNextTrack, "", nil)
					m = newM.(*Model)
				} else {
					m, _ = update(m, msg)
				}
				switch {
				case repeatQueue:
					if !m.IsPlaying() || m.currentIndex != 0 {
						t.Fatalf("next with queue repeat: playing=%v index=%d, want index 0", m.IsPlaying(), m.currentIndex)
					}
				case name == "mpris":
					// MPRIS Next stops the playback at the end of the tracklist.
					if m.IsPlaying() {
						t.Fatalf("MPRIS next at the last track: index=%d, want a stop", m.currentIndex)
					}
				default:
					if !m.IsPlaying() || m.currentIndex != 1 {
						t.Fatalf("next at the last track: playing=%v index=%d, want index 1", m.IsPlaying(), m.currentIndex)
					}
				}
			})
		}
	}
}
