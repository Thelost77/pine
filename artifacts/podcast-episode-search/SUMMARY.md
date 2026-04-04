# Summary: podcast-episode-search

## Goal
- Keep search minimal, but make it actually useful for podcast libraries by searching **episodes**, not podcast show names.

## Final behavior
- In **podcast libraries**, search uses **normalized ranked local matching** over cached episode entries.
- Results render as:
  - **title:** matching episode title
  - **subtitle:** podcast title
- Opening a podcast result keeps the matched episode selected when the full feed loads.
- Search now uses a **per-library in-memory snapshot cache**, so typing no longer rebuilds search data from the network on every query.
- Search is now available from both **Home** and **Library** via `/`.

## Main findings

### 1. Show-level podcast search was not enough
- The first implementation searched ABS for matching **podcast shows**, then expanded those shows into episodes.
- That looked reasonable, but it only worked when the query also matched the **podcast title**.
- This was wrong for cases like:
  - podcast: `Joe Rogan`
  - episode: `Jason...`
  - query: `Jas`

### 2. Episode search must use episode data as the source of truth
- Correct behavior required searching the library's podcast items, expanding each podcast, and filtering on actual episode titles.
- The final implementation in `internal/abs/libraries.go` pages through podcast library items and applies prefix matching directly to episode titles.

### 3. Search hot-path work caused visible blink
- A later regression was not a rendering issue.
- The search screen was re-resolving library metadata on each debounced query because it did not know the selected library media type up front.
- Passing `libraryMediaType` into the search model removed that extra round trip and restored stable search behavior.

### 4. Full podcast scans on each query still caused blink
- Even after removing repeated library resolution, podcast search still rebuilt results by scanning the library and expanding podcasts on every debounced query.
- That meant each typed letter could trigger a full library walk plus item-detail fetches.
- The final fix was to build a lightweight per-library search snapshot once and reuse it for local filtering.

### 5. Cached local matching still needed to be smarter
- The first cache-backed matcher still used prefix/substring rules that were too literal.
- That caused misses like query `100` not matching `$100M...`, and it kept books/podcasts on inconsistent matching rules.
- The final fix normalized cached text, added ranked fuzzy matching, removed the transient Search list title, and wired Library into the same search flow.

## Key implementation points
- `internal/screens/search/model.go`
  - prewarms the selected library snapshot and filters locally while typing
  - treats normalized-empty queries as idle state and never leaks the list title
- `internal/screens/search/cache.go`
  - owns lightweight per-library search snapshots with TTL-based reuse
  - stores only minimal flattened fields needed for result rendering and navigation
  - normalizes cached text and ranks matches with exact/token boosts plus fuzzy fallback
- `internal/screens/library/model.go`
  - adds `/` search entry from the current library context
- `internal/ui/list_item.go`
  - renders episode title + podcast title for podcast search hits
- `internal/screens/detail/model.go`
  - preserves the matched episode selection after opening detail
- `internal/app/model.go`
  - owns the cache so snapshots survive search screen recreation and library switching
- `internal/screens/home/model.go`
  - provides the selected library media type when opening search from Home

## Regressions found and fixed
- **Blink/flicker while typing in podcast search**
  - root cause: repeated library resolution in the debounced search path
  - details: `DEBUG-search-blink.md`
- **Longer prefix stops matching valid episode**
  - root cause: search depended on show-level ABS results before episode filtering
  - details: `DEBUG-episode-prefix-search.md`
- **Blink/flicker still present while typing after episode-search fix**
  - root cause: every query still rebuilt podcast search data from the network
  - details: `DEBUG-search-layout-and-key-handling.md`
- **Cached search still missed obvious queries and flashed `Results`**
  - root cause: punctuation-sensitive prefix/substring matching and transient list rendering before a completed search
  - details: `DEBUG-search-matching-blink-and-library-entry.md`

## Tests covering the final behavior
- `internal/abs/libraries_test.go`
  - podcast episode HTTP search behavior
  - regression where show title does not match episode prefix
- `internal/screens/search/cache_test.go`
  - snapshot reuse across repeated queries
  - cache retention per `libraryID`
  - stale snapshot rebuild behavior
  - punctuation-insensitive numeric matching
  - fuzzy fallback matching
- `internal/screens/search/model_test.go`
  - podcast-library search returns episode hits
  - regression for `Joe Rogan` / `Jason...` style mismatch
  - fixed-height search view and cached local search path
  - whitespace-idle behavior and no transient `Results` title
- `internal/screens/library/model_test.go`
  - `/` emits search navigation with library context
- `internal/app/render_test.go`
  - Library hints advertise `/ search`
- `internal/screens/detail/model_test.go`
  - matched episode remains selected after detail hydration

## Tradeoff
- Podcast search is now more correct and much smoother while typing, but the first snapshot build still does real work to hydrate a library.
- That tradeoff is intentional: pay the cost once per library, then make the interactive search path fast and predictable.

## Progress

### Task 1: per-library search cache
- **Status:** done
- **Commit:** not created in current worktree
- **Deviations:** implemented from the approved session `plan.md` rather than an `artifacts/.../PLAN.md` file
- **Decisions:** cache ownership lives above the search screen, keyed by `libraryID`, with lightweight flattened entries instead of full ABS payloads

### Task 2: local filtering and invalidation
- **Status:** done
- **Commit:** not created in current worktree
- **Deviations:** no additional user-facing refresh control was added
- **Decisions:** snapshots survive library switching, rebuild when stale, and reset on auth/session replacement

### Task 3: normalized fuzzy local matching
- **Status:** done
- **Commit:** not created in current worktree
- **Deviations:** used the already-available `github.com/sahilm/fuzzy` dependency instead of adding a new matcher
- **Decisions:** ranking uses exact normalized substring and token-prefix boosts before fuzzy fallback so short numeric queries still behave predictably

### Task 4: search UI cleanup and library entrypoint
- **Status:** done
- **Commit:** not created in current worktree
- **Deviations:** Search now hides the list title entirely rather than trying to preserve it conditionally
- **Decisions:** `/` opens Search from Library using the same contextual library routing as Home

## Metrics
- Tasks completed: 4/4
- Deviations: 4
