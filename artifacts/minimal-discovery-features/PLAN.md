# Plan: minimal-discovery-features

## Problem and target
- Add three minimal discovery/navigation features without turning pine into a dashboard or metadata browser:
  1. **Recently Added** as a tiny subsection under Continue Listening
  2. **Queue** as session-only manual “play next / add to queue”
  3. **Series navigation** as a compact row in book detail opening a dedicated Series screen
- Good looks like:
  - home still shows **one list at a time**
  - detail stays compact
  - queue remains mostly invisible until used
  - playback flow remains reliable when queue and manual play interact

## Fixed decisions
- **Recently Added scope:** a small secondary subsection under **Continue Listening** for the current home library/context
- **Home behavior:** keep the current home/list interaction; Recently Added is **not** a separate source or tab
- **Recently Added cap:** **3 items**
- **Recently Added dedupe:** exclude titles already present in Continue Listening
- **Queue scope:** **session-only**, in memory
- **Queue entry points:** **Detail screen only** for books and podcast episodes
- **Queue continuity:** manual play does **not** clear the queue
- **Series UX:** compact `Series: Name #N` row in book detail
- **Series navigation surface:** opens a dedicated **Series** screen
- **Series loading:** fetch enriched book detail in the background when entering book detail

## Assumptions
- ABS already exposes enough data for:
  - a usable “recently added” source, either via personalized shelves or another library endpoint
  - series identifiers plus ordered books for a series
- `GetLibraryItem` can be extended or supplemented without breaking existing podcast episode hydration
- Queue progression should trigger only on **natural playback completion**, not on every stop/quit path

## Non-goals
- Queue persistence across restarts
- Queue actions from home/library/search
- Auto-enqueue behavior
- Playlist management
- General metadata browsing (authors, narrators, collections, tags)
- Expanding detail into a multi-section metadata screen

## Architecture / delivery shape
- **ABS client layer**
  - extend `internal/abs/types.go` and `internal/abs/libraries.go`
  - add response types/endpoints for:
    - Recently Added source
    - enriched book detail series metadata
    - series detail / ordered series items
- **Home screen**
  - extend `internal/screens/home/model.go` to fetch and prepare a tiny Recently Added subsection alongside Continue Listening
  - keep Continue Listening as the main list and render Recently Added as secondary content
- **Detail screen**
  - extend detail model/view with:
    - queue actions for book / selected episode
    - optional compact series row for books
    - focus/enter handling for that row without breaking bookmarks/episodes
- **Navigation**
  - add a new `ScreenSeries` path in `internal/app/messages.go`, `navigation.go`, `render.go`, and root `model.go`
- **Playback / queue**
  - queue lives in root app state because playback lifecycle already lives there
  - queue advancement hooks into playback completion path, not screen models

## Tasks / delivery order

### 1. Extend ABS client contracts for Recently Added and series [size: M]
- **Goal:** make the API layer capable of supplying all three features cleanly
- **Files:** `internal/abs/types.go`, `internal/abs/libraries.go`, related tests
- **Action:**
  - add types for home source data and series metadata
  - add client methods for:
    - merged Recently Added retrieval
    - enriched item detail with series info
    - fetching a series with ordered books
  - keep existing podcast/item behavior intact
- **Edge cases:**
  - item has no series
  - ABS returns partial/missing series ordering data
  - Recently Added source exists for some libraries but not others
- **TDD:** yes
  - RED: write client tests for Recently Added and series responses
  - Verify RED: `go test ./internal/abs/... -count=1` fails for missing methods/types
  - GREEN: implement minimal client/types
  - Verify GREEN: `go test ./internal/abs/... -count=1` passes
  - REFACTOR: normalize shared decode/helpers only if response handling gets repetitive
- **Verify:** client tests cover no-series and multi-item series cases
- **Done:** app code can request all needed data without placeholder contracts

### 2. Refactor home to support a Recently Added subsection [size: M]
- **Depends on:** Task 1
- **Goal:** preserve the current high-signal home while adding a small Recently Added subsection cleanly
- **Files:** `internal/screens/home/model.go`, `internal/screens/home/view.go`, home tests, possibly app wiring if source messaging changes
- **Action:**
  - keep continue-listening behavior for audiobook/podcast libraries
  - fetch Recently Added alongside Continue Listening for the active library/context
  - render a tiny Recently Added subsection beneath the main list
  - exclude titles already present in Continue Listening
  - cap the subsection at 3 items
- **Edge cases:**
  - no continue-listening and no recently added items
  - only one library type present
  - dedupe by title across mixed media/library contexts
