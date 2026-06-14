package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/screens/detail"
	"github.com/Thelost77/pine/internal/screens/metadataedit"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) openMetadataEditor(msg detail.EditMetadataCmd) (Model, tea.Cmd) {
	if msg.Item.MediaType == "podcast" && msg.Episode != nil {
		m.metadataEdit = metadataedit.NewEpisode(m.styles, msg.Item, *msg.Episode)
		return m.navigate(ScreenMetadataEdit)
	}
	if msg.Item.MediaType != "book" {
		return m, nil
	}
	m.metadataEdit = metadataedit.New(m.styles, msg.Item)
	return m.navigate(ScreenMetadataEdit)
}

func (m Model) handleMetadataSave(msg metadataedit.SaveCmd) (Model, tea.Cmd) {
	if m.client == nil {
		m.metadataEdit, _ = m.metadataEdit.Update(metadataedit.SavedMsg{ItemID: msg.ItemID, Generation: msg.Generation, Err: fmt.Errorf("not authenticated")})
		return m, nil
	}
	client := m.client
	return m, func() tea.Msg {
		updated, err := client.UpdateLibraryItemMedia(context.Background(), msg.ItemID, msg.Request)
		if err != nil {
			if abs.IsHTTPStatus(err, http.StatusForbidden) {
				err = fmt.Errorf("ABS user does not have metadata update permission: %w", err)
			}
			return metadataedit.SavedMsg{ItemID: msg.ItemID, Generation: msg.Generation, Err: err}
		}
		if updated != nil {
			return metadataedit.SavedMsg{ItemID: msg.ItemID, Generation: msg.Generation, Item: updated}
		}
		fetched, fetchErr := client.GetLibraryItem(context.Background(), msg.ItemID)
		if fetchErr == nil {
			return metadataedit.SavedMsg{ItemID: msg.ItemID, Generation: msg.Generation, Item: fetched}
		}
		return metadataedit.SavedMsg{ItemID: msg.ItemID, Generation: msg.Generation, Err: fetchErr}
	}
}

func (m Model) handleEpisodeMetadataSave(msg metadataedit.SaveEpisodeCmd) (Model, tea.Cmd) {
	if m.client == nil {
		m.metadataEdit, _ = m.metadataEdit.Update(metadataedit.SavedEpisodeMsg{ItemID: msg.ItemID, EpisodeID: msg.EpisodeID, Generation: msg.Generation, Err: fmt.Errorf("not authenticated")})
		return m, nil
	}
	client := m.client
	return m, func() tea.Msg {
		updated, err := client.UpdatePodcastEpisode(context.Background(), msg.ItemID, msg.EpisodeID, msg.Request)
		if err != nil {
			if abs.IsHTTPStatus(err, http.StatusForbidden) {
				err = fmt.Errorf("ABS user does not have podcast episode update permission: %w", err)
			}
			return metadataedit.SavedEpisodeMsg{ItemID: msg.ItemID, EpisodeID: msg.EpisodeID, Generation: msg.Generation, Err: err}
		}

		if updated != nil {
			return metadataedit.SavedEpisodeMsg{ItemID: msg.ItemID, EpisodeID: msg.EpisodeID, Generation: msg.Generation, Item: updated}
		}
		fetched, fetchErr := client.GetLibraryItem(context.Background(), msg.ItemID)
		if fetchErr == nil {
			return metadataedit.SavedEpisodeMsg{ItemID: msg.ItemID, EpisodeID: msg.EpisodeID, Generation: msg.Generation, Item: fetched}
		}
		return metadataedit.SavedEpisodeMsg{ItemID: msg.ItemID, EpisodeID: msg.EpisodeID, Generation: msg.Generation, Err: fetchErr}
	}
}

