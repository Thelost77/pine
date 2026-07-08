package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/mpris"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/screens/home"
	"github.com/Thelost77/pine/internal/screens/library"
	"github.com/Thelost77/pine/internal/screens/login"
	"github.com/Thelost77/pine/internal/screens/metadataedit"
	"github.com/Thelost77/pine/internal/screens/search"
	"github.com/Thelost77/pine/internal/screens/series"
	"github.com/Thelost77/pine/internal/screens/serieslist"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/quarckster/go-mpris-server/pkg/types"
)

const headerHeight = 2
const errorBannerHeight = 1
const playerFooterHeight = 1
const syncInterval = 30 * time.Second

// Model is the root application model that manages screen routing.
type Model struct {
	screen    Screen
	backStack []Screen

	login        login.Model
	home         home.Model
	library      library.Model
	detail       detail.Model
	metadataEdit metadataedit.Model
	seriesList   serieslist.Model
	searchCache  *search.Cache
	series       series.Model
	player       player.Model

	// Playback session state
	sessionID                string
	itemID                   string
	episodeID                string
	timeListened             float64
	lastSyncPos              float64
	playGeneration           uint64
	chapters                 []abs.Chapter
	chapterOverlayVisible    bool
	chapterOverlayIndex      int
	trackStartOffset         float64
	trackDuration            float64
	sleepDeadline            time.Time
	sleepDuration            time.Duration
	sleepGeneration          uint64
	queue                    []QueueEntry
	restorePaused            bool
	propertyUnavailableCount int
	lastMprisEmit            time.Time
	seekPending              bool
	lastEvict                time.Time

	// Series auto-continue
	playbackLibraryID string
	playbackSeriesID  string

	keys        KeyMap
	err         components.ErrorBanner
	help        components.HelpOverlay
	width       int
	height      int
	styles      ui.Styles
	config      config.Config
	db          *db.Store
	client      *cache.Client
	cacheStore  *cache.Store
	mpv         player.Player
	mprisBridge *mpris.Bridge
	program     *tea.Program
	mprisState  *MprisState

	palette components.Palette

	lastPlayedTitle   string
	lastPlayedItemID  string
	lastPlayedAuthors []string
	currentAuthors    []string
}

// New creates a new root model. If client is non-nil (authenticated),
// the initial screen is Home; otherwise it starts at Login.
func New(cfg config.Config, store *db.Store, client *cache.Client, cacheStore *cache.Store) Model {
	return NewWithPlayer(cfg, store, client, cacheStore, player.NewMpv())
}

// NewWithPlayer creates a new root model with a specific player implementation.
func NewWithPlayer(cfg config.Config, store *db.Store, client *cache.Client, cacheStore *cache.Store, mpv player.Player) Model {
	styles := ui.NewStyles(cfg.Theme)
	searchCache := search.NewCache(client, cacheStore)
	palette := components.NewPalette()
	palette.SetStyles(styles)
	initialScreen := ScreenLogin
	if client != nil {
		initialScreen = ScreenHome
	}
	return Model{
		screen:       initialScreen,
		backStack:    nil,
		login:        login.New(styles),
		home:         home.New(styles, client),
		library:      library.New(styles, client, searchCache, "", nil),
		searchCache:  searchCache,
		metadataEdit: metadataedit.New(styles, abs.LibraryItem{MediaType: "book"}),
		seriesList:   serieslist.New(styles, client, "", ""),
		series:       series.New(styles, client, "", "", ""),
		player:       player.NewModel(mpv, cfg, styles),
		keys:         DefaultKeyMap(cfg.Keybinds),
		err:          components.NewErrorBanner(styles.Error.Background(lipgloss.Color(cfg.Theme.Error)).Foreground(lipgloss.Color(cfg.Theme.Background))),
		help:         components.NewHelpOverlay(styles),
		styles:       styles,
		config:       cfg,
		db:           store,
		client:       client,
		cacheStore:   cacheStore,
		mpv:          mpv,
		mprisState:   &MprisState{},
		palette:      palette,
	}
}

// Queue returns a copy of the current playback queue.
func (m Model) Queue() []QueueEntry {
	cp := make([]QueueEntry, 0, len(m.queue))
	for _, entry := range m.queue {
		cp = append(cp, cloneQueueEntry(entry))
	}
	return cp
}

// SetProgram sets the bubbletea program reference and starts the MPRIS bridge.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
	m.mprisBridge = mpris.NewBridge(p)
	state := m.mprisState
	m.mprisBridge.Bind(func() mpris.ModelAccessor {
		return mprisStateAccessor{state}
	}, float64(m.config.Player.SeekSeconds))
	m.mprisBridge.Start()
}

// mprisStateAccessor implements mpris.ModelAccessor by reading from shared MprisState.
type mprisStateAccessor struct{ s *MprisState }

func (a mprisStateAccessor) IsPlaying() bool          { return a.s.IsPlaying }
func (a mprisStateAccessor) IsPaused() bool           { return a.s.IsPaused }
func (a mprisStateAccessor) HasActiveItem() bool      { return a.s.HasActiveItem }
func (a mprisStateAccessor) CurrentTitle() string     { return a.s.Title }
func (a mprisStateAccessor) CurrentAuthors() []string { return a.s.Authors }
func (a mprisStateAccessor) CurrentItemID() string    { return a.s.ItemID }
func (a mprisStateAccessor) PlayerPosition() float64  { return a.s.Position }
func (a mprisStateAccessor) PlayerDuration() float64  { return a.s.Duration }
func (a mprisStateAccessor) PlayerVolume() int        { return a.s.Volume }
func (a mprisStateAccessor) PlayerSpeed() float64     { return a.s.Speed }
func (a mprisStateAccessor) QueueLength() int         { return a.s.QueueLength }

func (m *Model) mprisPlaybackCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	handler := m.mprisBridge.EventHandler()
	return func() tea.Msg {
		_ = handler.Player.OnPlayback()
		return nil
	}
}

func (m *Model) mprisPlayPauseCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	handler := m.mprisBridge.EventHandler()
	return func() tea.Msg {
		_ = handler.Player.OnPlayPause()
		return nil
	}
}

func (m *Model) mprisEndedCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	handler := m.mprisBridge.EventHandler()
	return func() tea.Msg {
		_ = handler.Player.OnEnded()
		return nil
	}
}