- **TDD:** yes
  - RED: add home-model/view tests for Recently Added subsection rendering, duplicate exclusion, and 3-item cap
  - Verify RED: `go test ./internal/screens/home/... -count=1` fails for the intended behavior
  - GREEN: implement fetch wiring plus subsection preparation/rendering
  - Verify GREEN: `go test ./internal/screens/home/... -count=1` passes
  - REFACTOR: extract subsection preparation helpers if dedupe/cap logic starts cluttering the model
- **Verify:** home remains focused on Continue Listening and Recently Added stays visually secondary
- **Done:** home shows a small deduplicated Recently Added subsection without creating a new navigation mode

### 3. Add root queue state and detail-level queue actions [size: M]
- **Depends on:** Task 1
- **Goal:** support manual queueing without adding a new management surface
- **Files:** `internal/app/model.go`, `internal/app/messages.go`, `internal/app/playback.go`, `internal/screens/detail/model.go`, `internal/screens/detail/view.go`, tests
- **Action:**
  - define a queue entry type in root app state (book vs podcast episode)
  - add detail messages/actions for **play next** and **add to queue**
  - expose minimal UI/hints from detail only
  - ensure manual play leaves queue intact
- **Edge cases:**
  - duplicate enqueue of the same item/episode
  - queue actions on currently playing item
  - queue actions when nothing is playing yet
- **TDD:** yes
  - RED: add app/detail tests for queue insertion and manual-play-preserves-queue behavior
  - Verify RED: `go test ./internal/app/... ./internal/screens/detail/... -count=1` fails for missing queue behavior
  - GREEN: implement root queue state and detail action wiring
  - Verify GREEN: `go test ./internal/app/... ./internal/screens/detail/... -count=1` passes
  - REFACTOR: extract queue-entry helpers if book/episode branching duplicates logic
- **Verify:** queue actions exist only in detail contexts and do not clutter other screens
- **Done:** user can queue from detail for books and selected podcast episodes

### 4. Add enriched book detail and dedicated Series screen [size: M]
- **Depends on:** Task 1
- **Goal:** turn the series concept into compact navigation, not metadata sprawl
- **Files:** `internal/screens/detail/*`, new `internal/screens/series/*`, `internal/app/messages.go`, `internal/app/model.go`, `internal/app/navigation.go`, `internal/app/render.go`, tests
- **Action:**
  - fetch enriched book detail when opening book detail
  - show a compact series row only when series data exists
  - add `ScreenSeries` and a dedicated list model
  - open the series screen from the row, highlighting the current book
- **Edge cases:**
  - book without series
  - series fetch failure after detail already rendered
  - current book absent from returned ordering
- **TDD:** yes
  - RED: add detail and navigation tests for series-row visibility/opening
  - Verify RED: `go test ./internal/screens/detail/... ./internal/app/... -count=1` fails for missing series flow
  - GREEN: implement enrichment + screen routing + series list
  - Verify GREEN: `go test ./internal/screens/detail/... ./internal/app/... -count=1` passes
  - REFACTOR: move any series-formatting helpers out of detail if they start bloating the view
- **Verify:** detail remains compact; series appears only when available; Series screen shows ordered neighbors
- **Done:** user can go from a book detail to an ordered series list without extra metadata clutter

### 5. Hook queue advancement into playback completion and run integration coverage [size: M]
- **Depends on:** Task 3
- **Goal:** make queue actually useful in the listening lifecycle
- **Files:** `internal/app/playback.go`, app E2E tests, possibly player/app helpers
- **Action:**
  - detect natural playback completion and start the next queued item
  - avoid advancing the queue on manual stop/quit/navigation
  - cover manual-play-with-queue semantics and queue consumption order
- **Edge cases:**
  - queued podcast episode after book completion
  - empty queue on playback completion
  - playback error vs true completion
  - sleep timer/manual stop should not accidentally consume queue
- **TDD:** yes
  - RED: add app/E2E tests for natural completion -> next queued item
  - Verify RED: `go test ./internal/app/... -count=1` fails for missing queue advancement
  - GREEN: implement queue consumption in playback lifecycle
  - Verify GREEN: `go test ./internal/app/... -count=1` passes
  - REFACTOR: tighten completion detection if the first implementation over-couples to existing stop paths
- **Verify:** queue advances only on true completion and preserves order
- **Done:** queue behaves predictably under real playback transitions

## Product-level verification criteria
- Home still renders **Continue Listening** as the primary high-signal surface
- Recently Added adds value without becoming a separate browsing mode
- Detail screen remains compact and readable
- Queue is usable without introducing a dedicated management UI
- Series navigation answers “what comes next?” in one step from detail
- Playback reliability is unchanged for users who never touch queue or series

## Open implementation questions
- None product-blocking; the remaining work is implementation detail around the exact ABS response shapes
