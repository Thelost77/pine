# Summary: bookmarks

## Progress

### Task 1: Classify bookmark fetch outcomes at the ABS/app boundary
- **Status:** done
- **Commit:** 64cfea6 fix(abs): treat missing progress as empty bookmarks
- **Deviations:** none
- **Decisions:** Added an inspectable HTTP status error in the ABS client so bookmark fetching can treat `404` as empty while preserving real failures.

### Task 2: Carry bookmark-empty vs bookmark-error state into the detail screen
- **Status:** done
- **Commit:** 1ccb36f feat(detail): track bookmark load state
- **Deviations:** none
- **Decisions:** Added explicit detail-model state for bookmark load completion and bookmark-specific errors so slice length no longer has to stand in for load outcome.

### Task 3: Render explicit bookmark list, empty state, and load error in the detail view
- **Status:** done
- **Commit:** 5b51bf1 feat(detail): show explicit bookmark states
- **Deviations:** none
- **Decisions:** Bookmark rendering now stays hidden until loading completes, then shows either rows, `No bookmarks yet`, or the bookmark-specific error text. Help text was tightened so non-focusable bookmark states are not advertised as tabbable.

### Task 4: Keep bookmark mutation flows consistent with the new state model
- **Status:** done
- **Commit:** bbcf321 fix(app): keep bookmark refresh states consistent
- **Deviations:** none
- **Decisions:** Successful add/delete operations now surface refresh failures through the bookmark-local update channel instead of the global playback error path.

### Task 5: Add end-to-end coverage for visible bookmark states on detail entry
- **Status:** done
- **Commit:** 3568522 test(app): cover bookmark visibility states
- **Deviations:** none
- **Decisions:** The mock ABS server now models progress-endpoint status overrides so E2E coverage can verify `404` empty-state semantics and non-404 bookmark error rendering.

### Task 6: Final verification
- **Status:** done
- **Commit:** n/a
- **Deviations:** used `go build -o /tmp/pine-bookmarks-build ./cmd/` because `go build ./cmd/` attempts to write a binary named `cmd`, which conflicts with the existing directory name.
- **Decisions:** Full automated verification passed with the adjusted build command and `go test ./... -count=1`.

### Debug follow-up: Disable stale bookmark focus during bookmark error state
- **Status:** done
- **Commit:** e86a5be fix(detail): block stale bookmark focus on errors
- **Deviations:** none
- **Decisions:** Detail-model bookmark focus and actions now use one shared focusability guard so error-state rendering and input handling cannot diverge.

### Debug follow-up: Read bookmarks from the ABS user bookmark collection
- **Status:** done
- **Commit:** 6f42d50 fix(abs): read bookmarks from user profile
- **Deviations:** This superseded the earlier bookmark-source assumption from the bookmark plan after log evidence and upstream ABS code showed created bookmarks are not stored in media progress.
- **Decisions:** `GetBookmarks()` now reads `/api/me` and filters the authenticated user's top-level bookmark collection by `libraryItemId`, with tests and E2E mocks updated to the real ABS contract.

### Debug follow-up: Preserve bookmark timestamp precision on delete
- **Status:** done
- **Commit:** fc7bb8c fix(abs): preserve bookmark time on delete
- **Deviations:** none
- **Decisions:** Delete requests now send the bookmark's exact time value instead of truncating to three decimals, matching ABS exact-time bookmark lookup.

### Debug follow-up: Start playback from bookmark enter when stopped
- **Status:** done
- **Commit:** n/a
- **Deviations:** none
- **Decisions:** Bookmark enter now carries the detail item and starts a fresh book play session at the bookmark's book-global position, reusing the same explicit track-selection logic as cross-track restarts so stopped playback lands on the correct track and offset.

## Metrics
- Tasks completed: 6/6
- Deviations: 1
