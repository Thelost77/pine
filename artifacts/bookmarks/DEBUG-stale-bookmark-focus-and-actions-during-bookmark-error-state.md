# Debug: stale bookmark focus and actions during bookmark error state

## Phase 1: Root Cause Investigation
- **Error:** No runtime crash. Verification found inconsistent behavior: the detail view rendered a bookmark-load error, but bookmark focus and actions could still remain active using stale bookmark rows.
- **Reproduction:**  
  1. Start from a detail model with bookmarks loaded.  
  2. Focus bookmarks with `Tab`.  
  3. Deliver `BookmarksUpdatedMsg{Err: ...}` to simulate a bookmark refresh failure.  
  4. Observe that bookmark focus remains active, and `Tab` can refocus bookmarks even though the error state is visible.
- **Recent changes:**  
  - `1ccb36f` introduced explicit bookmark load/error state in the detail model.  
  - `5b51bf1` introduced explicit bookmark empty/error rendering in the detail view.
- **Evidence:**  
  - `internal/screens/detail/model.go:262-273` stored `bookmarkLoadErr` but only cleared bookmark focus when `len(m.bookmarks) == 0`.  
  - `internal/screens/detail/model.go:322,335,352,371,397,403` gated bookmark interactions and focus cycling on bookmark count only.  
  - `internal/screens/detail/view.go:149-177` already treated `bookmarkLoadErr != nil` as disabling bookmark help/focus hints.  
  - This created split-brain behavior between rendering/help and interaction state.
- **Data flow trace:**  
  `BookmarksUpdatedMsg{Err}` -> detail model stores `bookmarkLoadErr` but preserves old `m.bookmarks` -> focus/action guards still see `len(m.bookmarks) > 0` -> stale bookmark interactions remain available under the error state.

## Phase 2: Pattern Analysis
- **Working example:** `internal/screens/detail/view.go:149-177` already had the correct guard concept via `hasBookmarkFocus := m.bookmarkLoadErr == nil && len(m.bookmarks) > 0`.
- **Differences:**  
  - Rendering/help respected the error state.  
  - Model update, key handlers, and `cycleFocus()` did not.
- **Dependencies:**  
  - Bookmark focusability depends on both bookmark count and absence of a bookmark load error.  
  - The fix had to preserve existing bookmark rows for recovery while still disabling interactions during the error state.

## Phase 3: Hypothesis
- **Hypothesis:** The root cause is that bookmark interaction logic keys off stale bookmark count instead of a shared “bookmark focusable” rule, so error-state rendering and input handling diverge.
- **Test:** Add focused detail-model tests that:
  1. verify bookmark focus clears when `BookmarksUpdatedMsg{Err}` arrives,
  2. verify `Tab` cannot refocus bookmarks while the bookmark error is active.
- **Result:** confirmed  
  - `TestBookmarksUpdatedMsg_ErrorDisablesBookmarkFocus` failed  
  - `TestBookmarkErrorState_DoesNotAllowRefocus` failed

## Phase 4: Fix
- **Root cause:** Bookmark error state and bookmark interaction state used different rules.
- **Fix:** Added a single shared `hasFocusableBookmarks()` guard in the detail model and used it to:
  - clear bookmark focus when an error is active,
  - block bookmark seek/delete/up/down handling during error state,
  - block focus cycling into bookmarks during error state.
- **Test:** Added failing tests that now pass:
  - `TestBookmarksUpdatedMsg_ErrorDisablesBookmarkFocus`
  - `TestBookmarkErrorState_DoesNotAllowRefocus`
- **Verification:**  
  - `go test ./internal/screens/detail/... -count=1`  
  - `go test ./internal/app/... -count=1`  
  - `go test ./... -count=1`

## Attempts
- Attempt 1: Hypothesis that stale bookmark interactions survived because model guards only checked bookmark count, not bookmark error state -> confirmed and fixed.
