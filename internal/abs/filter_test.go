package abs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFilterAudioLibraries_KeepsPodcasts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not fetch items for podcast libraries")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "pod-1", Name: "Podcasts", MediaType: "podcast"}}
	result := c.FilterAudioLibraries(context.Background(), libs)
	if len(result) != 1 {
		t.Fatalf("expected 1 library, got %d", len(result))
	}
}

// TestFilterAudioLibraries_KeepsAudiobooks verifies that the check reads one
// item, the longest, and keeps the library when that item has audio.
func TestFilterAudioLibraries_KeepsAudiobooks(t *testing.T) {
	dur := 55908.0
	numFiles := 138
	numTracks := 138
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/audio-lib/items" {
			t.Errorf("path = %q, want /api/libraries/audio-lib/items", r.URL.Path)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{"limit": "1", "sort": "media.duration", "desc": "1"} {
			if got := query.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		resp := LibraryItemsResponse{
			Results: []LibraryItem{{
				ID:        "item-1",
				MediaType: "book",
				Media: Media{
					Duration:      &dur,
					NumAudioFiles: &numFiles,
					NumTracks:     &numTracks,
				},
			}},
			Total: 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "audio-lib", Name: "Audiobooks", MediaType: "book"}}
	result := c.FilterAudioLibraries(context.Background(), libs)
	if len(result) != 1 {
		t.Fatalf("expected audiobook library to be kept, got %d libraries", len(result))
	}
}

func TestFilterAudioLibraries_ExcludesEbooks(t *testing.T) {
	zero := 0.0
	zeroInt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LibraryItemsResponse{
			Results: []LibraryItem{{
				ID:        "ebook-1",
				MediaType: "book",
				Media: Media{
					Duration:      &zero,
					NumAudioFiles: &zeroInt,
					NumTracks:     &zeroInt,
				},
			}},
			Total: 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "ebook-lib", Name: "Books", MediaType: "book"}}
	result := c.FilterAudioLibraries(context.Background(), libs)
	if len(result) != 0 {
		t.Fatalf("expected ebook library to be excluded, got %d libraries", len(result))
	}
}

func TestFilterAudioLibraries_DetectsAudioByNumTracks(t *testing.T) {
	numTracks := 5
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LibraryItemsResponse{
			Results: []LibraryItem{{
				ID:        "item-1",
				MediaType: "book",
				Media:     Media{NumTracks: &numTracks},
			}},
			Total: 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "lib-1", Name: "Books with Audio", MediaType: "book"}}
	result := c.FilterAudioLibraries(context.Background(), libs)
	if len(result) != 1 {
		t.Fatal("expected library with numTracks>0 to be kept")
	}
}

func TestFilterAudioLibraries_DetectsAudioByNumAudioFiles(t *testing.T) {
	numFiles := 3
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LibraryItemsResponse{
			Results: []LibraryItem{{
				ID:        "item-1",
				MediaType: "book",
				Media:     Media{NumAudioFiles: &numFiles},
			}},
			Total: 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "lib-1", Name: "Audiobooks", MediaType: "book"}}
	result := c.FilterAudioLibraries(context.Background(), libs)
	if len(result) != 1 {
		t.Fatal("expected library with numAudioFiles>0 to be kept")
	}
}

// TestFilterAudioLibraries_KeepsEmptyLibrary verifies that a book library with
// no items stays in the list: it can get audiobooks later.
func TestFilterAudioLibraries_KeepsEmptyLibrary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(LibraryItemsResponse{Results: []LibraryItem{}, Total: 0})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "new-lib", Name: "New", MediaType: "book"}}
	result := c.FilterAudioLibraries(context.Background(), libs)
	if len(result) != 1 {
		t.Fatalf("expected the empty library to be kept, got %d libraries", len(result))
	}
}