func (m *Model) mprisTitleCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	handler := m.mprisBridge.EventHandler()
	return func() tea.Msg {
		_ = handler.Player.OnTitle()
		return nil
	}
}

func (m *Model) mprisPositionCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	now := time.Now()
	if now.Sub(m.lastMprisEmit) < time.Second {
		return nil
	}
	m.lastMprisEmit = now
	handler := m.mprisBridge.EventHandler()
	pos := types.Microseconds(m.player.Position * 1_000_000)
	return func() tea.Msg {
		_ = handler.Player.OnSeek(pos)
		return nil
	}
}

func (m *Model) mprisVolumeCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	handler := m.mprisBridge.EventHandler()
	return func() tea.Msg {
		_ = handler.Player.OnVolume()
		return nil
	}
}

func (m *Model) syncMprisState() {
	m.mprisState.IsPlaying = m.isPlaying() && m.player.Playing
	m.mprisState.IsPaused = m.isPlaying() && !m.player.Playing
	m.mprisState.HasActiveItem = m.isPlaying()
	m.mprisState.ItemID = m.itemID
	if m.itemID == "" {
		m.mprisState.ItemID = m.lastPlayedItemID
	}
	m.mprisState.Title = m.player.Title
	if m.player.Title == "" {
		m.mprisState.Title = m.lastPlayedTitle
	}
	m.mprisState.Authors = m.currentAuthors
	if m.player.Title == "" {
		m.mprisState.Authors = m.lastPlayedAuthors
	}
	m.mprisState.Position = m.player.Position
	m.mprisState.Duration = m.player.Duration
	m.mprisState.Volume = m.player.Volume
	m.mprisState.Speed = m.player.Speed
	m.mprisState.QueueLength = len(m.queue)
}

// Init returns the initial command for the active screen.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.initScreen(m.screen)}
	if m.client != nil && m.db != nil {
		cmds = append(cmds, restoreSessionCmd(m.client, m.db))
	}
	return tea.Batch(cmds...)
}

