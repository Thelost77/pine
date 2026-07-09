package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	fetch := func() error {
		n := calls.Add(1)
		if n == 1 {
			// Leader: announce we're in flight, then block until released.
			close(leaderInflight)
			<-releaseLeader
			return context.Canceled
		}
		// Retry leader: succeed.
		return nil
	}

	ctx := context.Background()

	leaderDone := make(chan error, 1)
	go func() {
		leaderDone <- client.getOrFetch(ctx, "retry-key", fetch)
	}()

	// Wait until the leader is actually in flight so the follower observes the
	// existing inflight call rather than becoming a fresh leader.
	<-leaderInflight

	followerDone := make(chan error, 1)
	go func() {
		followerDone <- client.getOrFetch(ctx, "retry-key", fetch)
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
	if err := <-followerDone; err != nil {
		t.Fatalf("follower getOrFetch err = %v, want nil after retry", err)
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
