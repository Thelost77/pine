package abs

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed testdata/bookmarks.json
var bookmarksFixture []byte

// --- Deserialization tests ---

func TestMediaProgressWithBookmarksDeserialization(t *testing.T) {
	var progress MediaProgressWithBookmarks
	if err := json.Unmarshal(bookmarksFixture, &progress); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if progress.LibraryItemID != "li-001" {
		t.Errorf("LibraryItemID = %q, want %q", progress.LibraryItemID, "li-001")
	}
	if progress.CurrentTime != 1234.56 {
		t.Errorf("CurrentTime = %f, want 1234.56", progress.CurrentTime)
	}
	if len(progress.Bookmarks) != 2 {
		t.Fatalf("expected 2 bookmarks, got %d", len(progress.Bookmarks))
	}
	bm := progress.Bookmarks[0]
	if bm.Title != "Great passage" {
		t.Errorf("bookmark title = %q, want %q", bm.Title, "Great passage")
	}
	if bm.Time != 300.5 {
		t.Errorf("bookmark time = %f, want 300.5", bm.Time)
	}
	if bm.CreatedAt != 1700000000000 {
		t.Errorf("bookmark createdAt = %d, want 1700000000000", bm.CreatedAt)
	}
}

// --- HTTP tests ---

func TestGetBookmarksHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/progress/li-001" {
			t.Errorf("path = %q, want /api/me/progress/li-001", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bookmarksFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	bookmarks, err := c.GetBookmarks(context.Background(), "li-001")
	if err != nil {
		t.Fatalf("GetBookmarks() error: %v", err)
	}
	if len(bookmarks) != 2 {
		t.Fatalf("expected 2 bookmarks, got %d", len(bookmarks))
	}
	if bookmarks[0].Title != "Great passage" {
		t.Errorf("bookmark[0] title = %q, want %q", bookmarks[0].Title, "Great passage")
	}
	if bookmarks[1].Time != 1500.0 {
		t.Errorf("bookmark[1] time = %f, want 1500.0", bookmarks[1].Time)
	}
}

func TestCreateBookmarkHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/item/li-001/bookmark" {
			t.Errorf("path = %q, want /api/me/item/li-001/bookmark", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write(bookmarksFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.CreateBookmark(context.Background(), "li-001", 300.5, "Great passage")
	if err != nil {
		t.Fatalf("CreateBookmark() error: %v", err)
	}

	var body struct {
		Time  float64 `json:"time"`
		Title string  `json:"title"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.Time != 300.5 {
		t.Errorf("time = %f, want 300.5", body.Time)
	}
	if body.Title != "Great passage" {
		t.Errorf("title = %q, want %q", body.Title, "Great passage")
	}
}

func TestDeleteBookmarkHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/item/li-001/bookmark/300.500" {
			t.Errorf("path = %q, want /api/me/item/li-001/bookmark/300.500", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.DeleteBookmark(context.Background(), "li-001", 300.5)
	if err != nil {
		t.Fatalf("DeleteBookmark() error: %v", err)
	}
}

func TestGetBookmarksEmptyResponse(t *testing.T) {
	emptyProgress := `{"libraryItemId": "li-002", "currentTime": 0, "progress": 0, "isFinished": false, "bookmarks": []}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(emptyProgress))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	bookmarks, err := c.GetBookmarks(context.Background(), "li-002")
	if err != nil {
		t.Fatalf("GetBookmarks() error: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("expected 0 bookmarks, got %d", len(bookmarks))
	}
}

func TestGetBookmarksNotFoundReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	bookmarks, err := c.GetBookmarks(context.Background(), "li-404")
	if err != nil {
		t.Fatalf("GetBookmarks() error: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("expected 0 bookmarks, got %d", len(bookmarks))
	}
}

func TestGetBookmarksServerErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.GetBookmarks(context.Background(), "li-500")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetBookmarksMalformedResponseReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"bookmarks":`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.GetBookmarks(context.Background(), "li-bad")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}
