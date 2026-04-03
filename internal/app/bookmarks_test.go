package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/screens/detail"
)

func testBookmarkItem(id string) abs.LibraryItem {
	return abs.LibraryItem{
		ID:        id,
		MediaType: "book",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title: "Bookmark Test",
			},
		},
	}
}

func TestHandleAddBookmarkReturnsBookmarkUpdateErrorWhenRefreshFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/me/item/item-001/bookmark":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/me":
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(config.Default(), nil, abs.NewClient(srv.URL, "tok"))
	m.sessionID = "sess-001"
	m.player.Position = 120

	_, cmd := m.handleAddBookmark(detail.AddBookmarkCmd{Item: testBookmarkItem("item-001")})
	if cmd == nil {
		t.Fatal("expected bookmark command")
	}

	msg := cmd()
	updateMsg, ok := msg.(detail.BookmarksUpdatedMsg)
	if !ok {
		t.Fatalf("expected BookmarksUpdatedMsg, got %T", msg)
	}
	if updateMsg.Err == nil {
		t.Fatal("expected bookmark refresh error")
	}
}

func TestHandleDeleteBookmarkReturnsEmptyBookmarksWhenLastBookmarkRemoved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/me/item/item-001/bookmark/300.5":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/me":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"bookmarks":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(config.Default(), nil, abs.NewClient(srv.URL, "tok"))

	_, cmd := m.handleDeleteBookmark(detail.DeleteBookmarkCmd{
		ItemID: "item-001",
		Bookmark: abs.Bookmark{
			Title: "Only bookmark",
			Time:  300.5,
		},
	})
	if cmd == nil {
		t.Fatal("expected delete bookmark command")
	}

	msg := cmd()
	updateMsg, ok := msg.(detail.BookmarksUpdatedMsg)
	if !ok {
		t.Fatalf("expected BookmarksUpdatedMsg, got %T", msg)
	}
	if updateMsg.Err != nil {
		t.Fatalf("expected empty bookmark update, got error %v", updateMsg.Err)
	}
	if len(updateMsg.Bookmarks) != 0 {
		t.Fatalf("expected empty bookmarks after delete, got %d", len(updateMsg.Bookmarks))
	}
}

func TestHandleUpdateBookmarkRefreshesBookmarks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/item/item-001/bookmark":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/me":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"bookmarks":[{"libraryItemId":"item-001","title":"Renamed bookmark","time":300.5,"createdAt":1700000000000}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(config.Default(), nil, abs.NewClient(srv.URL, "tok"))

	_, cmd := m.handleUpdateBookmark(detail.UpdateBookmarkCmd{
		ItemID: "item-001",
		Bookmark: abs.Bookmark{
			Title: "Old title",
			Time:  300.5,
		},
		Title: "Renamed bookmark",
	})
	if cmd == nil {
		t.Fatal("expected update bookmark command")
	}

	msg := cmd()
	updateMsg, ok := msg.(detail.BookmarksUpdatedMsg)
	if !ok {
		t.Fatalf("expected BookmarksUpdatedMsg, got %T", msg)
	}
	if updateMsg.Err != nil {
		t.Fatalf("expected no refresh error, got %v", updateMsg.Err)
	}
	if len(updateMsg.Bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(updateMsg.Bookmarks))
	}
	if updateMsg.Bookmarks[0].Title != "Renamed bookmark" {
		t.Fatalf("bookmark title = %q, want Renamed bookmark", updateMsg.Bookmarks[0].Title)
	}
}

func TestHandleSeekToBookmarkStartsPlaybackWhenStopped(t *testing.T) {
	var progressBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/items/item-multitrack/play":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(abs.PlaySession{
				ID: "sess-bookmark-start",
				AudioTracks: []abs.AudioTrack{
					{Index: 0, StartOffset: 0, ContentURL: "/s/item/item-multitrack/track0.mp3", Duration: 1800},
					{Index: 1, StartOffset: 1800, ContentURL: "/s/item/item-multitrack/track1.mp3", Duration: 1800},
				},
				CurrentTime: 42,
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/me/progress/item-multitrack":
			if err := json.NewDecoder(r.Body).Decode(&progressBody); err != nil {
				t.Fatalf("decode progress body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	item := testBookmarkItem("item-multitrack")
	dur := 3600.0
	item.Media.Duration = &dur

	m := New(config.Default(), nil, abs.NewClient(srv.URL, "tok"))

	_, cmd := m.handleSeekToBookmark(detail.SeekToBookmarkCmd{Item: item, Time: 2209})
	if cmd == nil {
		t.Fatal("expected bookmark seek command")
	}

	msg := cmd()
	playMsg, ok := msg.(PlaySessionMsg)
	if !ok {
		t.Fatalf("expected PlaySessionMsg, got %T", msg)
	}
	if playMsg.Session.ItemID != "item-multitrack" {
		t.Errorf("session itemID = %q, want item-multitrack", playMsg.Session.ItemID)
	}
	if playMsg.Session.CurrentTime != 409 {
		t.Errorf("session currentTime = %v, want 409", playMsg.Session.CurrentTime)
	}
	if playMsg.Session.TrackStartOffset != 1800 {
		t.Errorf("track start offset = %v, want 1800", playMsg.Session.TrackStartOffset)
	}
	if progressBody != nil {
		t.Errorf("expected no progress patch before starting playback, got %v", progressBody)
	}
}
