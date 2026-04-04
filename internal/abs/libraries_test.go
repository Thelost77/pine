package abs

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed testdata/libraries.json
var librariesFixture []byte

//go:embed testdata/personalized.json
var personalizedFixture []byte

//go:embed testdata/library_items.json
var libraryItemsFixture []byte

//go:embed testdata/search.json
var searchFixture []byte

// --- Deserialization tests (fixture-based) ---

func TestGetLibrariesDeserialization(t *testing.T) {
	var resp struct {
		Libraries []Library `json:"libraries"`
	}
	if err := json.Unmarshal(librariesFixture, &resp); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if len(resp.Libraries) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(resp.Libraries))
	}
	lib := resp.Libraries[0]
	if lib.ID != "lib-books-001" {
		t.Errorf("ID = %q, want %q", lib.ID, "lib-books-001")
	}
	if lib.Name != "Audiobooks" {
		t.Errorf("Name = %q, want %q", lib.Name, "Audiobooks")
	}
	if lib.MediaType != "book" {
		t.Errorf("MediaType = %q, want %q", lib.MediaType, "book")
	}
	lib2 := resp.Libraries[1]
	if lib2.ID != "lib-pods-002" {
		t.Errorf("ID = %q, want %q", lib2.ID, "lib-pods-002")
	}
	if lib2.MediaType != "podcast" {
		t.Errorf("MediaType = %q, want %q", lib2.MediaType, "podcast")
	}
}

func TestGetPersonalizedDeserialization(t *testing.T) {
	var sections []PersonalizedResponse
	if err := json.Unmarshal(personalizedFixture, &sections); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0].ID != "continue-listening" {
		t.Errorf("section ID = %q, want %q", sections[0].ID, "continue-listening")
	}
	if len(sections[0].Entities) != 1 {
		t.Fatalf("expected 1 entity in first section, got %d", len(sections[0].Entities))
	}
	item := sections[0].Entities[0]
	if item.ID != "li-001" {
		t.Errorf("entity ID = %q, want %q", item.ID, "li-001")
	}
	if item.Media.Metadata.Title != "The Great Adventure" {
		t.Errorf("title = %q, want %q", item.Media.Metadata.Title, "The Great Adventure")
	}
	if item.UserMediaProgress == nil {
		t.Fatal("expected UserMediaProgress to be set")
	}
	if item.UserMediaProgress.Progress != 0.45 {
		t.Errorf("progress = %f, want 0.45", item.UserMediaProgress.Progress)
	}
}

