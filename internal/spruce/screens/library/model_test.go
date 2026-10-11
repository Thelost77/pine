package library

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestLibraryModel_NavigationAndPlayback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemTypes := r.URL.Query().Get("IncludeItemTypes")
		parentID := r.URL.Query().Get("ParentId")

		switch {
		case itemTypes == "MusicAlbum" && parentID == "lib-1":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Album `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Album{{ID: "alb-1", Name: "OK Computer", ProductionYear: 1997, Artists: []string{"Radiohead"}}},
				TotalRecordCount: 1,
			})
		case itemTypes == "Audio" && parentID == "alb-1":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Track `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items: []jellyfin.Track{
					{ID: "t-1", Name: "Airbag", IndexNumber: 1, RunTimeTicks: 2840000000},
					{ID: "t-2", Name: "Paranoid Android", IndexNumber: 2, RunTimeTicks: 3870000000},
				},
				TotalRecordCount: 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := jellyfin.NewClient(server.URL, "tok", "usr", "Test device", "test-device-1")
	m := New(ui.DefaultStyles())
	m.SetSize(80, 24)
	m.SetClient(client, "lib-1")

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected init cmd")
	}
	msg := cmd()
	m, _ = m.Update(msg)
	if m.CurrentLevel() != LevelAlbums {
		t.Fatalf("expected LevelAlbums, got %v", m.CurrentLevel())
	}
	if len(m.albumList.Items()) != 1 {
		t.Fatalf("expected 1 album item, got %d", len(m.albumList.Items()))
	}

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected fetch tracks cmd")
	}
	msg = cmd()
	m, _ = m.Update(msg)
	if m.CurrentLevel() != LevelTracks {
		t.Fatalf("expected LevelTracks, got %v", m.CurrentLevel())
	}
	if len(m.trackList.Items()) != 2 {
		t.Fatalf("expected 2 track items, got %d", len(m.trackList.Items()))
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected play tracks cmd")
	}
	playMsg := cmd()
	ptm, ok := playMsg.(PlayTracksMsg)
	if !ok {
		t.Fatalf("expected PlayTracksMsg, got %T", playMsg)
	}
	if len(ptm.Tracks) != 1 || ptm.StartIndex != 0 {
		t.Errorf("unexpected PlayTracksMsg: %+v", ptm)
	}
	if ptm.Confirmation.Origin != QueueOriginLibrary || !slices.Equal(ptm.Confirmation.ItemIDs, []string{"t-1"}) {
		t.Fatalf("enter confirmation = %+v, want library track t-1", ptm.Confirmation)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.CurrentLevel() != LevelAlbums {
		t.Errorf("expected back to LevelAlbums, got %v", m.CurrentLevel())
	}

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}

