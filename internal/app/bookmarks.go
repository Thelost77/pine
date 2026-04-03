package app

import (
	"context"
	"fmt"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
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
			return detail.BookmarksUpdatedMsg{Err: err}
		}
		logger.Info("bookmark list refreshed", "itemID", itemID, "count", len(bookmarks))
		return detail.BookmarksUpdatedMsg{Bookmarks: bookmarks}
	}
}

// handleSeekToBookmark seeks the player to a bookmark's timestamp (book-global).
func (m Model) handleSeekToBookmark(msg detail.SeekToBookmarkCmd) (Model, tea.Cmd) {
	if !m.isPlaying() {
		if m.client == nil || msg.Item.ID == "" {
			return m, nil
		}
		return m, m.startPlaybackAtBookPositionCmd(msg.Item, msg.Time)
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
			return detail.BookmarksUpdatedMsg{Err: err}
		}
		logger.Info("bookmark list refreshed", "itemID", itemID, "count", len(bookmarks))
		return detail.BookmarksUpdatedMsg{Bookmarks: bookmarks}
	}
}

// handleUpdateBookmark updates a bookmark title and refreshes the list.
func (m Model) handleUpdateBookmark(msg detail.UpdateBookmarkCmd) (Model, tea.Cmd) {
	if m.client == nil {
		return m, nil
	}
	client := m.client
	itemID := msg.ItemID
	bmTime := msg.Bookmark.Time
	title := msg.Title

	return m, func() tea.Msg {
		err := client.UpdateBookmark(context.Background(), itemID, bmTime, title)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		bookmarks, err := client.GetBookmarks(context.Background(), itemID)
		if err != nil {
			return detail.BookmarksUpdatedMsg{Err: err}
		}
		logger.Info("bookmark list refreshed", "itemID", itemID, "count", len(bookmarks))
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
			return detail.BookmarksUpdatedMsg{Err: err}
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

// fetchBookDetailCmd returns a command that fetches an enriched book item from ABS.
func (m Model) fetchBookDetailCmd(itemID string) tea.Cmd {
	if m.client == nil {
		return nil
	}
	client := m.client
	return func() tea.Msg {
		item, err := client.GetLibraryItem(context.Background(), itemID)
		if err != nil {
			return BookDetailLoadedMsg{Err: err}
		}
		return BookDetailLoadedMsg{Item: item}
	}
}

func (m Model) detailLoadCmds(item abs.LibraryItem, navCmd tea.Cmd) []tea.Cmd {
	cmds := []tea.Cmd{navCmd, m.fetchBookmarksCmd(item.ID)}
	if item.MediaType == "podcast" {
		return append(cmds, m.fetchEpisodesCmd(item.ID))
	}
	if item.MediaType == "book" {
		return append(cmds, m.fetchBookDetailCmd(item.ID))
	}
	return cmds
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
		logger.Debug("in-track seek", "bookPosition", bookPos, "trackStart", m.trackStartOffset, "trackEnd", trackEnd, "trackRelative", trackRelative)
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
	logger.Info("cross-track seek, restarting playback", "from", m.player.Position, "to", bookPos, "trackStart", m.trackStartOffset, "trackEnd", trackEnd)
	return m.restartPlaybackAt(bookPos)
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
