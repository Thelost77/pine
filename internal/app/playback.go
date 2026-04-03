package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	tea "github.com/charmbracelet/bubbletea"
)

// isPlaying returns true if there's an active playback session.
func (m Model) isPlaying() bool {
	return m.sessionID != ""
}

// handlePlayCmd initiates a play session by calling the ABS API.
// If the same item is already playing, toggles pause.
// If a different item is playing, stops it first and starts the new one.
func (m Model) handlePlayCmd(msg detail.PlayCmd) (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}

	// Same book already playing → toggle pause
	if m.isPlaying() && m.itemID == msg.Item.ID && m.episodeID == "" {
		m.player.Playing = !m.player.Playing
		if m.mpv != nil {
			return m, player.TogglePauseCmd(m.mpv, m.player.Playing)
		}
		return m, nil
	}

	// Different item playing → stop it first
	var stopCmd tea.Cmd
	if m.isPlaying() {
		logger.Info("switching playback", "from", m.itemID, "to", msg.Item.ID)
		m, stopCmd = m.stopPlayback()
	}

	item := msg.Item
	client := m.client
	startCmd := func() tea.Msg {
		device := abs.DeviceInfo{
			DeviceID:   "pine",
			ClientName: "pine",
		}
		session, err := client.StartPlaySession(context.Background(), item.ID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		if len(session.AudioTracks) == 0 {
			return PlaybackErrorMsg{Err: fmt.Errorf("no audio tracks")}
		}

		// Find the correct track for the current position.
		// For multi-track audiobooks, currentTime is the total book position
		// and each track has a StartOffset within the book.
		track := session.AudioTracks[0]
		seekTime := session.CurrentTime
		for _, t := range session.AudioTracks {
			if session.CurrentTime >= t.StartOffset && session.CurrentTime < t.StartOffset+t.Duration {
				track = t
				seekTime = session.CurrentTime - t.StartOffset
				break
			}
		}
		logger.Info("track selected for playback", "itemID", item.ID, "sessionID", session.ID, "trackIndex", track.Index, "trackStart", track.StartOffset, "trackDuration", track.Duration, "bookPosition", session.CurrentTime, "seekTime", seekTime)

		streamURL := client.BaseURL() + track.ContentURL + "?token=" + client.Token()

		dur := item.Media.TotalDuration()

		return PlaySessionMsg{
			Session: PlaySessionData{
				SessionID:        session.ID,
				ItemID:           item.ID,
				CurrentTime:      seekTime,
				Duration:         dur,
				Title:            item.Media.Metadata.Title,
				Chapters:         playSessionChapters(session),
				TrackStartOffset: track.StartOffset,
				TrackDuration:    track.Duration,
			},
			StreamURL: streamURL,
		}
	}

	if stopCmd != nil {
		return m, tea.Batch(stopCmd, startCmd)
	}
	return m, startCmd
}

// handlePlayEpisodeCmd initiates a play session for a podcast episode.
// If the same episode is already playing, toggles pause.
// If a different item/episode is playing, stops it first and starts the new one.
func (m Model) handlePlayEpisodeCmd(msg detail.PlayEpisodeCmd) (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}

	// Same episode already playing → toggle pause
	if m.isPlaying() && m.itemID == msg.Item.ID && m.episodeID == msg.Episode.ID {
		m.player.Playing = !m.player.Playing
		if m.mpv != nil {
			return m, player.TogglePauseCmd(m.mpv, m.player.Playing)
		}
		return m, nil
	}

	// Different item/episode playing → stop it first
	var stopCmd tea.Cmd
	if m.isPlaying() {
		logger.Info("switching playback", "from", m.itemID+"/"+m.episodeID, "to", msg.Item.ID+"/"+msg.Episode.ID)
		m, stopCmd = m.stopPlayback()
	}

	item := msg.Item
	episode := msg.Episode
	client := m.client
	startCmd := func() tea.Msg {
		device := abs.DeviceInfo{
			DeviceID:   "pine",
			ClientName: "pine",
		}
		session, err := client.StartEpisodePlaySession(context.Background(), item.ID, episode.ID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		if len(session.AudioTracks) == 0 {
			return PlaybackErrorMsg{Err: fmt.Errorf("no audio tracks")}
		}

		streamURL := client.BaseURL() + session.AudioTracks[0].ContentURL + "?token=" + client.Token()
		logger.Info("track selected for episode playback", "itemID", item.ID, "episodeID", episode.ID, "sessionID", session.ID, "trackIndex", session.AudioTracks[0].Index, "trackStart", session.AudioTracks[0].StartOffset, "trackDuration", session.AudioTracks[0].Duration, "bookPosition", session.CurrentTime)

		return PlaySessionMsg{
			Session: PlaySessionData{
				SessionID:        session.ID,
				ItemID:           item.ID,
				EpisodeID:        episode.ID,
				CurrentTime:      session.CurrentTime,
				Duration:         episode.Duration,
				Title:            episode.Title,
				Chapters:         playSessionChapters(session),
				TrackStartOffset: session.AudioTracks[0].StartOffset,
				TrackDuration:    session.AudioTracks[0].Duration,
			},
			StreamURL: streamURL,
		}
	}

	if stopCmd != nil {
		return m, tea.Batch(stopCmd, startCmd)
	}
	return m, startCmd
}