// Update dispatches messages to the active screen and handles navigation.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Workaround for terminal emulators (like Ghostty with kitty keyboard protocol)
	// that set the Alt modifier when typing international characters via AltGr.
	// bubbles/textinput ignores KeyRunes if Alt is true.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyRunes && keyMsg.Alt {
		if len(keyMsg.Runes) == 1 && keyMsg.Runes[0] >= 0x80 {
			keyMsg.Alt = false
			msg = keyMsg
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.player.SetWidth(msg.Width)
		m.err.SetWidth(msg.Width)
		m.help.SetSize(msg.Width, msg.Height)
		m.propagateSize()
		return m, nil

	case components.ErrMsg:
		if msg.Err == nil {
			return m, nil
		}
		logger.Error("app error", "err", msg.Err, "screen", m.screen)
		if components.IsUnauthorized(msg.Err) {
			logger.Warn("401 unauthorized, redirecting to login")
			m.client = nil
			m.searchCache = search.NewCache(nil, nil)
			m.screen = ScreenLogin
			m.backStack = nil
			m.login = login.New(m.styles)
			return m, m.login.Init()
		}
		cmd := m.err.SetError(msg.Err)
		m.propagateSize()
		return m, cmd

	case components.ErrorDismissMsg:
		m.err.Dismiss()
		m.propagateSize()
		return m, nil

	case NavigateMsg:
		return m.navigate(msg.Screen)

	case BackMsg:
		if len(m.backStack) == 0 {
			return m, nil
		}
		return m.back()

	case login.LoginSuccessMsg:
		logger.Info("login success", "server", msg.ServerURL, "user", msg.Username)
		absClient := abs.NewClient(msg.ServerURL, msg.Token)
		m.client = cache.NewClient(absClient, m.cacheStore)
		if m.db != nil {
			accountID := fmt.Sprintf("%s@%s", msg.ServerURL, msg.Username)
			if err := m.db.SaveAccount(db.Account{
				ID:        accountID,
				ServerURL: msg.ServerURL,
				Username:  msg.Username,
				Token:     msg.Token,
				IsDefault: true,
			}); err != nil {
				logger.Warn("failed to save account", "err", err)
			}
		}
		m.home = home.New(m.styles, m.client)
		m.library = library.New(m.styles, m.client, m.searchCache, "", nil)
		m.searchCache = search.NewCache(m.client, m.cacheStore)
		m.metadataEdit = metadataedit.New(m.styles, abs.LibraryItem{MediaType: "book"})
		m.seriesList = serieslist.New(m.styles, m.client, "", "")
		m.series = series.New(m.styles, m.client, "", "", "")
		m.login, _ = m.login.Update(msg)
		m.backStack = nil
		m.screen = ScreenHome
		m.propagateSize()
		cmd := m.initScreen(ScreenHome)
		cmds := []tea.Cmd{cmd, m.prewarmCacheCmd()}
		return m, tea.Batch(cmds...)

	case home.NavigateDetailMsg:
		logger.Info("navigate to detail", "itemID", msg.Item.ID, "mediaType", msg.Item.MediaType, "title", msg.Item.Media.Metadata.Title)
		m.detail = detail.New(m.styles, msg.Item)
		m, navCmd := m.navigate(ScreenDetail)
		return m, tea.Batch(m.detailLoadCmds(msg.Item, navCmd)...)

	case home.PlayEpisodeMsg:
		logger.Info("play episode from home", "itemID", msg.Item.ID, "episodeID", msg.Episode.ID, "title", msg.Episode.Title)
		return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: msg.Episode})

	case home.PlayMsg:
		logger.Info("play book from home", "itemID", msg.Item.ID, "title", msg.Item.Media.Metadata.Title)
		return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})

	case home.AddToQueueMsg:
		if !m.isPlaying() {
			if msg.Episode != nil {
				return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: *msg.Episode})
			}
			return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
		}
		m.enqueueQueueEntry(QueueEntry{Item: msg.Item, Episode: cloneEpisodePtr(msg.Episode)}, false)
		return m, nil

	case home.PlayNextMsg:
		if !m.isPlaying() {
			if msg.Episode != nil {
				return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: *msg.Episode})
			}
			return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
		}
		m.enqueueQueueEntry(QueueEntry{Item: msg.Item, Episode: cloneEpisodePtr(msg.Episode)}, true)
		return m, nil

	case home.NavigateLibraryMsg:
		m.library.Configure(msg.LibraryID, msg.Libraries)
		return m.navigate(ScreenLibrary)

	case home.GoBackMsg:
		if len(m.backStack) > 0 {
			return m.back()
		}
		return m, nil

	case home.PersonalizedMsg:
		var cmd tea.Cmd
		m.home, cmd = m.home.Update(msg)
		cmds := []tea.Cmd{}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.prewarmCacheCmd())
		if len(cmds) == 1 {
			return m, cmds[0]
		}
		return m, tea.Batch(cmds...)

	case library.LibraryItemsMsg:
		var cmd tea.Cmd
		m.library, cmd = m.library.Update(msg)
		return m, cmd

	case library.NavigateDetailMsg:
		m.detail = detail.New(m.styles, msg.Item)
		m, navCmd := m.navigate(ScreenDetail)
		return m, tea.Batch(m.detailLoadCmds(msg.Item, navCmd)...)

	case library.GoBackMsg:
		if len(m.backStack) > 0 {
			return m.back()
		}
		return m, nil

	case library.NavigateSeriesListMsg:
		m.seriesList = serieslist.New(m.styles, m.client, msg.LibraryID, msg.LibraryName)
		return m.navigate(ScreenSeriesList)

	case EpisodesLoadedMsg:
		if msg.Err != nil {
			logger.Error("failed to load episodes", "err", msg.Err)
			if m2, cmd, ok := m.checkUnauthorized(msg.Err); ok {
				return m2, cmd
			}
		} else {
			logger.Info("episodes loaded", "count", len(msg.Episodes))
		}
		if msg.ItemID != "" && msg.ItemID != m.detail.ItemID() {
			return m, nil
		}
		if msg.Err == nil && len(msg.Episodes) > 0 {
			m.detail.SetEpisodes(msg.Episodes)
		}
		return m, nil

	case BookDetailLoadedMsg:
		if msg.Err != nil {
			logger.Error("failed to load detail item", "err", msg.Err)
			if m2, cmd, ok := m.checkUnauthorized(msg.Err); ok {
				return m2, cmd
			}
			return m, nil
		}
		if msg.ItemID != "" && msg.ItemID != m.detail.ItemID() {
			return m, nil
		}
		if msg.Item != nil {
			m.detail.SetItem(*msg.Item)
		}
		return m, nil

	// --- Playback lifecycle ---

	case detail.PlayCmd:
		logger.Info("play cmd", "itemID", msg.Item.ID, "title", msg.Item.Media.Metadata.Title)
		return m.handlePlayCmd(msg)

	case detail.PlayEpisodeCmd:
		logger.Info("play episode cmd", "itemID", msg.Item.ID, "episodeID", msg.Episode.ID, "title", msg.Episode.Title)
		return m.handlePlayEpisodeCmd(msg)

	case detail.AddBookmarkCmd:
		return m.handleAddBookmark(msg)

	case detail.AddToQueueCmd:
		if !m.isPlaying() {
			if msg.Episode != nil {
				return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: *msg.Episode})
			}
			return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
		}
		m.enqueueQueueEntry(QueueEntry{Item: msg.Item, Episode: cloneEpisodePtr(msg.Episode)}, false)
		return m, nil

	case detail.PlayNextCmd:
		if !m.isPlaying() {
			if msg.Episode != nil {
				return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: *msg.Episode})
			}
			return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
		}
		m.enqueueQueueEntry(QueueEntry{Item: msg.Item, Episode: cloneEpisodePtr(msg.Episode)}, true)
		return m, nil

	case detail.EditMetadataCmd:
		return m.openMetadataEditor(msg)

	case metadataedit.BackMsg:
		return m.back()

	case metadataedit.SaveCmd:
		return m.handleMetadataSave(msg)

	case metadataedit.SaveEpisodeCmd:
		return m.handleEpisodeMetadataSave(msg)

	case metadataedit.SavedMsg:
		return m.handleMetadataSaved(msg)

	case metadataedit.SavedEpisodeMsg:
		return m.handleEpisodeMetadataSaved(msg)

	case detail.NavigateSeriesMsg:
		m.series = series.New(m.styles, m.client, msg.LibraryID, msg.SeriesID, msg.CurrentItemID)
		return m.navigate(ScreenSeries)

	case serieslist.NavigateSeriesMsg:
		m.series = series.New(m.styles, m.client, msg.LibraryID, msg.SeriesID, "")
		return m.navigate(ScreenSeries)

	case serieslist.BackMsg:
		return m.back()

	case detail.MarkFinishedCmd:
		return m.handleMarkFinished(msg)

	case detail.MarkFinishedMsg:
		m.detail, _ = m.detail.Update(msg)
		return m, nil

	case detail.DeleteItemCmd:
		return m.handleDeleteItem(msg)

	case detail.ItemDeletedMsg:
		m.home.RemoveItem(msg.ItemID)
		m.library.RemoveItem(msg.ItemID)
		m.home.InvalidateLibrary(m.home.SelectedLibraryID())
		m.library.InvalidateLibrary(m.home.SelectedLibraryID())
		if m.searchCache != nil {
			m.searchCache.Invalidate(m.home.SelectedLibraryID())
		}
		m, navCmd := m.back()
		return m, tea.Batch(navCmd, m.home.ReloadCmd(), m.library.ReloadCmd(), m.prewarmCacheCmd())

	case detail.DeleteEpisodeCmd:
		return m.handleDeleteEpisode(msg)

	case detail.EpisodeDeletedMsg:
		m.home.RemoveEpisode(msg.ItemID, msg.EpisodeID)
		m.library.RemoveEpisode(msg.ItemID, msg.EpisodeID)
		m.home.InvalidateLibrary(m.home.SelectedLibraryID())
		m.library.InvalidateLibrary(m.home.SelectedLibraryID())
		if m.searchCache != nil {
			m.searchCache.Invalidate(m.home.SelectedLibraryID())
		}
		m.detail, _ = m.detail.Update(msg)
		return m, tea.Batch(m.home.ReloadCmd(), m.library.ReloadCmd(), m.prewarmCacheCmd())

	case detail.SeekToBookmarkCmd:
		return m.handleSeekToBookmark(msg)

	case detail.SeekToChapterCmd:
		return m.handleSeekToBookmark(detail.SeekToBookmarkCmd{Item: m.detail.Item(), Time: msg.Time})

	case detail.DeleteBookmarkCmd:
		return m.handleDeleteBookmark(msg)

	case detail.UpdateBookmarkCmd:
		return m.handleUpdateBookmark(msg)

	case detail.BookmarksUpdatedMsg:
		m.detail, _ = m.detail.Update(msg)
		return m, nil

	case series.NavigateDetailMsg:
		m.detail = detail.New(m.styles, msg.Item)
		m, navCmd := m.navigate(ScreenDetail)
		return m, tea.Batch(m.detailLoadCmds(msg.Item, navCmd)...)

	case series.BackMsg:
		return m.back()

	case PlaySessionMsg:
		logger.Info("play session started", "sessionID", msg.Session.SessionID, "itemID", msg.Session.ItemID, "episodeID", msg.Session.EpisodeID, "currentTime", msg.Session.CurrentTime, "duration", msg.Session.Duration)
		return m.handlePlaySessionMsg(msg)

	case RestoreSessionMsg:
		if msg.Item == nil {
			logger.Debug("no session to restore")
			return m, nil
		}
		logger.Info("restoring session", "itemID", msg.Item.ID, "savedEpisodeID", msg.SavedEpisodeID, "resolvedEpisodeID", func() string {
			if msg.Episode == nil {
				return ""
			}
			return msg.Episode.ID
		}())
		m.setSeriesContext(*msg.Item)
		return m, m.startRestorePlaybackCmd(msg)

	case RestorePlaySessionMsg:
		m.restorePaused = true
		return m.handlePlaySessionMsg(msg.PlaySessionMsg)

	case player.PlayerReadyMsg:
		return m.handlePlayerReady()

	case player.PositionMsg:
		return m.handlePositionMsg(msg)

	case SyncTickMsg:
		m, cmd := m.handleSyncTick()
		if m.cacheStore != nil && time.Since(m.lastEvict) > 5*time.Minute {
			_ = m.cacheStore.EvictExpired()
			m.lastEvict = time.Now()
		}
		return m, cmd

	case player.PlayerLaunchErrMsg:
		logger.Error("player launch failed", "err", msg.Err)
		m.clearPlaybackSessionState()
		errCmd := m.err.SetError(msg.Err)
		m.propagateSize()
		return m, tea.Batch(errCmd, player.QuitCmd(m.mpv))

	case player.PlayerQuitMsg:
		return m, nil

	// --- MPRIS control messages ---

	case mpris.PlayPauseMsg:
		if m.isPlaying() {
			m.player.Playing = !m.player.Playing
			m.syncMprisState()
			if m.mpv != nil {
				return m, tea.Batch(m.mprisPlayPauseCmd(), player.TogglePauseCmd(m.mpv, m.player.Playing))
			}
			return m, m.mprisPlayPauseCmd()
		}
		return m, nil

	case mpris.SeekMsg:
		if m.isPlaying() {
			return m.handleSeek(msg.Offset)
		}
		return m, nil

	case mpris.SetVolumeMsg:
		m.player.Volume = msg.Volume
		m.syncMprisState()
		if m.mpv != nil {
			return m, tea.Batch(m.mprisVolumeCmd(), player.SetVolumeCmd(m.mpv, msg.Volume))
		}
		return m, m.mprisVolumeCmd()

	case mpris.SetRateMsg:
		m.player.Speed = msg.Rate
		m.syncMprisState()
		if m.mpv != nil {
			return m, tea.Batch(m.mprisPlaybackCmd(), player.SetSpeedCmd(m.mpv, msg.Rate))
		}
		return m, m.mprisPlaybackCmd()

	case PlaybackStoppedMsg:
		return m, nil

	case PrewarmDoneMsg:
		var cmd tea.Cmd
		m.library, cmd = m.library.Update(library.SearchCacheReadyMsg{})
		return m, cmd

	case search.CacheReadyMsg:
		return m.updateScreen(msg)

	case SleepTimerExpiredMsg:
		if m.isPlaying() && !m.sleepDeadline.IsZero() && msg.Generation == m.sleepGeneration {
			logger.Info("sleep timer expired, stopping playback")
			m.sleepDeadline = time.Time{}
			m.sleepDuration = 0
			m.player.SleepRemaining = ""
			m, stopCmd := m.stopPlayback()
			return m, stopCmd
		}
		return m, nil

	case SeriesContinueMsg:
		if msg.Err != nil {
			logger.Warn("series continue failed", "err", msg.Err)
			if m2, cmd, ok := m.checkUnauthorized(msg.Err); ok {
				return m2, cmd
			}
			cmd := m.err.SetError(msg.Err)
			m.propagateSize()
			return m, cmd
		}
		if msg.Item.ID == "" {
			return m, nil
		}
		logger.Info("auto-continuing series", "nextItemID", msg.Item.ID, "title", msg.Item.Media.Metadata.Title)
		return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})

	case PlaybackErrorMsg:
		if msg.Err != nil {
			logger.Error("playback error", "err", msg.Err)
			if m2, cmd, ok := m.checkUnauthorized(msg.Err); ok {
				return m2, cmd
			}
			cmd := m.err.SetError(msg.Err)
			m.propagateSize()
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.isPlaying() {
				_, stopCmd := m.stopPlayback()
				return m, tea.Batch(stopCmd, tea.Quit)
			}
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Help) {
			m.help.Toggle()
			return m, nil
		}
		if m.help.Visible() {
			if key.Matches(msg, m.keys.Back) {
				m.help.Hide()
			}
			return m, nil
		}
		if m.palette.Visible() {
			cmd, handled := m.palette.Update(msg)
			if !m.palette.Visible() {
				if action, payload, libID, itemID, data := m.palette.SelectedAction(); action != components.ActionNone {
					m.palette.ClearSelection()
					return m.handlePaletteAction(action, payload, libID, itemID, data)
				}
				return m, nil
			}
			if handled {
				return m, cmd
			}
			return m.updateScreen(msg)
		}
		if m.screen == ScreenMetadataEdit {
			return m.updateScreen(msg)
		}
		if m.chapterOverlayVisible {
			if key.Matches(msg, m.keys.Back) {
				m.closeChapterOverlay()
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				if m.chapterOverlayIndex >= 0 && m.chapterOverlayIndex < len(m.chapters) {
					target := m.chapters[m.chapterOverlayIndex].Start
					m.closeChapterOverlay()
					return m.seekToChapter(target, true)
				}
				m.closeChapterOverlay()
				return m, nil
			}
			if msg.String() == "j" || msg.String() == "down" {
				m.moveChapterOverlaySelection(1)
				return m, nil
			}
			if msg.String() == "k" || msg.String() == "up" {
				m.moveChapterOverlaySelection(-1)
				return m, nil
			}
			if msg.String() == "H" {
				m.setChapterOverlaySelection(0)
				return m, nil
			}
			if msg.String() == "L" {
				m.setChapterOverlaySelection(len(m.chapters) - 1)
				return m, nil
			}
		}
		if m.err.HasError() {
			m.err.Dismiss()
			m.propagateSize()
			return m, nil
		}
		if key.Matches(msg, m.keys.GlobalPalette) {
			m.openGlobalPalette()
			return m, nil
		}
		if key.Matches(msg, m.keys.ChapterOverlay) {
			if m.canOpenChapterOverlay() {
				m.openChapterOverlay()
			}
			return m, nil
		}
		// Global quit: q always quits the app.
		if m.screen != ScreenLogin && key.Matches(msg, m.keys.Quit) {
			if m.isPlaying() {
				m, stopCmd := m.stopPlayback()
				return m, tea.Batch(stopCmd, tea.Quit)
			}
			return m, tea.Quit
		}
		// Global back: esc/left goes back but never quits.
		if m.screen != ScreenLogin {
			isConfirmVisible := m.screen == ScreenDetail && m.detail.ConfirmOverlayVisible()
			if !isConfirmVisible && key.Matches(msg, m.keys.Back) {
				if len(m.backStack) > 0 {
					return m.back()
				}
				return m, nil
			}
		}
		// When playing, playback keys take priority over screen keys.
		if m.isPlaying() {
			if key.Matches(msg, m.keys.NextInQueue) {
				return m.skipToNextQueued()
			}
			if len(m.chapters) > 0 {
				if key.Matches(msg, m.keys.NextChapter) {
					return m.seekToChapter(m.nextChapter())
				}
				if key.Matches(msg, m.keys.PrevChapter) {
					return m.seekToChapter(m.prevChapter())
				}
			}
			if key.Matches(msg, m.keys.SleepTimer) {
				return m.cycleSleepTimer()
			}
			// Handle seek keys with offset conversion (player model doesn't know about track offsets)
			if key.Matches(msg, m.player.SeekForwardKey()) {
				return m.handleSeek(float64(m.config.Player.SeekSeconds))
			}
			if key.Matches(msg, m.player.SeekBackKey()) {
				return m.handleSeek(-float64(m.config.Player.SeekSeconds))
			}
			pm, pcmd := m.player.Update(msg)
			m.player = pm
			if pcmd != nil {
				return m, pcmd
			}
		}
		return m.updateScreen(msg)
	}

	m.syncMprisState()
	return m.updateScreen(msg)
}