func TestLibraryModel_Actions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemTypes := r.URL.Query().Get("IncludeItemTypes")
		parentID := r.URL.Query().Get("ParentId")
		switch {
		case itemTypes == "MusicAlbum" && parentID == "lib-1":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Album `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Album{{ID: "alb-1", Name: "OK Computer"}},
				TotalRecordCount: 1,
			})
		case itemTypes == "Audio" && parentID == "alb-1":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Track `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items: []jellyfin.Track{
					{ID: "t-1", Name: "Airbag", IndexNumber: 1},
					{ID: "t-2", Name: "Paranoid Android", IndexNumber: 2},
				},
				TotalRecordCount: 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := jellyfin.NewClient(server.URL, "tok", "usr", "Test device", "test-device-2")
	m := New(ui.DefaultStyles())
	m.SetSize(80, 24)
	m.SetClient(client, "lib-1")

	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{{ID: "alb-1", Name: "OK Computer"}}})
	if m.CurrentLevel() != LevelAlbums {
		t.Fatalf("expected LevelAlbums, got %v", m.CurrentLevel())
	}

	// Test 'a' at LevelAlbums
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected cmd on 'a' at LevelAlbums")
	}
	addMsg, ok := cmd().(AddTracksToQueueMsg)
	if !ok || len(addMsg.Tracks) != 2 {
		t.Fatalf("expected AddTracksToQueueMsg with 2 tracks, got %T %+v", cmd(), addMsg)
	}
	if addMsg.Confirmation.Origin != QueueOriginLibrary || !slices.Equal(addMsg.Confirmation.ItemIDs, []string{"alb-1"}) {
		t.Fatalf("album confirmation = %+v, want album source row", addMsg.Confirmation)
	}

	// Move to LevelTracks
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{{ID: "t-1", Name: "Airbag"}, {ID: "t-2", Name: "Paranoid Android"}}})
	if m.CurrentLevel() != LevelTracks {
		t.Fatalf("expected LevelTracks, got %v", m.CurrentLevel())
	}

	// Test 'a' at LevelTracks
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected cmd on 'a' at LevelTracks")
	}
	addOne, ok := cmd().(AddTrackToQueueMsg)
	if !ok || addOne.Track.ID != "t-1" {
		t.Fatalf("expected AddTrackToQueueMsg for t-1, got %T %+v", cmd(), addOne)
	}

	// Test 'A' at LevelTracks
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if cmd == nil {
		t.Fatal("expected cmd on 'A' at LevelTracks")
	}
	addAll, ok := cmd().(AddTracksToQueueMsg)
	if !ok || len(addAll.Tracks) != 2 {
		t.Fatalf("expected AddTracksToQueueMsg with 2 tracks, got %T %+v", cmd(), addAll)
	}

	// Test Esc resets filter when active
	if m.HasActiveFilter() {
		t.Fatal("expected no active filter initially")
	}
}

func TestLibraryModel_SortsLibraryItemsAlphabetically(t *testing.T) {
	m := New(ui.DefaultStyles())

	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{
		{ID: "z", Name: "Zebra"},
		{ID: "s2", Name: "Świt"},
		{ID: "a", Name: "alpha"},
		{ID: "b", Name: "Beta"},
		{ID: "s1", Name: "Siano"},
	}})

	gotAlbums := m.Albums()
	if got := names(gotAlbums); got[0] != "alpha" || got[1] != "Beta" || got[2] != "Siano" || got[3] != "Świt" || got[4] != "Zebra" {
		t.Fatalf("albums sorted = %v, want [alpha Beta Siano Świt Zebra]", got)
	}
	firstItem, ok := m.albumList.Items()[0].(albumItem)
	if !ok || firstItem.Album.Name != "alpha" {
		t.Fatalf("first visible album = %#v, want alpha", m.albumList.Items()[0])
	}

	m, _ = m.Update(AllTracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "z", Name: "Zebra"},
		{ID: "s2", Name: "Świt"},
		{ID: "a", Name: "alpha"},
		{ID: "b", Name: "Beta"},
		{ID: "s1", Name: "Siano"},
	}})

	gotTracks := m.AllTracks()
	if got := trackNames(gotTracks); got[0] != "alpha" || got[1] != "Beta" || got[2] != "Siano" || got[3] != "Świt" || got[4] != "Zebra" {
		t.Fatalf("all tracks sorted = %v, want [alpha Beta Siano Świt Zebra]", got)
	}

	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "t-12", Name: "Cemetery Drive", ParentIndexNumber: 1, IndexNumber: 12},
		{ID: "t-2", Name: "Give 'Em Hell, Kid", ParentIndexNumber: 1, IndexNumber: 2},
		{ID: "t-10", Name: "Hang 'Em High", ParentIndexNumber: 1, IndexNumber: 10},
		{ID: "t-1", Name: "Helena", ParentIndexNumber: 1, IndexNumber: 1},
	}})

	gotAlbumTracks := m.Tracks()
	if got := trackNames(gotAlbumTracks); got[0] != "Helena" || got[1] != "Give 'Em Hell, Kid" || got[2] != "Hang 'Em High" || got[3] != "Cemetery Drive" {
		t.Fatalf("album tracks sorted = %v, want [Helena Give 'Em Hell, Kid Hang 'Em High Cemetery Drive]", got)
	}
	firstTrackItem, ok := m.trackList.Items()[0].(trackItem)
	if !ok || firstTrackItem.Track.Name != "Helena" {
		t.Fatalf("first visible track = %#v, want Helena", m.trackList.Items()[0])
	}
}