// handlePlaySessionMsg launches mpv with the stream URL.
func (m Model) handlePlaySessionMsg(msg PlaySessionMsg) (Model, tea.Cmd) {
	m.playGeneration++
	m.sessionID = msg.Session.SessionID
	m.itemID = msg.Session.ItemID
	m.episodeID = msg.Session.EpisodeID
	m.chapters = msg.Session.Chapters
	m.resetChapterOverlay()
	m.trackStartOffset = msg.Session.TrackStartOffset
	m.trackDuration = msg.Session.TrackDuration
	m.timeListened = 0

	bookPos := msg.Session.CurrentTime + m.trackStartOffset
	m.lastSyncPos = bookPos

	// Update player model title/state
	m.player, _ = m.player.Update(player.StartPlayMsg{Title: msg.Session.Title})
	m.player.Position = bookPos
	m.player.Duration = msg.Session.Duration
	m.propagateSize()
	logger.Info("playback session loaded", "sessionID", msg.Session.SessionID, "itemID", msg.Session.ItemID, "episodeID", msg.Session.EpisodeID, "bookPosition", bookPos, "trackStart", msg.Session.TrackStartOffset, "trackDuration", msg.Session.TrackDuration, "chapters", len(msg.Session.Chapters))

	return m, player.LaunchCmd(m.mpv, msg.StreamURL, msg.Session.CurrentTime)
}

// handlePlayerReady starts the position tick and sync tick.
func (m Model) handlePlayerReady() (Model, tea.Cmd) {
	logger.Info("player ready, starting position ticks", "generation", m.playGeneration)
	return m, tea.Batch(
		player.TickCmd(m.mpv, m.playGeneration),
		syncTickCmd(),
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
		// If mpv exited or errored, clean up session
		logger.Warn("player position error, stopping playback", "err", msg.Err)
		if m.isPlaying() {
			return m.stopPlayback()
		}
		return m, nil
	}

	// Convert track-relative position to book-global
	bookPos := msg.Position + m.trackStartOffset

	// Track time listened (delta from last position)
	if bookPos > m.player.Position {
		m.timeListened += bookPos - m.player.Position
	}

	m.player.Position = bookPos
	m.player.Playing = !msg.Paused

	// Update sleep timer display
	if !m.sleepDeadline.IsZero() {
		remaining := time.Until(m.sleepDeadline)
		if remaining <= 0 {
			m.player.SleepRemaining = ""
		} else {
			m.player.SleepRemaining = formatSleepRemaining(remaining)
		}
	}

	if m.mpv != nil {
		return m, player.TickCmd(m.mpv, m.playGeneration)
	}
	return m, nil
}