// --- Sleep timer ---

var sleepDurations = []time.Duration{
	0,
	15 * time.Minute,
	30 * time.Minute,
	45 * time.Minute,
	60 * time.Minute,
}

func (m Model) cycleSleepTimer() (Model, tea.Cmd) {
	nextIdx := 0
	for i, d := range sleepDurations {
		if d == m.sleepDuration {
			nextIdx = (i + 1) % len(sleepDurations)
			break
		}
	}

	m.sleepDuration = sleepDurations[nextIdx]
	if m.sleepDuration == 0 {
		m.sleepDeadline = time.Time{}
		m.player.SleepRemaining = ""
		logger.Info("sleep timer disabled")
		return m, nil
	}

	m.sleepGeneration++
	m.sleepDeadline = time.Now().Add(m.sleepDuration)
	m.player.SleepRemaining = formatSleepRemaining(m.sleepDuration)
	logger.Info("sleep timer set", "duration", m.sleepDuration)
	return m, sleepTimerCmd(m.sleepDuration, m.sleepGeneration)
}

func sleepTimerCmd(d time.Duration, generation uint64) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return SleepTimerExpiredMsg{Generation: generation}
	})
}

func formatSleepRemaining(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m Model) canOpenChapterOverlay() bool {
	return m.isPlaying() && len(m.chapters) > 0
}

