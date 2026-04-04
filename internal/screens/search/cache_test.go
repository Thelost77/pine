package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/abs"
)

func TestCacheReusesPodcastSnapshotAcrossQueries(t *testing.T) {
	var listCalls int
	var itemCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-pod/items":
			listCalls++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results":[{"id":"pod-001","libraryId":"lib-pod","mediaType":"podcast","media":{"metadata":{"title":"Joe Rogan"}}}],
				"total":1,
				"limit":100,
				"page":0
			}`))
		case "/api/items/pod-001":
			itemCalls++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id":"pod-001",
				"libraryId":"lib-pod",
				"mediaType":"podcast",
				"media":{
					"metadata":{"title":"Joe Rogan"},
					"episodes":[
						{"id":"ep-001","title":"Jason...","duration":3600},
						{"id":"ep-002","title":"Carl...","duration":1800}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cache := NewCache(abs.NewClient(srv.URL, "tok"))

	first, err := cache.Search(context.Background(), "lib-pod", "podcast", "Jas")
	if err != nil {
		t.Fatalf("first Search() error: %v", err)
	}
	second, err := cache.Search(context.Background(), "lib-pod", "podcast", "Car")
	if err != nil {
		t.Fatalf("second Search() error: %v", err)
	}

	if len(first) != 1 || first[0].RecentEpisode == nil || first[0].RecentEpisode.Title != "Jason..." {
		t.Fatalf("unexpected first results: %#v", first)
	}
	if len(second) != 1 || second[0].RecentEpisode == nil || second[0].RecentEpisode.Title != "Carl..." {
		t.Fatalf("unexpected second results: %#v", second)
	}
	if listCalls != 1 {
		t.Fatalf("library list calls = %d, want 1", listCalls)
	}
	if itemCalls != 1 {
		t.Fatalf("item expansion calls = %d, want 1", itemCalls)
	}
}

func TestCacheKeepsSnapshotsPerLibraryID(t *testing.T) {
	calls := map[string]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-a/items":
			calls["lib-a"]++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results":[{"id":"book-001","libraryId":"lib-a","mediaType":"book","media":{"metadata":{"title":"Alpha","authorName":"Ann","duration":3600}}}],
				"total":1,
				"limit":100,
				"page":0
			}`))
		case "/api/libraries/lib-b/items":
			calls["lib-b"]++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results":[{"id":"book-002","libraryId":"lib-b","mediaType":"book","media":{"metadata":{"title":"Beta","authorName":"Bob","duration":2400}}}],
				"total":1,
				"limit":100,
				"page":0
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cache := NewCache(abs.NewClient(srv.URL, "tok"))

	if _, err := cache.Search(context.Background(), "lib-a", "book", "alp"); err != nil {
		t.Fatalf("search lib-a: %v", err)
	}
	if _, err := cache.Search(context.Background(), "lib-b", "book", "bet"); err != nil {
		t.Fatalf("search lib-b: %v", err)
	}
	if _, err := cache.Search(context.Background(), "lib-a", "book", "ann"); err != nil {
		t.Fatalf("search lib-a again: %v", err)
	}

	if calls["lib-a"] != 1 {
		t.Fatalf("lib-a calls = %d, want 1", calls["lib-a"])
	}
	if calls["lib-b"] != 1 {
		t.Fatalf("lib-b calls = %d, want 1", calls["lib-b"])
	}
}

func TestCacheRebuildsStaleSnapshot(t *testing.T) {
	var listCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-a/items" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		listCalls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results":[{"id":"book-001","libraryId":"lib-a","mediaType":"book","media":{"metadata":{"title":"Alpha","authorName":"Ann","duration":3600}}}],
			"total":1,
			"limit":100,
			"page":0
		}`))
	}))
	defer srv.Close()

	now := time.Unix(1000, 0)
	cache := NewCache(abs.NewClient(srv.URL, "tok"))
	cache.ttl = time.Minute
	cache.now = func() time.Time { return now }

	if _, err := cache.Search(context.Background(), "lib-a", "book", "alp"); err != nil {
		t.Fatalf("first search: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := cache.Search(context.Background(), "lib-a", "book", "alp"); err != nil {
		t.Fatalf("second search: %v", err)
	}

	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2", listCalls)
	}
}

func TestCachePodcastSearchMatchesNumericTokenAfterPunctuation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-pod/items":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results":[{"id":"pod-001","libraryId":"lib-pod","mediaType":"podcast","media":{"metadata":{"title":"Business"}}}],
				"total":1,
				"limit":100,
				"page":0
			}`))
		case "/api/items/pod-001":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"id":"pod-001",
				"libraryId":"lib-pod",
				"mediaType":"podcast",
				"media":{
					"metadata":{"title":"Business"},
					"episodes":[
						{"id":"ep-001","title":"$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]","duration":3600}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cache := NewCache(abs.NewClient(srv.URL, "tok"))

	results, err := cache.Search(context.Background(), "lib-pod", "podcast", "100")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RecentEpisode == nil || results[0].RecentEpisode.Title != "$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]" {
		t.Fatalf("unexpected result: %#v", results)
	}
}

func TestCacheBookSearchUsesFuzzyFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-a/items" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"results":[
				{"id":"book-001","libraryId":"lib-a","mediaType":"book","media":{"metadata":{"title":"The Great Gatsby","authorName":"F. Scott Fitzgerald","duration":3600}}},
				{"id":"book-002","libraryId":"lib-a","mediaType":"book","media":{"metadata":{"title":"Gone Girl","authorName":"Gillian Flynn","duration":4200}}}
			],
			"total":2,
			"limit":100,
			"page":0
		}`))
	}))
	defer srv.Close()

	cache := NewCache(abs.NewClient(srv.URL, "tok"))

	results, err := cache.Search(context.Background(), "lib-a", "book", "tgg")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected fuzzy results")
	}
	if results[0].Media.Metadata.Title != "The Great Gatsby" {
		t.Fatalf("top fuzzy result = %q, want %q", results[0].Media.Metadata.Title, "The Great Gatsby")
	}
}

func TestNormalizeSearchText(t *testing.T) {
	got := normalizeSearchText("$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]")
	want := "100m advice that ll piss off every business guru ft dhh 9xoaqikabzq"
	if got != want {
		t.Fatalf("normalizeSearchText() = %q, want %q", got, want)
	}
}