func TestLibraryModel_TogglesAlbumTrackSort(t *testing.T) {
	m := New(ui.DefaultStyles())
	m.selectedAlbum = jellyfin.Album{Name: "Three Cheers for Sweet Revenge"}
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "t-12", Name: "Cemetery Drive", ParentIndexNumber: 1, IndexNumber: 12},
		{ID: "t-2", Name: "Give 'Em Hell, Kid", ParentIndexNumber: 1, IndexNumber: 2},
		{ID: "t-10", Name: "Hang 'Em High", ParentIndexNumber: 1, IndexNumber: 10},
		{ID: "t-1", Name: "Helena", ParentIndexNumber: 1, IndexNumber: 1},
	}})

	m.trackList.Select(0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Cemetery Drive", "Give 'Em Hell, Kid", "Hang 'Em High", "Helena"}) {
		t.Fatalf("title order = %v", got)
	}
	if m.trackList.Title != "Tracks — Three Cheers for Sweet Revenge [title]" {
		t.Fatalf("title = %q", m.trackList.Title)
	}
	if selected, ok := m.SelectedTrack(); !ok || selected.ID != "t-1" {
		t.Fatalf("selected track = %+v, %v; want Helena", selected, ok)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	queued, ok := cmd().(AddTracksToQueueMsg)
	if !ok || !slices.Equal(trackNames(queued.Tracks), []string{"Cemetery Drive", "Give 'Em Hell, Kid", "Hang 'Em High", "Helena"}) {
		t.Fatalf("queued tracks = %#v", queued)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Helena", "Give 'Em Hell, Kid", "Hang 'Em High", "Cemetery Drive"}) {
		t.Fatalf("album order = %v", got)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if m.trackList.FilterValue() != "t" {
		t.Fatalf("filter value = %q, want t", m.trackList.FilterValue())
	}
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Helena", "Give 'Em Hell, Kid", "Hang 'Em High", "Cemetery Drive"}) {
		t.Fatalf("tracks changed while filtering = %v", got)
	}
}

func TestLibraryModel_FavoritesStayFirstWithinActiveSort(t *testing.T) {
	m := New(ui.DefaultStyles())
	m.selectedAlbum = jellyfin.Album{Name: "Album"}
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "t-3", Name: "Charlie", IndexNumber: 3},
		{ID: "t-1", Name: "Alpha", IndexNumber: 1},
		{ID: "t-4", Name: "Delta", IndexNumber: 4, UserData: jellyfin.UserItemData{IsFavorite: true}},
		{ID: "t-2", Name: "Bravo", IndexNumber: 2, UserData: jellyfin.UserItemData{IsFavorite: true}},
	}})

	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Bravo", "Delta", "Alpha", "Charlie"}) {
		t.Fatalf("album order = %v", got)
	}
	first := m.trackList.Items()[0].(trackItem)
	if first.Title() != "♥ 2. Bravo" {
		t.Fatalf("favorite title = %q", first.Title())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Bravo", "Delta", "Alpha", "Charlie"}) {
		t.Fatalf("title order = %v", got)
	}

	m.PatchTrackFavorite("t-2", false)
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Delta", "Alpha", "Bravo", "Charlie"}) {
		t.Fatalf("order after unmark = %v", got)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	queued := cmd().(AddTracksToQueueMsg)
	if got := trackNames(queued.Tracks); !slices.Equal(got, []string{"Delta", "Alpha", "Bravo", "Charlie"}) {
		t.Fatalf("queued order = %v", got)
	}
}

