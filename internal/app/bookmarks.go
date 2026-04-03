package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/ui"
)

// handleAddBookmark creates a bookmark at the current playback position.
func (m Model) handleAddBookmark(msg detail.AddBookmarkCmd) (Model, tea.Cmd) {
	if m.client == nil {
		logger.Warn("bookmark skipped: not authenticated")
		return m, nil
	}
	if !m.isPlaying() {
		logger.Warn("bookmark skipped: not playing", "itemID", msg.Item.ID)
		cmd := m.err.SetError(fmt.Errorf("start playback before adding a bookmark"))
		m.propagateSize()
		return m, cmd
	}
	client := m.client
	itemID := msg.Item.ID
	currentTime := m.player.Position
	title := fmt.Sprintf("Bookmark at %s", ui.FormatTimestamp(currentTime))
	logger.Info("creating bookmark", "itemID", itemID, "time", currentTime, "title", title)

	return m, func() tea.Msg {
		err := client.CreateBookmark(context.Background(), itemID, currentTime, title)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		bookmarks, err := client.GetBookmarks(context.Background(), itemID)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return detail.BookmarksUpdatedMsg{Bookmarks: bookmarks}
	}
}

// handleSeekToBookmark seeks the player to a bookmark's timestamp (book-global).
func (m Model) handleSeekToBookmark(msg detail.SeekToBookmarkCmd) (Model, tea.Cmd) {
	if !m.isPlaying() {
		return m, nil
	}
	return m.seekToBookGlobalPosition(msg.Time)
}

// handleDeleteBookmark deletes a bookmark and refreshes the list.
func (m Model) handleDeleteBookmark(msg detail.DeleteBookmarkCmd) (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	client := m.client
	itemID := msg.ItemID
	bmTime := msg.Bookmark.Time

	return m, func() tea.Msg {
		err := client.DeleteBookmark(context.Background(), itemID, bmTime)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		bookmarks, err := client.GetBookmarks(context.Background(), itemID)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return detail.BookmarksUpdatedMsg{Bookmarks: bookmarks}
	}
}

// fetchBookmarksCmd returns a command that fetches bookmarks for an item from ABS.
func (m Model) fetchBookmarksCmd(itemID string) tea.Cmd {
	if m.client == nil {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		bookmarks, err := client.GetBookmarks(context.Background(), itemID)
		if err != nil {
			return detail.BookmarksUpdatedMsg{Bookmarks: nil}
		}
		return detail.BookmarksUpdatedMsg{Bookmarks: bookmarks}
	}
}

// fetchEpisodesCmd returns a command that fetches the full item details (including episodes) from ABS.
func (m Model) fetchEpisodesCmd(itemID string) tea.Cmd {
	if m.client == nil {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		item, err := client.GetLibraryItem(context.Background(), itemID)
		if err != nil {
			return EpisodesLoadedMsg{Err: err}
		}
		return EpisodesLoadedMsg{Episodes: item.Media.Episodes}
	}
}

// nextChapter returns the start time of the next chapter after current position (book-global).
func (m Model) nextChapter() (float64, bool) {
	for _, ch := range m.chapters {
		if ch.Start > m.player.Position {
			return ch.Start, true
		}
	}
	return 0, false
}

// prevChapter returns the start of the previous chapter (book-global).
// If more than 10s into the current chapter, restarts it.
// If within 10s, goes to the chapter before.
func (m Model) prevChapter() (float64, bool) {
	if len(m.chapters) == 0 {
		return 0, false
	}
	// Find the current chapter index
	curIdx := 0
	for i, ch := range m.chapters {
		if m.player.Position >= ch.Start {
			curIdx = i
		}
	}
	// If more than 10s into current chapter, restart it
	if m.player.Position-m.chapters[curIdx].Start > 10 {
		return m.chapters[curIdx].Start, true
	}
	// Otherwise go to the previous chapter
	if curIdx > 0 {
		return m.chapters[curIdx-1].Start, true
	}
	return m.chapters[0].Start, true
}

