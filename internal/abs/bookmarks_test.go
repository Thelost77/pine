package abs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBookmarksHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/progress/li-001" {
			t.Errorf("path = %q, want /api/me/progress/li-001", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"libraryItemId": "li-001",
			"currentTime": 0,
			"progress": 0,
			"isFinished": false,
			"bookmarks": [
				{"libraryItemId": "li-001", "title": "Great passage", "time": 300.5, "createdAt": 1700000000000},
				{"libraryItemId": "li-001", "title": "Important quote", "time": 1500.0, "createdAt": 1700001000000}
			]
		}`))
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
		w.Write([]byte(`{}`))
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
		w.Write([]byte(`{}`))
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
		w.Write([]byte(`{"libraryItemId":"li-002","currentTime":0,"progress":0,"isFinished":false,"bookmarks":[]}`))
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
		w.Write([]byte(`{"libraryItemId":`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.GetBookmarks(context.Background(), "li-bad")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}
