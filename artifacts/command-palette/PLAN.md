# Plan: command-palette

## Problem and target
- The app requires users to memorize screen-specific hotkeys for navigation, actions, and playback. This creates cognitive friction.
- The target is a unified, "Search Everywhere" command palette (`Ctrl+P`) that dynamically generates both global navigation actions and screen-contextual actions, while simultaneously returning search results for audiobooks, podcasts, and series from local memory caches for sub-millisecond filtering.

## Fixed decisions
- Trigger via `Ctrl+P`.
- Extend existing `search.Cache` to include series.
- Use `bubbles/list` with non-selectable "header" pseudo-items for visual grouping.
- Active screens will expose their own context actions to the root model via a method.
- Pre-warm the cache on library load to ensure immediate responsiveness.

## Assumptions
- Memory usage for keeping the entire active library's items and series in the search cache is acceptable (this matches the current Search screen's design constraints).

## Non-goals
- Searching for authors (no dedicated author screen in pine yet).
- Asynchronous network searching while typing in the palette (must remain local-cache-first for speed).

## Architecture / delivery shape
- **Data Layer:** `search.Cache` expanded.
- **UI Layer:** `components.Palette` handles list, grouping, and inputs.
- **Integration Layer:** `app/model.go` wires `Ctrl+P`, intercepts all keys, queries the search cache on input change, asks active screens for context actions, and dispatches actions on Enter.

## Tasks / delivery order

### 1. Extend Search Cache for Series [Size: S]
- **Goal:** Ensure series are available in the local in-memory search snapshot.
- **Files:** `internal/screens/search/cache.go`, `internal/screens/search/cache_test.go`
- **Action:** Extend `snapshotEntry` to hold series metadata. Update `ensureSnapshot` to concurrently call `client.GetLibrarySeries`, normalize series text, and append to the snapshot.
- **Edge cases:** Handle server timeout or empty series lists gracefully without failing the entire cache build.
- **TDD:** yes
  - RED: Write test for `Cache.Search` returning series.
  - Verify RED: `go test ./internal/screens/search/...` fails.
  - GREEN: Implement series fetch and text normalization.
  - Verify GREEN: `go test ./internal/screens/search/...` passes.
- **Verify:** Tests pass.

### 2. Build Core Palette Component [Size: M]
- **Goal:** Create the visual overlay component capable of grouped lists and filtering.
- **Files:** `internal/ui/components/palette.go`, `internal/ui/components/palette_test.go`
- **Action:** Implement `Palette` wrapping `textinput` and `list`. Define `PaletteItem` with an `IsHeader` flag. Implement a custom list `Delegate` that skips `IsHeader` items during `Up/Down` navigation.
- **TDD:** yes
  - RED: Write component test asserting `Update` skips header items when moving the cursor.
  - Verify RED: Test fails.
  - GREEN: Implement custom list delegate logic.
  - Verify GREEN: Test passes.
- **Verify:** Component tests pass.

### 3. Root Model Integration & Static Actions [Size: M]
- **Goal:** Connect `Ctrl+P` to open the palette with static global actions.
- **Files:** `internal/app/model.go`, `internal/app/messages.go`, `internal/app/render.go`
- **Action:** Add `paletteOpen` and `palette` to `Model`. Intercept `Ctrl+P` in `Update()` to open the palette. Render via `normalizeOverlayCanvas`. Populate with Go Home, Play/Pause, Sleep Timer, etc.
- **Edge cases:** Ensure palette swallows all background keys (no scrolling the library while typing in the palette).
- **TDD:** no
- **Verify:** Press `Ctrl+P`, ensure modal appears and background input is disabled.

### 4. Context Actions from Active Screen [Size: S]
- **Depends on:** Task 3
- **Goal:** Append screen-specific actions to the palette.
- **Files:** `internal/app/model.go`, `internal/screens/library/model.go`, `internal/screens/detail/model.go`
- **Action:** Create `PaletteContextProvider` interface with `SelectedPaletteActions()`. In root `Update()`, type-assert the active screen and append its actions.
- **Verify:** Open palette on Library screen vs Detail screen, observe correct contextual actions.

### 5. Content Search Integration [Size: M]
- **Depends on:** Task 1, Task 2, Task 3
- **Goal:** Merge search cache results into the palette list.
- **Files:** `internal/app/model.go`
- **Action:** Wire the palette's textinput changes to query `searchCache.Search()`. Map `abs.LibraryItem` results to `PaletteItem`s under the "Content" header.
- **Edge cases:** Handle debouncing; handle an empty cache.
- **Verify:** Type "Dune" in palette, verify books/series appear below actions.

### 6. Execution & Navigation [Size: S]
- **Depends on:** Task 5
- **Goal:** Execute the selected palette item.
- **Files:** `internal/app/model.go`
- **Action:** On `Enter`, dispatch navigation or playback commands for actions. For content items, dispatch navigation to `ScreenDetail`. Close palette.
- **Verify:** Select a book from the palette, verify screen navigates to Detail.

### 7. Cache Pre-Warming [Size: S]
- **Depends on:** Task 1
- **Goal:** Ensure instant feedback by warming the cache immediately.
- **Files:** `internal/app/model.go`
- **Action:** Call `cache.Prepare()` on app init and when active library changes.
- **Verify:** Restart app, press `Ctrl+P` immediately, verify results are populated.

## Product-level verification criteria
- The palette must open instantly without blocking the main thread.
- Fuzzy filtering must respond in sub-millisecond time.
- The UI must visually distinguish between actions and content using clear headers.

## Bug Fix: Palette Height Jitter & Item Wrapping

### Problem
- Dialog height jumped unpredictably when typing search queries — especially noticeable with long episode titles.
- Items in the list wrapped to multiple lines instead of being truncated with "…".
- `screenHeight()` in `navigation.go` did not account for the hints bar, causing the body to be 1 row too tall.

### Root Cause
Two compounding issues in `palette.go`:

1. **Height calculation**: `SetSize` used magic numbers (`height - inputHeight - footerHeight - 4`) that didn't account for the border's `Padding(1, 2)`. Replaced with explicit `paletteChromeHeight = 7` (border2 + padding2 + title1 + input1 + footer1).

2. **Item truncation**: The delegate truncated labels to `l.Width() - 4` (= 60 chars), but the border's inner content area is only 58 chars (64 total width minus borders and padding). The 2-char overflow caused `lipgloss` to word-wrap items, making the dialog taller and shifting layout. Fixed by changing to `l.Width() - 6` (= 58 chars label → 60 chars total item → fits inner area).