// seekToChapter seeks to a chapter position. If the chapter is outside the
// current track, restarts playback at the new position via ABS.
func (m Model) seekToChapter(target float64, ok bool) (Model, tea.Cmd) {
	if !ok || !m.isPlaying() {
		return m, nil
	}
	return m.seekToBookGlobalPosition(target)
}

// seekToBookGlobalPosition seeks to a book-global position. If the position
// falls within the current track, seeks mpv directly. Otherwise stops playback
// and starts a new session so ABS picks the correct track.
func (m Model) seekToBookGlobalPosition(bookPos float64) (Model, tea.Cmd) {
	trackDur := m.trackDuration
	if trackDur == 0 {
		trackDur = m.player.Duration
	}
	trackEnd := m.trackStartOffset + trackDur
	if bookPos >= m.trackStartOffset && bookPos < trackEnd {
		trackRelative := bookPos - m.trackStartOffset
		m.player.Position = bookPos
		mpvPlayer := m.mpv
		return m, func() tea.Msg {
			if err := mpvPlayer.Seek(trackRelative); err != nil {
				return player.PositionMsg{Err: err}
			}
			return nil
		}
	}

	// Cross-track seek: restart playback at the new book-global position.
	logger.Info("cross-track seek, restarting playback", "from", m.player.Position, "to", bookPos)
	client := m.client
	itemID := m.itemID
	sessionID := m.sessionID
	currentTime := m.player.Position
	timeListened := m.timeListened
	duration := m.player.Duration
	title := m.player.Title
	mpvPlayer := m.mpv

	// Clear current session but keep itemID/chapters for the restart
	m.sessionID = ""
	m.timeListened = 0

	return m, func() tea.Msg {
		// Update progress to target position so new session starts there
		if client != nil && itemID != "" {
			var progress float64
			if duration > 0 {
				progress = bookPos / duration
			}
			_ = client.UpdateProgress(context.Background(), itemID, bookPos, progress, false)
		}
		// Close current session
		if client != nil && sessionID != "" {
			_ = client.CloseSession(context.Background(), sessionID, currentTime, timeListened)
		}
		if mpvPlayer != nil {
			_ = mpvPlayer.Quit()
		}

		// Start fresh session — ABS picks up from the updated progress
		device := abs.DeviceInfo{DeviceID: "pine", ClientName: "pine"}
		session, err := client.StartPlaySession(context.Background(), itemID, device)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		if len(session.AudioTracks) == 0 {
			return PlaybackErrorMsg{Err: fmt.Errorf("no audio tracks")}
		}

		track := session.AudioTracks[0]
		seekTime := session.CurrentTime
		for _, t := range session.AudioTracks {
			if session.CurrentTime >= t.StartOffset && session.CurrentTime < t.StartOffset+t.Duration {
				track = t
				seekTime = session.CurrentTime - t.StartOffset
				break
			}
		}

		streamURL := client.BaseURL() + track.ContentURL + "?token=" + client.Token()

		return PlaySessionMsg{
			Session: PlaySessionData{
				SessionID:        session.ID,
				ItemID:           itemID,
				CurrentTime:      seekTime,
				Duration:         duration,
				Title:            title,
				Chapters:         session.Chapters,
				TrackStartOffset: track.StartOffset,
				TrackDuration:    track.Duration,
			},
			StreamURL: streamURL,
		}
	}
}

// handleSeek seeks by a delta (in seconds), crossing track boundaries if needed.
func (m Model) handleSeek(seconds float64) (Model, tea.Cmd) {
	if m.mpv == nil {
		return m, nil
	}
	target := m.player.Position + seconds
	if target < 0 {
		target = 0
	}
	if m.player.Duration > 0 && target > m.player.Duration {
		target = m.player.Duration
	}
	return m.seekToBookGlobalPosition(target)
}
