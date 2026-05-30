# Persistent API Cache — Implementation Plan

## Overview

Replace screen-local in-memory caches with a single **persistent disk cache** backed by SQLite. All ABS API responses are stored as gob-encoded blobs with per-endpoint TTLs. Screens render cached data instantly and refresh in the background.

**Branch:** `feature/persistent-api-cache`

---

## Principles

1. **Cache-first, revalidate-after** — every read returns stale data immediately and triggers a background refresh.
2. **Centralized policy** — TTLs and serialization live in one package, not spread across screens.
3. **Zero breaking changes to `abs.Client`** — we wrap it, we don't rewrite it.
4. **Preserve existing UX** — skeletons still show when there is *no* cache; they just appear far less often.

---

## Phase 1 — Database & Cache Package

### Step 1.1 — Add `api_cache` table

**Goal:** Give the cache a place to live.

**Files:**
- `internal/db/db.go`

**Work:**
- Add migration for:
  ```sql
  CREATE TABLE IF NOT EXISTS api_cache (
      cache_key   TEXT PRIMARY KEY,
      data        BLOB NOT NULL,
      cached_at   DATETIME NOT NULL,
      expires_at  DATETIME NOT NULL
  );
  ```
- Add `CREATE INDEX IF NOT EXISTS idx_api_cache_expires ON api_cache(expires_at)` for cheap eviction.

**Verification:**
- `go test ./internal/db/... -v` passes.
- `db_test.go` has a new test that opens the DB and asserts the table exists via `PRAGMA table_info(api_cache)`.

---

### Step 1.2 — Implement `internal/cache/store.go`

**Goal:** Low-level get/put/delete with gob encoding and TTL enforcement.

**Files:**
- `internal/cache/store.go` (new)
- `internal/cache/store_test.go` (new)

**API:**
```go
package cache

type Store struct { db *db.Store }
func NewStore(db *db.Store) *Store

func (s *Store) Get(key string, dest any) (bool, error)
func (s *Store) Put(key string, value any, ttl time.Duration) error
func (s *Store) Delete(key string) error
func (s *Store) EvictExpired() error
```

**Work:**
- `Get` selects `data` by `cache_key` where `expires_at > datetime('now')`. On hit, gob-decode into `dest`.
- `Put` gob-encodes `value`, upserts with `cached_at = now`, `expires_at = now + ttl`.
- `Delete` removes by key.
- `EvictExpired` deletes rows where `expires_at <= datetime('now')`.

**Verification:**
- `go test ./internal/cache/... -v` passes with cases:
  - round-trip get/put
  - expired entry returns miss
  - delete removes entry
  - evict removes only stale rows

---

### Step 1.3 — Add typed cache helpers

**Goal:** Wrap the generic store with convenience methods for the most common ABS response types.

**Files:**
- `internal/cache/helpers.go` (new)
- `internal/cache/helpers_test.go` (new)

**API (example):**
```go
func (s *Store) GetLibraries() ([]abs.Library, bool, error)
func (s *Store) PutLibraries(v []abs.Library, ttl time.Duration) error
func (s *Store) GetPersonalized(libID string) ([]abs.PersonalizedResponse, bool, error)
func (s *Store) PutPersonalized(libID string, v []abs.PersonalizedResponse, ttl time.Duration) error
// ... mirror for LibraryItems, LibrarySeries, SeriesContents,
//     MediaProgress, Bookmarks, Episodes, RecentEpisodes
```

**Work:**
- Each helper builds the deterministic key (`"personalized:" + libID`), delegates to `Get`/`Put`, and handles gob typing.

**Verification:**
- Unit tests for each helper assert correct keys and round-trip fidelity.

---

## Phase 2 — Wrap `abs.Client`

### Step 2.1 — Create `internal/cache/client.go`

**Goal:** Transparent wrapper that caches reads without changing the `abs.Client` interface footprint.

