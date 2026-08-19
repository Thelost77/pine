package library

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Thelost77/spruce/internal/jellyfin"
	"github.com/Thelost77/spruce/internal/logger"
	"github.com/Thelost77/spruce/internal/ui"
	"github.com/Thelost77/spruce/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

var libraryCollator = collate.New(language.Polish, collate.IgnoreCase)

type Level int

const (
	LevelAlbums Level = iota
	LevelTracks
)

type TrackSort int

const (
	TrackSortAlbum TrackSort = iota
	TrackSortTitle
)

type Model struct {
	client     *jellyfin.Client
	libraryID  string
	libraryIDs []string

	level Level

	artistList list.Model
	albumList  list.Model
	trackList  list.Model

	selectedArtist jellyfin.Artist
	selectedAlbum  jellyfin.Album
	artists        []jellyfin.Artist
	albums         []jellyfin.Album
	trackSource    []jellyfin.Track
	tracks         []jellyfin.Track
	trackSort      TrackSort
	allTracks      []jellyfin.Track
	allTracksErr   error

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

	initList := func(title string, includeSortKey bool) list.Model {
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
			bindings := []key.Binding{
				key.NewBinding(key.WithKeys("esc", "left"), key.WithHelp("esc", "back")),
				key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add track to queue")),
				key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "add album to queue")),
				key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "add album shuffled")),
			}
			if includeSortKey {
				bindings = append(bindings, key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle sort")))
			}
			return bindings
		}
		return l
	}

	return Model{
		level:      LevelAlbums,
		artistList: initList("Artists", false),
		albumList:  initList("Albums", false),
		trackList:  initList("Tracks", true),
		styles:     styles,
		flash:      flash,
		loading:    true,
	}
}

func (m *Model) SetClient(client *jellyfin.Client, libraryID string) {
	m.client = client
	m.libraryID = libraryID
	if libraryID != "" {
		m.libraryIDs = []string{libraryID}
	} else {
		m.libraryIDs = nil
	}
}

func (m *Model) SetLibraries(client *jellyfin.Client, libraries []jellyfin.Library) {
	m.client = client
	m.libraryIDs = m.libraryIDs[:0]
	for _, lib := range libraries {
		if lib.ID != "" {
			m.libraryIDs = append(m.libraryIDs, lib.ID)
		}
	}
	if len(m.libraryIDs) > 0 {
		m.libraryID = m.libraryIDs[0]
	} else {
		m.libraryID = ""
	}
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.artistList.SetSize(width, height)
	m.albumList.SetSize(width, height)
	m.trackList.SetSize(width, height)
}

func (m Model) Init() tea.Cmd {
	if m.client != nil && len(m.musicLibraryIDs()) > 0 && len(m.albums) == 0 {
		return m.fetchAllAlbumsCmd()
	}
	return nil
}

func (m Model) musicLibraryIDs() []string {
	if len(m.libraryIDs) > 0 {
		return m.libraryIDs
	}
	if m.libraryID != "" {
		return []string{m.libraryID}
	}
	return nil
}

func (m Model) fetchAllAlbumsCmd() tea.Cmd {
	client := m.client
	libIDs := append([]string(nil), m.musicLibraryIDs()...)
	return func() tea.Msg {
		var all []jellyfin.Album
		for _, libID := range libIDs {
			logger.Info("fetching all albums", "libraryID", libID)
			albums, err := client.GetAllAlbums(context.Background(), libID)
			if err != nil {
				return AlbumsLoadedMsg{Err: err}
			}
			all = append(all, albums...)
		}
		return AlbumsLoadedMsg{Albums: all}
	}
}

func (m Model) fetchAlbumsCmd(artistID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		logger.Info("fetching albums", "artistID", artistID)
		albums, err := client.GetAlbums(context.Background(), artistID)
		return AlbumsLoadedMsg{Albums: albums, Err: err}
	}
}

func (m Model) fetchTracksCmd(albumID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		logger.Info("fetching tracks", "albumID", albumID)
		tracks, err := client.GetTracks(context.Background(), albumID)
		return TracksLoadedMsg{AlbumID: albumID, Tracks: tracks, Err: err}
	}
}

func (m Model) fetchAlbumTracksForQueueCmd(albumID string) tea.Cmd {
	client := m.client
	confirmation := QueueConfirmation{Origin: QueueOriginLibrary, ItemIDs: []string{albumID}}
	return func() tea.Msg {
		logger.Info("fetching tracks for queue", "albumID", albumID)
		if client == nil {
			return AddTracksToQueueMsg{Err: errors.New("cannot queue album without a Jellyfin client")}
		}
		tracks, err := client.GetTracks(context.Background(), albumID)
		if err != nil {
			return AddTracksToQueueMsg{Err: fmt.Errorf("queue album: %w", err)}
		}
		if len(tracks) == 0 {
			return AddTracksToQueueMsg{Err: errors.New("album has no tracks to queue")}
		}
		return AddTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
	}
}

