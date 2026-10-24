package cache

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/abs"
)

func TestClient_CacheHit(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
	}))
	defer srv.Close()

	dbStore := openTestDB(t)
	store := NewStore(dbStore)
	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, store)

	libs1, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if len(libs1) != 1 || libs1[0].ID != "lib1" {
		t.Fatalf("unexpected first result: %+v", libs1)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 request after first call, got %d", requestCount)
	}

	libs2, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(libs2) != 1 || libs2[0].ID != "lib1" {
		t.Fatalf("unexpected second result: %+v", libs2)
	}
	if requestCount != 1 {
		t.Fatalf("expected still 1 request after cache hit, got %d", requestCount)
	}
}

func TestClient_TTLExpiry(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
	}))
	defer srv.Close()

	dbStore := openTestDB(t)
	store := NewStore(dbStore)
	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, store)

	// Pre-populate cache with a short-lived entry
	if err := store.Put(librariesKey, []abs.Library{{ID: "old"}}, 1*time.Millisecond); err != nil {
		t.Fatalf("put error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	libs, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if libs[0].ID != "lib1" {
		t.Fatalf("got %q, want lib1", libs[0].ID)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 request after expiry, got %d", requestCount)
	}
}

func TestClient_ErrorNotCached(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`error`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
	}))
	defer srv.Close()

	dbStore := openTestDB(t)
	store := NewStore(dbStore)
	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, store)

	_, err := client.GetLibraries(context.Background())
	if err == nil {
		t.Fatal("expected error on first call")
	}

	libs, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs))
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
}

func TestClient_NilStore(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
	}))
	defer srv.Close()

	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, nil)

	libs1, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if len(libs1) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs1))
	}

	libs2, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(libs2) != 1 {
		t.Fatalf("expected 1 library, got %d", len(libs2))
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests (no cache), got %d", requestCount)
	}
}

func TestClient_GetLibraryItem_CacheHit(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"li1","mediaType":"book","media":{"metadata":{"title":"Book One"}}}`))
	}))
	defer srv.Close()

	dbStore := openTestDB(t)
	store := NewStore(dbStore)
	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, store)

	item1, err := client.GetLibraryItem(context.Background(), "li1")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if item1.ID != "li1" {
		t.Fatalf("got %q, want li1", item1.ID)
	}

	item2, err := client.GetLibraryItem(context.Background(), "li1")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if item2.ID != "li1" {
		t.Fatalf("got %q, want li1", item2.ID)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 request, got %d", requestCount)
	}
}

// TestClient_GetBookmarks_CacheHit verifies that the cache keeps the bookmarks
// of an item, also if the item has no bookmarks.
func TestClient_GetBookmarks_CacheHit(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/me/bookmarks/li1":
			_, _ = w.Write([]byte(`{"bookmarks":[{"libraryItemId":"li1","time":30,"title":"mark1","createdAt":1700000000000}]}`))
		case "/api/me/bookmarks/li2":
			_, _ = w.Write([]byte(`{"bookmarks":[]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), NewStore(openTestDB(t)))

	for itemID, want := range map[string]int{"li1": 1, "li2": 0} {
		for call := 1; call <= 2; call++ {
			bookmarks, err := client.GetBookmarks(context.Background(), itemID)
			if err != nil {
				t.Fatalf("GetBookmarks(%s) call %d error: %v", itemID, call, err)
			}
			if len(bookmarks) != want {
				t.Errorf("GetBookmarks(%s) call %d = %d bookmarks, want %d", itemID, call, len(bookmarks), want)
			}
		}
	}
	if requestCount != 2 {
		t.Errorf("%d requests, want 1 for each item", requestCount)
	}
}

// closedStore returns a cache store with a closed database, so that each
// cache read and each cache write fails at once.
func closedStore(t *testing.T) *Store {
	t.Helper()
	dbStore := openTestDB(t)
	if err := dbStore.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return NewStore(dbStore)
}