**Files:**
- `internal/cache/client.go` (new)
- `internal/cache/client_test.go` (new)

**API:**
```go
type Client struct {
    inner *abs.Client
    store *Store
}

func NewClient(inner *abs.Client, store *Store) *Client

// Methods mirror abs.Client exactly:
func (c *Client) GetLibraries(ctx context.Context) ([]abs.Library, error)
func (c *Client) GetPersonalized(ctx context.Context, libraryID string) ([]abs.PersonalizedResponse, error)
// ... etc
```

**Work:**
- Every method does:
  1. Try cache hit → return immediately.
  2. On miss, call `c.inner.*`.
  3. On success, write to cache.
  4. Return result.
- Errors are **never** cached.

**TTL map (hard-coded in `client.go` as constants):**

| Method | TTL |
|--------|-----|
| `GetLibraries` | 24h |
| `GetPersonalized` | 5m |
| `GetLibraryItems` | 15m |
| `GetLibrarySeries` | 15m |
| `GetSeriesContents` | 15m |
| `GetRecentEpisodes` | 5m |
| `GetMediaProgress` | 1m |
| `GetBookmarks` | 1m |
| `GetEpisodes` | 15m |
| `GetLibraryItemsBatch` | 15m |

**Verification:**
- Tests use a fake `abs.Client` (or mock HTTP server) and assert:
  - First call hits network, second call hits cache.
  - Cache miss after TTL expiry.
  - Errors are not cached.

---

### Step 2.2 — Wire `cache.Client` into `app.Model`

**Goal:** The root model owns one cached client; screens receive it instead of raw `abs.Client`.

**Files:**
- `internal/app/model.go`
- `cmd/pine/main.go` (or wherever `app.New` is called)

**Work:**
- In `main.go` (or `cmd/pine/main.go`):
  ```go
  cacheStore := cache.NewStore(dbStore)
  cachedClient := cache.NewClient(absClient, cacheStore)
  model := app.New(cfg, dbStore, cachedClient)
  ```
- Change `app.New` signature to accept `*abs.Client` as before (it does not need to know about caching). The caller injects the wrapper.
  - **Alternative:** If `app.New` currently stores `client *abs.Client`, keep the field type as `*abs.Client`. `cache.Client` does not embed `abs.Client`, so we must either:
    - Make `cache.Client` embed `*abs.Client` and add cache methods as overrides, **or**
    - Change `app.Model.client` to an interface that both `*abs.Client` and `*cache.Client` satisfy.
  - **Decision:** Define a narrow interface in `internal/app` (or reuse one if it exists) that covers the methods screens actually call. `cache.Client` implements it; `abs.Client` implements it. This keeps `app.New` unchanged.

**Verification:**
- `go build -o pine .` succeeds.
- `go test ./internal/app/... -count=1` passes (E2E tests still use raw `abs.Client` if they construct models directly).

---

## Phase 3 — Screen Integration

### Step 3.1 — Home screen: cache-first render + background refresh

**Goal:** Home shows cached shelves immediately; fetches in background.

**Files:**
- `internal/screens/home/model.go`
- `internal/screens/home/model_test.go`

**Work:**
- **Remove** `itemCache` and `recentCache` fields from `Model`. The persistent cache replaces them.
- In `fetchPersonalizedCmd()`:
  - The `cache.Client` already returns a cache hit instantly, so the command returns fast.
  - **However**, `tea.Cmd` is blocking for that message. To do *background* refresh, we need a second command that always fetches.
- **Pattern:** Split into two commands:
  1. `fetchPersonalizedCmd` — reads from cache (fast, may return stale).
  2. `refreshPersonalizedCmd` — always hits network, returns `PersonalizedMsg` with a `FromRefresh bool` flag.
- In `Init()`:
  ```go
  return tea.Batch(m.fetchPersonalizedCmd(), m.loadingRevealCmd())
  // Do NOT fire refreshPersonalizedCmd here; wait until after first paint.
  ```
