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
	result, err := c.FilterAudioLibraries(context.Background(), libs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 library, got %d", len(result))
	}
}

func TestFilterAudioLibraries_KeepsAudiobooks(t *testing.T) {
	dur := 55908.0
	numFiles := 138
	numTracks := 138
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "audio-lib", Name: "Audiobooks", MediaType: "book"}}
	result, err := c.FilterAudioLibraries(context.Background(), libs)
	if err != nil {
		t.Fatal(err)
	}
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "ebook-lib", Name: "Books", MediaType: "book"}}
	result, err := c.FilterAudioLibraries(context.Background(), libs)
	if err != nil {
		t.Fatal(err)
	}
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "lib-1", Name: "Books with Audio", MediaType: "book"}}
	result, err := c.FilterAudioLibraries(context.Background(), libs)
	if err != nil {
		t.Fatal(err)
	}
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	libs := []Library{{ID: "lib-1", Name: "Audiobooks", MediaType: "book"}}
	result, err := c.FilterAudioLibraries(context.Background(), libs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatal("expected library with numAudioFiles>0 to be kept")
	}
}
