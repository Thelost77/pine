# Debug: adding bookmarks doesn't work

## Phase 1: Root Cause Investigation
- **Error:** No crash or HTTP failure. Runtime logs show:
  - `creating bookmark`
  - `POST /api/me/item/{id}/bookmark status=200`
  - immediate `GET /api/me/progress/{id} status=200`
  - `bookmark list refreshed ... count=0`
- **Reproduction:**  
  1. Start playback for a book.  
  2. Press `b` on the detail screen.  
  3. Observe the log sequence above and the bookmark list staying empty.
- **Recent changes:**  
  - `64cfea6` normalized bookmark fetch results from `/api/me/progress/{id}`.  
  - `1ccb36f`, `5b51bf1`, and follow-up fixes improved bookmark state/rendering, but did not change the bookmark read source.
- **Evidence:**  
  - App log around `2026-04-03T21:54:48` shows POST bookmark creation succeeding repeatedly, followed by GET progress succeeding and still returning zero bookmarks.  
  - `internal/app/bookmarks.go` creates bookmarks via `CreateBookmark()` and then refreshes with `GetBookmarks()`.  
  - `internal/abs/bookmarks.go` currently implements `GetBookmarks()` as `GET /api/me/progress/{itemId}`.
- **Data flow trace:**  
  `b` key -> `detail.AddBookmarkCmd` -> `handleAddBookmark()` -> `POST /api/me/item/{id}/bookmark` succeeds -> refresh calls `GET /api/me/progress/{id}` -> UI stays empty because that endpoint is not where ABS stores created bookmarks.

## Phase 2: Pattern Analysis
- **Working example:** Upstream ABS bookmark creation path in `server/models/User.js` writes into `this.bookmarks` via `createBookmark(libraryItemId, time, title)`.
- **Differences:**  
  - ABS `getOldMediaProgress(libraryItemId, episodeId)` returns media progress only.  
  - ABS `toOldJSONForBrowser()` exposes bookmarks separately via top-level `bookmarks`.  
  - pine currently reads bookmarks from progress, not from the top-level user bookmark collection.
- **Dependencies:**  
  - Correct bookmark reads depend on whichever ABS endpoint exposes `user.bookmarks` for the authenticated user.  
  - A fix likely needs a new ABS client read path and updated tests/mocks.

## Phase 3: Hypothesis
- **Hypothesis:** Bookmark creation appears broken because pine refreshes from the wrong ABS endpoint: bookmarks are written to the user's top-level bookmark collection, but pine reads media progress.
- **Test:** Compare runtime logs with upstream ABS server code:
  - logs prove POST bookmark creation succeeds while GET progress returns zero bookmarks,
  - upstream code shows `createBookmark()` mutates `user.bookmarks`, while `getOldMediaProgress()` is a separate path.
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** pine reads bookmarks from `/api/me/progress/{itemId}`, but ABS stores created bookmarks in `user.bookmarks`.
- **Fix:** Switched `GetBookmarks()` to read `/api/me`, decode the authenticated user's top-level bookmark collection, and filter by `libraryItemId`. Updated the bookmark type to carry `libraryItemId`, and aligned unit/E2E mocks and tests with the real ABS contract.
- **Test:** Updated failing coverage that now passes:
  - `TestUserBookmarksDeserialization`
  - `TestGetBookmarksHTTP`
  - `TestHandleDeleteBookmarkReturnsEmptyBookmarksWhenLastBookmarkRemoved`
  - `TestE2E_BookmarkCRUD`
- **Verification:**  
  - `go test ./internal/abs/... -count=1`  
  - `go test ./internal/app/... -count=1`  
  - `go test ./... -count=1`

## Attempts
- Attempt 1: Hypothesis that bookmark creation was failing at the POST boundary -> rejected by logs showing repeated `POST ... status=200`.
- Attempt 2: Hypothesis that pine was refreshing from the wrong ABS data source -> confirmed by logs plus upstream ABS `User.js` code.
