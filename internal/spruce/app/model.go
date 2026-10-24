package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/mpris"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/spruce/config"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/spruce/screens/login"
	"github.com/Thelost77/pine/internal/spruce/screens/metadataedit"
	"github.com/Thelost77/pine/internal/spruce/screens/playlists"
	"github.com/Thelost77/pine/internal/spruce/screens/queue"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type musicLibrariesLoadedMsg struct {
	libraries []jellyfin.Library
	err       error
}

type Model struct {
	screen    Screen
	backStack []Screen
	keys      KeyMap

	loginScreen         login.Model
	libraryScreen       library.Model
	playlistsScreen     playlists.Model
	queueScreen         queue.Model
	metadataEditScreen  metadataedit.Model
	palette             components.Palette
	help                components.HelpOverlay
	err                 components.ErrorBanner
	repeatTrackID       string
	repeatQueue         bool
	repeatGeneration    uint64
	pendingRepeatIndex  int
	pendingRepeatItemID string
	repeatContext       context.Context
	repeatCancel        context.CancelFunc
	mediaCache          *player.SessionCache
	mediaContext        context.Context
	mediaCancel         context.CancelFunc

	client *jellyfin.Client
	cfg    *config.Config
	mpv    *player.Mpv

	subliminalGroup        *player.Group
	subliminalGroupFactory func() *player.Group
	subliminalRetired      []*player.Group
	subliminalState        subliminalState
	subliminalGeneration   uint64
	subliminalCancel       context.CancelFunc
	subliminalContext      context.Context
	subliminalSnapshot     []jellyfin.Track
	subliminalCacheTracks  []jellyfin.Track
	subliminalCachedCount  int
	normalVolume           int

	program     *tea.Program
	mprisBridge *mpris.Bridge
	mprisState  *MprisState

	playerState player.Model

	tracks       []jellyfin.Track
	currentIndex int

	playGeneration uint64
	playSessionID  string
	lastHeartbeat  time.Time
	lastMprisEmit  time.Time

	sleepDeadline   time.Time
	sleepDuration   time.Duration
	sleepGeneration uint64

	width  int
	height int
	styles ui.Styles
}

func New(cfg *config.Config, mpv *player.Mpv) *Model {

	actualCfg := config.Default()
	if cfg != nil {
		if cfg.Player.Speed != 0 {
			actualCfg.Player.Speed = cfg.Player.Speed
		}
		if cfg.Player.SeekSeconds != 0 {
			actualCfg.Player.SeekSeconds = cfg.Player.SeekSeconds
		}
		actualCfg.Server = cfg.Server
		if cfg.Theme.Background != "" {
			actualCfg.Theme = cfg.Theme
		}
		if cfg.Keybinds.Quit != "" {
			actualCfg.Keybinds.Quit = cfg.Keybinds.Quit
		}
		if cfg.Keybinds.PlayPause != "" {
			actualCfg.Keybinds.PlayPause = cfg.Keybinds.PlayPause
		}
		if cfg.Keybinds.SeekForward != "" {
			actualCfg.Keybinds.SeekForward = cfg.Keybinds.SeekForward
		}
		if cfg.Keybinds.SeekBackward != "" {
			actualCfg.Keybinds.SeekBackward = cfg.Keybinds.SeekBackward
		}
		if cfg.Keybinds.SpeedUp != "" {
			actualCfg.Keybinds.SpeedUp = cfg.Keybinds.SpeedUp
		}
		if cfg.Keybinds.SpeedDown != "" {
			actualCfg.Keybinds.SpeedDown = cfg.Keybinds.SpeedDown
		}
		if cfg.Keybinds.VolumeUp != "" {
			actualCfg.Keybinds.VolumeUp = cfg.Keybinds.VolumeUp
		}
		if cfg.Keybinds.VolumeDown != "" {
			actualCfg.Keybinds.VolumeDown = cfg.Keybinds.VolumeDown
		}
		if cfg.Keybinds.NextTrack != "" {
			actualCfg.Keybinds.NextTrack = cfg.Keybinds.NextTrack
		}
		if cfg.Keybinds.PrevTrack != "" {
			actualCfg.Keybinds.PrevTrack = cfg.Keybinds.PrevTrack
		}
		if cfg.Keybinds.SleepTimer != "" {
			actualCfg.Keybinds.SleepTimer = cfg.Keybinds.SleepTimer
		}
		if cfg.Keybinds.Back != "" {
			actualCfg.Keybinds.Back = cfg.Keybinds.Back
		}
	}
	styles := ui.NewStyles(actualCfg.Theme)
	var p player.Player
	if mpv != nil {
		p = mpv
	}
	pal := components.NewPalette()
	pal.SetStyles(styles)
	playlistsScreen := playlists.New(styles)
	loginScreen := login.New(styles)
	loginScreen.SetDeviceIdentity(actualCfg.Server.DeviceName, actualCfg.Server.DeviceID)
	initialScreen := ScreenLogin
	var client *jellyfin.Client
	if hasSavedLogin(actualCfg) {
		client = jellyfin.NewClient(actualCfg.Server.Address, actualCfg.Server.Token, actualCfg.Server.UserID, actualCfg.Server.DeviceName, actualCfg.Server.DeviceID)
		playlistsScreen.SetClient(client)
		initialScreen = ScreenLibrary
	}
	var subliminalGroupFactory func() *player.Group
	if mpv != nil {
		subliminalGroupFactory = player.NewGroup
	}
	return &Model{
		screen:                 initialScreen,
		keys:                   DefaultKeyMap(actualCfg.Keybinds),
		loginScreen:            loginScreen,
		libraryScreen:          library.New(styles),
		playlistsScreen:        playlistsScreen,
		queueScreen:            queue.New(styles),
		palette:                pal,
		help:                   components.NewHelpOverlay(styles, helpGroups()),
		client:                 client,
		cfg:                    cfg,
		mpv:                    mpv,
		subliminalGroupFactory: subliminalGroupFactory,
		mprisState: &MprisState{
			speed:       actualCfg.Player.Speed,
			minimumRate: 0.5,
			maximumRate: 4.0,
			volume:      100,
		},
		playerState:        player.NewModel(p, actualCfg.Player.Speed, playerKeys(actualCfg.Keybinds), styles),
		err:                components.NewErrorBanner(styles.Error),
		currentIndex:       -1,
		pendingRepeatIndex: -1,
		normalVolume:       100,
		styles:             styles,
	}
}