func (m Model) fetchAlbumTracksForShuffledQueueCmd(albumID string) tea.Cmd {
	client := m.client
	confirmation := QueueConfirmation{Origin: QueueOriginLibrary, ItemIDs: []string{albumID}}
	return func() tea.Msg {
		logger.Info("fetching shuffled tracks for queue", "albumID", albumID)
		if client == nil {
			return AddShuffledTracksToQueueMsg{Err: errors.New("cannot queue album without a Jellyfin client")}
		}
		tracks, err := client.GetTracks(context.Background(), albumID)
		if err != nil {
			return AddShuffledTracksToQueueMsg{Err: fmt.Errorf("queue shuffled album: %w", err)}
		}
		if len(tracks) == 0 {
			return AddShuffledTracksToQueueMsg{Err: errors.New("album has no tracks to queue")}
		}
		return AddShuffledTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
	}
}

func (m Model) FetchAllTracksCmd() tea.Cmd {
	client := m.client
	libIDs := append([]string(nil), m.musicLibraryIDs()...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		var all []jellyfin.Track
		for _, libID := range libIDs {
			logger.Info("fetching all library tracks", "libraryID", libID)
			tracks, err := client.GetAllTracks(ctx, libID)
			if err != nil {
				return AllTracksLoadedMsg{Err: err}
			}
			all = append(all, tracks...)
		}
		return AllTracksLoadedMsg{Tracks: all}
	}
}

// RefreshCmd re-fetches the current level items from Jellyfin.
func (m *Model) RefreshCmd() tea.Cmd {
	m.loading = true
	if m.allTracks != nil {
		return m.FetchAllTracksCmd()
	}
	switch m.level {
	case LevelAlbums:
		m.albumList.SetItems(components.BuildSkeletonRows(m.styles))
		return m.fetchAllAlbumsCmd()
	case LevelTracks:
		if m.selectedAlbum.ID != "" {
			m.trackList.SetItems(components.BuildSkeletonRows(m.styles))
			return m.fetchTracksCmd(m.selectedAlbum.ID)
		}
		return m.FetchAllTracksCmd()
	}
	return nil
}

