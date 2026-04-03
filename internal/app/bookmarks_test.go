package app

import (
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
		case r.Method == http.MethodDelete && r.URL.Path == "/api/me/item/item-001/bookmark/300.500":
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