func hasSavedLogin(cfg config.Config) bool {
	return cfg.Server.Address != "" && cfg.Server.Token != "" && cfg.Server.UserID != ""
}

func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
	m.mprisBridge = mpris.NewBridge(p, "spruce")
	state := m.mprisState
	m.mprisBridge.Bind(func() mpris.ModelAccessor {
		return state
	}, 0)
	m.mprisBridge.Start()
}

func (m *Model) Cleanup() {
	if m.subliminalCancel != nil {
		m.subliminalCancel()
	}
	if m.repeatCancel != nil {
		m.repeatCancel()
	}
	if m.mediaCancel != nil {
		m.mediaCancel()
	}
	if m.mprisBridge != nil {
		_ = m.mprisBridge.Stop()
	}
	if m.subliminalGroup != nil {
		_ = m.subliminalGroup.Quit()
	}
	for _, group := range m.subliminalRetired {
		_ = group.Quit()
	}
	if m.mpv != nil {
		_ = m.mpv.Quit()
	}
	if m.mediaCache != nil {
		_ = m.mediaCache.Close()
	}
}

// screenHeight returns the available height for screen content.
func (m *Model) screenHeight() int {
	h := m.height
	h -= headerHeight
	if m.err.HasError() {
		h -= errorBannerHeight
	}
	h -= hintsHeight
	if m.playerState.Title != "" {
		h -= playerFooterHeight
	}
	if h < 0 {
		return 0
	}
	return h
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	w := normalizeViewWidth(width)
	sh := m.screenHeight()
	m.loginScreen.SetSize(w, sh)
	m.libraryScreen.SetSize(w, sh)
	m.playlistsScreen.SetSize(w, sh)
	m.queueScreen.SetSize(w, sh)
	m.playerState.SetWidth(w)
	m.err.SetWidth(w)
	m.palette.SetSize(width, height)
	m.help.SetSize(width, height)
}

func (m *Model) Init() tea.Cmd {
	if m.client != nil {
		return m.fetchMusicLibrariesCmd()
	}
	return m.loginScreen.Init()
}

func (m *Model) fetchMusicLibrariesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		libs, err := client.GetMusicLibraries(context.Background())
		return musicLibrariesLoadedMsg{libraries: libs, err: err}
	}
}

