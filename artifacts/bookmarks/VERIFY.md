# Verify: bookmarks

## Code Review

### Spec Compliance
- [PASS] Progress-endpoint `404` is treated as an empty bookmark state — `internal/abs/bookmarks.go:18-32`
- [PASS] Detail rendering now distinguishes populated, empty, and error bookmark outcomes — `internal/screens/detail/view.go:115-178`
- [PASS] Initial-load and mutation refresh paths both flow through bookmark-local update messages instead of silently collapsing failures — `internal/app/bookmarks.go:32-88`
- [ISSUE] Bookmark error state can still leave stale bookmark focus and actions active after a refresh failure — `internal/screens/detail/model.go:262-405`

### Code Quality
- [PASS] Responsibilities remain separated cleanly across ABS client normalization, root app fetch/mutation wiring, and detail-screen state/rendering.
- [PASS] Tests follow the current repo patterns and avoid the loaded anti-patterns; they exercise real behavior at unit and E2E layers rather than asserting on mock-only artifacts.
- [ISSUE] Bookmark interaction guards in the detail model are inconsistent with the new error-state rendering rules, which creates split-brain UI behavior — `internal/screens/detail/model.go:271,322,335,352,371,397,403`

### Review Issues
| # | File:Line | Severity | Issue | Fix |
|---|-----------|----------|-------|-----|
| 1 | `internal/screens/detail/model.go:271,322,335,352,371,397,403` | Important | After a bookmark refresh error, the view shows an error state but bookmark focus, seek, delete, and focus-cycling can still use stale bookmark data because those guards only check `len(m.bookmarks) > 0`. | Clear bookmark focus when `bookmarkLoadErr` is set, and gate bookmark interactions/focus-cycling on `bookmarkLoadErr == nil` in the same way the help text already does. |

## Checklist

| # | Check | Source | Result | Evidence |
|---|-------|--------|--------|----------|
| 1 | ABS bookmark fetch semantics (`404` empty, malformed/non-404 error) | Task 1 | PASS | `go test ./internal/abs/... -count=1` -> `ok github.com/Thelost77/pine/internal/abs 0.010s` |
| 2 | Detail model carries bookmark success/empty/error state | Task 2 | PASS | `go test ./internal/screens/detail/... -count=1` -> `ok github.com/Thelost77/pine/internal/screens/detail 0.005s` |
| 3 | Detail view renders explicit bookmark list/empty/error states | Task 3 | PASS | `go test ./internal/screens/detail/... -count=1` -> `ok github.com/Thelost77/pine/internal/screens/detail 0.005s` |
| 4 | Bookmark mutation refreshes preserve empty/error semantics | Task 4 | PASS | `go test ./internal/app/... -count=1` -> `ok github.com/Thelost77/pine/internal/app 3.989s` |
| 5 | E2E coverage for detail-entry bookmark visibility states | Task 5 | PASS | `go test ./internal/app/... -count=1` -> `ok github.com/Thelost77/pine/internal/app 3.989s` |
| 6 | Repo build and full test suite | Task 6 | PASS | `go build -o /tmp/pine-bookmarks-verify-build ./cmd/ && go test ./... -count=1` -> build succeeded; all listed packages passed, including `internal/abs`, `internal/app`, and `internal/screens/detail` |
| 7 | Manual terminal sanity check of list/empty/error bookmark presentation | Task 6 | SKIP | Not run because the stale-focus bug in bookmark error state means verification cannot be marked PASS yet. |

## Defense-in-Depth (if bug fix)
| Layer | Check | Result |
|-------|-------|--------|
| Entry point | Progress `404` is normalized at the ABS client boundary instead of leaking as a generic failure | PASS |
| Business logic | Bookmark load outcomes are carried explicitly into detail state instead of being inferred from slice length | PASS |
| Guards | Bookmark interactions are not consistently disabled when `bookmarkLoadErr` is set | FAIL |
| Instrumentation | No new runtime instrumentation was added for bookmark-load failures; existing UX signaling is local to the detail screen | N/A |

## Issues Found
### Stale bookmark focus remains active during bookmark error state
- **Severity:** medium
- **Root cause:** `BookmarksUpdatedMsg` preserves existing bookmark rows on error, but the detail model only clears/blocks bookmark focus and actions based on bookmark count. That leaves stale bookmark interactions available even while the view is explicitly rendering the bookmark error state.
- **Suggested fix:** Update the detail model so bookmark focus is cleared when `bookmarkLoadErr` is set, and require `bookmarkLoadErr == nil` for bookmark focus-cycling, seek, delete, and up/down navigation.

## Verdict: PARTIAL
Automated verification commands passed, and the main bookmark visibility behavior is implemented. Verification cannot be marked PASS because the detail model still allows stale bookmark interaction while the bookmark error state is visible.
