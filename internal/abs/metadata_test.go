package abs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateLibraryItemMediaSendsPatchPayload(t *testing.T) {
	title := "Correct Title"
	authors := []Author{{Name: "Correct Author"}}
	series := []SeriesSequence{{Name: "Correct Series", Sequence: "2"}}

	var method, path string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if auth := r.Header.Get("Authorization"); auth != "Bearer tok" {
			t.Fatalf("Authorization = %q, want Bearer tok", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"updated":true,"libraryItem":{"id":"item-1","mediaType":"book","media":{"metadata":{"title":"Correct Title"}}}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	item, err := client.UpdateLibraryItemMedia(context.Background(), "item-1", UpdateMediaRequest{
		Metadata: UpdateMediaMetadata{
			Title:   &title,
			Authors: &authors,
			Series:  &series,
		},
	})
	if err != nil {
		t.Fatalf("UpdateLibraryItemMedia() error = %v", err)
	}
	if method != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", method)
	}
	if path != "/api/items/item-1/media" {
		t.Fatalf("path = %q, want /api/items/item-1/media", path)
	}
	if item == nil || item.ID != "item-1" || item.Media.Metadata.Title != "Correct Title" {
		t.Fatalf("updated item = %+v", item)
	}

	metadata, ok := body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata payload missing: %#v", body)
	}
	if got := metadata["title"]; got != title {
		t.Fatalf("metadata.title = %#v, want %q", got, title)
	}
	gotAuthors, ok := metadata["authors"].([]any)
	if !ok || len(gotAuthors) != 1 {
		t.Fatalf("metadata.authors = %#v", metadata["authors"])
	}
	if gotAuthors[0].(map[string]any)["name"] != "Correct Author" {
		t.Fatalf("metadata.authors[0] = %#v", gotAuthors[0])
	}
	gotSeries, ok := metadata["series"].([]any)
	if !ok || len(gotSeries) != 1 {
		t.Fatalf("metadata.series = %#v", metadata["series"])
	}
	if gotSeries[0].(map[string]any)["sequence"] != "2" {
		t.Fatalf("metadata.series[0] = %#v", gotSeries[0])
	}
	if _, ok := gotSeries[0].(map[string]any)["id"]; ok {
		t.Fatalf("metadata.series[0].id present, want omitted for empty ID: %#v", gotSeries[0])
	}
}

func TestUpdatePodcastEpisodeSendsPatchPayload(t *testing.T) {
	title := "Correct Episode"
	description := "Correct Description"
	season := "2"
	episode := "7"
	episodeType := "bonus"

	var method, path string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"pod-1","mediaType":"podcast","media":{"metadata":{"title":"Podcast"},"episodes":[{"id":"ep-1","title":"Correct Episode","description":"Correct Description","season":"2","episode":"7","episodeType":"bonus","duration":120}]}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	item, err := client.UpdatePodcastEpisode(context.Background(), "pod-1", "ep-1", UpdatePodcastEpisodeRequest{
		Title:       &title,
		Description: &description,
		Season:      &season,
		Episode:     &episode,
		EpisodeType: &episodeType,
	})
	if err != nil {
		t.Fatalf("UpdatePodcastEpisode() error = %v", err)
	}
	if method != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", method)
	}
	if path != "/api/podcasts/pod-1/episode/ep-1" {
		t.Fatalf("path = %q, want /api/podcasts/pod-1/episode/ep-1", path)
	}
	if item == nil || len(item.Media.Episodes) != 1 || item.Media.Episodes[0].Title != title {
		t.Fatalf("updated item = %+v", item)
	}

	if got := body["title"]; got != title {
		t.Fatalf("title = %#v, want %q", got, title)
	}
	if got := body["description"]; got != description {
		t.Fatalf("description = %#v, want %q", got, description)
	}
	if got := body["season"]; got != season {
		t.Fatalf("season = %#v, want %q", got, season)
	}
	if got := body["episode"]; got != episode {
		t.Fatalf("episode = %#v, want %q", got, episode)
	}
	if got := body["episodeType"]; got != episodeType {
		t.Fatalf("episodeType = %#v, want %q", got, episodeType)
	}
}

func TestUpdateLibraryItemMediaOmitsNilFieldsAndAllowsEmptySeries(t *testing.T) {
	emptySeries := []SeriesSequence{}
	var metadata map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Metadata map[string]any `json:"metadata"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		metadata = body.Metadata
		_, _ = w.Write([]byte(`{"updated":true}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	_, err := client.UpdateLibraryItemMedia(context.Background(), "item-1", UpdateMediaRequest{
		Metadata: UpdateMediaMetadata{Series: &emptySeries},
	})
	if err != nil {
		t.Fatalf("UpdateLibraryItemMedia() error = %v", err)
	}
	if _, ok := metadata["title"]; ok {
		t.Fatalf("metadata.title present, want omitted: %#v", metadata)
	}
	gotSeries, ok := metadata["series"].([]any)
	if !ok || len(gotSeries) != 0 {
		t.Fatalf("metadata.series = %#v, want empty array", metadata["series"])
	}
}

func TestUpdateLibraryItemMediaSurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	_, err := client.UpdateLibraryItemMedia(context.Background(), "item-1", UpdateMediaRequest{})
	if err == nil {
		t.Fatal("UpdateLibraryItemMedia() error = nil, want error")
	}
	if !IsHTTPStatus(err, http.StatusForbidden) {
		t.Fatalf("IsHTTPStatus(403) = false for %v", err)
	}
}
