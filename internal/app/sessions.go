package app

import (
	"context"
	"fmt"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/screens/sessions"
	tea "github.com/charmbracelet/bubbletea"
)

// handleSessionsKey opens the listening sessions. On the detail screen they
// are the sessions of the book, or of the episode under the cursor. On another
// screen they are the sessions of the item that plays, or of all items if no
// item plays.
func (m Model) handleSessionsKey() (Model, tea.Cmd) {
	switch {
	case m.screen == ScreenSessions:
		return m, nil
	case m.screen == ScreenDetail:
		item, episode, ok := m.detail.PlayTarget()
		if !ok {
			cmd := m.err.SetError(fmt.Errorf("select an episode to see its listening sessions"))
			m.propagateSize()
			return m, cmd
		}
		return m.openSessions(sessionsTarget(item, episode))
	case m.isPlaying():
		return m.openSessions(sessions.Target{ItemID: m.itemID, EpisodeID: m.episodeID, Title: m.player.Title})
	}
	return m.openSessions(sessions.Target{})
}

// openSessions shows the sessions of target. On the sessions screen, the new
// target replaces the old one, so that the back stack gets no second entry.
func (m Model) openSessions(target sessions.Target) (Model, tea.Cmd) {
	m.sessions = sessions.New(m.styles, m.client, target, m.config.DeviceID)
	if m.screen == ScreenSessions {
		m.propagateSize()
		return m, m.sessions.Init()
	}
	return m.navigate(ScreenSessions)
}

func sessionsTarget(item abs.LibraryItem, episode *abs.PodcastEpisode) sessions.Target {
	if episode != nil {
		return sessions.Target{ItemID: item.ID, EpisodeID: episode.ID, Title: episode.Title}
	}
	return sessions.Target{ItemID: item.ID, Title: item.Media.Metadata.Title}
}

// handlePlayFromSession leaves the sessions screen and plays the item of a
// session from the position where the session ended. If that item plays, it
// seeks. Else it gets the item first, because the session has only its ID.
func (m Model) handlePlayFromSession(msg sessions.PlayFromMsg) (Model, tea.Cmd) {
	m, _ = m.back()
	if m.isPlaying() && m.itemID == msg.ItemID && m.episodeID == msg.EpisodeID {
		return m.seekToBookGlobalPosition(msg.Position)
	}
	if m.client == nil {
		return m, nil
	}
	client := m.client
	return m, func() tea.Msg {
		item, err := client.GetLibraryItem(context.Background(), msg.ItemID)
		if err != nil {
			return PlaybackErrorMsg{Err: fmt.Errorf("get the item of the session: %w", err)}
		}
		var episode *abs.PodcastEpisode
		if msg.EpisodeID != "" {
			for _, ep := range item.Media.Episodes {
				if ep.ID == msg.EpisodeID {
					episode = &ep
					break
				}
			}
			if episode == nil {
				return PlaybackErrorMsg{Err: fmt.Errorf("cannot find the episode of this session")}
			}
		}
		return SessionItemLoadedMsg{Item: *item, Episode: episode, Position: msg.Position}
	}
}
