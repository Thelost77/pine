package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/screens/home"
	"github.com/Thelost77/pine/internal/screens/library"
	"github.com/Thelost77/pine/internal/screens/login"
	"github.com/Thelost77/pine/internal/screens/search"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
)

const headerHeight = 2
const errorBannerHeight = 1
const playerFooterHeight = 1
const syncInterval = 30 * time.Second

// Model is the root application model that manages screen routing.
type Model struct {
	screen    Screen
	backStack []Screen

	login   login.Model
	home    home.Model
	library library.Model
	detail  detail.Model
	search  search.Model
	player  player.Model

	// Playback session state
	sessionID        string
	itemID           string
	episodeID        string
	timeListened     float64
	lastSyncPos      float64
	playGeneration   uint64
	chapters         []abs.Chapter
	chapterOverlayVisible bool
	chapterOverlayIndex   int
	trackStartOffset float64
	trackDuration    float64
	sleepDeadline    time.Time
	sleepDuration    time.Duration
	sleepGeneration  uint64

	keys   KeyMap
	err    components.ErrorBanner
	help   components.HelpOverlay
	width  int
	height int
	styles ui.Styles
	config config.Config
	db     *db.Store
	client *abs.Client
	mpv    player.Player
}

// New creates a new root model. If client is non-nil (authenticated),
// the initial screen is Home; otherwise it starts at Login.
func New(cfg config.Config, store *db.Store, client *abs.Client) Model {
	return NewWithPlayer(cfg, store, client, player.NewMpv())
}

// NewWithPlayer creates a new root model with a specific player implementation.
func NewWithPlayer(cfg config.Config, store *db.Store, client *abs.Client, mpv player.Player) Model {
	styles := ui.NewStyles(cfg.Theme)
	initialScreen := ScreenLogin
	if client != nil {
		initialScreen = ScreenHome
	}
	return Model{
		screen:    initialScreen,
		backStack: nil,
		login:     login.New(styles),
		home:      home.New(styles, client),
		library:   library.New(styles, client, "", nil),
		search:    search.New(styles, client, ""),
		player:    player.NewModel(mpv, cfg, styles),
		keys:      DefaultKeyMap(cfg.Keybinds),
		err:       components.NewErrorBanner(styles.Error.Background(lipgloss.Color(cfg.Theme.Error)).Foreground(lipgloss.Color(cfg.Theme.Background))),
		help:      components.NewHelpOverlay(styles),
		styles:    styles,
		config:    cfg,
		db:        store,
		client:    client,
		mpv:       mpv,
	}
}

// Init returns the initial command for the active screen.
func (m Model) Init() tea.Cmd {
	return m.initScreen(m.screen)
}