- In `Update`, after receiving the first `PersonalizedMsg` (cache hit), fire `refreshPersonalizedCmd`:
  ```go
  case PersonalizedMsg:
      // ... handle data ...
      if !msg.FromRefresh {
          return m, m.refreshPersonalizedCmd() // background network call
      }
  ```
- If the refresh returns identical data, the screen does not flicker (bubbletea diffing handles this).

**Verification:**
- `go test ./internal/screens/home/... -v` passes.
- New test: mock client with 200 ms delay. First `Init` returns cached data immediately; after `Update` with cache hit, a refresh command is queued; after refresh `PersonalizedMsg`, data updates.

---

### Step 3.2 — Library screen: cache-first + infinite-scroll cache

**Goal:** Library pages are cached per `(libraryID, page)`.

**Files:**
- `internal/screens/library/model.go`
- `internal/screens/library/model_test.go`

**Work:**
- **Remove** `cache map[string]libraryCacheEntry` from `Model`.
- In `fetchLibraryItemsCmd(page, limit)`:
  - `cache.Client.GetLibraryItems` already handles the cache, so the command is unchanged in structure.
  - But to support background refresh, add `refreshLibraryItemsCmd(page, limit)` that bypasses cache.
- In `Init()` / `Configure()`:
  - If `cache.Client` returns a hit for page 0, render instantly and queue a refresh.
  - If miss, show skeletons and fetch normally.
- `maybePrefetch()` already fetches the next page; `cache.Client` makes this free if that page was visited before.

**Verification:**
- `go test ./internal/screens/library/... -v` passes.
- New test: switch library tabs twice. Second switch is instant (cache hit). Prefetch of page 1 is also instant if cached.

---

### Step 3.3 — Search screen: pre-warm + persistent snapshot

**Goal:** Replace the in-memory-only search snapshot with persistent cache so it survives restarts.

**Files:**
- `internal/screens/search/cache.go`
- `internal/screens/search/model.go`

**Work:**
- Currently `search.Cache` stores `snapshots map[string]*librarySnapshot` in memory with a 15 min TTL.
- **Keep** the in-memory `snapshots` map for fast filtering (it holds tokenized/searchable structs, not raw API responses).
- **Add** persistence: when a snapshot is built successfully, serialize the raw `[]abs.LibraryItem` / series data to the disk cache. On startup, if a persisted snapshot exists and is unexpired, rebuild the in-memory index from it instead of fetching.
- Alternatively, persist the already-tokenized `[]snapshotEntry` slice as gob. This makes restore instant (no re-tokenization).
- **Decision:** Persist `[]snapshotEntry` (the tokenized form) because it is the expensive part.
- Change `Cache` to accept `*cache.Store`.
- In `ensureSnapshot`:
  1. Check in-memory map (fastest).
  2. If miss, check disk cache for `[]snapshotEntry`.
  3. If disk hit, restore in-memory map and return.
  4. If miss, build from network, then persist to disk and memory.

**Verification:**
- `go test ./internal/screens/search/... -v` passes.
- New test: build a snapshot, create a new `Cache` instance (simulating restart), assert `ensureSnapshot` returns without network calls.

---

### Step 3.4 — Detail screen: cache media progress + bookmarks

**Goal:** Detail opens instantly with stale data, refreshes bookmarks in background.

**Files:**
- `internal/screens/detail/model.go`
- `internal/app/bookmarks.go` (if bookmark fetching lives here)

**Work:**
- Detail currently fetches `GetMediaProgress` + `GetBookmarks` + `GetEpisodes` every time.
- `cache.Client` already caches these. The screen's `Init()` command will return cached data immediately.
- Add a background refresh command similar to Home (fire after cache-hit `DetailMsg`).
- For bookmarks specifically: because the user can create/delete bookmarks inside the detail screen, **invalidate** the bookmark cache key on any bookmark mutation.

