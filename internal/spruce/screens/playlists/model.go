package playlists

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/Thelost77/pine/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

var playlistCollator = collate.New(language.Polish, collate.IgnoreCase)

type Level int

const (
	LevelPlaylists Level = iota
	LevelTracks
)

type Model struct {
	client *jellyfin.Client

	level Level

	playlistList list.Model
	trackList    list.Model

	selectedPlaylist jellyfin.Playlist
	playlists        []jellyfin.Playlist
	trackSource      []jellyfin.Track
	tracks           []jellyfin.Track

	loading bool
	err     error
	width   int
	height  int
	styles  ui.Styles
	flash   *components.ItemFlash
}

func New(styles ui.Styles) Model {
	flash := components.NewItemFlash()
	del := components.NewFlashDelegate(styles, flash)

	initList := func(title string) list.Model {
		l := list.New(components.BuildSkeletonRows(styles), del, 0, 0)
		l.KeyMap.Quit.SetKeys("q")
		l.KeyMap.PrevPage.SetKeys("pgup", "b", "u")
		l.KeyMap.NextPage.SetKeys("pgdown")
		l.Title = title
		l.SetShowTitle(false)
		l.SetShowHelp(false)
		l.SetShowStatusBar(true)
		l.SetFilteringEnabled(true)
		l.Filter = components.UnicodeFilter
		l.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{
				key.NewBinding(key.WithKeys("esc", "left"), key.WithHelp("esc", "back")),
				key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add selected to queue")),
				key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "add all to queue")),
				key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "shuffle all to queue")),
			}
		}
		return l
	}

	return Model{
		level:        LevelPlaylists,
		playlistList: initList("Playlists"),
		trackList:    initList("Playlist Tracks"),
		styles:       styles,
		flash:        flash,
		loading:      true,
	}
}

func (m *Model) SetClient(client *jellyfin.Client) {
	m.client = client
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.playlistList.SetSize(width, height)
	m.trackList.SetSize(width, height)
}

func (m Model) Init() tea.Cmd {
	if m.client != nil && len(m.playlists) == 0 {
		return m.fetchPlaylistsCmd()
	}
	return nil
}

func (m Model) fetchPlaylistsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		logger.Info("fetching playlists")
		playlists, err := client.GetPlaylists(context.Background())
		return PlaylistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

func (m Model) fetchPlaylistTracksCmd(playlistID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		logger.Info("fetching playlist tracks", "playlistID", playlistID)
		tracks, err := client.GetPlaylistTracks(context.Background(), playlistID)
		return PlaylistTracksLoadedMsg{Tracks: tracks, Err: err}
	}
}

func (m Model) fetchPlaylistTracksForQueueCmd(playlistID string, shuffled bool) tea.Cmd {
	client := m.client
	confirmation := library.QueueConfirmation{
		Origin:  library.QueueOriginPlaylists,
		ItemIDs: []string{playlistID},
	}
	failed := func(err error) tea.Msg {
		if shuffled {
			return library.AddShuffledTracksToQueueMsg{Err: err}
		}
		return library.AddTracksToQueueMsg{Err: err}
	}
	return func() tea.Msg {
		logger.Info("fetching playlist tracks for queue", "playlistID", playlistID, "shuffled", shuffled)
		if client == nil {
			return failed(errors.New("cannot queue playlist without a Jellyfin client"))
		}
		tracks, err := client.GetPlaylistTracks(context.Background(), playlistID)
		if err != nil {
			return failed(fmt.Errorf("queue playlist: %w", err))
		}
		if len(tracks) == 0 {
			return failed(errors.New("playlist has no tracks to queue"))
		}
		if shuffled {
			return library.AddShuffledTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
		}
		return library.AddTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ItemFlashExpiredMsg:
		m.flash.HandleExpired(msg)
		return m, nil

	case RefilterMsg:
		return m.Update(msg.Msg)

	case PlaylistsLoadedMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.playlists = append([]jellyfin.Playlist(nil), msg.Playlists...)
			slices.SortFunc(m.playlists, func(a, b jellyfin.Playlist) int {
				c := playlistCollator.CompareString(a.Name, b.Name)
				if c != 0 {
					return c
				}
				return strings.Compare(a.ID, b.ID)
			})
			items := make([]list.Item, len(m.playlists))
			for i, p := range m.playlists {
				items[i] = playlistItem{Playlist: p}
			}
			cmd := m.playlistList.SetItems(items)
			m.level = LevelPlaylists
			return m, cmd
		}
		return m, nil

	case PlaylistTracksLoadedMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.trackSource = append([]jellyfin.Track(nil), msg.Tracks...)
			cmd := m.rebuildTrackList("")
			m.level = LevelTracks
			return m, cmd
		}
		m.level = LevelPlaylists
		return m, nil

	case tea.KeyMsg:
		if m.activeList().FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "esc", "left":
			if m.HasActiveFilter() {
				m.activeList().ResetFilter()
				return m, nil
			}
			if m.level == LevelTracks {
				m.level = LevelPlaylists
				return m, nil
			}
		case "enter", "right":
			if m.level == LevelPlaylists {
				if sel, ok := m.playlistList.SelectedItem().(playlistItem); ok {
					return m, m.SelectPlaylist(sel.Playlist)
				}
			}
			if m.level == LevelTracks {
				if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
					track := sel.Track
					m.trackList.ResetFilter()
					confirmation := library.QueueConfirmation{
						Origin:  library.QueueOriginPlaylists,
						ItemIDs: []string{track.ID},
					}
					return m, func() tea.Msg {
						return library.PlayTracksMsg{
							Tracks:       []jellyfin.Track{track},
							StartIndex:   0,
							Confirmation: confirmation,
						}
					}
				}
			}
		case "f":
			if m.level == LevelTracks && m.client != nil {
				if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
					client := m.client
					track := sel.Track
					return m, func() tea.Msg {
						userData, err := client.SetFavorite(context.Background(), track.ID, !track.UserData.IsFavorite)
						return library.FavoriteChangedMsg{TrackID: track.ID, IsFavorite: userData.IsFavorite, Err: err}
					}
				}
			}
		case "a", "A", "S":
			if m.level == LevelPlaylists {
				if sel, ok := m.playlistList.SelectedItem().(playlistItem); ok {
					return m, m.fetchPlaylistTracksForQueueCmd(sel.Playlist.ID, msg.String() == "S")
				}
			}
			if msg.String() == "a" && m.level == LevelTracks {
				if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
					track := sel.Track
					confirmation := library.QueueConfirmation{
						Origin:  library.QueueOriginPlaylists,
						ItemIDs: []string{track.ID},
					}
					return m, func() tea.Msg {
						return library.AddTrackToQueueMsg{Track: track, Confirmation: confirmation}
					}
				}
			}
			if (msg.String() == "A" || msg.String() == "S") && m.level == LevelTracks && len(m.tracks) > 0 {
				tracks := m.tracks
				confirmation := library.QueueConfirmation{
					Origin:  library.QueueOriginPlaylists,
					ItemIDs: components.VisibleFlashIDs(m.trackList),
				}
				return m, func() tea.Msg {
					if msg.String() == "S" {
						return library.AddShuffledTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
					}
					return library.AddTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
				}
			}
		case "L":
			al := m.activeList()
			before := al.GlobalIndex()
			al.NextPage()
			if al.GlobalIndex() == before {
				al.GoToEnd()
			}
			return m, nil
		case "H":
			al := m.activeList()
			before := al.GlobalIndex()
			al.PrevPage()
			if al.GlobalIndex() == before {
				al.GoToStart()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.level {
	case LevelTracks:
		m.trackList, cmd = m.trackList.Update(msg)
	default:
		m.playlistList, cmd = m.playlistList.Update(msg)
	}
	return m, cmd
}