func (m *Model) handleAuthReset(err error) (tea.Model, tea.Cmd) {
	logger.Warn("resetting session to login screen due to error or unauthorized status", "err", err)
	var stopCmd tea.Cmd
	if m.isSubliminalActive() {
		var subliminalStop tea.Cmd
		m, subliminalStop = m.stopSubliminalMode()
		stopCmd = tea.Batch(stopCmd, subliminalStop)
	}
	if m.IsPlaying() {
		var normalStop tea.Cmd
		m, normalStop = m.stopPlayback()
		stopCmd = tea.Batch(stopCmd, normalStop)
	}
	m.tracks = nil
	m.currentIndex = -1
	m.repeatTrackID = ""
	m.repeatQueue = false
	m.cancelRepeatWork()
	m.repeatGeneration++
	m.pendingRepeatIndex = -1
	m.pendingRepeatItemID = ""
	m.playerState.RepeatStatus = ""
	if m.mediaCancel != nil {
		m.mediaCancel()
		m.mediaCancel = nil
		m.mediaContext = nil
	}
	m.syncQueueScreen()
	m.client = nil
	m.playlistsScreen.SetClient(nil)
	m.screen = ScreenLogin
	return m, tea.Batch(stopCmd, m.err.SetError(err), m.loginScreen.Init())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(components.ItemFlashExpiredMsg); ok {
		msg.Owner.HandleExpired(msg)
		return m, nil
	}

	if m.palette.Visible() {
		cmd, handled := m.palette.Update(msg)
		if handled {
			action, _, _, itemID, data := m.palette.SelectedAction()
			if action != components.ActionNone {
				m.palette.ClearSelection()
				m.palette.Close()
				return m.handlePaletteAction(action, itemID, data)
			}
			return m, cmd
		}
	}

	switch typed := msg.(type) {
	case repeatCacheMsg:
		return m.handleRepeatCacheMsg(typed)
	case subliminalCacheMsg:
		return m.handleSubliminalCacheMsg(typed)
	case player.GroupReadyMsg:
		return m.handleGroupReadyMsg(typed)
	case player.GroupControlMsg:
		return m.handleGroupControlMsg(typed)
	case player.GroupEndedMsg:
		return m.handleGroupEndedMsg(typed)
	case player.GroupStoppedMsg:
		m.handleGroupStoppedMsg(typed)
		return m, nil
	}

	if newM, cmd, handled := m.handlePlayerEvent(msg); handled {
		return newM, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			newM, stopCmd := m.stopAllPlayback()
			return newM, tea.Batch(stopCmd, tea.Quit)
		}
		if m.screen == ScreenMetadataEdit {
			var cmd tea.Cmd
			m.metadataEditScreen, cmd = m.metadataEditScreen.Update(msg)
			return m, cmd
		}
		if m.screen != ScreenLogin {
			isFiltering := false
			if m.screen == ScreenLibrary && m.libraryScreen.IsFiltering() {
				isFiltering = true
			} else if m.screen == ScreenPlaylists && m.playlistsScreen.IsFiltering() {
				isFiltering = true
			} else if m.screen == ScreenQueue && m.queueScreen.IsFiltering() {
				isFiltering = true
			}
			if !isFiltering && key.Matches(msg, m.keys.Help) {
				m.help.Toggle()
				return m, nil
			}
			if m.help.Visible() {
				if key.Matches(msg, m.keys.Back) {
					m.help.Hide()
				}
				return m, nil
			}
			if key.Matches(msg, m.keys.GlobalPalette) {
				m.openCommandPalette()
				return m, nil
			}
			if !isFiltering {
				if key.Matches(msg, m.keys.OpenPlaylists) {
					return m.navigate(ScreenPlaylists)
				}
				if key.Matches(msg, m.keys.Quit) {
					newM, stopCmd := m.stopAllPlayback()
					return newM, tea.Batch(stopCmd, tea.Quit)
				}
				if m.isSubliminalRunning() {
					if key.Matches(msg, m.keys.SleepTimer) {
						return m.cycleSleepTimer()
					}
					if key.Matches(msg, m.playerState.VolumeUpKey()) {
						return m.setSubliminalVolume(m.playerState.Volume + 5)
					}
					if key.Matches(msg, m.playerState.VolumeDownKey()) {
						return m.setSubliminalVolume(m.playerState.Volume - 5)
					}
					if key.Matches(msg, m.playerState.PlayPauseKey()) {
						return m.setSubliminalPaused(m.playerState.Playing)
					}
				}
				if m.IsPlaying() {
					if key.Matches(msg, m.keys.SleepTimer) {
						return m.cycleSleepTimer()
					}
					if key.Matches(msg, m.playerState.SeekForwardKey()) {
						seek := 10.0
						if m.cfg != nil {
							seek = float64(m.cfg.Player.SeekSeconds)
						}
						return m.handleSeek(seek)
					}
					if key.Matches(msg, m.playerState.SeekBackKey()) {
						seek := 10.0
						if m.cfg != nil {
							seek = float64(m.cfg.Player.SeekSeconds)
						}
						return m.handleSeek(-seek)
					}
					if key.Matches(msg, m.keys.NextTrack) {
						if len(m.tracks) > 0 && m.currentIndex+1 < len(m.tracks) {
							return m.startPlaybackAt(m.nextIndex(m.currentIndex + 1))
						}
						return m, nil
					}
					if key.Matches(msg, m.keys.PrevTrack) {
						if m.playerState.Position > 3.0 {
							return m.handleSeek(-m.playerState.Position)
						}
						if len(m.tracks) > 0 && m.currentIndex-1 >= 0 {
							return m.startPlaybackAt(m.currentIndex - 1)
						}
						return m, nil
					}
					if key.Matches(msg, m.keys.RepeatTrack) {
						return m.Update(queue.QueueActionMsg{
							Action:  "repeat_track",
							Index:   m.currentIndex,
							TrackID: m.tracks[m.currentIndex].ID,
						})
					}
					if key.Matches(msg, m.keys.RepeatQueue) {
						return m.Update(queue.QueueActionMsg{Action: "repeat_queue"})
					}
					oldSpeed := m.playerState.Speed
					oldVol := m.playerState.Volume
					oldPlaying := m.playerState.Playing
					newPlayer, playerCmd := m.playerState.Update(msg)
					m.playerState = newPlayer
					if newPlayer.Speed != oldSpeed || newPlayer.Volume != oldVol || newPlayer.Playing != oldPlaying || playerCmd != nil {
						m.syncQueueScreen()
					}
					if playerCmd != nil {
						return m, playerCmd
					}
				}
			}
		}

		if msg.String() == "tab" && m.screen != ScreenLogin {
			if m.screen == ScreenLibrary || m.screen == ScreenPlaylists {
				m.screen = ScreenQueue
				m.propagateSize()
				return m, m.initScreen(ScreenQueue)
			}
			if m.screen == ScreenQueue {
				m.screen = ScreenLibrary
				m.propagateSize()
				return m, m.initScreen(ScreenLibrary)
			}
		}
		if key.Matches(msg, m.keys.Back) && m.screen != ScreenLogin {
			if m.screen == ScreenPlaylists {
				if m.playlistsScreen.IsFiltering() || m.playlistsScreen.HasActiveFilter() || m.playlistsScreen.CurrentLevel() == playlists.LevelTracks {
					var cmd tea.Cmd
					m.playlistsScreen, cmd = m.playlistsScreen.Update(msg)
					return m, cmd
				}
				return m.back()
			}
			if m.screen == ScreenQueue {
				if m.queueScreen.IsFiltering() || m.queueScreen.HasActiveFilter() {
					var cmd tea.Cmd
					m.queueScreen, cmd = m.queueScreen.Update(msg)
					return m, cmd
				}
				return m, nil
			}
			if m.screen == ScreenLibrary && (m.libraryScreen.IsFiltering() || m.libraryScreen.HasActiveFilter() || m.libraryScreen.CurrentLevel() == library.LevelTracks) {
				var cmd tea.Cmd
				m.libraryScreen, cmd = m.libraryScreen.Update(msg)
				return m, cmd
			}
			return m.back()
		}

	case NavigateMsg:
		return m.navigate(msg.Target)

	case metadataedit.SavedMsg:
		var cmd tea.Cmd
		m.metadataEditScreen, cmd = m.metadataEditScreen.Update(msg)
		if msg.Err == nil {
			for i := range m.tracks {
				if m.tracks[i].ID == msg.ItemID {
					if msg.Req.Name != "" {
						m.tracks[i].Name = msg.Req.Name
					}
					if msg.Req.Artists != nil {
						m.tracks[i].Artists = msg.Req.Artists
					}
					if msg.Req.Album != "" {
						m.tracks[i].Album = msg.Req.Album
					}
					if i == m.currentIndex {
						m.playerState.Title = m.tracks[i].Name
					}
				}
			}
			m.libraryScreen.PatchTrack(msg.ItemID, msg.Req.Name, msg.Req.Artists, msg.Req.Album)
			m.syncQueueScreen()
		}
		return m, cmd

	case BackMsg, metadataedit.BackMsg:
		return m.back()

	case login.LoginSuccessMsg:
		logger.Info("login succeeded, setting client", "user", msg.Username)
		if m.repeatCancel != nil {
			m.cancelRepeatWork()
		}
		if m.mediaCancel != nil {
			m.mediaCancel()
			m.mediaCancel = nil
			m.mediaContext = nil
		}
		if m.mediaCache != nil {
			_ = m.mediaCache.Close()
			m.mediaCache = nil
		}
		if m.cfg != nil {
			m.cfg.Server.Address = msg.ServerURL
			m.cfg.Server.Username = msg.Username
			m.cfg.Server.Token = msg.Token
			m.cfg.Server.UserID = msg.UserID
			if err := config.Save(filepath.Join(config.ConfigDir(), "config.toml"), *m.cfg); err != nil {
				_ = m.err.SetError(fmt.Errorf("failed to save config: %w", err))
			}
		}
		deviceName, deviceID := "", ""
		if m.cfg != nil {
			deviceName = m.cfg.Server.DeviceName
			deviceID = m.cfg.Server.DeviceID
		}
		m.client = jellyfin.NewClient(msg.ServerURL, msg.Token, msg.UserID, deviceName, deviceID)
		m.playlistsScreen.SetClient(m.client)
		var cmd tea.Cmd
		m.loginScreen, cmd = m.loginScreen.Update(msg)
		return m, tea.Batch(cmd, m.fetchMusicLibrariesCmd())

	case musicLibrariesLoadedMsg:
		if msg.err == nil && len(msg.libraries) > 0 {
			m.libraryScreen.SetLibraries(m.client, msg.libraries)
			m.screen = ScreenLibrary
			return m, tea.Batch(m.libraryScreen.Init(), m.libraryScreen.FetchAllTracksCmd(), m.playlistsScreen.Init())
		}
		logger.Warn("failed to load music libraries", "err", msg.err, "count", len(msg.libraries))
		var errMsg error
		if msg.err != nil {
			errMsg = msg.err
		} else {
			errMsg = errors.New("no music libraries found on server")
		}
		if jellyfin.IsHTTPStatus(errMsg, 401) {
			return m.handleAuthReset(errMsg)
		}
		return m, m.err.SetError(errMsg)

	case playlists.PlaylistsLoadedMsg:
		var cmd tea.Cmd
		m.playlistsScreen, cmd = m.playlistsScreen.Update(msg)
		if msg.Err != nil && jellyfin.IsHTTPStatus(msg.Err, 401) {
			return m.handleAuthReset(msg.Err)
		}
		return m, cmd

	case playlists.PlaylistTracksLoadedMsg:
		var cmd tea.Cmd
		m.playlistsScreen, cmd = m.playlistsScreen.Update(msg)
		if msg.Err != nil && jellyfin.IsHTTPStatus(msg.Err, 401) {
			return m.handleAuthReset(msg.Err)
		}
		return m, cmd

	case library.AlbumsLoadedMsg:
		var cmd tea.Cmd
		m.libraryScreen, cmd = m.libraryScreen.Update(msg)
		if msg.Err != nil && jellyfin.IsHTTPStatus(msg.Err, 401) {
			return m.handleAuthReset(msg.Err)
		}
		return m, cmd

	case library.TracksLoadedMsg:
		var cmd tea.Cmd
		m.libraryScreen, cmd = m.libraryScreen.Update(msg)
		if msg.Err != nil && jellyfin.IsHTTPStatus(msg.Err, 401) {
			return m.handleAuthReset(msg.Err)
		}
		return m, cmd

	case library.AllTracksLoadedMsg:
		var cmd tea.Cmd
		m.libraryScreen, cmd = m.libraryScreen.Update(msg)
		if msg.Err != nil {
			if jellyfin.IsHTTPStatus(msg.Err, 401) {
				return m.handleAuthReset(msg.Err)
			}
			return m, m.err.SetError(msg.Err)
		}
		return m, cmd

	case library.FavoriteChangedMsg:
		if msg.Err != nil {
			if jellyfin.IsHTTPStatus(msg.Err, 401) {
				return m.handleAuthReset(msg.Err)
			}
			return m, m.err.SetError(msg.Err)
		}
		for i := range m.tracks {
			if m.tracks[i].ID == msg.TrackID {
				m.tracks[i].UserData.IsFavorite = msg.IsFavorite
			}
		}
		libraryCmd := m.libraryScreen.PatchTrackFavorite(msg.TrackID, msg.IsFavorite)
		playlistsCmd := m.playlistsScreen.PatchTrackFavorite(msg.TrackID, msg.IsFavorite)
		m.syncQueueScreen()
		return m, tea.Batch(libraryCmd, playlistsCmd)

	case library.AlbumFavoriteChangedMsg:
		if msg.Err != nil {
			if jellyfin.IsHTTPStatus(msg.Err, 401) {
				return m.handleAuthReset(msg.Err)
			}
			return m, m.err.SetError(msg.Err)
		}
		return m, m.libraryScreen.PatchAlbumFavorite(msg.AlbumID, msg.IsFavorite)

	case library.RefilterMsg:
		var cmd tea.Cmd
		m.libraryScreen, cmd = m.libraryScreen.Update(msg)
		return m, cmd

	case playlists.RefilterMsg:
		var cmd tea.Cmd
		m.playlistsScreen, cmd = m.playlistsScreen.Update(msg)
		return m, cmd

	case library.PlayTracksMsg:
		if m.isSubliminalActive() {
			return m, m.err.SetError(errors.New("stop subliminal mode before starting another track"))
		}
		logger.Info("received PlayTracksMsg", "count", len(msg.Tracks), "start", msg.StartIndex)
		var flashCmd tea.Cmd
		if len(msg.Tracks) > 0 {
			m.tracks = append(msg.Tracks, m.tracks...)
			flashCmd = m.flashQueuedItems(msg.Confirmation)
		}
		newM, cmd := m.startPlaybackAt(msg.StartIndex)
		return newM, tea.Batch(cmd, flashCmd)

	case library.AddTrackToQueueMsg:
		if m.isSubliminalActive() {
			return m, m.err.SetError(errors.New("stop subliminal mode before changing the queue"))
		}
		logger.Info("adding track to queue", "title", msg.Track.Name)
		wasEmpty := !m.IsPlaying()
		m.tracks = append(m.tracks, msg.Track)
		flashCmd := m.flashQueuedItems(msg.Confirmation)
		if wasEmpty {
			newM, cmd := m.startPlaybackAt(len(m.tracks) - 1)
			return newM, tea.Batch(cmd, flashCmd)
		}
		m.syncQueueScreen()
		return m, flashCmd

	case library.AddTracksToQueueMsg:
		if msg.Err != nil {
			if jellyfin.IsHTTPStatus(msg.Err, 401) {
				return m.handleAuthReset(msg.Err)
			}
			return m, m.err.SetError(msg.Err)
		}
		if len(msg.Tracks) == 0 {
			return m, nil
		}
		if m.isSubliminalActive() {
			return m, m.err.SetError(errors.New("stop subliminal mode before changing the queue"))
		}
		logger.Info("adding tracks to queue", "count", len(msg.Tracks))
		wasEmpty := !m.IsPlaying()
		firstIdx := len(m.tracks)
		m.tracks = append(m.tracks, msg.Tracks...)
		flashCmd := m.flashQueuedItems(msg.Confirmation)
		if wasEmpty {
			newM, cmd := m.startPlaybackAt(firstIdx)
			return newM, tea.Batch(cmd, flashCmd)
		}
		m.syncQueueScreen()
		return m, flashCmd

	case library.AddShuffledTracksToQueueMsg:
		if msg.Err != nil {
			if jellyfin.IsHTTPStatus(msg.Err, 401) {
				return m.handleAuthReset(msg.Err)
			}
			return m, m.err.SetError(msg.Err)
		}
		if len(msg.Tracks) == 0 {
			return m, nil
		}
		if m.isSubliminalActive() {
			return m, m.err.SetError(errors.New("stop subliminal mode before changing the queue"))
		}
		logger.Info("adding shuffled tracks to queue", "count", len(msg.Tracks))
		wasEmpty := !m.IsPlaying()
		firstIdx := len(m.tracks)
		shuffled := make([]jellyfin.Track, len(msg.Tracks))
		copy(shuffled, msg.Tracks)
		rand.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		m.tracks = append(m.tracks, shuffled...)
		flashCmd := m.flashQueuedItems(msg.Confirmation)
		if wasEmpty {
			newM, cmd := m.startPlaybackAt(firstIdx)
			return newM, tea.Batch(cmd, flashCmd)
		}
		m.syncQueueScreen()
		return m, flashCmd

	case library.EditMetadataMsg:
		m.metadataEditScreen = metadataedit.New(m.styles, m.client, msg.ItemID, msg.ItemType, msg.Track, msg.Album)
		m.metadataEditScreen.SetSize(m.width, m.screenHeight())
		return m.navigate(ScreenMetadataEdit)

	case components.ErrorDismissMsg:
		m.err.Dismiss()
		return m, nil

	case queue.JumpQueueMsg:
		if m.isSubliminalActive() {
			return m, m.err.SetError(errors.New("stop subliminal mode before selecting a track"))
		}
		newM, cmd := m.startPlaybackAt(msg.Index)
		return newM, cmd

	case queue.RemoveQueueMsg:
		if m.isSubliminalActive() {
			return m, m.err.SetError(errors.New("stop subliminal mode before changing the queue"))
		}
		if msg.Index >= 0 && msg.Index < len(m.tracks) {
			removedPendingTarget := m.pendingRepeatItemID != "" && m.tracks[msg.Index].ID == m.pendingRepeatItemID
			if m.tracks[msg.Index].ID == m.repeatTrackID {
				m.repeatTrackID = ""
				m.cancelRepeatWork()
				m.repeatGeneration++
				m.pendingRepeatIndex = -1
				m.pendingRepeatItemID = ""
				m.updateRepeatStatus(false)
			}
			m.tracks = append(m.tracks[:msg.Index], m.tracks[msg.Index+1:]...)
			if m.currentIndex == msg.Index {
				if len(m.tracks) == 0 {
					newM, cmd := m.stopPlayback()
					return newM, cmd
				}
				if m.currentIndex >= len(m.tracks) {
					m.currentIndex = len(m.tracks) - 1
				}
				newM, cmd := m.startPlaybackAt(m.currentIndex)
				return newM, cmd
			} else if m.currentIndex > msg.Index {
				m.currentIndex--
			}
			if removedPendingTarget && m.resolvePendingRepeatIndex() < 0 {
				m.cancelRepeatWork()
				m.repeatGeneration++
				m.pendingRepeatIndex = -1
				m.pendingRepeatItemID = ""
				m.updateRepeatStatus(false)
				if m.repeatQueue && len(m.tracks) > 0 {
					next := m.currentIndex + 1
					if next < 0 || next >= len(m.tracks) {
						next = 0
					}
					return m.startRepeatedPlaybackAt(next)
				}
			}
			m.syncQueueScreen()
		}
		return m, nil

	case queue.QueueActionMsg:
		if msg.Action == "subliminal" {
			return m.toggleSubliminalMode()
		}
		if m.isSubliminalActive() && msg.Action != "edit_metadata" {
			if msg.Action == "toggle_pause" && m.isSubliminalRunning() {
				return m.setSubliminalPaused(m.playerState.Playing)
			}
			return m, m.err.SetError(errors.New("stop subliminal mode before using this queue action"))
		}
		switch msg.Action {
		case "edit_metadata":
			if msg.Index >= 0 && msg.Index < len(m.tracks) {
				trk := m.tracks[msg.Index]
				m.metadataEditScreen = metadataedit.New(m.styles, m.client, trk.ID, "Track", &trk, nil)
				m.metadataEditScreen.SetSize(m.width, m.screenHeight())
				return m.navigate(ScreenMetadataEdit)
			}
		case "toggle_pause":
			m.playerState.Playing = !m.playerState.Playing
			m.syncQueueScreen()
			if m.mpv != nil {
				return m, player.SetPauseCmd(m.mpv, !m.playerState.Playing)
			}
		case "next":
			newM, cmd := m.startPlaybackAt(m.nextIndex(m.currentIndex + 1))
			return newM, cmd
		case "prev":
			if m.playerState.Position > 3.0 {
				if m.mpv != nil {
					return m, player.SeekCmd(m.mpv, 0)
				}
			} else {
				newM, cmd := m.startPlaybackAt(m.currentIndex - 1)
				return newM, cmd
			}
		case "shuffle":
			if len(m.tracks) > 1 {
				currentID := ""
				if m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
					currentID = m.tracks[m.currentIndex].ID
				}
				rand.Shuffle(len(m.tracks), func(i, j int) {
					m.tracks[i], m.tracks[j] = m.tracks[j], m.tracks[i]
				})
				if currentID != "" {
					for idx, t := range m.tracks {
						if t.ID == currentID {
							m.currentIndex = idx
							break
						}
					}
				}
				m.syncQueueScreen()
			}
			return m, nil
		case "repeat_track":
			if msg.TrackID != "" {
				pending := m.resolvePendingRepeatIndex()
				m.cancelRepeatWork()
				m.repeatGeneration++
				m.pendingRepeatIndex = -1
				m.pendingRepeatItemID = ""
				if m.repeatTrackID == msg.TrackID {
					m.repeatTrackID = ""
				} else {
					m.repeatTrackID = msg.TrackID
					m.repeatQueue = false
				}
				m.updateRepeatStatus(false)
				m.syncQueueScreen()
				var cmds []tea.Cmd
				if m.repeatTrackID != "" {
					cmds = append(cmds, m.prefetchRepeatTrack(msg.Index))
				}
				if pending >= 0 && pending < len(m.tracks) {
					newM, playCmd := m.startPlaybackAt(pending)
					m = newM
					cmds = append(cmds, playCmd)
				}
				return m, tea.Batch(cmds...)
			}
			return m, nil
		case "repeat_queue":
			pending := m.resolvePendingRepeatIndex()
			m.cancelRepeatWork()
			m.repeatGeneration++
			m.pendingRepeatIndex = -1
			m.pendingRepeatItemID = ""
			m.repeatQueue = !m.repeatQueue
			if m.repeatQueue {
				m.repeatTrackID = ""
			}
			m.updateRepeatStatus(false)
			m.syncQueueScreen()
			var cmds []tea.Cmd
			if m.repeatQueue {
				cmds = append(cmds, m.prefetchRepeatQueue(m.currentIndex))
			}
			if pending >= 0 && pending < len(m.tracks) {
				newM, playCmd := m.startPlaybackAt(pending)
				m = newM
				cmds = append(cmds, playCmd)
			}
			return m, tea.Batch(cmds...)
		case "clear":
			m.repeatTrackID = ""
			m.repeatQueue = false
			m.cancelRepeatWork()
			m.repeatGeneration++
			m.pendingRepeatIndex = -1
			m.pendingRepeatItemID = ""
			m.playerState.RepeatStatus = ""
			m.tracks = nil
			m.currentIndex = -1
			newM, cmd := m.stopPlayback()
			return newM, cmd
		}

	case mpris.PlayPauseMsg:
		if m.isSubliminalRunning() {
			return m.setSubliminalPaused(m.playerState.Playing)
		}
		m.playerState.Playing = !m.playerState.Playing
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetPauseCmd(m.mpv, !m.playerState.Playing)
		}

	case mpris.PauseMsg:
		if m.isSubliminalRunning() {
			return m.setSubliminalPaused(true)
		}
		return m.setPlaybackPaused(true)

	case mpris.PlayMsg:
		if m.isSubliminalRunning() {
			return m.setSubliminalPaused(false)
		}
		return m.setPlaybackPaused(false)

	case mpris.NextMsg:
		if m.isSubliminalActive() {
			return m, nil
		}
		newM, cmd := m.startPlaybackAt(m.nextIndex(m.currentIndex + 1))
		return newM, cmd

	case mpris.PreviousMsg:
		if m.isSubliminalActive() {
			return m, nil
		}
		if m.playerState.Position > 3.0 {
			if m.mpv != nil {
				return m, player.SeekCmd(m.mpv, 0)
			}
		} else {
			newM, cmd := m.startPlaybackAt(m.currentIndex - 1)
			return newM, cmd
		}

	case mpris.SeekRelativeMsg:
		return m.handleSeek(msg.OffsetSeconds)

	case mpris.SeekAbsoluteMsg:
		return m.handleSeekAbsolute(msg.PositionSeconds)

	case mpris.SetVolumeMsg:
		if m.isSubliminalRunning() {
			return m.setSubliminalVolume(msg.Volume)
		}
		m.playerState.Volume = msg.Volume
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetVolumeCmd(m.mpv, msg.Volume)
		}

	case mpris.SetRateMsg:
		if m.isSubliminalActive() {
			return m, nil
		}
		m.playerState.Speed = msg.Rate
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetSpeedCmd(m.mpv, msg.Rate)
		}
	}

	var cmd tea.Cmd
	switch m.screen {
	case ScreenLogin:
		m.loginScreen, cmd = m.loginScreen.Update(msg)
	case ScreenLibrary:
		m.libraryScreen, cmd = m.libraryScreen.Update(msg)
	case ScreenPlaylists:
		m.playlistsScreen, cmd = m.playlistsScreen.Update(msg)
	case ScreenQueue:
		m.queueScreen, cmd = m.queueScreen.Update(msg)
	case ScreenMetadataEdit:
		m.metadataEditScreen, cmd = m.metadataEditScreen.Update(msg)
	}

	return m, cmd
}

