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
