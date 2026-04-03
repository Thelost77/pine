# Debug: bookmark enter when stopped doesn't start playback

## Phase 1: Root Cause Investigation
- **Error:** Pressing `Enter` on a focused bookmark while the book is not already playing did nothing.
- **Reproduction:**  
  1. Open a book detail screen with bookmarks loaded.  
  2. Ensure there is no active playback session.  
  3. Focus bookmarks with `Tab` and press `Enter`.  
  4. Observe no playback start and no seek.
- **Recent changes:**  
  - Recent bookmark work made bookmark focus and `Enter` interactions reliable, which exposed that the inactive-playback path still dropped bookmark seeks.
- **Evidence:**  
  - Detail `Enter` on a bookmark emitted `SeekToBookmarkCmd{Time: ...}` with no item context.  
  - Root `handleSeekToBookmark()` returned early on `!m.isPlaying()`.  
  - Existing working cross-track seeking already used `restartPlaybackAt(bookPos)` to start a fresh ABS session and choose the correct track from an explicit book-global position.
- **Data flow trace:**  
  Focused bookmark -> detail emits `SeekToBookmarkCmd` -> app `handleSeekToBookmark()` checks `m.isPlaying()` -> inactive path returns `nil` -> no play session starts -> no mpv launch occurs.

## Phase 2: Pattern Analysis
- **Working example:** `restartPlaybackAt(bookPos)` already restarts playback for cross-track seeks by starting a new ABS session and selecting the track from the requested book-global position rather than trusting ABS `currentTime`.
- **Differences:**  
  - Active bookmark/chapter seeks had a path that could seek or restart.  
  - Inactive bookmark enter had no equivalent start-session path and lacked the item needed to start one.
- **Dependencies:**  
  - Starting playback needs the library item ID.  
  - Multi-track books need track selection based on the requested book-global timestamp, not just server-reported session progress.

## Phase 3: Hypothesis
- **Hypothesis:** Bookmark enter is a no-op when stopped because the command drops item context and the app has no inactive-playback startup path; reusing the explicit-position track-selection pattern from `restartPlaybackAt()` should make bookmark enter start at the correct timestamp.
- **Test:** Add tests that require bookmark enter to carry the item and require inactive bookmark seek to return a `PlaySessionMsg` positioned at the bookmark time for a multi-track book.
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** Inactive bookmark enter had no way to start playback because `SeekToBookmarkCmd` only carried time, and `handleSeekToBookmark()` intentionally ignored requests when no session was active.
- **Fix:**  
  - Extended `SeekToBookmarkCmd` to carry the detail item.  
  - Changed detail bookmark `Enter` to emit item + timestamp.  
  - Added an inactive-playback path that starts a fresh ABS play session and builds playback state from the explicit bookmark position.  
  - Extracted shared book track-selection logic so normal play, restart, and bookmark-start all choose tracks consistently.
- **Test:** Added/updated tests that now pass:
  - `TestEnterKey_SeeksToBookmark`
  - `TestHandleSeekToBookmarkStartsPlaybackWhenStopped`
  - `TestE2E_BookmarkEnterStartsPlaybackWhenStopped`
- **Verification:**  
  - `go test ./internal/screens/detail/... -count=1`  
  - `go test ./internal/app/... -count=1`  
  - `go test ./... -count=1`  
  - `go build -o /tmp/pine-bookmark-start ./cmd/`

## Attempts
- Attempt 1: Hypothesis that inactive bookmark enter should patch ABS progress before starting playback -> rejected as unnecessary after comparing with the existing explicit-position restart path.
- Attempt 2: Hypothesis that a fresh play session can start from the bookmark by reusing explicit track selection and carrying item context -> confirmed and fixed.
