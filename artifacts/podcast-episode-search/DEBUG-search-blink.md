# Debug: search blink

## Phase 1: Root Cause Investigation
- **Error:** Search in a podcast library blinked/flickered after the new episode-level search behavior was added.
- **Reproduction:** Open search from a podcast library, type a query, and observe a visible delay/flicker before results settle.
- **Recent changes:** Podcast search was changed to expand podcast-show results into episode-level hits.
- **Evidence:** `internal/screens/search/model.go` started calling `resolveSearchLibrary()` on every debounced search. That function fetches `/api/libraries` and runs audio-library filtering before the actual search request.
- **Data flow trace:** `home.NavigateSearchMsg` only carried `libraryID`, so search could not know the selected library type up front. The search model therefore re-resolved library metadata on every query before deciding whether to use podcast episode expansion.

## Phase 2: Pattern Analysis
- **Working example:** The older search path searched directly against the known library without any extra library-resolution round trip on each keystroke.
- **Differences:** The new podcast-aware flow added repeated library discovery in the hot path, unlike the previous direct search flow.
- **Dependencies:** The search screen only needs the selected library's media type once, at navigation time, to choose the correct search strategy.

## Phase 3: Hypothesis
- **Hypothesis:** The flicker is caused by extra network work in the debounced search hot path, not by list rendering itself.
- **Test:** Pass the selected library media type into the search screen when navigating, and skip `resolveSearchLibrary()` for normal in-library searches.
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** Search lacked the selected library media type, so each query re-fetched library metadata and audio-library filtering before executing the real search.
- **Fix:** Extended `NavigateSearchMsg` and `search.New(...)` to carry `libraryMediaType`, then used that cached value to choose between direct book search and podcast episode expansion without repeated library resolution.
- **Test:** `go test ./internal/screens/search/... ./internal/screens/home/... ./internal/app/... -count=1`
- **Verification:** `go test ./... -count=1 && go build -o /tmp/pine ./cmd/`

## Attempts
- Attempt 1: Hypothesis confirmed via code-path comparison; removing repeated library resolution from the hot path resolved the regression.