// PatchTrack updates a single track in all in-memory lists without any network call.
func (m *Model) PatchTrack(id, name string, artists []string, album string) {
	patch := func(t *jellyfin.Track) {
		if name != "" {
			t.Name = name
		}
		if artists != nil {
			t.Artists = artists
		}
		if album != "" {
			t.Album = album
		}
	}
	selectedID := ""
	if selected, ok := m.trackList.SelectedItem().(trackItem); ok {
		selectedID = selected.Track.ID
	}
	for i := range m.trackSource {
		if m.trackSource[i].ID == id {
			patch(&m.trackSource[i])
			m.rebuildTrackList(selectedID)
			break
		}
	}
	for i := range m.allTracks {
		if m.allTracks[i].ID == id {
			patch(&m.allTracks[i])
		}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ItemFlashExpiredMsg:
		m.flash.HandleExpired(msg)
		return m, nil

	case RefilterMsg:
		return m.Update(msg.Msg)

	case AlbumsLoadedMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.albums = append([]jellyfin.Album(nil), msg.Albums...)
			cmd := m.rebuildAlbumList("")
			m.level = LevelAlbums
			return m, cmd
		}
		return m, nil

	case TracksLoadedMsg:
		if msg.AlbumID != "" && msg.AlbumID != m.selectedAlbum.ID {
			return m, nil
		}
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.trackSource = append([]jellyfin.Track(nil), msg.Tracks...)
			cmd := m.rebuildTrackList("")
			m.level = LevelTracks
			return m, cmd
		}
		m.level = LevelAlbums
		return m, nil

	case AllTracksLoadedMsg:
		if msg.Err == nil {
			m.allTracks = append([]jellyfin.Track(nil), msg.Tracks...)
			slices.SortFunc(m.allTracks, func(a, b jellyfin.Track) int {
				c := libraryCollator.CompareString(a.Name, b.Name)
				if c != 0 {
					return c
				}
				if a.ParentIndexNumber != b.ParentIndexNumber {
					return a.ParentIndexNumber - b.ParentIndexNumber
				}
				if a.IndexNumber != b.IndexNumber {
					return a.IndexNumber - b.IndexNumber
				}
				return strings.Compare(a.ID, b.ID)
			})
			m.allTracksErr = nil
		} else {
			m.allTracksErr = msg.Err
		}
		return m, nil

	case tea.KeyMsg:
		if m.activeList().FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "m":
			if m.level == LevelAlbums {
				if sel, ok := m.albumList.SelectedItem().(albumItem); ok {
					alb := sel.Album
					return m, func() tea.Msg {
						return EditMetadataMsg{ItemID: alb.ID, ItemType: "Album", Album: &alb}
					}
				}
			} else if m.level == LevelTracks {
				if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
					trk := sel.Track
					return m, func() tea.Msg {
						return EditMetadataMsg{ItemID: trk.ID, ItemType: "Track", Track: &trk}
					}
				}
			}
		case "r":
			if m.allTracksErr != nil {
				m.allTracksErr = nil
				return m, m.FetchAllTracksCmd()
			}
		case "t":
			if m.level == LevelTracks {
				selectedID := ""
				if selected, ok := m.trackList.SelectedItem().(trackItem); ok {
					selectedID = selected.Track.ID
				}
				m.trackSort = (m.trackSort + 1) % 2
				return m, m.rebuildTrackList(selectedID)
			}
		case "f":
			if m.client != nil {
				switch m.level {
				case LevelAlbums:
					if sel, ok := m.albumList.SelectedItem().(albumItem); ok {
						client := m.client
						album := sel.Album
						return m, func() tea.Msg {
							userData, err := client.SetFavorite(context.Background(), album.ID, !album.UserData.IsFavorite)
							return AlbumFavoriteChangedMsg{AlbumID: album.ID, IsFavorite: userData.IsFavorite, Err: err}
						}
					}
				case LevelTracks:
					if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
						client := m.client
						track := sel.Track
						return m, func() tea.Msg {
							userData, err := client.SetFavorite(context.Background(), track.ID, !track.UserData.IsFavorite)
							return FavoriteChangedMsg{TrackID: track.ID, IsFavorite: userData.IsFavorite, Err: err}
						}
					}
				}
			}
		case "esc", "left":
			if m.HasActiveFilter() {
				if al := m.activeList(); al != nil {
					al.ResetFilter()
				}
				return m, nil
			}
			if m.level == LevelTracks {
				m.level = LevelAlbums
				return m, nil
			}
			// If already at Albums level, parent handles esc/back

		case "enter", "right":
			switch m.level {
			case LevelAlbums:
				if sel, ok := m.albumList.SelectedItem().(albumItem); ok {
					return m, m.SelectAlbum(sel.Album)
				}
			case LevelTracks:
				if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
					track := sel.Track
					m.trackList.ResetFilter()
					confirmation := QueueConfirmation{
						Origin:  QueueOriginLibrary,
						ItemIDs: []string{track.ID},
					}
					return m, func() tea.Msg {
						return PlayTracksMsg{
							Tracks:       []jellyfin.Track{track},
							StartIndex:   0,
							Confirmation: confirmation,
						}
					}
				}
			}
		case "a", "A", "S":
			if m.level == LevelAlbums {
				if sel, ok := m.albumList.SelectedItem().(albumItem); ok {
					if msg.String() == "S" {
						return m, m.fetchAlbumTracksForShuffledQueueCmd(sel.Album.ID)
					}
					return m, m.fetchAlbumTracksForQueueCmd(sel.Album.ID)
				}
			}
			if msg.String() == "a" && m.level == LevelTracks {
				if sel, ok := m.trackList.SelectedItem().(trackItem); ok {
					track := sel.Track
					confirmation := QueueConfirmation{
						Origin:  QueueOriginLibrary,
						ItemIDs: []string{track.ID},
					}
					return m, func() tea.Msg {
						return AddTrackToQueueMsg{Track: track, Confirmation: confirmation}
					}
				}
			}
			if (msg.String() == "A" || msg.String() == "S") && m.level == LevelTracks && len(m.tracks) > 0 {
				tracks := m.tracks
				confirmation := QueueConfirmation{
					Origin:  QueueOriginLibrary,
					ItemIDs: components.VisibleFlashIDs(m.trackList),
				}
				return m, func() tea.Msg {
					if msg.String() == "S" {
						return AddShuffledTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
					}
					return AddTracksToQueueMsg{Tracks: tracks, Confirmation: confirmation}
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
		m.albumList, cmd = m.albumList.Update(msg)
	}
	return m, cmd
}