func TestGetLibraryItemsDeserialization(t *testing.T) {
	var resp LibraryItemsResponse
	if err := json.Unmarshal(libraryItemsFixture, &resp); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if resp.Total != 42 {
		t.Errorf("Total = %d, want 42", resp.Total)
	}
	if resp.Limit != 10 {
		t.Errorf("Limit = %d, want 10", resp.Limit)
	}
	if resp.Page != 0 {
		t.Errorf("Page = %d, want 0", resp.Page)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[0].ID != "li-001" {
		t.Errorf("first result ID = %q, want %q", resp.Results[0].ID, "li-001")
	}
	if resp.Results[1].Media.Metadata.Title != "Mystery Novel" {
		t.Errorf("second result title = %q, want %q", resp.Results[1].Media.Metadata.Title, "Mystery Novel")
	}
}

func TestSearchLibraryDeserialization(t *testing.T) {
	var resp SearchResult
	if err := json.Unmarshal(searchFixture, &resp); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if len(resp.Book) != 1 {
		t.Fatalf("expected 1 book result, got %d", len(resp.Book))
	}
	if resp.Book[0].LibraryItem.Media.Metadata.Title != "The Great Adventure" {
		t.Errorf("book title = %q, want %q", resp.Book[0].LibraryItem.Media.Metadata.Title, "The Great Adventure")
	}
	if len(resp.Podcast) != 1 {
		t.Fatalf("expected 1 podcast result, got %d", len(resp.Podcast))
	}
	if resp.Podcast[0].LibraryItem.MediaType != "podcast" {
		t.Errorf("podcast mediaType = %q, want %q", resp.Podcast[0].LibraryItem.MediaType, "podcast")
	}
}

// --- HTTP integration tests ---

func TestGetLibrariesHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries" {
			t.Errorf("path = %q, want /api/libraries", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(librariesFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs, err := c.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("GetLibraries() error: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(libs))
	}
	if libs[0].Name != "Audiobooks" {
		t.Errorf("first library name = %q, want %q", libs[0].Name, "Audiobooks")
	}
}

func TestGetPersonalizedHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-1/personalized" {
			t.Errorf("path = %q, want /api/libraries/lib-1/personalized", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(personalizedFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	sections, err := c.GetPersonalized(context.Background(), "lib-1")
	if err != nil {
		t.Fatalf("GetPersonalized() error: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
}

func TestGetLibraryItemsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-1/items" {
			t.Errorf("path = %q, want /api/libraries/lib-1/items", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("page") != "0" {
			t.Errorf("page = %q, want 0", q.Get("page"))
		}
		if q.Get("limit") != "10" {
			t.Errorf("limit = %q, want 10", q.Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(libraryItemsFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	resp, err := c.GetLibraryItems(context.Background(), "lib-1", 0, 10)
	if err != nil {
		t.Fatalf("GetLibraryItems() error: %v", err)
	}
	if resp.Total != 42 {
		t.Errorf("Total = %d, want 42", resp.Total)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
}

func TestGetLibraryItemHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/li-pod-001" {
			t.Errorf("path = %q, want /api/items/li-pod-001", r.URL.Path)
		}
		if r.URL.Query().Get("expanded") != "1" {
			t.Error("expected expanded=1 query param")
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "li-pod-001",
			"mediaType": "podcast",
			"media": {
				"metadata": {"title": "My Podcast"},
				"episodes": [
					{"id": "ep-001", "title": "Pilot", "duration": 1200.5},
					{"id": "ep-002", "title": "Part 2", "duration": 900.0}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	item, err := c.GetLibraryItem(context.Background(), "li-pod-001")
	if err != nil {
		t.Fatalf("GetLibraryItem() error: %v", err)
	}
	if item.ID != "li-pod-001" {
		t.Errorf("item ID = %q, want li-pod-001", item.ID)
	}
	if item.MediaType != "podcast" {
		t.Errorf("mediaType = %q, want podcast", item.MediaType)
	}
	if len(item.Media.Episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(item.Media.Episodes))
	}
	if item.Media.Episodes[0].ID != "ep-001" {
		t.Errorf("episode[0].ID = %q, want ep-001", item.Media.Episodes[0].ID)
	}
	if item.Media.Episodes[1].Duration != 900.0 {
		t.Errorf("episode[1].Duration = %f, want 900.0", item.Media.Episodes[1].Duration)
	}
}

func TestGetLibraryItemHTTP_WithSeriesMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/li-book-001" {
			t.Errorf("path = %q, want /api/items/li-book-001", r.URL.Path)
		}
		if r.URL.Query().Get("expanded") != "1" {
			t.Error("expected expanded=1 query param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "li-book-001",
			"libraryId": "lib-books-001",
			"addedAt": 1711111111111,
			"mediaType": "book",
			"media": {
				"metadata": {
					"title": "Caliban's War",
					"series": {
						"id": "series-expanse",
						"name": "The Expanse",
						"sequence": "2"
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	item, err := c.GetLibraryItem(context.Background(), "li-book-001")
	if err != nil {
		t.Fatalf("GetLibraryItem() error: %v", err)
	}
	if item.LibraryID != "lib-books-001" {
		t.Errorf("libraryID = %q, want lib-books-001", item.LibraryID)
	}
	if item.AddedAt != 1711111111111 {
		t.Errorf("addedAt = %d, want 1711111111111", item.AddedAt)
	}
	if item.Media.Metadata.Series == nil {
		t.Fatal("expected series metadata to be set")
	}
	if item.Media.Metadata.Series.ID != "series-expanse" {
		t.Errorf("series ID = %q, want series-expanse", item.Media.Metadata.Series.ID)
	}
	if item.Media.Metadata.Series.Sequence != "2" {
		t.Errorf("sequence = %q, want 2", item.Media.Metadata.Series.Sequence)
	}
}

func TestGetLibraryItemHTTP_WithSeriesMetadataArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/li-book-001" {
			t.Errorf("path = %q, want /api/items/li-book-001", r.URL.Path)
		}
		if r.URL.Query().Get("expanded") != "1" {
			t.Error("expected expanded=1 query param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "li-book-001",
			"libraryId": "lib-books-001",
			"addedAt": 1711111111111,
			"mediaType": "book",
			"media": {
				"metadata": {
					"title": "Caliban's War",
					"series": [
						{
							"id": "series-expanse",
							"name": "The Expanse",
							"sequence": "2"
						}
					]
				}
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	item, err := c.GetLibraryItem(context.Background(), "li-book-001")
	if err != nil {
		t.Fatalf("GetLibraryItem() error: %v", err)
	}
	if item.Media.Metadata.Series == nil {
		t.Fatal("expected series metadata to be set from array payload")
	}
	if item.Media.Metadata.Series.ID != "series-expanse" {
		t.Errorf("series ID = %q, want series-expanse", item.Media.Metadata.Series.ID)
	}
}

func TestGetSeriesHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-books-001/series/series-expanse" {
			t.Errorf("path = %q, want /api/libraries/lib-books-001/series/series-expanse", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "series-expanse",
			"name": "The Expanse",
			"books": [
				{
					"id": "li-book-001",
					"libraryId": "lib-books-001",
					"addedAt": 1711111111111,
					"mediaType": "book",
					"sequence": "2",
					"media": {
						"metadata": {
							"title": "Caliban's War"
						}
					}
				},
				{
					"id": "li-book-002",
					"libraryId": "lib-books-001",
					"addedAt": 1711111112222,
					"mediaType": "book",
					"sequence": "3",
					"media": {
						"metadata": {
							"title": "Abaddon's Gate"
						}
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	series, err := c.GetSeries(context.Background(), "lib-books-001", "series-expanse")
	if err != nil {
		t.Fatalf("GetSeries() error: %v", err)
	}
	if series.ID != "series-expanse" {
		t.Errorf("series ID = %q, want series-expanse", series.ID)
	}
	if len(series.Books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(series.Books))
	}
	if series.Books[0].Sequence != "2" {
		t.Errorf("book[0] sequence = %q, want 2", series.Books[0].Sequence)
	}
	if series.Books[1].LibraryItem.Media.Metadata.Title != "Abaddon's Gate" {
		t.Errorf("book[1] title = %q, want Abaddon's Gate", series.Books[1].LibraryItem.Media.Metadata.Title)
	}
}

func TestGetRecentlyAddedHTTP(t *testing.T) {
	responses := map[string]string{
		"/api/libraries/lib-books-001/personalized": `[
			{
				"id": "recently-added",
				"entities": [
					{
						"id": "li-book-001",
						"libraryId": "lib-books-001",
						"addedAt": 1711111111111,
						"mediaType": "book",
						"media": {"metadata": {"title": "Caliban's War"}}
					}
				]
			}
		]`,
		"/api/libraries/lib-pods-002/personalized": `[
			{
				"id": "recently-added",
				"entities": [
					{
						"id": "li-pod-001",
						"libraryId": "lib-pods-002",
						"addedAt": 1712222222222,
						"mediaType": "podcast",
						"media": {"metadata": {"title": "My Podcast"}}
					}
				]
			}
		]`,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	items, err := c.GetRecentlyAdded(context.Background(), []Library{
		{ID: "lib-books-001", MediaType: "book"},
		{ID: "lib-pods-002", MediaType: "podcast"},
	})
	if err != nil {
		t.Fatalf("GetRecentlyAdded() error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "li-pod-001" {
		t.Errorf("items[0].ID = %q, want li-pod-001", items[0].ID)
	}
	if items[1].ID != "li-book-001" {
		t.Errorf("items[1].ID = %q, want li-book-001", items[1].ID)
	}
}

func TestSearchLibraryHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-1/search" {
			t.Errorf("path = %q, want /api/libraries/lib-1/search", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "adventure" {
			t.Errorf("q = %q, want adventure", q.Get("q"))
		}
		if q.Get("limit") != "12" {
			t.Errorf("limit = %q, want 12", q.Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(searchFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	result, err := c.SearchLibrary(context.Background(), "lib-1", "adventure")
	if err != nil {
		t.Fatalf("SearchLibrary() error: %v", err)
	}
	if len(result.Book) != 1 {
		t.Fatalf("expected 1 book, got %d", len(result.Book))
	}
}

func TestSearchPodcastEpisodesHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-pod/items":
			if r.URL.Query().Get("page") != "0" {
				t.Errorf("page = %q, want 0", r.URL.Query().Get("page"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results": [
					{
						"id": "pod-001",
						"libraryId": "lib-pod",
						"mediaType": "podcast",
						"media": {"metadata": {"title": "Joe Rogan"}}
					}
				],
				"total": 1,
				"limit": 100,
				"page": 0
			}`))
		case "/api/libraries/lib-pod/search":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"book":[],"podcast":[]}`))
		case "/api/items/pod-001":
			if r.URL.Query().Get("expanded") != "1" {
				t.Error("expected expanded=1 query param")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id": "pod-001",
				"libraryId": "lib-pod",
				"mediaType": "podcast",
				"media": {
					"metadata": {"title": "Joe Rogan"},
					"episodes": [
						{"id": "ep-001", "title": "Joe Rogan Experience #1", "duration": 3600},
						{"id": "ep-002", "title": "Another Show", "duration": 1800},
						{"id": "ep-003", "title": "Joe Rogan Experience #2", "duration": 4200}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	items, err := c.SearchPodcastEpisodes(context.Background(), "lib-pod", "Joe Rogan")
	if err != nil {
		t.Fatalf("SearchPodcastEpisodes() error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 matching episode hits, got %d", len(items))
	}
	if items[0].RecentEpisode == nil || items[0].RecentEpisode.ID != "ep-001" {
		t.Fatalf("expected first hit to expose episode ep-001, got %#v", items[0].RecentEpisode)
	}
	if items[1].RecentEpisode == nil || items[1].RecentEpisode.ID != "ep-003" {
		t.Fatalf("expected second hit to expose episode ep-003, got %#v", items[1].RecentEpisode)
	}
}

func TestSearchPodcastEpisodesHTTP_FindsEpisodeWhenShowSearchWouldMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-pod/items":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results": [
					{
						"id": "pod-001",
						"libraryId": "lib-pod",
						"mediaType": "podcast",
						"media": {"metadata": {"title": "Joe Rogan"}}
					}
				],
				"total": 1,
				"limit": 100,
				"page": 0
			}`))
		case "/api/items/pod-001":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id": "pod-001",
				"libraryId": "lib-pod",
				"mediaType": "podcast",
				"media": {
					"metadata": {"title": "Joe Rogan"},
					"episodes": [
						{"id": "ep-001", "title": "Jason...", "duration": 3600}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	items, err := c.SearchPodcastEpisodes(context.Background(), "lib-pod", "Jas")
	if err != nil {
		t.Fatalf("SearchPodcastEpisodes() error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 matching episode hit, got %d", len(items))
	}
	if items[0].RecentEpisode == nil || items[0].RecentEpisode.Title != "Jason..." {
		t.Fatalf("expected Jason episode hit, got %#v", items[0].RecentEpisode)
	}
}
