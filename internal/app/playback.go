package app

import (
	"context"
	"errors"
	"math"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/captions"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	trackEndRolloverSlack            = 2.0
	propertyUnavailableMaxBackoffExp = 2 // 500ms << 2 = 2s

	// maxListeningTick is the longest time between two position ticks that
	// pine counts as listening time. The ticks are 500 ms apart. After a longer
	// time, for example a suspend of the machine, pine does not know how long
	// the player played.
	maxListeningTick = 5 * time.Second

	// syncMinChange is the smallest change of the position or of the
	// listening time, in seconds, that pine sends to the server. A paused
	// player can report a position that is a fraction of a second away from
	// the position that the session started at.
	syncMinChange = 1.0
)

// errNoAudioTracks is the error of a play session that has no audio track.
var errNoAudioTracks = errors.New("no audio tracks")

func pineVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return buildVersion(info.Main.Version)
	}
	return "dev"
}

func buildVersion(version string) string {
	if version == "" || version == "(devel)" {
		return "dev"
	}
	return version
}

func (m Model) playbackDevice() abs.DeviceInfo {
	return abs.DeviceInfo{
		DeviceID:      m.config.DeviceID,
		Manufacturer:  "Pine",
		Model:         strings.TrimSuffix(m.config.DeviceName, " (Pine)"),
		ClientName:    "pine",
		ClientVersion: pineVersion(),
	}
}

// isPlaying returns true if there's an active playback session.
func (m Model) isPlaying() bool {
	return m.sessionID != ""
}

// clock returns the current time.
func (m Model) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// trackListeningTime adds the time since the last position tick to the
// listening time, if the player played at that tick and plays now. A player
// that is paused or idle does not play: mpv reports no pause while it waits
// for data. The listening time is the time of the clock. It is not the change
// of the position: at the speed 2.0, the position moves two seconds in each
// second.
func (m *Model) trackListeningTime(playing bool) {
	if !playing {
		m.lastPlayingTick = time.Time{}
		return
	}
	now := m.clock()
	if !m.lastPlayingTick.IsZero() {
		if played := now.Sub(m.lastPlayingTick); played > 0 && played <= maxListeningTick {
			m.timeListened += played.Seconds()
		}
	}
	m.lastPlayingTick = now
}

// hasUnsyncedProgress returns true if the position or the listening time has
// a change that the server does not have.
func (m Model) hasUnsyncedProgress() bool {
	return m.timeListened >= syncMinChange || math.Abs(m.player.Position-m.lastSyncPos) >= syncMinChange
}

// capturesKeys returns true if the screen gets each key: a text input or a
// confirmation dialog is active. Update then gives the key to the screen, and
// the global keys and the playback keys have no function.
func (m Model) capturesKeys() bool {
	switch m.screen {
	case ScreenLogin, ScreenMetadataEdit:
		return true
	case ScreenHome:
		return m.home.IsFiltering()
	case ScreenLibrary:
		return m.library.IsFiltering()
	case ScreenDetail:
		return m.detail.EditingBookmark() || m.detail.ConfirmOverlayVisible()
	case ScreenSeries:
		return m.series.IsFiltering()
	case ScreenSeriesList:
		return m.seriesList.IsFiltering()
	case ScreenSessions:
		return m.sessions.IsFiltering()
	}
	return false
}

func (m Model) hasActiveFilter() bool {
	switch m.screen {
	case ScreenHome:
		return m.home.HasActiveFilter()
	case ScreenLibrary:
		return m.library.HasActiveFilter()
	case ScreenSeries:
		return m.series.HasActiveFilter()
	case ScreenSeriesList:
		return m.seriesList.HasActiveFilter()
	case ScreenSessions:
		return m.sessions.HasActiveFilter()
	}
	return false
}