// Update dispatches messages to the active screen and handles navigation.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m.navigateWithCleanup(msg.Screen)

	case BackMsg:
		return m.backWithCleanup()

	case login.LoginSuccessMsg:
		logger.Info("login success", "server", msg.ServerURL, "user", msg.Username)
		m.client = abs.NewClient(msg.ServerURL, msg.Token)
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
		m.library = library.New(m.styles, m.client, "", nil)
		m.search = search.New(m.styles, m.client, "")
		m.login, _ = m.login.Update(msg)
		m.backStack = nil
		m.screen = ScreenHome
		m.propagateSize()
		cmd := m.initScreen(ScreenHome)
		return m, cmd

	case home.NavigateDetailMsg:
		logger.Info("navigate to detail", "itemID", msg.Item.ID, "mediaType", msg.Item.MediaType, "title", msg.Item.Media.Metadata.Title)
		m.detail = detail.New(m.styles, msg.Item)
		m, navCmd := m.navigate(ScreenDetail)
		cmds := []tea.Cmd{navCmd, m.fetchBookmarksCmd(msg.Item.ID)}
		if msg.Item.MediaType == "podcast" {
			cmds = append(cmds, m.fetchEpisodesCmd(msg.Item.ID))
		}
		return m, tea.Batch(cmds...)

	case home.PlayEpisodeMsg:
		logger.Info("play episode from home", "itemID", msg.Item.ID, "episodeID", msg.Episode.ID, "title", msg.Episode.Title)
		return m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{Item: msg.Item, Episode: msg.Episode})

	case home.PlayMsg:
		logger.Info("play book from home", "itemID", msg.Item.ID, "title", msg.Item.Media.Metadata.Title)
		return m.handlePlayCmd(detail.PlayCmd{Item: msg.Item})

	case home.NavigateLibraryMsg:
		m.library = library.New(m.styles, m.client, msg.LibraryID, msg.Libraries)
		return m.navigate(ScreenLibrary)

	case home.NavigateSearchMsg:
		m.search = search.New(m.styles, m.client, msg.LibraryID)
		return m.navigate(ScreenSearch)

	case home.GoBackMsg:
		if len(m.backStack) > 0 {
			return m.back()
		}
		return m, nil

	case search.NavigateDetailMsg:
		m.detail = detail.New(m.styles, msg.Item)
		m, navCmd := m.navigate(ScreenDetail)
		cmds := []tea.Cmd{navCmd, m.fetchBookmarksCmd(msg.Item.ID)}
		if msg.Item.MediaType == "podcast" {
			cmds = append(cmds, m.fetchEpisodesCmd(msg.Item.ID))
		}
		return m, tea.Batch(cmds...)

	case search.BackMsg:
		return m.back()

	case library.NavigateDetailMsg:
		m.detail = detail.New(m.styles, msg.Item)
		m, navCmd := m.navigate(ScreenDetail)
		cmds := []tea.Cmd{navCmd, m.fetchBookmarksCmd(msg.Item.ID)}
		if msg.Item.MediaType == "podcast" {
			cmds = append(cmds, m.fetchEpisodesCmd(msg.Item.ID))
		}
		return m, tea.Batch(cmds...)

	case library.GoBackMsg:
		if len(m.backStack) > 0 {
			return m.back()
		}
		return m, nil

	case EpisodesLoadedMsg:
		if msg.Err != nil {
			logger.Error("failed to load episodes", "err", msg.Err)
		} else {
			logger.Info("episodes loaded", "count", len(msg.Episodes))
		}
		if msg.Err == nil && len(msg.Episodes) > 0 {
			m.detail.SetEpisodes(msg.Episodes)
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

	case detail.MarkFinishedCmd:
		return m.handleMarkFinished(msg)

	case detail.MarkFinishedMsg:
		m.detail, _ = m.detail.Update(msg)
		return m, nil

	case detail.SeekToBookmarkCmd:
		return m.handleSeekToBookmark(msg)

	case detail.SeekToChapterCmd:
		return m.handleSeekToBookmark(detail.SeekToBookmarkCmd{Time: msg.Time})

	case detail.DeleteBookmarkCmd:
		return m.handleDeleteBookmark(msg)

	case detail.BookmarksUpdatedMsg:
		m.detail, _ = m.detail.Update(msg)
		return m, nil

	case PlaySessionMsg:
		logger.Info("play session started", "sessionID", msg.Session.SessionID, "itemID", msg.Session.ItemID, "episodeID", msg.Session.EpisodeID, "currentTime", msg.Session.CurrentTime, "duration", msg.Session.Duration)
		return m.handlePlaySessionMsg(msg)

	case player.PlayerReadyMsg:
		return m.handlePlayerReady()

	case player.PositionMsg:
		return m.handlePositionMsg(msg)

	case SyncTickMsg:
		return m.handleSyncTick()

	case player.PlayerLaunchErrMsg:
		logger.Error("player launch failed", "err", msg.Err)
		m.sessionID = ""
		m.itemID = ""
		m.episodeID = ""
		m.timeListened = 0
		m.lastSyncPos = 0
		m.chapters = nil
		m.trackStartOffset = 0
		m.trackDuration = 0
		m.player.Playing = false
		m.player.Title = ""
		m.player.Position = 0
		m.player.Duration = 0
		errCmd := m.err.SetError(msg.Err)
		m.propagateSize()
		return m, tea.Batch(errCmd, player.QuitCmd(m.mpv))

	case player.PlayerQuitMsg:
		return m, nil

	case PlaybackStoppedMsg:
		return m, nil

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

	case PlaybackErrorMsg:
		if msg.Err != nil {
			logger.Error("playback error", "err", msg.Err)
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
		if m.chapterOverlayVisible {
			if key.Matches(msg, m.keys.Back) {
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
		}
		if m.err.HasError() {
			m.err.Dismiss()
			m.propagateSize()
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
		if m.screen != ScreenLogin && m.screen != ScreenSearch {
			if key.Matches(msg, m.keys.Back) {
				if len(m.backStack) > 0 {
					return m.back()
				}
				return m, nil
			}
		}
		// When playing, playback keys take priority over screen keys.
		if m.isPlaying() {
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
	if m.chapterOverlayIndex >= len(m.chapters) {
		m.chapterOverlayIndex = len(m.chapters) - 1
	}
	if m.chapterOverlayIndex < 0 {
		m.chapterOverlayIndex = 0
	}
	m.chapterOverlayVisible = true
}

func (m *Model) closeChapterOverlay() {
	m.chapterOverlayVisible = false
}

func (m *Model) moveChapterOverlaySelection(delta int) {
	if !m.chapterOverlayVisible || len(m.chapters) == 0 {
		return
	}
	m.chapterOverlayIndex += delta
	if m.chapterOverlayIndex < 0 {
		m.chapterOverlayIndex = 0
	}
	if m.chapterOverlayIndex >= len(m.chapters) {
		m.chapterOverlayIndex = len(m.chapters) - 1
	}
}