// TestClient_CacheFailureReturnsServerData verifies that a request that the
// server answers gives the data of the server, also if the cache cannot read
// or write.
func TestClient_CacheFailureReturnsServerData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/libraries":
			_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
		case "/api/libraries/lib1/items":
			_, _ = w.Write([]byte(`{"results":[{"id":"li1","mediaType":"book","media":{"metadata":{"title":"Book One"}}}],"total":7,"limit":50,"page":0}`))
		case "/api/libraries/lib1/series/se1":
			_, _ = w.Write([]byte(`{"id":"se1","name":"Saga"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), closedStore(t))
	ctx := context.Background()

	libs, err := client.GetLibraries(ctx)
	if err != nil || len(libs) != 1 || libs[0].ID != "lib1" {
		t.Errorf("GetLibraries() = %+v, %v; want lib1", libs, err)
	}

	items, err := client.GetLibraryItems(ctx, "lib1", 0, 50)
	if err != nil || len(items.Results) != 1 || items.Total != 7 {
		t.Errorf("GetLibraryItems() = %+v, %v; want 1 item of 7", items, err)
	}

	contents, err := client.GetSeriesContents(ctx, "lib1", "se1")
	if err != nil || contents.Series.Name != "Saga" || len(contents.Items) != 1 {
		t.Errorf("GetSeriesContents() = %+v, %v; want Saga with 1 item", contents, err)
	}
}

// TestClient_ParallelFetchesOfDifferentKeys verifies that parallel requests
// for different keys all succeed. At the start of pine, the home screen and
// the session restore fetch at the same time, and their cache writes meet.
func TestClient_ParallelFetchesOfDifferentKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":%q,"mediaType":"book","media":{"metadata":{"title":"t"}}}`, strings.TrimPrefix(r.URL.Path, "/api/items/"))
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), NewStore(openTestDB(t)))

	const n = 64
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			item, err := client.GetLibraryItem(context.Background(), id)
			if err == nil && item.ID != id {
				err = fmt.Errorf("got item %q, want %q", item.ID, id)
			}
			errs <- err
		}(fmt.Sprintf("li-%d", i))
	}
	wg.Wait()
	close(errs)

	failed := 0
	for err := range errs {
		if err != nil {
			if failed == 0 {
				t.Errorf("first error: %v", err)
			}
			failed++
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d parallel GetLibraryItem calls failed", failed, n)
	}
}

// TestClient_PageHitKeepsRequestedPage verifies that a cached page has the
// page and the limit of the request, from the server and from the cache. The
// library screen reads Page.
func TestClient_PageHitKeepsRequestedPage(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":"li1","mediaType":"book","media":{"metadata":{"title":"Book One"}}}],"total":7,"limit":99,"page":9}`))
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), NewStore(openTestDB(t)))

	for call := 1; call <= 2; call++ {
		resp, err := client.GetLibraryItems(context.Background(), "lib1", 2, 50)
		if err != nil {
			t.Fatalf("call %d error: %v", call, err)
		}
		if resp.Page != 2 || resp.Limit != 50 || resp.Total != 7 || len(resp.Results) != 1 {
			t.Errorf("call %d = page %d, limit %d, total %d, %d results; want page 2, limit 50, total 7, 1 result",
				call, resp.Page, resp.Limit, resp.Total, len(resp.Results))
		}
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("%d requests, want 1", got)
	}
}