func (m *Model) rebuildTrackList(selectedID string) tea.Cmd {
	m.tracks = append(m.tracks[:0], m.trackSource...)
	jellyfin.SortFavoritesFirst(m.tracks)
	items := make([]list.Item, len(m.tracks))
	for i, track := range m.tracks {
		items[i] = trackItem{Track: track}
	}
	m.trackList.Title = m.selectedPlaylist.Name
	cmd := m.trackList.SetItems(items)
	if selectedID == "" {
		m.trackList.ResetSelected()
	}
	for i, track := range m.tracks {
		if track.ID == selectedID {
			m.trackList.Select(i)
			break
		}
	}
	return cmd
}

func (m *Model) activeList() *list.Model {
	if m.level == LevelTracks {
		return &m.trackList
	}
	return &m.playlistList
}

func (m Model) CurrentLevel() Level {
	return m.level
}

func (m Model) IsFiltering() bool {
	return m.activeList().FilterState() == list.Filtering
}

func (m Model) HasActiveFilter() bool {
	return m.activeList().FilterValue() != "" || m.activeList().FilterState() == list.FilterApplied
}

func (m Model) Playlists() []jellyfin.Playlist {
	return m.playlists
}

func (m Model) Tracks() []jellyfin.Track {
	return m.tracks
}

// PatchTrackFavorite updates favorite state and restores playlist-relative ordering.
func (m *Model) PatchTrackFavorite(id string, favorite bool) tea.Cmd {
	selectedID := ""
	if selected, ok := m.trackList.SelectedItem().(trackItem); ok {
		selectedID = selected.Track.ID
	}
	for i := range m.trackSource {
		if m.trackSource[i].ID == id {
			m.trackSource[i].UserData.IsFavorite = favorite
		}
	}
	cmd := m.rebuildTrackList(selectedID)
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		return RefilterMsg{Msg: cmd()}
	}
}

func (m Model) SelectedPlaylist() (jellyfin.Playlist, bool) {
	if sel, ok := m.playlistList.SelectedItem().(playlistItem); ok {
		return sel.Playlist, true
	}
	return jellyfin.Playlist{}, false
}

func (m Model) SelectedTrack() (jellyfin.Track, bool) {
	if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
		return sel.Track, true
	}
	return jellyfin.Track{}, false
}

// FlashItems highlights the given item IDs briefly to confirm a queue action.
func (m *Model) FlashItems(ids ...string) tea.Cmd {
	if m.flash == nil {
		return nil
	}
	return m.flash.Start(ids...)
}

// IsFlashing reports whether id is currently highlighted.
func (m Model) IsFlashing(id string) bool {
	return m.flash != nil && m.flash.Active(id)
}

func (m *Model) SelectPlaylist(playlist jellyfin.Playlist) tea.Cmd {
	m.selectedPlaylist = playlist
	m.loading = true
	m.level = LevelTracks
	m.trackSource = nil
	m.tracks = nil
	m.trackList.Title = fmt.Sprintf("Playlist Tracks — %s", playlist.Name)
	m.trackList.SetItems(components.BuildSkeletonRows(m.styles))
	return m.fetchPlaylistTracksCmd(playlist.ID)
}
