package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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

// handleSeekToBookmark seeks the player to a bookmark's timestamp.
func (m Model) handleSeekToBookmark(msg detail.SeekToBookmarkCmd) (Model, tea.Cmd) {
	if m.mpv == nil || !m.isPlaying() {
		return m, nil
	}
	mpvPlayer := m.mpv
	target := msg.Time
	return m, func() tea.Msg {
		err := mpvPlayer.Seek(target)
		if err != nil {
			return player.PositionMsg{Err: err}
		}
		return nil
	}
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

// nextChapter returns the start time of the next chapter after current position.
func (m Model) nextChapter() (float64, bool) {
	for _, ch := range m.chapters {
		if ch.Start > m.player.Position {
			return ch.Start, true
		}
	}
	return 0, false
}

// prevChapter returns the start time of the previous chapter.
// Uses 3s grace: if within 3s of current chapter start, goes to the one before.
func (m Model) prevChapter() (float64, bool) {
	var target float64
	found := false
	for _, ch := range m.chapters {
		if ch.Start < m.player.Position-3 {
			target = ch.Start
			found = true
		}
	}
	if !found && len(m.chapters) > 0 {
		return m.chapters[0].Start, true
	}
	return target, found
}
