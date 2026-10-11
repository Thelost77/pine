package app

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/spruce/screens/metadataedit"
	"github.com/Thelost77/pine/internal/spruce/screens/playlists"
	"github.com/Thelost77/pine/internal/spruce/screens/queue"
	"github.com/Thelost77/pine/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

func (m *Model) openCommandPalette() {
	staticItems := []components.PaletteItem{
		{Label: "Navigation", IsHeader: true},
		{Label: "Go to Library", Action: components.ActionGoLibrary},
		{Label: "Go to Playlists", Action: components.ActionGoPlaylists},
		{Label: "Go to Queue", Action: components.ActionShowQueue},
	}
	staticItems = append(staticItems, m.contextPaletteItems()...)
	if m.IsPlaying() {
		staticItems = append(staticItems,
			components.PaletteItem{Label: "Player", IsHeader: true},
			components.PaletteItem{Label: "Play / Pause", Action: components.ActionTogglePlay},
			components.PaletteItem{Label: "Seek Forward", Action: components.ActionSeekForward},
			components.PaletteItem{Label: "Seek Backward", Action: components.ActionSeekBackward},
			components.PaletteItem{Label: "Speed Up", Action: components.ActionSpeedUp},
			components.PaletteItem{Label: "Speed Down", Action: components.ActionSpeedDown},
			components.PaletteItem{Label: "Volume Up", Action: components.ActionVolumeUp},
			components.PaletteItem{Label: "Volume Down", Action: components.ActionVolumeDown},
			components.PaletteItem{Label: "Next Track", Action: components.ActionNextTrack},
			components.PaletteItem{Label: "Previous Track", Action: components.ActionPrevTrack},
			components.PaletteItem{Label: "Sleep Timer", IsHeader: true},
			components.PaletteItem{Label: "Sleep Timer: 15m", Action: components.ActionSleep15},
			components.PaletteItem{Label: "Sleep Timer: 30m", Action: components.ActionSleep30},
			components.PaletteItem{Label: "Sleep Timer: 45m", Action: components.ActionSleep45},
			components.PaletteItem{Label: "Sleep Timer: 60m", Action: components.ActionSleep60},
			components.PaletteItem{Label: "Sleep Timer: Off", Action: components.ActionSleepOff},
		)
	}
	if len(m.tracks) > 0 {
		subliminalLabel := "Start Subliminal Mode"
		if m.isSubliminalActive() {
			subliminalLabel = "Stop Subliminal Mode"
		}
		staticItems = append(staticItems,
			components.PaletteItem{Label: "Queue", IsHeader: true},
			components.PaletteItem{Label: subliminalLabel, Action: components.ActionSubliminalMode},
			components.PaletteItem{Label: "Shuffle Queue", Action: components.ActionShuffleQueue},
			components.PaletteItem{Label: "Repeat Current Track", Action: components.ActionRepeatTrack},
			components.PaletteItem{Label: "Repeat Queue", Action: components.ActionRepeatQueue},
			components.PaletteItem{Label: "Clear Queue", Action: components.ActionClearQueue},
		)
	}
	m.palette.Open(staticItems, m.contentSearchFunc())
}

