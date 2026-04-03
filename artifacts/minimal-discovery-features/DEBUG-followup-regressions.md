# Debug: followup regressions

## Phase 1: Root Cause Investigation
- **Error:** Home showed an inflated item count, podcast bookmarks lacked episode context, and detail bottom hints omitted queue actions.
- **Reproduction:** On Home with 5 continue-listening items and 2 recently-added items, the list rendered `8 items`. On podcast playback, creating a bookmark produced `Bookmark at <time>` with no episode name. On Detail, the bottom status bar showed play/bookmark/focus/back only.
- **Recent changes:** Home recently switched to synthetic section rows for Recently Added; queue actions were added to detail/root flow but only documented in detail-local help text; bookmark creation remained generic from earlier book-oriented behavior.
- **Evidence:** `internal/screens/home/model.go` still had `l.SetShowStatusBar(true)` while the list now contains synthetic section/empty rows. `internal/app/bookmarks.go` always built bookmark titles from timestamp only. `internal/app/render.go` omitted `a`/`A` and queue count from `viewHints()`.
- **Data flow trace:** Home status count comes from the Bubble list model item count, which now includes synthetic rows. Bookmark titles originate in `handleAddBookmark()`. Bottom-bar hints originate in `viewHints()`, not in detail-local help text.

## Phase 2: Pattern Analysis
- **Working example:** Library uses the list status bar because every row is a real library item. Detail-local help text already documented queue actions correctly. Playback state already tracks `episodeID`, `player.Title`, and queue length.
- **Differences:** Home now mixes interactive item rows with synthetic rows. Podcast bookmark creation did not branch on podcast episode playback. App-level bottom hints had not been updated alongside the queue feature.
- **Dependencies:** Fixing queue visibility needed only root render state; fixing bookmark titles needed current playback episode context; fixing Home count needed the list status bar configuration to match the new row model.

## Phase 3: Hypothesis
- **Hypothesis:** The broken Home count is caused by leaving the list status bar enabled after adding synthetic rows; podcast bookmark ambiguity is caused by `handleAddBookmark()` never including current episode context; queue actions feel broken because the visible bottom-bar hints omit them and provide no queue count.
- **Test:** Add failing regressions for the Home count, podcast bookmark title, and detail bottom hints/queue count.
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** Three separate UI regressions from feature follow-up work: mismatched Home status bar semantics, missing podcast bookmark context, and missing global queue visibility.
- **Fix:** Disabled the Home list status bar, prefixed new podcast bookmark titles with the current episode title, and added `a` / `A` plus queue count to detail bottom hints.
- **Test:** `go test ./internal/screens/home/... ./internal/app/... -count=1`
- **Verification:** `go test ./... -count=1 && go build -o /tmp/pine ./cmd/`

## Post-debug follow-up
- Queue UX was clarified after this debug pass:
  - `A` remains a queue-ordering action ("move as next")
  - `>` was added as the explicit transport action to jump to the next queued item immediately
- Additional regression coverage was added for the new skip-next behavior and visible footer/help hints.

## Attempts
- Attempt 1: Hypothesis confirmed via focused regressions; direct source fixes resolved all three issues.