func (m *Model) rebuildAlbumList(selectedID string) tea.Cmd {
	slices.SortFunc(m.albums, func(a, b jellyfin.Album) int {
		if a.UserData.IsFavorite != b.UserData.IsFavorite {
			if a.UserData.IsFavorite {
				return -1
			}
			return 1
		}
		c := libraryCollator.CompareString(a.Name, b.Name)
		if c != 0 {
			return c
		}
		if a.ProductionYear != b.ProductionYear {
			return a.ProductionYear - b.ProductionYear
		}
		return strings.Compare(a.ID, b.ID)
	})
	items := make([]list.Item, len(m.albums))
	for i, album := range m.albums {
		items[i] = albumItem{Album: album}
	}
	m.albumList.Title = "Albums"
	cmd := m.albumList.SetItems(items)
	if selectedID == "" {
		m.albumList.ResetSelected()
	}
	for i, album := range m.albums {
		if album.ID == selectedID {
			m.albumList.Select(i)
			break
		}
	}
	return cmd
}

func (m *Model) rebuildTrackList(selectedID string) tea.Cmd {
	m.tracks = append(m.tracks[:0], m.trackSource...)
	switch m.trackSort {
	case TrackSortTitle:
		slices.SortStableFunc(m.tracks, func(a, b jellyfin.Track) int {
			return libraryCollator.CompareString(a.Name, b.Name)
		})
	default:
		jellyfin.SortAlbumTracks(m.tracks)
	}
	jellyfin.SortFavoritesFirst(m.tracks)

	items := make([]list.Item, len(m.tracks))
	for i, track := range m.tracks {
		items[i] = trackItem{Track: track}
	}
	m.trackList.Title = fmt.Sprintf("Tracks — %s [%s]", m.selectedAlbum.Name, m.trackSort)
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

func (s TrackSort) String() string {
	if s == TrackSortTitle {
		return "title"
	}
	return "album"
}

func (m *Model) activeList() *list.Model {
	switch m.level {
	case LevelTracks:
		return &m.trackList
	default:
		return &m.albumList
	}
}

func (m Model) CurrentLevel() Level {
	return m.level
}

func (m Model) Loading() bool {
	return m.loading
}

func (m Model) Error() error {
	return m.err
}

func (m Model) Artists() []jellyfin.Artist {
	return m.artists
}

func (m Model) Albums() []jellyfin.Album {
	return m.albums
}

func (m Model) Tracks() []jellyfin.Track {
	return m.tracks
}

func (m Model) SelectedAlbum() (jellyfin.Album, bool) {
	if sel, ok := m.albumList.SelectedItem().(albumItem); ok {
		return sel.Album, true
	}
	return jellyfin.Album{}, false
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

func (m Model) AllTracks() []jellyfin.Track {
	return m.allTracks
}

func (m *Model) SetAllTracks(tracks []jellyfin.Track) {
	m.allTracks = tracks
}

// PatchTrackFavorite updates favorite state in all library track caches.
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
	for i := range m.allTracks {
		if m.allTracks[i].ID == id {
			m.allTracks[i].UserData.IsFavorite = favorite
		}
	}
	return libraryRefilterCmd(m.rebuildTrackList(selectedID))
}

// PatchAlbumFavorite updates favorite state and rebuilds the album list.
func (m *Model) PatchAlbumFavorite(id string, favorite bool) tea.Cmd {
	selectedID := ""
	if selected, ok := m.albumList.SelectedItem().(albumItem); ok {
		selectedID = selected.Album.ID
	}
	for i := range m.albums {
		if m.albums[i].ID == id {
			m.albums[i].UserData.IsFavorite = favorite
		}
	}
	if m.selectedAlbum.ID == id {
		m.selectedAlbum.UserData.IsFavorite = favorite
	}
	return libraryRefilterCmd(m.rebuildAlbumList(selectedID))
}

func libraryRefilterCmd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		return RefilterMsg{Msg: cmd()}
	}
}

func (m Model) IsFiltering() bool {
	return m.activeList().FilterState() == list.Filtering
}

func (m Model) HasActiveFilter() bool {
	return m.activeList().FilterValue() != "" || m.activeList().FilterState() == list.FilterApplied
}

func (m *Model) SelectAlbum(album jellyfin.Album) tea.Cmd {
	m.selectedAlbum = album
	m.loading = true
	m.level = LevelTracks
	m.trackSource = nil
	m.tracks = nil
	m.trackList.Title = fmt.Sprintf("Tracks — %s", album.Name)
	m.trackList.SetItems(components.BuildSkeletonRows(m.styles))
	return m.fetchTracksCmd(album.ID)
}