func (m *Model) contextPaletteItems() []components.PaletteItem {
	switch m.screen {
	case ScreenLibrary:
		if m.libraryScreen.CurrentLevel() == library.LevelAlbums {
			if album, ok := m.libraryScreen.SelectedAlbum(); ok {
				return []components.PaletteItem{
					{Label: "Context Actions", IsHeader: true},
					{Label: "Open Selected", Action: components.ActionOpenSelected, ItemID: album.ID, Data: album},
					{Label: "Add Album to Queue", Action: components.ActionQueueItem, ItemID: album.ID, Data: album},
					{Label: "Shuffle Album to Queue", Action: components.ActionShuffleItem, ItemID: album.ID, Data: album},
					{Label: "Edit Metadata", Action: components.ActionEditMetadata, ItemID: album.ID, Data: album},
				}
			}
		}
		if track, ok := m.libraryScreen.SelectedTrack(); ok {
			return []components.PaletteItem{
				{Label: "Context Actions", IsHeader: true},
				{Label: "Play Selected", Action: components.ActionPlayDirect, ItemID: track.ID, Data: track},
				{Label: "Add Track to Queue", Action: components.ActionQueueItem, ItemID: track.ID, Data: track},
				{Label: "Edit Metadata", Action: components.ActionEditMetadata, ItemID: track.ID, Data: track},
			}
		}
	case ScreenPlaylists:
		if m.playlistsScreen.CurrentLevel() == playlists.LevelPlaylists {
			if playlist, ok := m.playlistsScreen.SelectedPlaylist(); ok {
				return []components.PaletteItem{
					{Label: "Context Actions", IsHeader: true},
					{Label: "Open Selected", Action: components.ActionOpenSelected, ItemID: playlist.ID, Data: playlist},
					{Label: "Add Playlist to Queue", Action: components.ActionQueueItem, ItemID: playlist.ID, Data: playlist},
					{Label: "Shuffle Playlist to Queue", Action: components.ActionShuffleItem, ItemID: playlist.ID, Data: playlist},
				}
			}
		}
		if track, ok := m.playlistsScreen.SelectedTrack(); ok {
			return []components.PaletteItem{
				{Label: "Context Actions", IsHeader: true},
				{Label: "Play Selected", Action: components.ActionPlayDirect, ItemID: track.ID, Data: track},
				{Label: "Add Track to Queue", Action: components.ActionQueueItem, ItemID: track.ID, Data: track},
			}
		}
	}
	return nil
}

func (m *Model) contentSearchFunc() components.SearchFunc {
	return func(query string) []components.PaletteItem {
		if query == "" {
			return nil
		}
		var candidates []components.PaletteItem
		var texts []string

		for _, a := range m.libraryScreen.Albums() {
			artist := "Unknown Artist"
			if len(a.Artists) > 0 {
				artist = strings.Join(a.Artists, ", ")
			}
			label := fmt.Sprintf("Album: %s — %s", a.Name, artist)
			candidates = append(candidates, components.PaletteItem{
				Label:  label,
				Action: components.ActionOpenSelected,
				ItemID: a.ID,
				Data:   a,
			})
			texts = append(texts, label)
		}

		for _, p := range m.playlistsScreen.Playlists() {
			label := fmt.Sprintf("Playlist: %s", p.Name)
			candidates = append(candidates, components.PaletteItem{
				Label:  label,
				Action: components.ActionOpenSelected,
				ItemID: p.ID,
				Data:   p,
			})
			texts = append(texts, label)
		}

		tracksToSearch := m.libraryScreen.AllTracks()
		if len(tracksToSearch) == 0 {
			tracksToSearch = m.libraryScreen.Tracks()
		}
		for _, t := range tracksToSearch {
			label := fmt.Sprintf("Track: %s — %s", t.Name, t.DisplayArtist())
			candidates = append(candidates, components.PaletteItem{
				Label:  label,
				Action: components.ActionPlayDirect,
				ItemID: t.ID,
				Data:   t,
			})
			texts = append(texts, label)
		}

		for _, t := range m.tracks {
			label := fmt.Sprintf("Queue: %s — %s", t.Name, t.DisplayArtist())
			candidates = append(candidates, components.PaletteItem{
				Label:  label,
				Action: components.ActionPlayDirect,
				ItemID: t.ID,
				Data:   t,
			})
			texts = append(texts, label)
		}

		matches := fuzzy.Find(query, texts)
		results := make([]components.PaletteItem, len(matches))
		for i, match := range matches {
			results[i] = candidates[match.Index]
		}
		return results
	}
}

