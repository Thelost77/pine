package playlists

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPlaylistsModel_NavigationAndQueueActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users/usr/Items":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Playlist `json:"Items"`
				TotalRecordCount int                 `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Playlist{{ID: "pl-1", Name: "Road Trip", Count: 2}},
				TotalRecordCount: 1,
			})
		case "/Playlists/pl-1/Items":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Track `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items: []jellyfin.Track{
					{ID: "t-1", Name: "One"},
					{ID: "t-2", Name: "Two"},
				},
				TotalRecordCount: 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := New(ui.DefaultStyles())
	m.SetSize(80, 24)
	m.SetClient(jellyfin.NewClient(server.URL, "tok", "usr", "Test device", "test-device"))

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected playlists init cmd")
	}
	m, _ = m.Update(cmd())
	if len(m.Playlists()) != 1 || m.Playlists()[0].Name != "Road Trip" {
		t.Fatalf("unexpected playlists: %+v", m.Playlists())
	}

	_, queueCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	queuedPlaylist := queueCmd().(library.AddTracksToQueueMsg)
	if queuedPlaylist.Confirmation.Origin != library.QueueOriginPlaylists || !slices.Equal(queuedPlaylist.Confirmation.ItemIDs, []string{"pl-1"}) {
		t.Fatalf("playlist confirmation = %+v, want playlist source row", queuedPlaylist.Confirmation)
	}

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected playlist tracks cmd")
	}
	m, _ = m.Update(cmd())
	if m.CurrentLevel() != LevelTracks || len(m.Tracks()) != 2 {
		t.Fatalf("expected tracks level with 2 tracks, got level=%v tracks=%+v", m.CurrentLevel(), m.Tracks())
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	playOne, ok := cmd().(library.PlayTracksMsg)
	if !ok || len(playOne.Tracks) != 1 || playOne.Tracks[0].ID != "t-1" {
		t.Fatalf("enter returned %#v, want PlayTracksMsg t-1", playOne)
	}
	if playOne.Confirmation.Origin != library.QueueOriginPlaylists || !slices.Equal(playOne.Confirmation.ItemIDs, []string{"t-1"}) {
		t.Fatalf("enter confirmation = %+v, want playlist track t-1", playOne.Confirmation)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg := cmd()
	addOne, ok := msg.(library.AddTrackToQueueMsg)
	if !ok || addOne.Track.ID != "t-1" {
		t.Fatalf("a returned %T %+v, want AddTrackToQueueMsg t-1", msg, addOne)
	}
	if addOne.Confirmation.Origin != library.QueueOriginPlaylists || !slices.Equal(addOne.Confirmation.ItemIDs, []string{"t-1"}) {
		t.Fatalf("a confirmation = %+v, want playlist track t-1", addOne.Confirmation)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	msg = cmd()
	addAll, ok := msg.(library.AddTracksToQueueMsg)
	if !ok || len(addAll.Tracks) != 2 {
		t.Fatalf("A returned %T %+v, want AddTracksToQueueMsg with 2 tracks", msg, addAll)
	}
	if addAll.Confirmation.Origin != library.QueueOriginPlaylists || !slices.Equal(addAll.Confirmation.ItemIDs, []string{"t-1", "t-2"}) {
		t.Fatalf("A confirmation = %+v, want both visible playlist tracks", addAll.Confirmation)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	msg = cmd()
	addShuffled, ok := msg.(library.AddShuffledTracksToQueueMsg)
	if !ok || len(addShuffled.Tracks) != 2 {
		t.Fatalf("S returned %T %+v, want AddShuffledTracksToQueueMsg with 2 tracks", msg, addShuffled)
	}
	if addShuffled.Confirmation.Origin != library.QueueOriginPlaylists || !slices.Equal(addShuffled.Confirmation.ItemIDs, []string{"t-1", "t-2"}) {
		t.Fatalf("S confirmation = %+v, want both visible playlist tracks", addShuffled.Confirmation)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.CurrentLevel() != LevelPlaylists {
		t.Fatalf("Esc should return to playlist level, got %v", m.CurrentLevel())
	}
}

func TestPlaylistsModel_FavoritesStayFirstInPlaylistOrder(t *testing.T) {
	m := New(ui.DefaultStyles())
	if slices.Contains(m.playlistList.KeyMap.NextPage.Keys(), "f") || slices.Contains(m.trackList.KeyMap.NextPage.Keys(), "f") {
		t.Fatal("f remains bound to next page")
	}
	m.selectedPlaylist = jellyfin.Playlist{Name: "Road Trip"}
	m, _ = m.Update(PlaylistTracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Bravo", UserData: jellyfin.UserItemData{IsFavorite: true}},
		{ID: "c", Name: "Charlie"},
		{ID: "d", Name: "Delta", UserData: jellyfin.UserItemData{IsFavorite: true}},
	}})

	if got := playlistTrackNames(m.Tracks()); !slices.Equal(got, []string{"Bravo", "Delta", "Alpha", "Charlie"}) {
		t.Fatalf("playlist order = %v", got)
	}
	first := m.trackList.Items()[0].(trackItem)
	if first.Title() != "♥ Bravo" {
		t.Fatalf("favorite title = %q", first.Title())
	}

	m.PatchTrackFavorite("b", false)
	if got := playlistTrackNames(m.Tracks()); !slices.Equal(got, []string{"Delta", "Alpha", "Bravo", "Charlie"}) {
		t.Fatalf("order after unmark = %v", got)
	}
	m.PatchTrackFavorite("c", true)
	if got := playlistTrackNames(m.Tracks()); !slices.Equal(got, []string{"Charlie", "Delta", "Alpha", "Bravo"}) {
		t.Fatalf("order after mark = %v", got)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	queued := cmd().(library.AddTracksToQueueMsg)
	if got := playlistTrackNames(queued.Tracks); !slices.Equal(got, []string{"Charlie", "Delta", "Alpha", "Bravo"}) {
		t.Fatalf("queued order = %v", got)
	}
}

func TestPlaylistsModel_TrackFavoritePreservesActiveFilter(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(PlaylistTracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "a-1", Name: "Alpha"},
		{ID: "b-1", Name: "Bravo"},
	}})
	m = applyPlaylistKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = applyPlaylistKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Alpha")})
	m = applyPlaylistKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := len(m.trackList.VisibleItems()); got != 1 {
		t.Fatalf("visible tracks before favorite = %d, want 1", got)
	}

	m = applyPlaylistCmd(m, m.PatchTrackFavorite("a-1", true))
	if got := len(m.trackList.VisibleItems()); got != 1 {
		t.Fatalf("visible tracks after favorite = %d, want 1", got)
	}
}

func applyPlaylistKey(m Model, key tea.KeyMsg) Model {
	var cmd tea.Cmd
	m, cmd = m.Update(key)
	return applyPlaylistCmd(m, cmd)
}

func applyPlaylistCmd(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, batchCmd := range msg {
			m = applyPlaylistCmd(m, batchCmd)
		}
	case list.FilterMatchesMsg:
		m, _ = m.Update(msg)
	case RefilterMsg:
		var next tea.Cmd
		m, next = m.Update(msg)
		m = applyPlaylistCmd(m, next)
	}
	return m
}

func playlistTrackNames(tracks []jellyfin.Track) []string {
	names := make([]string, len(tracks))
	for i, track := range tracks {
		names[i] = track.Name
	}
	return names
}