**Verification:**
- `go test ./internal/screens/detail/... -v` passes.
- New test: open detail twice. Second open is instant. After adding a bookmark, next open fetches fresh bookmarks.

---

### Step 3.5 — SeriesList & Series screens

**Goal:** Same pattern — cache pages, render instantly, refresh in background.

**Files:**
- `internal/screens/serieslist/model.go`
- `internal/screens/series/model.go`

**Work:**
- These screens currently have no manual cache. `cache.Client` gives them persistence for free once the client is injected.
- Add background refresh commands in their `Update` handlers after cache hits.

**Verification:**
- `go test ./internal/screens/serieslist/... -v` and `./internal/screens/series/... -v` pass.

---

## Phase 4 — Cleanup

### Step 4.1 — Remove obsolete in-memory caches

**Goal:** No dead code.

**Files:**
- `internal/screens/home/model.go`
- `internal/screens/library/model.go`

**Work:**
- Delete `itemCache`, `recentCache` from Home `Model`.
- Delete `cache map[string]libraryCacheEntry` from Library `Model`.
- Delete `storeCurrentLibraryCache` and `applyCachedLibrary` from Library.
- Delete `limitItems` and `dedupeRecentlyAdded` from Home if they are no longer needed (they are still needed for slicing the API response, so keep them).

**Verification:**
- `go vet ./...` passes.
- `golangci-lint run` passes.

---

### Step 4.2 — Evict on write operations

**Goal:** Mutations do not leave stale cache entries.

**Files:**
- `internal/app/bookmarks.go`
- `internal/app/playback.go` (progress sync)
- `internal/cache/store.go`

**Work:**
- After `CreateBookmark` / `UpdateBookmark` / `DeleteBookmark` → `cacheStore.Delete("bookmarks:" + itemID)`.
- After `UpdateProgress` / `UpdateEpisodeProgress` → `cacheStore.Delete("progress:" + itemID)` and `cacheStore.Delete("personalized:" + libID)` (because continue-listening changes).
- After `CloseSession` with final progress → same as above.

**Verification:**
- Unit tests for bookmark/playback code assert that the cache store receives `Delete` calls.

---

## Phase 5 — Background Revalidation Polish

### Step 5.1 — Deduplicate in-flight refreshes

**Goal:** Do not fire 3 network requests because the user spammed Tab.

**Files:**
- `internal/cache/client.go`

**Work:**
- Add an in-flight request tracker (map[string]chan struct{}) inside `cache.Client`.
- If a refresh for key `"personalized:lib-1"` is already in flight, wait on that channel instead of starting a second HTTP request.
- On completion, broadcast to waiters and delete the entry.

**Verification:**
- Test: two concurrent `GetPersonalized` calls for the same library result in exactly one HTTP request.

---

### Step 5.2 — Periodic eviction

**Goal:** Prevent the SQLite file from growing unbounded.

**Files:**
- `internal/app/model.go`

**Work:**
- On `SyncTickMsg` (or a new `CacheEvictTickMsg` every 5 minutes), call `cacheStore.EvictExpired()`.

**Verification:**
- No explicit test needed; just verify the call site exists and compiles.

---

## Phase 6 — Testing & Validation

### Step 6.1 — Unit tests for cache package

**Files:**
- `internal/cache/*_test.go`

**Criteria:**
- 100 % statement coverage on `store.go` and `helpers.go`.
- Concurrent access test for `client.go` in-flight deduplication.

---

### Step 6.2 — Screen regression tests

**Files:**
- `internal/screens/home/model_test.go`
- `internal/screens/library/model_test.go`
- `internal/screens/detail/model_test.go`
- `internal/screens/search/*_test.go`

**Criteria:**
- All existing tests pass without modification (they prove caching is transparent).
- New tests added for cache-hit paths.

---

### Step 6.3 — E2E validation

**Files:**
- `internal/app/e2e_test.go`