func (m *Model) openChapterOverlay() {
	if !m.canOpenChapterOverlay() {
		return
	}
	m.chapterOverlayIndex = m.currentChapterOverlayIndex()
	m.chapterOverlayVisible = true
}

func (m *Model) closeChapterOverlay() {
	m.chapterOverlayVisible = false
}

func (m *Model) resetChapterOverlay() {
	m.chapterOverlayVisible = false
	m.chapterOverlayIndex = 0
}

func (m *Model) moveChapterOverlaySelection(delta int) {
	if !m.chapterOverlayVisible || len(m.chapters) == 0 {
		return
	}
	m.setChapterOverlaySelection(m.chapterOverlayIndex + delta)
}

func (m *Model) setChapterOverlaySelection(index int) {
	if len(m.chapters) == 0 {
		m.chapterOverlayIndex = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.chapters) {
		index = len(m.chapters) - 1
	}
	m.chapterOverlayIndex = index
}

func (m Model) currentChapterOverlayIndex() int {
	if len(m.chapters) == 0 {
		return 0
	}
	current := 0
	for i, ch := range m.chapters {
		if m.player.Position >= ch.Start {
			current = i
		}
		if m.player.Position >= ch.Start && m.player.Position < ch.End {
			return i
		}
	}
	return current
}

func (m *Model) clearPlaybackSessionState() {
	m.sessionID = ""
	m.itemID = ""
	m.episodeID = ""
	m.timeListened = 0
	m.lastSyncPos = 0
	m.chapters = nil
	m.resetChapterOverlay()
	m.trackStartOffset = 0
	m.trackDuration = 0
	m.sleepDeadline = time.Time{}
	m.sleepDuration = 0
	m.player.SleepRemaining = ""
	m.player.Playing = false
	m.player.Title = ""
	m.currentAuthors = nil
	m.player.Position = 0
	m.player.Duration = 0
	m.playbackSeriesID = ""
	m.playbackLibraryID = ""
}

// checkUnauthorized checks if the error indicates a 401 response.
// If so, it resets the client and redirects to login.
// Returns the updated model and init command plus true if 401 was handled.
func (m Model) checkUnauthorized(err error) (Model, tea.Cmd, bool) {
	if !components.IsUnauthorized(err) {
		return m, nil, false
	}
	logger.Warn("401 unauthorized, redirecting to login")
	m.client = nil
	m.searchCache = search.NewCache(nil, nil)
	m.screen = ScreenLogin
	m.backStack = nil
	m.login = login.New(m.styles)
	cmd := m.login.Init()
	m.propagateSize()
	return m, cmd, true
}