// TestClient_SeriesContentsHitSendsNoRequest verifies that the cache keeps the
// whole series contents, so that a hit needs no request for the series.
func TestClient_SeriesContentsHitSendsNoRequest(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/libraries/lib1/series/se1":
			_, _ = w.Write([]byte(`{"id":"se1","name":"Saga"}`))
		case "/api/libraries/lib1/items":
			_, _ = w.Write([]byte(`{"results":[{"id":"li1","mediaType":"book","media":{"metadata":{"title":"Book One"}}}],"total":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), NewStore(openTestDB(t)))

	if _, err := client.GetSeriesContents(context.Background(), "lib1", "se1"); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	before := requests.Load()
	contents, err := client.GetSeriesContents(context.Background(), "lib1", "se1")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if contents.Series.Name != "Saga" || len(contents.Items) != 1 {
		t.Errorf("second call = %+v, want Saga with 1 item", contents)
	}
	if got := requests.Load() - before; got != 0 {
		t.Errorf("second call sent %d requests, want 0", got)
	}
}

// TestClient_InvalidationClearsCache verifies that each change on the server
// clears the cache and tells the in-memory caches.
func TestClient_InvalidationClearsCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"li1","libraryItem":{"id":"li1"}}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	changes := map[string]func(c *Client) error{
		"DeleteItem":    func(c *Client) error { return c.DeleteItem(ctx, "li1", false) },
		"DeleteEpisode": func(c *Client) error { return c.DeleteEpisode(ctx, "pod1", "ep1", false) },
		"UpdateLibraryItemMedia": func(c *Client) error {
			_, err := c.UpdateLibraryItemMedia(ctx, "li1", abs.UpdateMediaRequest{})
			return err
		},
		"UpdatePodcastEpisode": func(c *Client) error {
			_, err := c.UpdatePodcastEpisode(ctx, "pod1", "ep1", abs.UpdatePodcastEpisodeRequest{})
			return err
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			store := NewStore(openTestDB(t))
			client := NewClient(abs.NewClient(srv.URL, "tok"), store)
			invalidated := false
			client.OnInvalidate = func() { invalidated = true }
			if err := store.Put(librariesKey, []abs.Library{{ID: "lib1"}}, time.Minute); err != nil {
				t.Fatalf("seed cache: %v", err)
			}

			if err := change(client); err != nil {
				t.Fatalf("%s error: %v", name, err)
			}

			var libs []abs.Library
			if hit, err := store.Get(librariesKey, &libs); hit || err != nil {
				t.Errorf("cache after %s = hit %v, %v; want a miss", name, hit, err)
			}
			if !invalidated {
				t.Errorf("%s did not call OnInvalidate", name)
			}
		})
	}
}

// TestClient_InvalidationWithNilStoreTellsTheInMemoryCaches verifies that a
// client with no store still tells the search cache about a change.
func TestClient_InvalidationWithNilStoreTellsTheInMemoryCaches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), nil)
	invalidated := false
	client.OnInvalidate = func() { invalidated = true }
	if err := client.DeleteItem(context.Background(), "li1", false); err != nil {
		t.Fatalf("DeleteItem error: %v", err)
	}
	if !invalidated {
		t.Error("DeleteItem did not call OnInvalidate")
	}
}

// TestClient_FetchThatEndsAfterAnInvalidationIsNotCached verifies that a
// fetch that started before a change on the server does not put its old
// value in the cache after the change cleared it.
func TestClient_FetchThatEndsAfterAnInvalidationIsNotCached(t *testing.T) {
	var title atomic.Value
	title.Store("old")
	started := make(chan struct{})
	release := make(chan struct{})
	var held atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			title.Store("new")
			w.WriteHeader(http.StatusOK)
			return
		}
		body := `{"id":"li1","media":{"metadata":{"title":"` + title.Load().(string) + `"}}}`
		if held.CompareAndSwap(false, true) {
			close(started)
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), NewStore(openTestDB(t)))
	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		_, err := client.GetLibraryItem(ctx, "li1")
		done <- err
	}()
	<-started
	if err := client.DeleteItem(ctx, "li1", false); err != nil {
		t.Fatalf("DeleteItem error: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first GetLibraryItem error: %v", err)
	}

	item, err := client.GetLibraryItem(ctx, "li1")
	if err != nil {
		t.Fatalf("second GetLibraryItem error: %v", err)
	}
	if got := item.Media.Metadata.Title; got != "new" {
		t.Errorf("title after the change = %q, want new", got)
	}
}