**Criteria:**
- `go test ./internal/app/... -v` passes.
- Add one E2E test that simulates:
  1. Open Home (fetch from network).
  2. Restart app (new model instance, same DB).
  3. Open Home again — asserts no `GetPersonalized` HTTP request is made on second open.

---

### Step 6.4 — Manual UAT checklist

Run the real binary against an ABS server:

- [ ] Cold start: skeletons appear, then shelves load.
- [ ] Warm start: shelves appear instantly, no skeletons.
- [ ] Tab through 3 libraries: first visit has skeletons; revisits are instant.
- [ ] Scroll library to page 2, switch library, return: page 1 is instant, page 2 is instant.
- [ ] Open detail for an item: description shows instantly; bookmarks refresh in background.
- [ ] Add a bookmark: bookmark list updates; go back and reopen detail — fresh bookmarks shown.
- [ ] Search: first search after startup is fast (snapshot restored from disk).

---

## File Inventory

### New files
- `internal/cache/store.go`
- `internal/cache/store_test.go`
- `internal/cache/helpers.go`
- `internal/cache/helpers_test.go`
- `internal/cache/client.go`
- `internal/cache/client_test.go`
- `artifacts/caching/PLAN.md`

### Modified files
- `internal/db/db.go` — migration
- `internal/db/db_test.go` — migration test
- `cmd/pine/main.go` — wire `cache.NewClient`
- `internal/app/model.go` — periodic eviction, detail/bookmark invalidation
- `internal/app/bookmarks.go` — cache invalidation on mutation
- `internal/app/playback.go` — cache invalidation on progress sync
- `internal/screens/home/model.go` — remove manual caches, add background refresh
- `internal/screens/home/model_test.go` — new cache-hit tests
- `internal/screens/library/model.go` — remove manual cache, add background refresh
- `internal/screens/library/model_test.go` — new cache-hit tests
- `internal/screens/search/cache.go` — persist tokenized snapshots
- `internal/screens/search/model.go` — wire disk cache
- `internal/screens/detail/model.go` — background refresh after cache hit
- `internal/screens/detail/model_test.go` — new tests
- `internal/screens/serieslist/model.go` — background refresh
- `internal/screens/series/model.go` — background refresh
- `internal/app/e2e_test.go` — new restart-resilience test

---

## Risk & Mitigations

| Risk | Mitigation |
|------|------------|
| gob encoding breaks when `abs` types change | Add a `cache_version` column or prefix keys with `"v1:"`. Bumping version invalidates all old entries. |
| SQLite file grows large | `EvictExpired()` runs every 5 min. If still large, add a `PRAGMA auto_vacuum=INCREMENTAL` migration later. |
| Stale data feels buggy | Short TTLs on personalized/progress (1–5 min). Background refresh always fetches fresh data. |
| Background refresh floods server on tab spam | In-flight deduplication in `cache.Client` prevents duplicate requests. |
| E2E tests use raw `abs.Client` | Keep `abs.Client` intact; E2E tests continue to work. Only `main.go` injects the wrapper. |

---

## Estimation

| Phase | Complexity | Rough effort |
|-------|-----------|--------------|
| 1.1 – DB migration | Low | 30 min |
| 1.2 – Store package | Medium | 2 h |
| 1.3 – Typed helpers | Low | 1 h |
| 2.1 – Client wrapper | Medium | 3 h |
| 2.2 – Wire into app | Low | 30 min |
| 3.1 – Home screen | Medium | 2 h |
| 3.2 – Library screen | Medium | 2 h |
| 3.3 – Search screen | Medium | 2 h |
| 3.4 – Detail screen | Low | 1 h |
| 3.5 – Series screens | Low | 1 h |
| 4.1 – Cleanup | Low | 1 h |
| 4.2 – Eviction on write | Low | 1 h |
| 5.1 – In-flight dedup | Medium | 1.5 h |
| 5.2 – Periodic eviction | Low | 30 min |
| 6.x – Tests | Medium | 3 h |
| **Total** | | **~22 h** |
