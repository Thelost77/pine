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
	if err := store.PutLibraries([]abs.Library{{ID: "old"}}, 1*time.Millisecond); err != nil {
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

func TestClient_ConcurrentDeduplication(t *testing.T) {
	var requestCount int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
	}))
	defer srv.Close()

	dbStore := openTestDB(t)
	store := NewStore(dbStore)
	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, store)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.GetLibraries(context.Background())
		}()
	}
	wg.Wait()

	mu.Lock()
	count := requestCount
	mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 request for 5 concurrent calls, got %d", count)
	}
}

func TestGetOrFetch_RetryOnLeaderCancellation(t *testing.T) {
	inner := abs.NewClient("http://127.0.0.1:0", "tok")
	client := NewClient(inner, nil)

	var calls atomic.Int32
	leaderInflight := make(chan struct{})
	releaseLeader := make(chan struct{})

	fetch := func() (any, error) {
		n := calls.Add(1)
		if n == 1 {
			// Leader: announce we're in flight, then block until released.
			close(leaderInflight)
			<-releaseLeader
			return nil, context.Canceled
		}
		// Retry leader: succeed.
		return "retried", nil
	}

	ctx := context.Background()

	leaderDone := make(chan error, 1)
	go func() {
		_, err := client.getOrFetch(ctx, "retry-key", fetch)
		leaderDone <- err
	}()

	// Wait until the leader is actually in flight so the follower observes the
	// existing inflight call rather than becoming a fresh leader.
	<-leaderInflight

	type result struct {
		val any
		err error
	}
	followerDone := make(chan result, 1)
	go func() {
		val, err := client.getOrFetch(ctx, "retry-key", fetch)
		followerDone <- result{val, err}
	}()

	// Give the follower time to park on the leader's call.done channel.
	time.Sleep(20 * time.Millisecond)

	// Release the leader: it returns context.Canceled. Its defer deletes the
	// inflight entry before closing call.done, so the follower's retry becomes a
	// new leader instead of re-joining a stale call.
	close(releaseLeader)

	if err := <-leaderDone; err != context.Canceled {
		t.Fatalf("leader getOrFetch err = %v, want context.Canceled", err)
	}
	if got := <-followerDone; got.err != nil || got.val != "retried" {
		t.Fatalf("follower getOrFetch = %v, %v; want the value of the retry", got.val, got.err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("fetch invoked %d times, want 2 (leader + one retry)", got)
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

func TestClient_GetRecentlyAdded_CacheHit(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/libraries/lib1/personalized":
			_, _ = w.Write([]byte(`[{"id":"recently-added","entities":[{"id":"li1","libraryId":"lib1","addedAt":2,"mediaType":"book","media":{"metadata":{"title":"A"}}}]}]`))
		case "/api/libraries/lib2/personalized":
			_, _ = w.Write([]byte(`[{"id":"recently-added","entities":[{"id":"li2","libraryId":"lib2","addedAt":1,"mediaType":"podcast","media":{"metadata":{"title":"B"}}}]}]`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	dbStore := openTestDB(t)
	store := NewStore(dbStore)
	inner := abs.NewClient(srv.URL, "tok")
	client := NewClient(inner, store)

	libs := []abs.Library{{ID: "lib1"}, {ID: "lib2"}}

	items1, err := client.GetRecentlyAdded(context.Background(), libs)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if len(items1) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items1))
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests after first call, got %d", requestCount)
	}

	items2, err := client.GetRecentlyAdded(context.Background(), libs)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(items2) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items2))
	}
	if requestCount != 2 {
		t.Fatalf("expected still 2 requests after cache hit, got %d", requestCount)
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

// TestClient_WaitersGetDataWhenCacheFails verifies that the callers that wait
// for the request of a different caller get its data, also if the cache
// cannot keep the data.
func TestClient_WaitersGetDataWhenCacheFails(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"libraries":[{"id":"lib1","name":"Books","mediaType":"book"}]}`))
	}))
	defer srv.Close()

	client := NewClient(abs.NewClient(srv.URL, "tok"), closedStore(t))

	errs := make(chan error, 5)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			libs, err := client.GetLibraries(context.Background())
			if err == nil && len(libs) != 1 {
				err = fmt.Errorf("got %d libraries, want 1", len(libs))
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("GetLibraries() error: %v", err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("%d requests for 5 parallel calls, want 1", got)
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