func (m *Model) handlePaletteAction(action components.PaletteAction, itemID string, data any) (tea.Model, tea.Cmd) {
	switch action {
	case components.ActionGoLibrary:
		m.screen = ScreenLibrary
		return m, nil
	case components.ActionGoPlaylists:
		return m.navigate(ScreenPlaylists)
	case components.ActionShowQueue:
		m.screen = ScreenQueue
		return m, nil
	case components.ActionSubliminalMode:
		return m.toggleSubliminalMode()
	case components.ActionTogglePlay:
		if m.isSubliminalRunning() {
			return m.setSubliminalPaused(m.playerState.Playing)
		}
		if !m.IsPlaying() {
			return m, nil
		}
		m.playerState.Playing = !m.playerState.Playing
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetPauseCmd(m.mpv, !m.playerState.Playing)
		}
		return m, nil
	case components.ActionSeekForward:
		seek := 10.0
		if m.cfg != nil && m.cfg.Player.SeekSeconds != 0 {
			seek = float64(m.cfg.Player.SeekSeconds)
		}
		if seek == 0 {
			seek = 10
		}
		return m.handleSeek(seek)
	case components.ActionSeekBackward:
		seek := 10.0
		if m.cfg != nil && m.cfg.Player.SeekSeconds != 0 {
			seek = float64(m.cfg.Player.SeekSeconds)
		}
		if seek == 0 {
			seek = 10
		}
		return m.handleSeek(-seek)
	case components.ActionSpeedUp:
		m.playerState.Speed = math.Round((m.playerState.Speed+0.1)*10) / 10
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetSpeedCmd(m.mpv, m.playerState.Speed)
		}
		return m, nil
	case components.ActionSpeedDown:
		newSpeed := math.Round((m.playerState.Speed-0.1)*10) / 10
		if newSpeed >= 0.1 {
			m.playerState.Speed = newSpeed
		}
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetSpeedCmd(m.mpv, m.playerState.Speed)
		}
		return m, nil
	case components.ActionVolumeUp:
		if m.playerState.Volume < 150 {
			m.playerState.Volume += 5
		}
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetVolumeCmd(m.mpv, m.playerState.Volume)
		}
		return m, nil
	case components.ActionVolumeDown:
		if m.playerState.Volume > 0 {
			m.playerState.Volume -= 5
		}
		m.syncQueueScreen()
		if m.mpv != nil {
			return m, player.SetVolumeCmd(m.mpv, m.playerState.Volume)
		}
		return m, nil
	case components.ActionNextTrack:
		if len(m.tracks) > 0 && m.currentIndex+1 < len(m.tracks) {
			return m.startPlaybackAt(m.nextIndex(m.currentIndex + 1))
		}
		return m, nil
	case components.ActionPrevTrack:
		if m.playerState.Position > 3.0 {
			if m.mpv != nil {
				return m, player.SeekCmd(m.mpv, 0)
			}
			return m, nil
		}
		if len(m.tracks) > 0 && m.currentIndex-1 >= 0 {
			return m.startPlaybackAt(m.currentIndex - 1)
		}
		return m, nil
	case components.ActionClearQueue:
		return m.Update(queue.QueueActionMsg{Action: "clear"})
	case components.ActionShuffleQueue:
		return m.Update(queue.QueueActionMsg{Action: "shuffle"})
	case components.ActionRepeatTrack:
		if m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
			return m.Update(queue.QueueActionMsg{
				Action:  "repeat_track",
				Index:   m.currentIndex,
				TrackID: m.tracks[m.currentIndex].ID,
			})
		}
		return m, nil
	case components.ActionRepeatQueue:
		return m.Update(queue.QueueActionMsg{Action: "repeat_queue"})
	case components.ActionQueueItem:
		switch item := data.(type) {
		case jellyfin.Track:
			newM, cmd := m.Update(library.AddTrackToQueueMsg{Track: item})
			return newM, cmd
		case jellyfin.Album:
			if m.client == nil {
				return m, m.err.SetError(fmt.Errorf("cannot queue album without a Jellyfin client"))
			}
			client := m.client
			return m, func() tea.Msg {
				tracks, err := client.GetTracks(context.Background(), item.ID)
				if err != nil {
					return library.AddTracksToQueueMsg{Err: fmt.Errorf("queue album: %w", err)}
				}
				if len(tracks) == 0 {
					return library.AddTracksToQueueMsg{Err: fmt.Errorf("album %q has no tracks to queue", item.Name)}
				}
				return library.AddTracksToQueueMsg{Tracks: tracks}
			}
		case jellyfin.Playlist:
			if m.client == nil {
				return m, m.err.SetError(fmt.Errorf("cannot queue playlist without a Jellyfin client"))
			}
			client := m.client
			return m, func() tea.Msg {
				tracks, err := client.GetPlaylistTracks(context.Background(), item.ID)
				if err != nil {
					return library.AddTracksToQueueMsg{Err: fmt.Errorf("queue playlist: %w", err)}
				}
				if len(tracks) == 0 {
					return library.AddTracksToQueueMsg{Err: fmt.Errorf("playlist %q has no tracks to queue", item.Name)}
				}
				return library.AddTracksToQueueMsg{Tracks: tracks}
			}
		}
		return m, nil
	case components.ActionShuffleItem:
		switch item := data.(type) {
		case jellyfin.Track:
			newM, cmd := m.Update(library.AddTrackToQueueMsg{Track: item})
			return newM, cmd
		case jellyfin.Album:
			if m.client == nil {
				return m, m.err.SetError(fmt.Errorf("cannot queue album without a Jellyfin client"))
			}
			client := m.client
			return m, func() tea.Msg {
				tracks, err := client.GetTracks(context.Background(), item.ID)
				if err != nil {
					return library.AddShuffledTracksToQueueMsg{Err: fmt.Errorf("queue shuffled album: %w", err)}
				}
				if len(tracks) == 0 {
					return library.AddShuffledTracksToQueueMsg{Err: fmt.Errorf("album %q has no tracks to queue", item.Name)}
				}
				return library.AddShuffledTracksToQueueMsg{Tracks: tracks}
			}
		case jellyfin.Playlist:
			if m.client == nil {
				return m, m.err.SetError(fmt.Errorf("cannot queue playlist without a Jellyfin client"))
			}
			client := m.client
			return m, func() tea.Msg {
				tracks, err := client.GetPlaylistTracks(context.Background(), item.ID)
				if err != nil {
					return library.AddShuffledTracksToQueueMsg{Err: fmt.Errorf("queue shuffled playlist: %w", err)}
				}
				if len(tracks) == 0 {
					return library.AddShuffledTracksToQueueMsg{Err: fmt.Errorf("playlist %q has no tracks to queue", item.Name)}
				}
				return library.AddShuffledTracksToQueueMsg{Tracks: tracks}
			}
		}
		return m, nil
	case components.ActionEditMetadata:
		switch item := data.(type) {
		case jellyfin.Track:
			m.metadataEditScreen = metadataedit.New(m.styles, m.client, item.ID, "Track", &item, nil)
			m.metadataEditScreen.SetSize(m.width, m.screenHeight())
			return m.navigate(ScreenMetadataEdit)
		case jellyfin.Album:
			m.metadataEditScreen = metadataedit.New(m.styles, m.client, item.ID, "Album", nil, &item)
			m.metadataEditScreen.SetSize(m.width, m.screenHeight())
			return m.navigate(ScreenMetadataEdit)
		}
		return m, nil
	case components.ActionOpenSelected:
		if album, ok := data.(jellyfin.Album); ok {
			m.screen = ScreenLibrary
			cmd := m.libraryScreen.SelectAlbum(album)
			return m, cmd
		}
		if playlist, ok := data.(jellyfin.Playlist); ok {
			m.screen = ScreenPlaylists
			cmd := m.playlistsScreen.SelectPlaylist(playlist)
			return m, cmd
		}
	case components.ActionPlayDirect:
		if m.isSubliminalActive() {
			return m, m.err.SetError(fmt.Errorf("stop subliminal mode before starting another track"))
		}
		if track, ok := data.(jellyfin.Track); ok {
			m.tracks = append(m.tracks, track)
			return m.startPlaybackAt(len(m.tracks) - 1)
		}
	case components.ActionSleep15:
		return m.setSleepTimer(15 * time.Minute)
	case components.ActionSleep30:
		return m.setSleepTimer(30 * time.Minute)
	case components.ActionSleep45:
		return m.setSleepTimer(45 * time.Minute)
	case components.ActionSleep60:
		return m.setSleepTimer(60 * time.Minute)
	case components.ActionSleepOff:
		return m.setSleepTimer(0)
	}
	return m, nil
}