func TestLibraryModel_FavoriteAlbumsStayFirst(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{
		{ID: "z", Name: "Zulu", UserData: jellyfin.UserItemData{IsFavorite: true}},
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Bravo", UserData: jellyfin.UserItemData{IsFavorite: true}},
		{ID: "c", Name: "Charlie"},
	}})

	if got := names(m.Albums()); !slices.Equal(got, []string{"Bravo", "Zulu", "Alpha", "Charlie"}) {
		t.Fatalf("album order = %v", got)
	}
	first := m.albumList.Items()[0].(albumItem)
	if first.Title() != "♥ Bravo" {
		t.Fatalf("favorite title = %q", first.Title())
	}

	m.albumList.Select(0)
	m.PatchAlbumFavorite("b", false)
	if got := names(m.Albums()); !slices.Equal(got, []string{"Zulu", "Alpha", "Bravo", "Charlie"}) {
		t.Fatalf("order after unmark = %v", got)
	}
	if selected, ok := m.SelectedAlbum(); !ok || selected.ID != "b" {
		t.Fatalf("selected album = %+v, %v", selected, ok)
	}
}

func TestLibraryModel_AlbumFavoriteKeyUsesJellyfin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/UserFavoriteItems/a-1" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(jellyfin.UserItemData{IsFavorite: true})
	}))
	defer server.Close()

	m := New(ui.DefaultStyles())
	m.client = jellyfin.NewClient(server.URL, "tok", "usr", "Test device", "test-device")
	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{{ID: "a-1", Name: "Album"}}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if cmd == nil {
		t.Fatal("favorite key returned nil command")
	}
	msg, ok := cmd().(AlbumFavoriteChangedMsg)
	if !ok || msg.Err != nil || msg.AlbumID != "a-1" || !msg.IsFavorite {
		t.Fatalf("favorite message = %#v", msg)
	}
	if slices.Contains(m.albumList.KeyMap.NextPage.Keys(), "f") {
		t.Fatal("f remains bound to next page")
	}
}

func TestLibraryModel_AlbumFavoritePreservesActiveFilter(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{
		{ID: "a-1", Name: "Alpha"},
		{ID: "b-1", Name: "Bravo"},
	}})
	m = applyLibraryKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = applyLibraryKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Alpha")})
	m = applyLibraryKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := len(m.albumList.VisibleItems()); got != 1 {
		t.Fatalf("visible albums before favorite = %d, want 1", got)
	}

	m = applyLibraryCmd(m, m.PatchAlbumFavorite("a-1", true))
	if got := len(m.albumList.VisibleItems()); got != 1 {
		t.Fatalf("visible albums after favorite = %d, want 1", got)
	}
}

func TestLibraryModel_TrackFavoritePreservesActiveFilter(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "a-1", Name: "Alpha"},
		{ID: "b-1", Name: "Bravo"},
	}})
	m = applyLibraryKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = applyLibraryKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Alpha")})
	m = applyLibraryKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := len(m.trackList.VisibleItems()); got != 1 {
		t.Fatalf("visible tracks before favorite = %d, want 1", got)
	}

	m = applyLibraryCmd(m, m.PatchTrackFavorite("a-1", true))
	if got := len(m.trackList.VisibleItems()); got != 1 {
		t.Fatalf("visible tracks after favorite = %d, want 1", got)
	}
}

func applyLibraryKey(m Model, key tea.KeyMsg) Model {
	var cmd tea.Cmd
	m, cmd = m.Update(key)
	return applyLibraryCmd(m, cmd)
}

func applyLibraryCmd(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, batchCmd := range msg {
			m = applyLibraryCmd(m, batchCmd)
		}
	case list.FilterMatchesMsg:
		m, _ = m.Update(msg)
	case RefilterMsg:
		var next tea.Cmd
		m, next = m.Update(msg)
		m = applyLibraryCmd(m, next)
	}
	return m
}

