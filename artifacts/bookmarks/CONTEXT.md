# Context: bookmarks

## Implementation Decisions
### Bookmark source of truth
- **Decision:** Use the Audiobookshelf progress endpoint as the primary bookmark source, with fallback behavior only when the response is unusable.
- **Rationale:** The current client and tests already center bookmark reads on progress data, so this preserves the existing architecture. Adding guarded fallback behavior reduces fragility without prematurely redesigning the bookmark model.
- **Options considered:** progress endpoint only; progress endpoint with fallback; broader progress-model redesign

### 404 handling
- **Decision:** Interpret `404` from the progress endpoint as a valid empty bookmark state rather than a user-facing error.
- **Rationale:** External API scouting showed Audiobookshelf returns `404` when no media-progress record exists. For the detail screen, that maps naturally to “no bookmarks yet” rather than a broken screen state.
- **Options considered:** treat `404` as empty state; treat `404` as visible load error

### Bookmark load failure behavior
- **Decision:** Show a visible, non-blocking bookmark-specific error for non-404 failures or malformed bookmark responses.
- **Rationale:** The current code collapses all fetch failures into an empty list, which makes real API or parsing problems indistinguishable from a legitimately empty item. A local, non-blocking error keeps the detail screen usable while making the failure diagnosable.
- **Options considered:** silent empty state; bookmark-local non-blocking error; global error banner only

### Scope boundary
- **Decision:** Make bookmark-loading behavior consistent for every bookmark-capable item type, not just books.
- **Rationale:** The current navigation/fetch path is already shared, and bookmarks are item-scoped product data rather than a book-only concept. Fixing the behavior consistently avoids duplicated logic and future divergence.
- **Options considered:** books only; all bookmark-capable item types

### Empty-state UX
- **Decision:** Show a visible empty bookmark section when an item supports bookmarks but has none.
- **Rationale:** Hiding the section entirely makes “supported but empty” indistinguishable from “bookmarks never loaded” or “this screen doesn't support bookmarks.” A clear empty state improves discoverability and complements the explicit error state.
- **Options considered:** visible empty-state section; hide section until bookmarks exist

## Existing Code Insights
- This is a **brownfield** change: bookmark creation, deletion, seeking, and fetch-on-detail-entry already exist in the client and test suite.
- Detail navigation from multiple screens already triggers bookmark fetching immediately when entering the detail view.
- The detail screen already owns bookmark list state, bookmark focus, and bookmark rendering, and only shows the section when the list is non-empty.
- Current bookmark fetch behavior silently converts fetch errors into an empty bookmark list, which is likely why “loading failed” can look like “bookmarks never appear.”
- The current API client reads bookmarks from `GET /api/me/progress/{itemId}` by deserializing a progress payload that includes a `bookmarks` array.
- Existing mock-server and end-to-end coverage already exercise successful bookmark fetch on detail entry, bookmark creation, bookmark seeking, and bookmark deletion.
- Audiobookshelf’s `GET /api/me/progress/:id/:episodeId?` returns `404` when no media-progress record exists, so absence of progress must not automatically be treated as a broken bookmark load.

## Integration Points
- ABS client bookmark fetch logic and response parsing
- Root app navigation path that loads bookmark data on detail entry
- Detail-screen state for bookmark data, bookmark error state, and empty-state visibility
- Detail-screen rendering and help text for bookmarks
- Root/app-level error handling only insofar as bookmark fetches should stop using silent failure for non-404 cases
- Existing unit tests for bookmark API parsing and detail view behavior
- Existing end-to-end tests that cover detail entry and bookmark interactions

## Deferred Ideas
- Bookmark editing/renaming remains out of scope.
- Any broader redesign of how progress and bookmarks are modeled across the client is deferred.
- If external ABS responses expose a richer alternate bookmark source later, that can be evaluated after the immediate visibility bug is fixed.