// View is defined in render.go.

// ModelAccessor implementation for mpris.Bridge.Bind

func (m *Model) IsPlaying() bool {
	return len(m.tracks) > 0 && m.currentIndex >= 0 && m.currentIndex < len(m.tracks)
}

func (m *Model) IsPaused() bool {
	return !m.playerState.Playing
}

func (m *Model) HasActiveItem() bool {
	return m.IsPlaying()
}

// flashQueuedItems briefly highlights source rows after a successful queue action.
func (m *Model) flashQueuedItems(confirmation library.QueueConfirmation) tea.Cmd {
	if len(confirmation.ItemIDs) == 0 {
		return nil
	}
	switch confirmation.Origin {
	case library.QueueOriginLibrary:
		return m.libraryScreen.FlashItems(confirmation.ItemIDs...)
	case library.QueueOriginPlaylists:
		return m.playlistsScreen.FlashItems(confirmation.ItemIDs...)
	default:
		return nil
	}
}

func (m *Model) CurrentTitle() string {
	if m.IsPlaying() && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
		return m.tracks[m.currentIndex].Name
	}
	return ""
}

func (m *Model) CurrentAuthors() []string {
	if m.IsPlaying() && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
		return m.tracks[m.currentIndex].Artists
	}
	return nil
}

func (m *Model) CurrentItemID() string {
	if m.IsPlaying() && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
		return m.tracks[m.currentIndex].ID
	}
	return ""
}

func (m *Model) PlayerPosition() float64 {
	return m.playerState.Position
}

func (m *Model) PlayerDuration() float64 {
	if m.IsPlaying() && m.playerState.Duration <= 0 && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
		return m.tracks[m.currentIndex].Duration()
	}
	return m.playerState.Duration
}

func (m *Model) PlayerSpeed() float64 {
	return m.playerState.Speed
}

func (m *Model) PlayerVolume() int {
	return m.playerState.Volume
}

func (m *Model) QueueLength() int {
	return len(m.tracks)
}

func (m *Model) nextIndex(defaultNext int) int {
	if m.repeatTrackID != "" {
		for idx, t := range m.tracks {
			if t.ID == m.repeatTrackID {
				return idx
			}
		}
	}
	if defaultNext >= len(m.tracks) && m.repeatQueue && len(m.tracks) > 0 {
		return 0
	}
	return defaultNext
}