func TestLibraryModel_FavoriteKeyUsesJellyfin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/UserFavoriteItems/t-1" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(jellyfin.UserItemData{IsFavorite: true})
	}))
	defer server.Close()

	m := New(ui.DefaultStyles())
	m.client = jellyfin.NewClient(server.URL, "tok", "usr", "Test device", "test-device")
	m.selectedAlbum = jellyfin.Album{Name: "Album"}
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{{ID: "t-1", Name: "One"}}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if cmd == nil {
		t.Fatal("favorite key returned nil command")
	}
	msg, ok := cmd().(FavoriteChangedMsg)
	if !ok || msg.Err != nil || msg.TrackID != "t-1" || !msg.IsFavorite {
		t.Fatalf("favorite message = %#v", msg)
	}
}

func TestLibraryModel_RestoresSourceOrderForUnnumberedTracks(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "second", Name: "Second"},
		{ID: "first", Name: "First"},
		{ID: "third", Name: "Third"},
	}})

	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Second", "First", "Third"}) {
		t.Fatalf("source order = %v", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"First", "Second", "Third"}) {
		t.Fatalf("title order = %v", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if got := trackNames(m.Tracks()); !slices.Equal(got, []string{"Second", "First", "Third"}) {
		t.Fatalf("restored source order = %v", got)
	}
}

func TestLibraryModel_SelectsFirstTrackWhenOpeningShorterAlbum(t *testing.T) {
	m := New(ui.DefaultStyles())
	longAlbumTracks := make([]jellyfin.Track, 30)
	for i := range longAlbumTracks {
		longAlbumTracks[i] = jellyfin.Track{ID: fmt.Sprintf("long-%d", i), Name: fmt.Sprintf("Track %d", i), IndexNumber: i + 1}
	}
	m, _ = m.Update(TracksLoadedMsg{Tracks: longAlbumTracks})
	m.trackList.Select(29)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.SelectAlbum(jellyfin.Album{ID: "short", Name: "Short Album"})
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{
		{ID: "short-1", Name: "First", IndexNumber: 1},
		{ID: "short-2", Name: "Second", IndexNumber: 2},
		{ID: "short-3", Name: "Third", IndexNumber: 3},
		{ID: "short-4", Name: "Fourth", IndexNumber: 4},
		{ID: "short-5", Name: "Fifth", IndexNumber: 5},
	}})

	selected, ok := m.SelectedTrack()
	if !ok || selected.ID != "short-1" {
		t.Fatalf("selected track = %+v, %v; want first track", selected, ok)
	}
}

func TestLibraryModel_IgnoresStaleAlbumTracks(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(TracksLoadedMsg{Tracks: []jellyfin.Track{{ID: "old", Name: "Old"}}})
	m.SelectAlbum(jellyfin.Album{ID: "album-a", Name: "Album A"})
	m.SelectAlbum(jellyfin.Album{ID: "album-b", Name: "Album B"})

	if len(m.Tracks()) != 0 {
		t.Fatalf("tracks while loading = %v, want none", trackNames(m.Tracks()))
	}
	m, _ = m.Update(TracksLoadedMsg{
		AlbumID: "album-a",
		Tracks:  []jellyfin.Track{{ID: "a-1", Name: "Album A track"}},
	})
	if !m.loading || len(m.Tracks()) != 0 {
		t.Fatalf("stale result changed model: loading=%v tracks=%v", m.loading, trackNames(m.Tracks()))
	}

	m, _ = m.Update(TracksLoadedMsg{
		AlbumID: "album-b",
		Tracks:  []jellyfin.Track{{ID: "b-1", Name: "Album B track"}},
	})
	if m.loading || !slices.Equal(trackNames(m.Tracks()), []string{"Album B track"}) {
		t.Fatalf("current result not loaded: loading=%v tracks=%v", m.loading, trackNames(m.Tracks()))
	}
}