// handlePlayCmd initiates a play session by calling the ABS API.
// If the same item is already playing, toggles pause.
// If a different item is playing, stops it first and starts the new one.
func (m Model) handlePlayCmd(msg detail.PlayCmd) (Model, tea.Cmd) {
	// Same book already playing → toggle pause
	if m.isPlaying() && m.itemID == msg.Item.ID && m.episodeID == "" {
		m.player.Playing = !m.player.Playing
		if m.mpv != nil {
			return m, player.SetPauseCmd(m.mpv, !m.player.Playing)
		}
		return m, nil
	}

	if m.client == nil {
		return m, nil
	}
	m.playRequested = true

	// Different item playing → stop it first
	var closeCmd, stopCmd tea.Cmd
	if m.isPlaying() {
		logger.Info("switching playback", "from", m.itemID, "to", msg.Item.ID)
		m, closeCmd, stopCmd = m.stopPlaybackCmds()
	}

	m.setSeriesContext(msg.Item)

	item := msg.Item
	client := m.client
	startCmd := func() tea.Msg {
		device := m.playbackDevice()
		session, err := client.StartPlaySession(context.Background(), item.ID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		playMsg, err := buildBookPlaySessionMsg(
			client,
			session,
			item.ID,
			item.Media.Metadata.Title,
			mprisAuthors(item),
			item.Media.TotalDuration(),
			session.CurrentTime,
		)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return playMsg
	}

	if stopCmd != nil {
		return m, tea.Batch(stopCmd, closeThen(closeCmd, startCmd))
	}
	return m, startCmd
}

// handlePlayEpisodeCmd initiates a play session for a podcast episode.
// If the same episode is already playing, toggles pause.
// If a different item/episode is playing, stops it first and starts the new one.
func (m Model) handlePlayEpisodeCmd(msg detail.PlayEpisodeCmd) (Model, tea.Cmd) {
	// Same episode already playing → toggle pause
	if m.isPlaying() && m.itemID == msg.Item.ID && m.episodeID == msg.Episode.ID {
		m.player.Playing = !m.player.Playing
		if m.mpv != nil {
			return m, player.SetPauseCmd(m.mpv, !m.player.Playing)
		}
		return m, nil
	}

	if m.client == nil {
		return m, nil
	}
	m.playRequested = true

	// Different item/episode playing → stop it first
	var closeCmd, stopCmd tea.Cmd
	if m.isPlaying() {
		logger.Info("switching playback", "from", m.itemID+"/"+m.episodeID, "to", msg.Item.ID+"/"+msg.Episode.ID)
		m, closeCmd, stopCmd = m.stopPlaybackCmds()
	}

	// An episode has no series to continue.
	m.playbackSeriesID = ""
	m.playbackLibraryID = msg.Item.LibraryID

	item := msg.Item
	episode := msg.Episode
	client := m.client
	startCmd := func() tea.Msg {
		device := m.playbackDevice()
		session, err := client.StartEpisodePlaySession(context.Background(), item.ID, episode.ID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		playMsg, err := buildEpisodePlaySessionMsg(client, session, item, episode, session.CurrentTime)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return playMsg
	}

	if stopCmd != nil {
		return m, tea.Batch(stopCmd, closeThen(closeCmd, startCmd))
	}
	return m, startCmd
}

// handlePlaySessionMsg launches mpv with the stream URL.
func (m Model) handlePlaySessionMsg(msg PlaySessionMsg) (Model, tea.Cmd) {
	// restartPlaybackAt keeps the item, so a session of the same item is a
	// restart, for example at a seek to another file of the book.
	restart := m.itemID == msg.Session.ItemID && m.episodeID == msg.Session.EpisodeID
	m.playGeneration++
	m.sessionID = msg.Session.SessionID
	m.itemID = msg.Session.ItemID
	m.episodeID = msg.Session.EpisodeID
	m.chapters = msg.Session.Chapters
	m.resetChapterOverlay()
	m.captionCues = nil
	m.trackStartOffset = msg.Session.TrackStartOffset
	m.trackDuration = msg.Session.TrackDuration
	m.timeListened = 0
	m.lastPlayingTick = time.Time{}
	m.currentAuthors = msg.Session.Authors

	bookPos := msg.Session.CurrentTime + m.trackStartOffset
	m.lastSyncPos = msg.Session.SyncedTime

	// Update player model title/state
	m.player, _ = m.player.Update(player.StartPlayMsg{Title: msg.Session.Title})
	m.player.Position = bookPos
	m.player.Duration = msg.Session.Duration
	m.propagateSize()
	m.syncMprisState()
	logger.Info("playback session loaded", "session", abs.SessionTag(msg.Session.SessionID), "itemID", msg.Session.ItemID, "episodeID", msg.Session.EpisodeID, "bookPosition", bookPos, "trackStart", msg.Session.TrackStartOffset, "trackDuration", msg.Session.TrackDuration, "chapters", len(msg.Session.Chapters), "pausedRestore", m.restorePaused)

	paused := m.restorePaused
	m.restorePaused = false
	var headers []string
	if msg.AuthToken != "" {
		headers = []string{"Authorization: Bearer " + msg.AuthToken}
	}
	cmds := []tea.Cmd{
		m.mprisPlaybackCmd(),
		player.LaunchCmd(m.mpv, msg.StreamURL, msg.Session.CurrentTime, paused, headers, m.player.Volume, m.playGeneration),
	}
	// An MPRIS client expects a new track at the position 0. A restart keeps
	// the track ID, so it sends Seeked also at the position 0.
	if bookPos > 0 || restart {
		cmds = append(cmds, m.mprisSeekedCmd())
	}
	if capCmd := loadCaptionsCmd(m.client, msg.Session.ItemID, msg.Session.TrackFilename, msg.Session.TranscriptIno, msg.Session.HasLibraryFiles, m.playGeneration); capCmd != nil {
		cmds = append(cmds, capCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleCaptionLoaded(msg CaptionLoadedMsg) (Model, tea.Cmd) {
	if msg.Generation != m.playGeneration {
		logger.Debug("ignoring stale captions", "msgGen", msg.Generation, "currentGen", m.playGeneration)
		return m, nil
	}
	if !m.isPlaying() {
		return m, nil
	}
	m.captionCues = msg.Cues
	m.propagateSize()
	return m, nil
}

func (m *Model) toggleCaptions() {
	if len(m.captionCues) == 0 {
		return
	}
	m.captionsHidden = !m.captionsHidden
	m.propagateSize()
}

// loadCaptionsCmd loads the VTT file ino. If ino is empty, it finds the VTT
// file of audioFilename in the files of the item. hasFiles is true if the play
// session gave the files of the item: ino is then complete, and an empty ino
// means that the item has no VTT file.
func loadCaptionsCmd(client *cache.Client, itemID, audioFilename, ino string, hasFiles bool, generation uint64) tea.Cmd {
	if client == nil || itemID == "" || (ino == "" && hasFiles) {
		return nil
	}
	inner := client.Client
	return func() tea.Msg {
		if ino == "" {
			files, err := inner.GetLibraryFiles(context.Background(), itemID)
			if err != nil {
				logger.Warn("captions: list files failed", "itemID", itemID, "err", err)
				return CaptionLoadedMsg{Generation: generation}
			}
			ino = abs.TranscriptIno(files, audioFilename)
		}
		if ino == "" {
			return CaptionLoadedMsg{Generation: generation}
		}
		data, err := inner.GetLibraryFile(context.Background(), itemID, ino)
		if err != nil {
			logger.Warn("captions: download failed", "itemID", itemID, "ino", ino, "err", err)
			return CaptionLoadedMsg{Generation: generation}
		}
		return CaptionLoadedMsg{Generation: generation, Cues: captions.ParseVTT(data)}
	}
}

// handlePlayerReady starts the position tick and sync tick. It sets the speed
// of the model in the player: each launch makes a new player, which plays at
// the speed 1.0.
// Stale PlayerReadyMsg events (from a superseded play session) are ignored so
// duplicate polling ticks don't race with the active session.
func (m Model) handlePlayerReady(msg player.PlayerReadyMsg) (Model, tea.Cmd) {
	if msg.Generation != m.playGeneration {
		logger.Debug("ignoring stale player ready", "msgGen", msg.Generation, "currentGen", m.playGeneration)
		return m, nil
	}
	logger.Info("player ready, starting position ticks", "generation", m.playGeneration)
	m.propertyUnavailableCount = 0
	return m, tea.Batch(
		player.SetSpeedCmd(m.mpv, m.player.Speed),
		player.TickCmd(m.mpv, m.playGeneration),
		syncTickCmd(m.playGeneration),
	)
}

// handlePositionMsg updates the player and tracks time listened.
func (m Model) handlePositionMsg(msg player.PositionMsg) (Model, tea.Cmd) {
	// Ignore ticks from a previous play session (stale after episode switch)
	if msg.Generation != m.playGeneration {
		logger.Debug("ignoring stale position tick", "msgGen", msg.Generation, "currentGen", m.playGeneration)
		return m, nil
	}

	if msg.Err != nil {
		// pine does not know if the player plays, so the time until the next
		// tick is not listening time.
		m.lastPlayingTick = time.Time{}
		if targetPos, ok := m.trackEndRolloverTarget(msg.Err); ok {
			if m.player.Duration > 0 && targetPos >= m.player.Duration-trackEndRolloverSlack {
				m.player.Position = m.player.Duration
				logger.Info("track ended on final segment, stopping playback", "position", m.player.Position, "trackEnd", targetPos, "duration", m.player.Duration)
				return m.handlePlaybackCompleted()
			}
			logger.Info("track ended, advancing playback", "from", m.player.Position, "to", targetPos, "trackStart", m.trackStartOffset, "trackDuration", m.trackDuration)
			return m.restartPlaybackAt(targetPos)
		}
		if m.didPlaybackComplete(msg.Err) {
			logger.Info("playback completed, advancing queue if available", "position", m.player.Position, "duration", m.player.Duration)
			return m.handlePlaybackCompleted()
		}

		if errors.Is(msg.Err, player.ErrPropertyUnavailable) {
			// The player is up; mpv has not finished opening or seeking yet.
			// Keep polling. Do not treat this as a dead player.
			m.propertyUnavailableCount++
			shift := m.propertyUnavailableCount
			if shift > propertyUnavailableMaxBackoffExp {
				shift = propertyUnavailableMaxBackoffExp
			}
			interval := 500 * time.Millisecond << shift
			logger.Debug("mpv properties not ready yet, backing off", "attempt", m.propertyUnavailableCount, "nextTick", interval)
			return m, player.TickAfterCmd(m.mpv, m.playGeneration, interval)
		}
		m.propertyUnavailableCount = 0

		if m.isPlaying() {
			return m.stopPlayback()
		}
		return m, nil
	}

	m.propertyUnavailableCount = 0
	m.trackListeningTime(!msg.Paused && !msg.Idle)

	// Convert track-relative position to book-global
	bookPos := msg.Position + m.trackStartOffset

	// If a seek is pending, wait for mpv to confirm the new position.
	// Clear it once mpv reports a position within ±2s of the target, or after
	// a 3s timeout to avoid freezing tracking/seeking if the confirmation
	// never arrives.
	if m.seekPending {
		if (bookPos >= m.player.Position-2 && bookPos <= m.player.Position+2) || time.Since(m.seekPendingSince) > 3*time.Second {
			m.seekPending = false
		}
		// Still update playing state and emit MPRIS signals
		wasPlaying := m.player.Playing
		m.player.Playing = !msg.Paused
		var mprisCmds []tea.Cmd
		if wasPlaying != m.player.Playing {
			m.syncMprisState()
			mprisCmds = append(mprisCmds, m.mprisPlaybackCmd())
		}
		if m.mpv != nil {
			return m, tea.Batch(append(mprisCmds, player.TickCmd(m.mpv, m.playGeneration))...)
		}
		return m, tea.Batch(mprisCmds...)
	}

	m.player.Position = bookPos
	wasPlaying := m.player.Playing
	m.player.Playing = !msg.Paused
	// MPRIS clients read the position from the state.
	m.syncMprisState()
	var mprisCmds []tea.Cmd
	if wasPlaying != m.player.Playing {
		mprisCmds = append(mprisCmds, m.mprisPlaybackCmd())
	}

	// Update sleep timer display. Update drops SleepTimerExpiredMsg while pine
	// has no session, for example at the change of a track, so a tick after
	// the deadline stops the playback.
	if !m.sleepDeadline.IsZero() {
		remaining := time.Until(m.sleepDeadline)
		if remaining <= 0 {
			logger.Info("sleep timer expired, stopping playback")
			return m.stopPlayback()
		}
		m.player.SleepRemaining = formatSleepRemaining(remaining)
	}

	if m.mpv != nil {
		return m, tea.Batch(append(mprisCmds, player.TickCmd(m.mpv, m.playGeneration))...)
	}
	return m, tea.Batch(mprisCmds...)
}

func (m Model) trackEndRolloverTarget(err error) (float64, bool) {
	if err == nil || m.episodeID != "" || m.trackDuration <= 0 {
		return 0, false
	}
	if !errors.Is(err, player.ErrClosed) {
		return 0, false
	}

	targetPos := m.trackStartOffset + m.trackDuration
	if m.player.Position < targetPos-trackEndRolloverSlack {
		return 0, false
	}
	return targetPos, true
}

func (m Model) didPlaybackComplete(err error) bool {
	if err == nil || m.player.Duration <= 0 {
		return false
	}
	if !errors.Is(err, player.ErrClosed) {
		return false
	}
	return m.player.Position >= m.player.Duration-trackEndRolloverSlack
}

func (m Model) restartPlaybackAt(bookPos float64) (Model, tea.Cmd) {
	client := m.client
	itemID := m.itemID
	sessionID := m.sessionID
	currentTime := m.player.Position
	timeListened := m.timeListened
	changed := m.hasUnsyncedProgress()
	duration := m.player.Duration
	title := m.player.Title
	authors := m.currentAuthors
	mpvPlayer := m.mpv
	targetPos := bookPos
	cacheStore := m.cacheStore
	libraryID := m.playbackLibraryID
	seriesID := m.playbackSeriesID

	// Bump generation so old position ticks get discarded.
	m.playGeneration++
	// Clear current session but keep itemID/chapters for the restart.
	m.sessionID = ""
	m.timeListened = 0
	m.captionCues = nil

	return m, func() tea.Msg {
		if client != nil && sessionID != "" {
			_ = closeSession(context.Background(), client, sessionID, currentTime, timeListened, changed)
			evictProgressCaches(cacheStore, itemID, libraryID, seriesID)
		}
		if mpvPlayer != nil {
			_ = mpvPlayer.Quit()
		}

		device := m.playbackDevice()
		session, err := client.StartPlaySession(context.Background(), itemID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		playMsg, err := buildBookPlaySessionMsg(client, session, itemID, title, authors, duration, targetPos)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return playMsg
	}
}

func (m Model) startPlaybackAtBookPositionCmd(item abs.LibraryItem, bookPos float64) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		device := m.playbackDevice()
		session, err := client.StartPlaySession(context.Background(), item.ID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		playMsg, err := buildBookPlaySessionMsg(
			client,
			session,
			item.ID,
			item.Media.Metadata.Title,
			mprisAuthors(item),
			item.Media.TotalDuration(),
			bookPos,
		)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return playMsg
	}
}

// playItemAt plays a book, or an episode of a podcast, from a position in the
// book or the episode. A different item or episode that plays stops first.
func (m Model) playItemAt(item abs.LibraryItem, episode *abs.PodcastEpisode, pos float64) (Model, tea.Cmd) {
	m.playRequested = true
	var closeCmd, stopCmd tea.Cmd
	if m.isPlaying() {
		episodeID := ""
		if episode != nil {
			episodeID = episode.ID
		}
		logger.Info("switching playback", "from", m.itemID+"/"+m.episodeID, "to", item.ID+"/"+episodeID)
		m, closeCmd, stopCmd = m.stopPlaybackCmds()
	}

	m.setSeriesContext(item)
	var startCmd tea.Cmd
	if episode != nil {
		startCmd = m.startEpisodePlaybackAtPositionCmd(item, *episode, pos)
	} else {
		startCmd = m.startPlaybackAtBookPositionCmd(item, pos)
	}
	if stopCmd != nil {
		return m, tea.Batch(stopCmd, closeThen(closeCmd, startCmd))
	}
	return m, startCmd
}

func (m Model) startEpisodePlaybackAtPositionCmd(item abs.LibraryItem, episode abs.PodcastEpisode, pos float64) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		device := m.playbackDevice()
		session, err := client.StartEpisodePlaySession(context.Background(), item.ID, episode.ID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		playMsg, err := buildEpisodePlaySessionMsg(client, session, item, episode, pos)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return playMsg
	}
}

func (m Model) startRestorePlaybackCmd(msg RestoreSessionMsg) tea.Cmd {
	if msg.Item == nil || m.client == nil {
		return nil
	}
	if msg.SavedEpisodeID != "" {
		if msg.Episode == nil {
			return clearSavedRestoreSessionCmd(
				m.db,
				"podcast episode missing",
				"itemID", msg.Item.ID,
				"episodeID", msg.SavedEpisodeID,
			)
		}
		return m.startRestoredEpisodePlaybackCmd(*msg.Item, *msg.Episode)
	}
	return m.startRestoredBookPlaybackCmd(*msg.Item)
}

// closeRestoredSessionCmd closes a session that the restore opened and that
// pine does not play. The session has no position to send.
func closeRestoredSessionCmd(client *cache.Client, sessionID string) tea.Cmd {
	if client == nil || sessionID == "" {
		return nil
	}
	return func() tea.Msg {
		if err := client.CloseSessionNoSync(context.Background(), sessionID); err != nil {
			logger.Warn("failed to close restored session", "session", abs.SessionTag(sessionID), "err", err)
		}
		return nil
	}
}

func (m Model) startRestoredBookPlaybackCmd(item abs.LibraryItem) tea.Cmd {
	client := m.client
	store := m.db
	return func() tea.Msg {
		device := m.playbackDevice()
		session, err := client.StartPlaySession(context.Background(), item.ID, device)
		if err != nil {
			if abs.IsHTTPStatus(err, http.StatusNotFound) {
				clearSavedRestoreSession(
					store,
					"book playback source missing",
					"itemID", item.ID,
					"err", err,
				)
				return nil
			}
			logSkippedRestore("book playback start failed", "itemID", item.ID, "err", err)
			return nil
		}
		playMsg, err := buildBookPlaySessionMsg(
			client,
			session,
			item.ID,
			item.Media.Metadata.Title,
			mprisAuthors(item),
			item.Media.TotalDuration(),
			session.CurrentTime,
		)
		if err != nil {
			if errors.Is(err, errNoAudioTracks) {
				clearSavedRestoreSession(
					store,
					"book playback returned no audio tracks",
					"itemID", item.ID,
					"session", abs.SessionTag(session.ID),
				)
				return nil
			}
			logSkippedRestore("book playback session invalid", "itemID", item.ID, "session", abs.SessionTag(session.ID), "err", err)
			return nil
		}
		return RestorePlaySessionMsg{PlaySessionMsg: playMsg}
	}
}

func (m Model) startRestoredEpisodePlaybackCmd(item abs.LibraryItem, episode abs.PodcastEpisode) tea.Cmd {
	client := m.client
	store := m.db
	return func() tea.Msg {
		device := m.playbackDevice()
		session, err := client.StartEpisodePlaySession(context.Background(), item.ID, episode.ID, device)
		if err != nil {
			if abs.IsHTTPStatus(err, http.StatusNotFound) {
				clearSavedRestoreSession(
					store,
					"podcast episode playback source missing",
					"itemID", item.ID,
					"episodeID", episode.ID,
					"err", err,
				)
				return nil
			}
			logSkippedRestore("podcast episode playback start failed", "itemID", item.ID, "episodeID", episode.ID, "err", err)
			return nil
		}
		playMsg, err := buildEpisodePlaySessionMsg(client, session, item, episode, session.CurrentTime)
		if err != nil {
			if errors.Is(err, errNoAudioTracks) {
				clearSavedRestoreSession(
					store,
					"podcast episode playback returned no audio tracks",
					"itemID", item.ID,
					"episodeID", episode.ID,
					"session", abs.SessionTag(session.ID),
				)
				return nil
			}
			logSkippedRestore("podcast episode playback session invalid", "itemID", item.ID, "episodeID", episode.ID, "session", abs.SessionTag(session.ID), "err", err)
			return nil
		}
		return RestorePlaySessionMsg{PlaySessionMsg: playMsg}
	}
}

func (m Model) handlePlaybackCompleted() (Model, tea.Cmd) {
	next, hasNext := m.dequeueQueueEntry()
	seriesID := m.playbackSeriesID
	libraryID := m.playbackLibraryID
	currentItemID := m.itemID
	client := m.client

	m, closeCmd, stopCmd := m.stopPlaybackCmds()
	if hasNext {
		return m.startQueuedEntry(next, closeCmd, stopCmd)
	}
	if seriesID == "" || libraryID == "" || client == nil {
		return m, tea.Batch(closeCmd, stopCmd)
	}
	cmd := seriesContinueCmd(client, libraryID, seriesID, currentItemID)
	return m, tea.Batch(stopCmd, closeThen(closeCmd, cmd))
}

func (m Model) skipToNextQueued() (Model, tea.Cmd) {
	if !m.isPlaying() {
		return m, nil
	}
	next, hasNext := m.dequeueQueueEntry()
	if !hasNext {
		return m, nil
	}
	m, closeCmd, stopCmd := m.stopPlaybackCmds()
	return m.startQueuedEntry(next, closeCmd, stopCmd)
}

// startQueuedEntry starts the next entry of the queue after closeCmd, the
// close of the old session.
func (m Model) startQueuedEntry(next QueueEntry, closeCmd, stopCmd tea.Cmd) (Model, tea.Cmd) {
	var nextCmd tea.Cmd
	if next.Episode != nil {
		m, nextCmd = m.handlePlayEpisodeCmd(detail.PlayEpisodeCmd{
			Item:    next.Item,
			Episode: *next.Episode,
		})
	} else {
		m, nextCmd = m.handlePlayCmd(detail.PlayCmd{Item: next.Item})
	}

	return m, tea.Batch(stopCmd, closeThen(closeCmd, nextCmd))
}

func buildBookPlaySessionMsg(client *cache.Client, session *abs.PlaySession, itemID, title string, authors []string, duration, bookPos float64) (PlaySessionMsg, error) {
	if len(session.AudioTracks) == 0 {
		return PlaySessionMsg{}, errNoAudioTracks
	}

	track := session.AudioTracks[0]
	seekTime := bookPos
	// A position at the end of the book is the end of the last track.
	if last := session.AudioTracks[len(session.AudioTracks)-1]; bookPos >= last.StartOffset+last.Duration {
		track = last
		seekTime = last.Duration
	}
	for _, t := range session.AudioTracks {
		if bookPos >= t.StartOffset && bookPos < t.StartOffset+t.Duration {
			track = t
			seekTime = bookPos - t.StartOffset
			break
		}
	}
	logger.Info("track selected for playback", "itemID", itemID, "session", abs.SessionTag(session.ID), "trackIndex", track.Index, "trackStart", track.StartOffset, "trackDuration", track.Duration, "bookPosition", bookPos, "seekTime", seekTime)

	streamURL := client.BaseURL() + track.ContentURL

	return PlaySessionMsg{
		Session: PlaySessionData{
			SessionID:        session.ID,
			ItemID:           itemID,
			CurrentTime:      seekTime,
			SyncedTime:       session.CurrentTime,
			Duration:         duration,
			Title:            title,
			Authors:          authors,
			Chapters:         playSessionChapters(session),
			TrackStartOffset: track.StartOffset,
			TrackDuration:    track.Duration,
			TrackFilename:    track.Metadata.Filename,
			TranscriptIno:    session.TranscriptIno(track.Metadata.Filename),
			HasLibraryFiles:  len(session.LibraryItem.LibraryFiles) > 0,
		},
		StreamURL: streamURL,
		AuthToken: client.Token(),
	}, nil
}

// buildEpisodePlaySessionMsg makes the PlaySessionMsg that starts a podcast
// episode at the position pos.
func buildEpisodePlaySessionMsg(client *cache.Client, session *abs.PlaySession, item abs.LibraryItem, episode abs.PodcastEpisode, pos float64) (PlaySessionMsg, error) {
	if len(session.AudioTracks) == 0 {
		return PlaySessionMsg{}, errNoAudioTracks
	}

	track := session.AudioTracks[0]
	streamURL := client.BaseURL() + track.ContentURL
	logger.Info("track selected for episode playback", "itemID", item.ID, "episodeID", episode.ID, "session", abs.SessionTag(session.ID), "trackIndex", track.Index, "trackStart", track.StartOffset, "trackDuration", track.Duration, "bookPosition", pos)

	return PlaySessionMsg{
		Session: PlaySessionData{
			SessionID:        session.ID,
			ItemID:           item.ID,
			EpisodeID:        episode.ID,
			CurrentTime:      pos,
			SyncedTime:       session.CurrentTime,
			Duration:         resolveEpisodeDuration(episode, session),
			Title:            episode.Title,
			Authors:          mprisAuthors(item),
			Chapters:         playSessionChapters(session),
			TrackStartOffset: track.StartOffset,
			TrackDuration:    track.Duration,
			TrackFilename:    track.Metadata.Filename,
			TranscriptIno:    session.TranscriptIno(track.Metadata.Filename),
			HasLibraryFiles:  len(session.LibraryItem.LibraryFiles) > 0,
		},
		StreamURL: streamURL,
		AuthToken: client.Token(),
	}, nil
}

func resolveEpisodeDuration(episode abs.PodcastEpisode, session *abs.PlaySession) float64 {
	if episode.Duration > 0 {
		return episode.Duration
	}
	if session != nil && session.MediaMetadata.Duration != nil && *session.MediaMetadata.Duration > 0 {
		return *session.MediaMetadata.Duration
	}
	if session == nil {
		return 0
	}

	var duration float64
	for _, track := range session.AudioTracks {
		end := track.StartOffset + track.Duration
		if end > duration {
			duration = end
		}
	}
	return duration
}

// handleSyncTick syncs progress with ABS and persists to DB. A tick of an
// earlier playback ends its chain of ticks: each playback starts its own chain
// in handlePlayerReady.
func (m Model) handleSyncTick(msg SyncTickMsg) (Model, tea.Cmd) {
	if msg.Generation != m.playGeneration || !m.isPlaying() {
		return m, nil
	}

	client := m.client
	sessionID := m.sessionID
	itemID := m.itemID
	episodeID := m.episodeID
	currentTime := m.player.Position
	timeListened := m.timeListened
	store := m.db

	cmds := []tea.Cmd{
		syncTickCmd(m.playGeneration),
		func() tea.Msg {
			if store != nil {
				if err := store.SaveListeningSession(db.ListeningSession{
					ItemID:    itemID,
					EpisodeID: episodeID,
				}); err != nil {
					logger.Warn("failed to save listening session", "itemID", itemID, "err", err)
				} else {
					logger.Debug("listening session saved", "itemID", itemID)
				}
			}
			return nil
		},
	}

	// A sync with an old position overwrites a newer position that a second
	// device wrote. So no sync goes out while nothing changed.
	if client != nil && m.hasUnsyncedProgress() {
		// The server adds the timeListened of each sync to the session. So the
		// sync takes the listening time, and handleSessionSynced gives it back
		// if the sync fails.
		m.timeListened = 0
		cmds = append(cmds, syncSessionCmd(client, m.playbackDevice(), sessionID, itemID, episodeID, currentTime, timeListened))
	}

	return m, tea.Batch(cmds...)
}

// syncSessionCmd sends the position and the listening time of a session to
// the server. The server answers 404 if it does not have the session: it
// loses its open sessions in a restart, and it closes a session after 36
// hours with no change. The command then opens a new session for the item,
// and sends the position and the listening time to that session.
func syncSessionCmd(client *cache.Client, device abs.DeviceInfo, sessionID, itemID, episodeID string, currentTime, timeListened float64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := SessionSyncedMsg{SessionID: sessionID, CurrentTime: currentTime, TimeListened: timeListened}
		msg.Err = client.SyncSession(ctx, sessionID, currentTime, timeListened)
		if abs.IsHTTPStatus(msg.Err, http.StatusNotFound) {
			logger.Info("session is gone on the server, opening a new session", "session", abs.SessionTag(sessionID), "itemID", itemID, "episodeID", episodeID)
			var session *abs.PlaySession
			if episodeID != "" {
				session, msg.Err = client.StartEpisodePlaySession(ctx, itemID, episodeID, device)
			} else {
				session, msg.Err = client.StartPlaySession(ctx, itemID, device)
			}
			if msg.Err == nil {
				msg.NewSessionID = session.ID
				msg.Err = client.SyncSession(ctx, session.ID, currentTime, timeListened)
			}
		}
		fields := []any{"session", abs.SessionTag(sessionID), "currentTime", currentTime, "timeListened", timeListened}
		if msg.NewSessionID != "" {
			fields = append(fields, "newSession", abs.SessionTag(msg.NewSessionID))
		}
		if msg.Err != nil {
			logger.Warn("failed to sync session", append(fields, "err", msg.Err)...)
		} else {
			logger.Debug("session synced", fields...)
		}
		return msg
	}
}

// handleSessionSynced records the result of a sync. An accepted sync gives the
// position that the server has. A failed sync gives its listening time back to
// the session, so that the next sync sends that time also.
func (m Model) handleSessionSynced(msg SessionSyncedMsg) Model {
	if msg.SessionID != m.sessionID {
		return m
	}
	if msg.NewSessionID != "" {
		m.sessionID = msg.NewSessionID
	}
	if msg.Err != nil {
		m.timeListened += msg.TimeListened
		return m
	}
	m.lastSyncPos = msg.CurrentTime
	return m
}

// evictProgressCaches drops cached data that embeds user progress for an item:
// the library item, the home shelves, and the series view.
func evictProgressCaches(store *cache.Store, itemID, libraryID, seriesID string) {
	if store == nil || itemID == "" {
		return
	}
	_ = store.Delete(cache.ItemKey(itemID))
	if libraryID != "" {
		_ = store.Delete(cache.PersonalizedKey(libraryID))
	}
	if seriesID != "" {
		_ = store.Delete(cache.SeriesContentsKey(seriesID))
	}
}

func (m Model) handleMarkFinished(msg detail.MarkFinishedCmd) (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	client := m.client
	item := msg.Item
	episode := msg.Episode
	episodeID := ""
	if episode != nil {
		episodeID = episode.ID
	}
	cacheStore := m.cacheStore
	itemID := item.ID
	libraryID := item.LibraryID
	seriesID := ""
	if item.Media.Metadata.Series != nil {
		seriesID = item.Media.Metadata.Series.ID
	}
	return m, func() tea.Msg {
		var err error
		if msg.Undo {
			err = client.UpdateProgress(context.Background(), item.ID, episodeID, 0, 0, false)
			if err != nil {
				logger.Warn("failed to unmark finished", "err", err)
				return PlaybackErrorMsg{Err: err}
			}
			evictProgressCaches(cacheStore, itemID, libraryID, seriesID)
			return detail.MarkFinishedMsg{EpisodeID: episodeID, Progress: &abs.UserMediaProgress{
				IsFinished: false,
			}}
		}
		duration := item.Media.TotalDuration()
		if episode != nil && episode.Duration > 0 {
			duration = episode.Duration
		}
		err = client.UpdateProgress(context.Background(), item.ID, episodeID, duration, 1.0, true)
		if err != nil {
			logger.Warn("failed to mark as finished", "err", err)
			return PlaybackErrorMsg{Err: err}
		}
		evictProgressCaches(cacheStore, itemID, libraryID, seriesID)
		progress := &abs.UserMediaProgress{
			CurrentTime: duration,
			Progress:    1.0,
			IsFinished:  true,
		}
		return detail.MarkFinishedMsg{EpisodeID: episodeID, Progress: progress}
	}
}

// closeSession closes a session on the server. The server reads a position in
// a close as a progress update, and a position that is not at the end of a
// finished book removes the finished mark. So the close sends the position
// and the listening time only if changed is true: the server does not have
// them.
func closeSession(ctx context.Context, client *cache.Client, sessionID string, currentTime, timeListened float64, changed bool) error {
	if !changed {
		return client.CloseSessionNoSync(ctx, sessionID)
	}
	return client.CloseSession(ctx, sessionID, currentTime, timeListened)
}

// closeThen returns a command that runs closeCmd, the close of a session, and
// then next. The server closes the open session of a device when that device
// starts a session, and it then answers 404 to the close of pine: the
// listening time that the close carries is lost. So a play request must not
// go out before the close completes or fails.
func closeThen(closeCmd, next tea.Cmd) tea.Cmd {
	if next == nil {
		return closeCmd
	}
	return func() tea.Msg {
		closeCmd()
		return next()
	}
}

// stopPlayback fires all cleanup commands: update progress, close session, quit mpv.
// It sends no position if the server has the position and the listening time.
func (m Model) stopPlayback() (Model, tea.Cmd) {
	m, closeCmd, stopCmd := m.stopPlaybackCmds()
	return m, tea.Batch(closeCmd, stopCmd)
}

// stopPlaybackCmds stops the playback as stopPlayback does. It returns the
// close of the session apart from the other cleanup commands, so that a
// caller can run a play request after the close with closeThen.
func (m Model) stopPlaybackCmds() (Model, tea.Cmd, tea.Cmd) {
	logger.Info("stopping playback", "session", abs.SessionTag(m.sessionID), "itemID", m.itemID, "episodeID", m.episodeID, "position", m.player.Position, "duration", m.player.Duration, "timeListened", m.timeListened)
	client := m.client
	sessionID := m.sessionID
	itemID := m.itemID
	episodeID := m.episodeID
	currentTime := m.player.Position
	timeListened := m.timeListened
	changed := m.hasUnsyncedProgress()
	duration := m.player.Duration
	store := m.db
	mpvPlayer := m.mpv
	cacheStore := m.cacheStore
	libraryID := m.playbackLibraryID
	seriesID := m.playbackSeriesID

	// Cache last played item for MPRIS metadata after stop
	m.lastPlayedTitle = m.player.Title
	m.lastPlayedTrackID = mprisTrackID(m.itemID, m.episodeID)
	m.lastPlayedAuthors = m.currentAuthors

	// Clear session state
	m.clearPlaybackSessionState()
	m.syncMprisState()
	m.propagateSize()

	var progress float64
	if duration > 0 {
		progress = currentTime / duration
	}

	invalidateCache := func() {
		evictProgressCaches(cacheStore, itemID, libraryID, seriesID)
	}

	closeCmd := func() tea.Msg {
		if client != nil && sessionID != "" {
			if err := closeSession(context.Background(), client, sessionID, currentTime, timeListened, changed); err != nil {
				logger.Warn("failed to close session", "session", abs.SessionTag(sessionID), "currentTime", currentTime, "timeListened", timeListened, "changed", changed, "err", err)
			} else {
				logger.Debug("session closed", "session", abs.SessionTag(sessionID), "currentTime", currentTime, "timeListened", timeListened, "changed", changed)
			}
		}
		invalidateCache()
		return nil
	}

	cmds := []tea.Cmd{
		func() tea.Msg {
			if changed && client != nil && itemID != "" {
				err := client.UpdatePosition(context.Background(), itemID, episodeID, currentTime, progress)
				if err != nil {
					logger.Warn("failed to update progress on stop", "itemID", itemID, "episodeID", episodeID, "currentTime", currentTime, "progress", progress, "err", err)
				} else {
					logger.Debug("progress updated on stop", "itemID", itemID, "episodeID", episodeID, "currentTime", currentTime, "progress", progress)
				}
			}
			invalidateCache()
			return nil
		},
		func() tea.Msg {
			if store != nil && itemID != "" {
				if err := store.SaveListeningSession(db.ListeningSession{
					ItemID:    itemID,
					EpisodeID: episodeID,
				}); err != nil {
					logger.Warn("failed to save listening session on stop", "itemID", itemID, "err", err)
				} else {
					logger.Debug("listening session saved on stop", "itemID", itemID)
				}
			}
			return nil
		},
		player.QuitCmd(mpvPlayer),
		m.mprisEndedCmd(),
		m.mprisTitleCmd(),
	}

	return m, closeCmd, tea.Batch(cmds...)
}

func playSessionChapters(session *abs.PlaySession) []abs.Chapter {
	if session == nil {
		return nil
	}
	if len(session.MediaMetadata.Chapters) > len(session.Chapters) {
		return session.MediaMetadata.Chapters
	}
	return session.Chapters
}

// Cleanup performs synchronous cleanup of playback resources.
func (m Model) Cleanup() {
	if bridge := m.mprisBridge(); bridge != nil {
		_ = bridge.Stop()
	}
	if m.mpv != nil {
		_ = m.mpv.Quit()
	}

	if !m.isPlaying() {
		return
	}

	currentTime := m.player.Position
	timeListened := m.timeListened
	changed := m.hasUnsyncedProgress()
	duration := m.player.Duration

	var progress float64
	if duration > 0 {
		progress = currentTime / duration
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	libraryID := m.playbackLibraryID
	seriesID := m.playbackSeriesID
	itemID := m.itemID

	if m.client != nil && m.sessionID != "" {
		if err := closeSession(ctx, m.client, m.sessionID, currentTime, timeListened, changed); err != nil {
			logger.Warn("cleanup: failed to close session", "session", abs.SessionTag(m.sessionID), "currentTime", currentTime, "timeListened", timeListened, "changed", changed, "err", err)
		} else {
			logger.Debug("cleanup: session closed", "session", abs.SessionTag(m.sessionID), "currentTime", currentTime, "timeListened", timeListened, "changed", changed)
		}
		evictProgressCaches(m.cacheStore, itemID, libraryID, seriesID)
	}

	if changed && m.client != nil && m.itemID != "" {
		err := m.client.UpdatePosition(ctx, m.itemID, m.episodeID, currentTime, progress)
		if err != nil {
			logger.Warn("cleanup: failed to update progress", "itemID", m.itemID, "episodeID", m.episodeID, "currentTime", currentTime, "progress", progress, "err", err)
		} else {
			logger.Debug("cleanup: progress updated", "itemID", m.itemID, "episodeID", m.episodeID, "currentTime", currentTime, "progress", progress)
		}
		evictProgressCaches(m.cacheStore, itemID, libraryID, seriesID)
	}

	if m.db != nil && m.itemID != "" {
		if err := m.db.SaveListeningSession(db.ListeningSession{
			ItemID:    m.itemID,
			EpisodeID: m.episodeID,
		}); err != nil {
			logger.Warn("cleanup: failed to save listening session", "itemID", m.itemID, "episodeID", m.episodeID, "err", err)
		} else {
			logger.Debug("cleanup: listening session saved", "itemID", m.itemID, "episodeID", m.episodeID)
		}
	}
}

// syncTickCmd returns a command that fires SyncTickMsg after the sync interval.
// generation is the playGeneration of the playback.
func syncTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(syncInterval, func(_ time.Time) tea.Msg {
		return SyncTickMsg{Generation: generation}
	})
}

// restoreSessionCmd fetches the last saved session from the DB and resolves
// the full LibraryItem from ABS so playback can resume.
func restoreSessionCmd(client *cache.Client, store *db.Store) tea.Cmd {
	return func() tea.Msg {
		session, err := store.GetLastSession()
		if err != nil {
			logger.Debug("no saved session to restore", "err", err)
			return RestoreSessionMsg{}
		}
		item, err := client.GetLibraryItem(context.Background(), session.ItemID)
		if err != nil {
			if abs.IsHTTPStatus(err, http.StatusNotFound) {
				clearSavedRestoreSession(
					store,
					"saved item missing",
					"itemID", session.ItemID,
					"episodeID", session.EpisodeID,
					"err", err,
				)
				return nil
			}
			logSkippedRestore("failed to fetch item for session restore", "itemID", session.ItemID, "episodeID", session.EpisodeID, "err", err)
			return RestoreSessionMsg{}
		}
		var episode *abs.PodcastEpisode
		if session.EpisodeID != "" {
			for _, ep := range item.Media.Episodes {
				if ep.ID == session.EpisodeID {
					ep := ep
					episode = &ep
					break
				}
			}
		}
		return RestoreSessionMsg{Item: item, Episode: episode, SavedEpisodeID: session.EpisodeID}
	}
}

func clearSavedRestoreSessionCmd(store *db.Store, reason string, keyvals ...any) tea.Cmd {
	return func() tea.Msg {
		clearSavedRestoreSession(store, reason, keyvals...)
		return nil
	}
}

func clearSavedRestoreSession(store *db.Store, reason string, keyvals ...any) {
	fields := append([]any{"reason", reason}, keyvals...)
	if store != nil {
		if err := store.ClearSession(); err != nil {
			fields = append(fields, "clearedSavedSession", false, "clearErr", err)
			logger.Info("skipping session restore", fields...)
			return
		}
		fields = append(fields, "clearedSavedSession", true)
	}
	logger.Info("skipping session restore", fields...)
}

func logSkippedRestore(reason string, keyvals ...any) {
	fields := append([]any{"reason", reason}, keyvals...)
	logger.Info("skipping session restore", fields...)
}

// setSeriesContext records the library and the series of the item that
// starts. The library is also the library of an item with no series: the
// home shelves of the library show the progress of the item.
func (m *Model) setSeriesContext(item abs.LibraryItem) {
	m.playbackLibraryID = item.LibraryID
	m.playbackSeriesID = ""
	if item.Media.Metadata.Series != nil {
		m.playbackSeriesID = item.Media.Metadata.Series.ID
	}
}

func seriesContinueCmd(client *cache.Client, libraryID, seriesID, currentItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		contents, err := client.GetSeriesContents(ctx, libraryID, seriesID)
		if err != nil {
			return SeriesContinueMsg{Err: err}
		}
		found := false
		for _, item := range contents.Items {
			if found {
				return SeriesContinueMsg{Item: item}
			}
			if item.ID == currentItemID {
				found = true
			}
		}
		return SeriesContinueMsg{}
	}
}