func (m Model) handleMetadataSaved(msg metadataedit.SavedMsg) (Model, tea.Cmd) {
	activeEditor := m.screen == ScreenMetadataEdit && m.metadataEdit.MatchesBookSave(msg.ItemID, msg.Generation)
	if msg.Err != nil {
		if m2, cmd, ok := m.checkUnauthorized(msg.Err); ok {
			return m2, cmd
		}
		if activeEditor {
			m.metadataEdit, _ = m.metadataEdit.Update(msg)
		}
		return m, nil
	}

	mprisChanged := false
	if msg.Item != nil {
		updated := *msg.Item
		if m.detail.ItemID() == updated.ID {
			m.detail.SetItem(updated)
		}
		mprisChanged = m.applyMetadataUpdate(updated)
	}
	if activeEditor {
		m.metadataEdit, _ = m.metadataEdit.Update(msg)
		m, backCmd := m.back()
		if mprisChanged {
			return m, tea.Batch(backCmd, m.mprisTitleCmd())
		}
		return m, backCmd
	}
	if mprisChanged {
		return m, m.mprisTitleCmd()
	}
	return m, nil
}

func (m Model) handleEpisodeMetadataSaved(msg metadataedit.SavedEpisodeMsg) (Model, tea.Cmd) {
	activeEditor := m.screen == ScreenMetadataEdit && m.metadataEdit.MatchesEpisodeSave(msg.ItemID, msg.EpisodeID, msg.Generation)
	if msg.Err != nil {
		if m2, cmd, ok := m.checkUnauthorized(msg.Err); ok {
			return m2, cmd
		}
		if activeEditor {
			m.metadataEdit, _ = m.metadataEdit.Update(msg)
		}
		return m, nil
	}

	mprisChanged := false
	if msg.Item != nil {
		updated := *msg.Item
		if m.detail.ItemID() == updated.ID {
			m.detail.SetItem(updated)
		}
		mprisChanged = m.applyMetadataUpdate(updated)
		mprisChanged = m.applyEpisodeMetadataUpdate(updated, msg.EpisodeID) || mprisChanged
	}
	if activeEditor {
		m.metadataEdit, _ = m.metadataEdit.Update(msg)
		m, backCmd := m.back()
		if mprisChanged {
			return m, tea.Batch(backCmd, m.mprisTitleCmd())
		}
		return m, backCmd
	}
	if mprisChanged {
		return m, m.mprisTitleCmd()
	}
	return m, nil
}

func (m *Model) applyMetadataUpdate(updated abs.LibraryItem) bool {
	mprisChanged := false
	m.home.ReplaceItem(updated)
	m.library.ReplaceItem(updated)
	if m.searchCache != nil {
		m.searchCache.Invalidate(updated.LibraryID)
	}

	for i := range m.queue {
		if m.queue[i].Item.ID == updated.ID {
			m.queue[i].Item = updated
		}
	}

	if m.itemID == updated.ID && m.episodeID == "" {
		m.player.Title = updated.Media.Metadata.Title
		m.currentAuthors = mprisAuthors(updated)
		m.syncMprisState()
		mprisChanged = true
	}
	if m.lastPlayedItemID == updated.ID && updated.MediaType != "podcast" {
		m.lastPlayedTitle = updated.Media.Metadata.Title
		m.lastPlayedAuthors = mprisAuthors(updated)
		m.syncMprisState()
		mprisChanged = true
	}
	return mprisChanged
}

func (m *Model) applyEpisodeMetadataUpdate(updated abs.LibraryItem, episodeID string) bool {
	episode, ok := findEpisode(updated.Media.Episodes, episodeID)
	if !ok {
		return false
	}
	for i := range m.queue {
		if m.queue[i].Item.ID == updated.ID && m.queue[i].Episode != nil && m.queue[i].Episode.ID == episodeID {
			ep := episode
			m.queue[i].Episode = &ep
		}
	}
	if m.itemID == updated.ID && m.episodeID == episodeID {
		m.player.Title = episode.Title
		m.currentAuthors = mprisAuthors(updated)
		m.syncMprisState()
		return true
	}
	return false
}

func findEpisode(episodes []abs.PodcastEpisode, episodeID string) (abs.PodcastEpisode, bool) {
	for _, episode := range episodes {
		if episode.ID == episodeID {
			return episode, true
		}
	}
	return abs.PodcastEpisode{}, false
}

func mprisAuthors(item abs.LibraryItem) []string {
	author := item.Media.Metadata.DisplayAuthor()
	if author == "" || author == "Unknown author" {
		return nil
	}
	return []string{author}
}