// handleSyncTick syncs progress with ABS and persists to DB.
func (m Model) handleSyncTick() (Model, tea.Cmd) {
	if !m.isPlaying() {
		return m, nil
	}

	client := m.client
	sessionID := m.sessionID
	itemID := m.itemID
	currentTime := m.player.Position
	timeListened := m.timeListened
	duration := m.player.Duration
	store := m.db

	m.lastSyncPos = currentTime

	cmds := []tea.Cmd{
		syncTickCmd(),
		func() tea.Msg {
			if client != nil {
				if err := client.SyncSession(context.Background(), sessionID, currentTime, timeListened); err != nil {
					logger.Warn("failed to sync session", "sessionID", sessionID, "currentTime", currentTime, "timeListened", timeListened, "err", err)
				} else {
					logger.Debug("session synced", "sessionID", sessionID, "currentTime", currentTime, "timeListened", timeListened)
				}
			}
			return nil
		},
		func() tea.Msg {
			if store != nil {
				if err := store.SaveListeningSession(db.ListeningSession{
					ItemID:      itemID,
					SessionID:   sessionID,
					CurrentTime: currentTime,
					Duration:    duration,
				}); err != nil {
					logger.Warn("failed to save listening session", "itemID", itemID, "sessionID", sessionID, "currentTime", currentTime, "duration", duration, "err", err)
				} else {
					logger.Debug("listening session saved", "itemID", itemID, "sessionID", sessionID, "currentTime", currentTime, "duration", duration)
				}
			}
			return nil
		},
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleMarkFinished(msg detail.MarkFinishedCmd) (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	client := m.client
	item := msg.Item
	duration := item.Media.TotalDuration()
	return m, func() tea.Msg {
		if err := client.UpdateProgress(context.Background(), item.ID, duration, 1.0, true); err != nil {
			logger.Warn("failed to mark as finished", "err", err)
			return PlaybackErrorMsg{Err: err}
		}
		return detail.MarkFinishedMsg{
			Progress: &abs.UserMediaProgress{
				CurrentTime: duration,
				Progress:    1.0,
				IsFinished:  true,
			},
		}
	}
}

// stopPlayback fires all cleanup commands: update progress, close session, quit mpv.
func (m Model) stopPlayback() (Model, tea.Cmd) {
	logger.Info("stopping playback", "sessionID", m.sessionID, "itemID", m.itemID, "episodeID", m.episodeID, "position", m.player.Position, "duration", m.player.Duration, "timeListened", m.timeListened)
	client := m.client
	sessionID := m.sessionID
	itemID := m.itemID
	episodeID := m.episodeID
	currentTime := m.player.Position
	timeListened := m.timeListened
	duration := m.player.Duration
	store := m.db
	mpvPlayer := m.mpv

	// Clear session state
	m.clearPlaybackSessionState()
	m.propagateSize()

	var progress float64
	if duration > 0 {
		progress = currentTime / duration
	}

	cmds := []tea.Cmd{
		func() tea.Msg {
			if client != nil && itemID != "" {
				var err error
				if episodeID != "" {
					err = client.UpdateEpisodeProgress(context.Background(), itemID, episodeID, currentTime, progress, false)
				} else {
					err = client.UpdateProgress(context.Background(), itemID, currentTime, progress, false)
				}
				if err != nil {
					logger.Warn("failed to update progress on stop", "itemID", itemID, "episodeID", episodeID, "currentTime", currentTime, "progress", progress, "err", err)
				} else {
					logger.Debug("progress updated on stop", "itemID", itemID, "episodeID", episodeID, "currentTime", currentTime, "progress", progress)
				}
			}
			return nil
		},
		func() tea.Msg {
			if client != nil && sessionID != "" {
				if err := client.CloseSession(context.Background(), sessionID, currentTime, timeListened); err != nil {
					logger.Warn("failed to close session", "sessionID", sessionID, "currentTime", currentTime, "timeListened", timeListened, "err", err)
				} else {
					logger.Debug("session closed", "sessionID", sessionID, "currentTime", currentTime, "timeListened", timeListened)
				}
			}
			return nil
		},
		func() tea.Msg {
			if store != nil && itemID != "" {
				if err := store.SaveListeningSession(db.ListeningSession{
					ItemID:      itemID,
					SessionID:   sessionID,
					CurrentTime: currentTime,
					Duration:    duration,
				}); err != nil {
					logger.Warn("failed to save listening session on stop", "itemID", itemID, "sessionID", sessionID, "currentTime", currentTime, "duration", duration, "err", err)
				} else {
					logger.Debug("listening session saved on stop", "itemID", itemID, "sessionID", sessionID, "currentTime", currentTime, "duration", duration)
				}
			}
			return nil
		},
		player.QuitCmd(mpvPlayer),
	}

	return m, tea.Batch(cmds...)
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
	if m.mpv != nil {
		_ = m.mpv.Quit()
	}

	if !m.isPlaying() {
		return
	}

	currentTime := m.player.Position
	timeListened := m.timeListened
	duration := m.player.Duration

	var progress float64
	if duration > 0 {
		progress = currentTime / duration
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if m.client != nil && m.sessionID != "" {
		if err := m.client.CloseSession(ctx, m.sessionID, currentTime, timeListened); err != nil {
			logger.Warn("cleanup: failed to close session", "sessionID", m.sessionID, "currentTime", currentTime, "timeListened", timeListened, "err", err)
		} else {
			logger.Debug("cleanup: session closed", "sessionID", m.sessionID, "currentTime", currentTime, "timeListened", timeListened)
		}
	}

	if m.client != nil && m.itemID != "" {
		var err error
		if m.episodeID != "" {
			err = m.client.UpdateEpisodeProgress(ctx, m.itemID, m.episodeID, currentTime, progress, false)
		} else {
			err = m.client.UpdateProgress(ctx, m.itemID, currentTime, progress, false)
		}
		if err != nil {
			logger.Warn("cleanup: failed to update progress", "itemID", m.itemID, "episodeID", m.episodeID, "currentTime", currentTime, "progress", progress, "err", err)
		} else {
			logger.Debug("cleanup: progress updated", "itemID", m.itemID, "episodeID", m.episodeID, "currentTime", currentTime, "progress", progress)
		}
	}

	if m.db != nil && m.itemID != "" {
		if err := m.db.SaveListeningSession(db.ListeningSession{
			ItemID:      m.itemID,
			SessionID:   m.sessionID,
			CurrentTime: currentTime,
			Duration:    duration,
		}); err != nil {
			logger.Warn("cleanup: failed to save listening session", "itemID", m.itemID, "sessionID", m.sessionID, "currentTime", currentTime, "duration", duration, "err", err)
		} else {
			logger.Debug("cleanup: listening session saved", "itemID", m.itemID, "sessionID", m.sessionID, "currentTime", currentTime, "duration", duration)
		}
	}
}

// syncTickCmd returns a command that fires SyncTickMsg after the sync interval.
func syncTickCmd() tea.Cmd {
	return tea.Tick(syncInterval, func(_ time.Time) tea.Msg {
		return SyncTickMsg{}
	})
}
