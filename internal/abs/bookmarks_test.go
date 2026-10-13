package abs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetBookmarksHTTP verifies that GetBookmarks reads the route of the
// bookmarks. The server answers as ABS 2.37.1 does: the progress of an item
// has no bookmarks.
func TestGetBookmarksHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/me/bookmarks/li-001":
			_, _ = w.Write([]byte(`{"bookmarks": [
				{"libraryItemId": "li-001", "time": 300.5, "title": "Great passage", "createdAt": 1700000000000},
				{"libraryItemId": "li-001", "time": 1500, "title": "Important quote", "createdAt": 1700001000000}
			]}`))
		case "/api/me/progress/li-001":
			_, _ = w.Write([]byte(`{"id": "mp-001", "libraryItemId": "li-001", "episodeId": null, "duration": 3600, "progress": 0.1, "currentTime": 360, "isFinished": false}`))
		default:
			http.NotFound(w, r)
		}
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

// TestGetBookmarksReadsUserBeforeABS236 verifies that GetBookmarks reads the
// bookmarks of the item from the user, if the server has no route for the
// bookmarks of one item. ABS before 2.36.0 answers 404 for that route.
func TestGetBookmarksReadsUserBeforeABS236(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "usr-001", "bookmarks": [
			{"libraryItemId": "li-001", "time": 300.5, "title": "Great passage", "createdAt": 1700000000000},
			{"libraryItemId": "li-002", "time": 10, "title": "Other book", "createdAt": 1700000500000},
			{"libraryItemId": "li-001", "time": 1500, "title": "Important quote", "createdAt": 1700001000000}
		]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	bookmarks, err := c.GetBookmarks(context.Background(), "li-001")
	if err != nil {
		t.Fatalf("GetBookmarks() error: %v", err)
	}
	if len(bookmarks) != 2 || bookmarks[0].Title != "Great passage" || bookmarks[1].Title != "Important quote" {
		t.Errorf("bookmarks = %+v, want the two bookmarks of li-001", bookmarks)
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
		_, _ = w.Write([]byte(`{}`))
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

func TestUpdateBookmarkHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/item/li-001/bookmark" {
			t.Errorf("path = %q, want /api/me/item/li-001/bookmark", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.UpdateBookmark(context.Background(), "li-001", 300.5, "Renamed passage")
	if err != nil {
		t.Fatalf("UpdateBookmark() error: %v", err)
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
	if body.Title != "Renamed passage" {
		t.Errorf("title = %q, want %q", body.Title, "Renamed passage")
	}
}

func TestDeleteBookmarkHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/item/li-001/bookmark/300.5" {
			t.Errorf("path = %q, want /api/me/item/li-001/bookmark/300.5", r.URL.Path)
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

func TestDeleteBookmarkHTTPPreservesBookmarkPrecision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/item/li-001/bookmark/4733.343044" {
			t.Errorf("path = %q, want /api/me/item/li-001/bookmark/4733.343044", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.DeleteBookmark(context.Background(), "li-001", 4733.343044)
	if err != nil {
		t.Fatalf("DeleteBookmark() error: %v", err)
	}
}

func TestGetBookmarksEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"bookmarks":[]}`))
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

func TestGetBookmarksNotFoundReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.GetBookmarks(context.Background(), "li-404")
	if err == nil {
		t.Fatal("expected error for 404 response")
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
		_, _ = w.Write([]byte(`{"bookmarks":`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.GetBookmarks(context.Background(), "li-bad")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}