func (m *Model) openGlobalPalette() {
	playerItems, navItems := m.buildStaticPaletteItems()
	contextItems := m.buildContextPaletteItems()

	items := playerItems
	if len(contextItems) > 0 {
		items = append(items, contextItems...)
	}
	items = append(items, navItems...)
	m.palette.Open(items, m.contentSearchFunc())
}

func (m Model) contentSearchFunc() components.SearchFunc {
	return func(query string) []components.PaletteItem {
		if m.searchCache == nil {
			return nil
		}
		libID, libMediaType := m.currentLibraryForSearch()
		if libID == "" {
			return nil
		}
		ctx := context.Background()
		results, err := m.searchCache.Search(ctx, libID, libMediaType, query)
		if err != nil || len(results) == 0 {
			return nil
		}
		items := make([]components.PaletteItem, 0, len(results))
		for _, res := range results {
			label := res.Media.Metadata.Title
			if res.MediaType == "podcast" && res.RecentEpisode != nil {
				label = res.RecentEpisode.Title + " — " + res.Media.Metadata.Title
			}
			itemCopy := res
			items = append(items, components.PaletteItem{
				Label:     label,
				Action:    components.ActionContentNavigate,
				LibraryID: res.LibraryID,
				ItemID:    res.ID,
				Data:      itemCopy,
			})
		}
		return items
	}
}

// currentLibraryForSearch returns the library ID and media type that should be
// searched based on the active screen. This ensures the palette searches the
// library the user is currently browsing, not just the home screen's library.
func (m Model) currentLibraryForSearch() (libID, libMediaType string) {
	switch m.screen {
	case ScreenLibrary:
		return m.library.SelectedLibraryID(), m.library.SelectedLibraryMediaType()
	case ScreenSeriesList:
		return m.seriesList.SelectedLibraryID(), "book"
	case ScreenDetail:
		item := m.detail.Item()
		if item.LibraryID != "" {
			return item.LibraryID, item.MediaType
		}
		return m.home.SelectedLibraryID(), m.home.SelectedLibraryMediaType()
	case ScreenHome:
		return m.home.SelectedLibraryID(), m.home.SelectedLibraryMediaType()
	default:
		return m.home.SelectedLibraryID(), m.home.SelectedLibraryMediaType()
	}
}

func (m Model) buildStaticPaletteItems() (player, nav []components.PaletteItem) {
	nav = []components.PaletteItem{
		{Label: "Navigation", IsHeader: true},
		{Label: "Go Home", Action: components.ActionGoHome},
		{Label: "Go Library", Action: components.ActionGoLibrary},
		{Label: "Go Series List", Action: components.ActionGoSeriesList},
	}

	if !m.isPlaying() {
		return nil, nav
	}

	player = []components.PaletteItem{
		{Label: "Player", IsHeader: true},
		{Label: "Play / Pause", Action: components.ActionTogglePlay},
		{Label: "Seek Forward", Action: components.ActionSeekForward},
		{Label: "Seek Backward", Action: components.ActionSeekBackward},
		{Label: "Speed Up", Action: components.ActionSpeedUp},
		{Label: "Speed Down", Action: components.ActionSpeedDown},
	}
	if len(m.chapters) > 0 {
		player = append(player,
			components.PaletteItem{Label: "Next Chapter", Action: components.ActionNextChapter},
			components.PaletteItem{Label: "Previous Chapter", Action: components.ActionPrevChapter},
		)
	}

	sleep := []components.PaletteItem{
		{Label: "Sleep Timer", IsHeader: true},
		{Label: "Sleep Timer: 15m", Action: components.ActionSleep15},
		{Label: "Sleep Timer: 30m", Action: components.ActionSleep30},
		{Label: "Sleep Timer: 45m", Action: components.ActionSleep45},
		{Label: "Sleep Timer: 60m", Action: components.ActionSleep60},
		{Label: "Sleep Timer: Off", Action: components.ActionSleepOff},
	}
	nav = append(nav, sleep...)

	if len(m.queue) > 0 {
		nav = append(nav,
			components.PaletteItem{Label: "Queue", IsHeader: true},
			components.PaletteItem{Label: "Clear Queue", Action: components.ActionClearQueue},
		)
	}

	return player, nav
}