func TestLibraryModel_FetchesAcrossMusicLibraries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		itemTypes := r.URL.Query().Get("IncludeItemTypes")
		parentID := r.URL.Query().Get("ParentId")
		switch {
		case itemTypes == "MusicAlbum" && parentID == "lib-1":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Album `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Album{{ID: "alb-1", Name: "Alpha"}},
				TotalRecordCount: 1,
			})
		case itemTypes == "MusicAlbum" && parentID == "lib-2":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Album `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Album{{ID: "alb-2", Name: "Beta"}},
				TotalRecordCount: 1,
			})
		case itemTypes == "Audio" && parentID == "lib-1":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Track `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Track{{ID: "t-1", Name: "One"}},
				TotalRecordCount: 1,
			})
		case itemTypes == "Audio" && parentID == "lib-2":
			_ = json.NewEncoder(w).Encode(struct {
				Items            []jellyfin.Track `json:"Items"`
				TotalRecordCount int              `json:"TotalRecordCount"`
			}{
				Items:            []jellyfin.Track{{ID: "t-2", Name: "Two"}},
				TotalRecordCount: 1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := jellyfin.NewClient(server.URL, "tok", "usr", "Test device", "test-device-3")
	m := New(ui.DefaultStyles())
	m.SetLibraries(client, []jellyfin.Library{
		{ID: "lib-1", Name: "Main", CollectionType: "music"},
		{ID: "lib-2", Name: "Imported", CollectionType: "music"},
	})

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected init cmd")
	}
	m, _ = m.Update(cmd())
	if got := names(m.Albums()); len(got) != 2 || got[0] != "Alpha" || got[1] != "Beta" {
		t.Fatalf("albums = %v, want [Alpha Beta]", got)
	}

	cmd = m.FetchAllTracksCmd()
	m, _ = m.Update(cmd())
	if got := trackNames(m.AllTracks()); len(got) != 2 || got[0] != "One" || got[1] != "Two" {
		t.Fatalf("tracks = %v, want [One Two]", got)
	}
}

func TestLibraryModel_FlashItems(t *testing.T) {
	m := New(ui.DefaultStyles())
	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{
		{ID: "alb-1", Name: "OK Computer"},
	}})

	if cmd := m.FlashItems("alb-1"); cmd == nil {
		t.Fatal("expected flash expiry command")
	}
	if !m.IsFlashing("alb-1") {
		t.Fatal("expected album to flash after FlashItems")
	}
}

func TestLibraryModel_QueueKeysSetVisibleConfirmations(t *testing.T) {
	m := New(ui.DefaultStyles())
	m.SetSize(80, 24)
	m, _ = m.Update(AlbumsLoadedMsg{Albums: []jellyfin.Album{
		{ID: "alb-1", Name: "OK Computer"},
	}})
	m, _ = m.Update(TracksLoadedMsg{
		Tracks: []jellyfin.Track{
			{ID: "t-1", Name: "Airbag"},
			{ID: "t-2", Name: "Paranoid Android"},
		},
	})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	addOne := cmd().(AddTrackToQueueMsg)
	if addOne.Confirmation.Origin != QueueOriginLibrary || !slices.Equal(addOne.Confirmation.ItemIDs, []string{"t-1"}) {
		t.Fatalf("a confirmation = %+v, want library track t-1", addOne.Confirmation)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	addAll := cmd().(AddTracksToQueueMsg)
	if addAll.Confirmation.Origin != QueueOriginLibrary || !slices.Equal(addAll.Confirmation.ItemIDs, []string{"t-1", "t-2"}) {
		t.Fatalf("A confirmation = %+v, want both visible tracks", addAll.Confirmation)
	}

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	addShuffled := cmd().(AddShuffledTracksToQueueMsg)
	if addShuffled.Confirmation.Origin != QueueOriginLibrary || !slices.Equal(addShuffled.Confirmation.ItemIDs, []string{"t-1", "t-2"}) {
		t.Fatalf("S confirmation = %+v, want both visible tracks", addShuffled.Confirmation)
	}
}

func names(albums []jellyfin.Album) []string {
	out := make([]string, len(albums))
	for i, album := range albums {
		out[i] = album.Name
	}
	return out
}

func trackNames(tracks []jellyfin.Track) []string {
	out := make([]string, len(tracks))
	for i, track := range tracks {
		out[i] = track.Name
	}
	return out
}
