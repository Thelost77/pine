package search

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/db"
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

	cache := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)

	first, err := cache.Search(context.Background(), "lib-pod", "podcast", "Joe")
	if err != nil {
		t.Fatalf("first Search() error: %v", err)
	}
	second, err := cache.Search(context.Background(), "lib-pod", "podcast", "Rogan")
	if err != nil {
		t.Fatalf("second Search() error: %v", err)
	}

	if len(first) != 1 || first[0].Media.Metadata.Title != "Joe Rogan" {
		t.Fatalf("unexpected first results: %#v", first)
	}
	if len(second) != 1 || second[0].Media.Metadata.Title != "Joe Rogan" {
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
		case "/api/libraries/lib-a/series", "/api/libraries/lib-b/series":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[],"total":0,"limit":100,"page":0}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cache := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)

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
		if r.URL.Path == "/api/libraries/lib-a/series" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[],"total":0,"limit":100,"page":0}`))
			return
		}
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
	cache := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)
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

func TestCacheInvalidationDiscardsInFlightBuild(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-1/items":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results":[{"id":"book-1","libraryId":"lib-1","mediaType":"book","media":{"metadata":{"title":"Old Title","authorName":"Author"}}}],
				"total":1,
				"limit":100,
				"page":0
			}`))
		case "/api/libraries/lib-1/series":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[],"total":0,"limit":100,"page":0}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cache := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)
	ctx := context.Background()
	_ = cache.Prepare(ctx, "lib-1", "book")
	cache.Invalidate("lib-1")

	cache.mu.Lock()
	_, ok := cache.snapshots["lib-1"]
	cache.mu.Unlock()
	if ok {
		t.Fatal("expected snapshot to be removed after invalidation")
	}
}

func TestCachePodcastSearchMatchesNumericTokenAfterPunctuation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/libraries/lib-pod/items":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"results":[{"id":"pod-001","libraryId":"lib-pod","mediaType":"podcast","media":{"metadata":{"title":"$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]"}}}],
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
					"metadata":{"title":"$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]"},
					"episodes":[]
				}
			}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	cache := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)

	results, err := cache.Search(context.Background(), "lib-pod", "podcast", "100")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Media.Metadata.Title != "$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]" {
		t.Fatalf("unexpected result: %#v", results)
	}
}

func TestCacheBookSearchUsesFuzzyFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/libraries/lib-a/series" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[],"total":0,"limit":100,"page":0}`))
			return
		}
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

	cache := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)

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

func TestCachePersistsSnapshotToDiskAndRestoresOnRestart(t *testing.T) {
	var listCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/libraries/lib-a/series" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[],"total":0,"limit":100,"page":0}`))
			return
		}
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

	dbPath := filepath.Join(t.TempDir(), "test.db")
	dbStore, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer dbStore.Close()

	cacheStore := cache.NewStore(dbStore)
	client := cache.NewClient(abs.NewClient(srv.URL, "tok"), nil)

	// Build snapshot with first cache instance
	firstCache := NewCache(client, cacheStore)
	results1, err := firstCache.Search(context.Background(), "lib-a", "book", "alp")
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if len(results1) != 1 || results1[0].Media.Metadata.Title != "Alpha" {
		t.Fatalf("unexpected first results: %#v", results1)
	}
	if listCalls != 1 {
		t.Fatalf("list calls after first search = %d, want 1", listCalls)
	}

	// Simulate restart: create new cache instance with same store
	secondCache := NewCache(client, cacheStore)
	results2, err := secondCache.Search(context.Background(), "lib-a", "book", "alp")
	if err != nil {
		t.Fatalf("second search after restart: %v", err)
	}
	if len(results2) != 1 || results2[0].Media.Metadata.Title != "Alpha" {
		t.Fatalf("unexpected second results: %#v", results2)
	}
	if listCalls != 1 {
		t.Fatalf("list calls after restart = %d, want 1 (should have loaded from disk)", listCalls)
	}
}

func TestBuildSnapshotPagination(t *testing.T) {
	var listCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/libraries/lib-pagi/items" && r.URL.Path != "/api/libraries/lib-pagi/series" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		
		if r.URL.Path == "/api/libraries/lib-pagi/series" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[],"total":0,"limit":50,"page":0}`))
			return
		}
		
		listCalls++

		// Query params: limit=50, page=...
		// But in cache test we just serve based on the listCalls
		var count int
		if listCalls <= 2 {
			count = 50 // Pages 0 and 1
		} else {
			count = 0 // Page 2
		}

		w.Header().Set("Content-Type", "application/json")
		if count == 0 {
			w.Write([]byte(`{"results":[],"total":100,"limit":50,"page":2}`))
			return
		}

		// generate count items
		results := ""
		for i := 0; i < count; i++ {
			if i > 0 {
				results += ","
			}
			results += fmt.Sprintf(`{"id":"book-%d","libraryId":"lib-pagi","mediaType":"book","media":{"metadata":{"title":"Book %d"}}}`, i, i)
		}
		w.Write([]byte(fmt.Sprintf(`{"results":[%s],"total":100,"limit":50,"page":%d}`, results, listCalls-1)))
	}))
	defer srv.Close()

	c := NewCache(cache.NewClient(abs.NewClient(srv.URL, "tok"), nil), nil)
	
	err := c.Prepare(context.Background(), "lib-pagi", "book")
	if err != nil {
		t.Fatalf("failed to prepare cache: %v", err)
	}

	if listCalls != 2 {
		t.Fatalf("expected exactly 2 pagination calls, got %d", listCalls)
	}

	items, ok := c.TryGetAll("lib-pagi", "book")
	if !ok {
		t.Fatalf("expected cache to be built")
	}
	if len(items) != 100 {
		t.Fatalf("expected 100 items in cache, got %d", len(items))
	}
}