func (m Model) buildContextPaletteItems() []components.PaletteItem {
	var items []components.PaletteItem
	switch m.screen {
	case ScreenHome:
		items = getPaletteActions(&m.home)
	case ScreenLibrary:
		items = getPaletteActions(&m.library)
	case ScreenDetail:
		items = getPaletteActions(&m.detail)
	case ScreenSeriesList:
		items = getPaletteActions(&m.seriesList)
	case ScreenSeries:
		items = getPaletteActions(&m.series)
	}

	if len(items) == 0 {
		return nil
	}

	filtered := make([]components.PaletteItem, 0, len(items))
	for _, item := range items {
		if item.Action == components.ActionAddBookmark {
			if !m.isPlaying() || m.itemID != item.ItemID {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	if len(filtered) == 0 || (len(filtered) == 1 && filtered[0].IsHeader) {
		return nil
	}

	return filtered
}

func getPaletteActions(provider PaletteContextProvider) []components.PaletteItem {
	return provider.SelectedPaletteActions()
}

type PaletteContextProvider interface {
	SelectedPaletteActions() []components.PaletteItem
}

func (m Model) handlePaletteAction(action components.PaletteAction, payload, libraryID, itemID string, data any) (Model, tea.Cmd) {
	switch action {
	case components.ActionGoHome:
		return m.navigate(ScreenHome)
	case components.ActionGoLibrary:
		return m.navigate(ScreenLibrary)
	case components.ActionGoSeriesList:
		m.seriesList = serieslist.New(m.styles, m.client, m.home.SelectedLibraryID(), "")
		return m.navigate(ScreenSeriesList)
	case components.ActionTogglePlay:
		if m.isPlaying() {
			m.player.Playing = !m.player.Playing
			m.syncMprisState()
			if m.mpv != nil {
				return m, tea.Batch(m.mprisPlayPauseCmd(), player.TogglePauseCmd(m.mpv, m.player.Playing))
			}
			return m, m.mprisPlayPauseCmd()
		}
		return m, nil
	case components.ActionSeekForward:
		return m.handleSeek(float64(m.config.Player.SeekSeconds))
	case components.ActionSeekBackward:
		return m.handleSeek(-float64(m.config.Player.SeekSeconds))
	case components.ActionSpeedUp:
		m.player.Speed += 0.25
		if m.player.Speed > 3.0 {
			m.player.Speed = 3.0
		}
		if m.mpv != nil {
			return m, player.SetSpeedCmd(m.mpv, m.player.Speed)
		}
		return m, nil
	case components.ActionSpeedDown:
		m.player.Speed -= 0.25
		if m.player.Speed < 0.25 {
			m.player.Speed = 0.25
		}
		if m.mpv != nil {
			return m, player.SetSpeedCmd(m.mpv, m.player.Speed)
		}
		return m, nil
	case components.ActionNextChapter:
		if len(m.chapters) > 0 {
			return m.seekToChapter(m.nextChapter())
		}
		return m, nil
	case components.ActionPrevChapter:
		if len(m.chapters) > 0 {
			return m.seekToChapter(m.prevChapter())
		}
		return m, nil
	case components.ActionSleep15:
		m.sleepDuration = 15 * time.Minute
		m.sleepGeneration++
		m.sleepDeadline = time.Now().Add(m.sleepDuration)
		m.player.SleepRemaining = formatSleepRemaining(m.sleepDuration)
		return m, sleepTimerCmd(m.sleepDuration, m.sleepGeneration)
	case components.ActionSleep30:
		m.sleepDuration = 30 * time.Minute
		m.sleepGeneration++
		m.sleepDeadline = time.Now().Add(m.sleepDuration)
		m.player.SleepRemaining = formatSleepRemaining(m.sleepDuration)
		return m, sleepTimerCmd(m.sleepDuration, m.sleepGeneration)
	case components.ActionSleep45:
		m.sleepDuration = 45 * time.Minute
		m.sleepGeneration++
		m.sleepDeadline = time.Now().Add(m.sleepDuration)
		m.player.SleepRemaining = formatSleepRemaining(m.sleepDuration)
		return m, sleepTimerCmd(m.sleepDuration, m.sleepGeneration)
	case components.ActionSleep60:
		m.sleepDuration = 60 * time.Minute
		m.sleepGeneration++
		m.sleepDeadline = time.Now().Add(m.sleepDuration)
		m.player.SleepRemaining = formatSleepRemaining(m.sleepDuration)
		return m, sleepTimerCmd(m.sleepDuration, m.sleepGeneration)
	case components.ActionSleepOff:
		m.sleepDeadline = time.Time{}
		m.sleepDuration = 0
		m.player.SleepRemaining = ""
		return m, nil
	case components.ActionClearQueue:
		m.queue = nil
		return m, nil
	case components.ActionOpenDetail:
		if payload == "series" {
			m.series = series.New(m.styles, m.client, libraryID, itemID, "")
			return m.navigate(ScreenSeries)
		}
		if data != nil {
			if item, ok := data.(abs.LibraryItem); ok {
				if item.MediaType == "series" {
					m.series = series.New(m.styles, m.client, libraryID, itemID, "")
					return m.navigate(ScreenSeries)
				}
				m.detail = detail.New(m.styles, item)
				m2, navCmd := m.navigate(ScreenDetail)
				return m2, tea.Batch(m.detailLoadCmds(item, navCmd)...)
			}
		}
		if itemID != "" && libraryID != "" {
			m.detail = detail.New(m.styles, abs.LibraryItem{ID: itemID, LibraryID: libraryID, MediaType: "book"})
			m2, navCmd := m.navigate(ScreenDetail)
			return m2, tea.Batch(m.detailLoadCmds(abs.LibraryItem{ID: itemID, LibraryID: libraryID, MediaType: "book"}, navCmd)...)
		}
		return m, nil
	case components.ActionQueueItem:
		if data != nil {
			if cmd, ok := data.(detail.AddToQueueCmd); ok {
				if !m.isPlaying() {
					if cmd.Item.MediaType == "podcast" && cmd.Episode != nil {
						return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: cmd.Item, Episode: *cmd.Episode})
					}
					return m.handlePlayCmd(detail.PlayCmd{Item: cmd.Item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: cmd.Item, Episode: cmd.Episode}, false)
				return m, nil
			}
			if msg, ok := data.(home.AddToQueueMsg); ok {
				if !m.isPlaying() {
					if msg.Item.MediaType == "podcast" && msg.Episode != nil {
						return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: *msg.Episode})
					}
					return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: msg.Item, Episode: msg.Episode}, false)
				return m, nil
			}
			if item, ok := data.(abs.LibraryItem); ok {
				if !m.isPlaying() {
					return m.handlePlayCmd(detail.PlayCmd{Item: item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: item}, false)
				return m, nil
			}
		}
		return m, nil
	case components.ActionPlayNextItem:
		if data != nil {
			if cmd, ok := data.(detail.PlayNextCmd); ok {
				if !m.isPlaying() {
					if cmd.Item.MediaType == "podcast" && cmd.Episode != nil {
						return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: cmd.Item, Episode: *cmd.Episode})
					}
					return m.handlePlayCmd(detail.PlayCmd{Item: cmd.Item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: cmd.Item, Episode: cmd.Episode}, true)
				return m, nil
			}
			if cmd, ok := data.(detail.AddToQueueCmd); ok {
				if !m.isPlaying() {
					if cmd.Item.MediaType == "podcast" && cmd.Episode != nil {
						return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: cmd.Item, Episode: *cmd.Episode})
					}
					return m.handlePlayCmd(detail.PlayCmd{Item: cmd.Item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: cmd.Item, Episode: cmd.Episode}, true)
				return m, nil
			}
			if msg, ok := data.(home.PlayNextMsg); ok {
				if !m.isPlaying() {
					if msg.Item.MediaType == "podcast" && msg.Episode != nil {
						return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: *msg.Episode})
					}
					return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: msg.Item, Episode: msg.Episode}, true)
				return m, nil
			}
			if item, ok := data.(abs.LibraryItem); ok {
				if !m.isPlaying() {
					return m.handlePlayCmd(detail.PlayCmd{Item: item})
				}
				m.enqueueQueueEntry(QueueEntry{Item: item}, true)
				return m, nil
			}
		}
		return m, nil
	case components.ActionPlayDirect:
		if data != nil {
			if cmd, ok := data.(detail.AddToQueueCmd); ok {
				if cmd.Item.MediaType == "podcast" && cmd.Episode != nil {
					return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{
						Item:    cmd.Item,
						Episode: *cmd.Episode,
					})
				}
				return m.handlePlayCmd(detail.PlayCmd{Item: cmd.Item})
			}
			if msg, ok := data.(home.PlayMsg); ok {
				return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})
			}
			if msg, ok := data.(home.PlayEpisodeMsg); ok {
				return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{
					Item:    msg.Item,
					Episode: msg.Episode,
				})
			}
			if item, ok := data.(abs.LibraryItem); ok {
				if item.MediaType == "podcast" && item.RecentEpisode != nil {
					return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{
						Item:    item,
						Episode: *item.RecentEpisode,
					})
				}
				return m.handlePlayCmd(detail.PlayCmd{Item: item})
			}
		}
		return m, nil
	case components.ActionContentNavigate:
		if data != nil {
			if item, ok := data.(abs.LibraryItem); ok {
				if item.MediaType == "series" {
					m.series = series.New(m.styles, m.client, libraryID, itemID, "")
					return m.navigate(ScreenSeries)
				}
				m.detail = detail.New(m.styles, item)
				m2, navCmd := m.navigate(ScreenDetail)
				return m2, tea.Batch(m.detailLoadCmds(item, navCmd)...)
			}
		}
		if itemID != "" {
			item := abs.LibraryItem{ID: itemID, LibraryID: libraryID, MediaType: "book"}
			m.detail = detail.New(m.styles, item)
			m2, navCmd := m.navigate(ScreenDetail)
			return m2, tea.Batch(m.detailLoadCmds(item, navCmd)...)
		}
		return m, nil
	case components.ActionAddBookmark:
		var targetID string
		if cmd, ok := data.(detail.AddToQueueCmd); ok {
			targetID = cmd.Item.ID
		} else if item, ok := data.(abs.LibraryItem); ok {
			targetID = item.ID
		} else {
			targetID = itemID
		}
		if targetID == "" {
			return m, nil
		}
		if m.isPlaying() && m.itemID == targetID {
			return m.handleAddBookmark(detail.AddBookmarkCmd{
				Item: abs.LibraryItem{ID: m.itemID, LibraryID: m.playbackLibraryID},
			})
		}
		cmd := m.err.SetError(fmt.Errorf("can only add bookmarks to the currently playing item"))
		m.propagateSize()
		return m, cmd
	case components.ActionMarkFinished:
		var targetItem abs.LibraryItem
		var targetEpisode *abs.PodcastEpisode
		hasTarget := false

		if cmd, ok := data.(detail.AddToQueueCmd); ok {
			targetItem = cmd.Item
			targetEpisode = cmd.Episode
			hasTarget = true
		} else if item, ok := data.(abs.LibraryItem); ok {
			targetItem = item
			hasTarget = true
		}

		if hasTarget {
			return m.handleMarkFinished(detail.MarkFinishedCmd{
				Item:    targetItem,
				Episode: targetEpisode,
			})
		}

		if m.isPlaying() {
			episodePtr := (*abs.PodcastEpisode)(nil)
			if m.episodeID != "" {
				episodePtr = &abs.PodcastEpisode{ID: m.episodeID}
			}
			return m.handleMarkFinished(detail.MarkFinishedCmd{
				Item:    abs.LibraryItem{ID: m.itemID, LibraryID: m.playbackLibraryID},
				Episode: episodePtr,
			})
		}
		return m, nil
	case components.ActionEditMetadata:
		if data != nil {
			if cmd, ok := data.(detail.EditMetadataCmd); ok {
				return m.openMetadataEditor(cmd)
			}
			if item, ok := data.(abs.LibraryItem); ok && item.MediaType == "book" {
				m.metadataEdit = metadataedit.New(m.styles, item)
				return m.navigate(ScreenMetadataEdit)
			}
		}
		if m.screen == ScreenDetail {
			item, episode, ok := m.detail.MetadataEditTarget()
			if ok {
				return m.openMetadataEditor(detail.EditMetadataCmd{Item: item, Episode: episode})
			}
		}
		return m, nil
	case components.ActionGoToSeries:
		if libraryID != "" && itemID != "" {
			m.series = series.New(m.styles, m.client, libraryID, itemID, payload)
			return m.navigate(ScreenSeries)
		}
		if m.playbackSeriesID != "" && m.playbackLibraryID != "" {
			m.series = series.New(m.styles, m.client, m.playbackLibraryID, m.playbackSeriesID, m.itemID)
			return m.navigate(ScreenSeries)
		}
		return m, nil
	case components.ActionBrowseSeries:
		m.seriesList = serieslist.New(m.styles, m.client, m.home.SelectedLibraryID(), "")
		return m.navigate(ScreenSeriesList)
	case components.ActionSwitchLibrary:
		return m, nil
	case components.ActionDeleteItem:
		if msg, ok := data.(detail.ShowDeleteConfirmMsg); ok && m.screen == ScreenDetail {
			m.detail, _ = m.detail.Update(msg)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) prewarmCacheCmd() tea.Cmd {
	return func() tea.Msg {
		cache := m.searchCache
		if cache == nil {
			return PrewarmDoneMsg{}
		}
		libID := m.home.SelectedLibraryID()
		libMediaType := m.home.SelectedLibraryMediaType()
		if libID == "" {
			return PrewarmDoneMsg{}
		}
		_ = cache.Prepare(context.Background(), libID, libMediaType)
		return PrewarmDoneMsg{}
	}
}

func (m *Model) handleDeleteItem(cmd detail.DeleteItemCmd) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		err := m.client.DeleteItem(context.Background(), cmd.ItemID, false)
		if err != nil {
			return components.ErrMsg{Err: fmt.Errorf("failed to delete item: %w", err)}
		}
		return detail.ItemDeletedMsg(cmd)
	}
}

func (m *Model) handleDeleteEpisode(cmd detail.DeleteEpisodeCmd) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		err := m.client.DeleteEpisode(context.Background(), cmd.ItemID, cmd.EpisodeID, false)
		if err != nil {
			return components.ErrMsg{Err: fmt.Errorf("failed to delete episode: %w", err)}
		}
		return detail.EpisodeDeletedMsg(cmd)
	}
}
