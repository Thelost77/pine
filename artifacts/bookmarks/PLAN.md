# Plan: bookmarks

## Problem and target
- Restore reliable bookmark visibility on the detail screen for bookmark-capable items.
- Good looks like: entering detail always results in one of three explicit states for bookmarks:
  1. populated bookmark list,
  2. visible empty state when there are none,
  3. visible non-blocking bookmark-load error for real failures.
- `404` from ABS progress must be treated as a valid empty state, not an error.

## Fixed decisions
- Keep the ABS progress endpoint as the primary bookmark source.
- Add guarded fallback behavior only for unusable responses/failures; do not redesign the bookmark model.
- Treat ABS progress `404` as empty bookmarks.
- Show a bookmark-local, non-blocking error for non-404 fetch failures or malformed responses.
- Apply the behavior consistently to all bookmark-capable item types.
- Show a visible empty bookmark section instead of hiding the section when supported-but-empty.

## Assumptions
- Detail entry remains the right time to load bookmarks.
- Bookmark support continues to live in the detail screen rather than the root app model.
- Existing ABS create/delete bookmark endpoints remain correct.
- The existing TUI test harness is sufficient for bookmark visibility regression coverage.

## Non-goals
- No bookmark editing/renaming.
- No backend/API redesign.
- No chapter UX changes.
- No generic loading/error framework extraction unless the implementation stays tiny.

## Architecture / delivery shape
- **ABS client layer:** normalize bookmark-fetch outcomes into three meaningful cases: bookmarks present, empty/no-progress, fetch/parsing failure.
- **Root app layer:** keep detail-entry wiring, but stop converting all bookmark failures into a silent empty list.
- **Detail screen state/rendering:** represent bookmark list plus bookmark load state, and render list/empty/error explicitly.
- **Verification boundary:** unit coverage for ABS fetch semantics and detail rendering; E2E coverage for detail entry and visible bookmark states.

## Tasks / delivery order

### 1. Classify bookmark fetch outcomes at the ABS/app boundary [size: M]
- **Goal:** Distinguish valid empty state from real bookmark-load failures.
- **Files:** `internal/abs/bookmarks.go`, `internal/abs/bookmarks_test.go`, `internal/app/bookmarks.go`, possibly `internal/app/messages.go`
- **Action:** Refine bookmark fetch handling so `404` from `GET /api/me/progress/{item}` becomes an explicit empty result, while non-404 HTTP failures and malformed bookmark payloads remain errors that can be surfaced to the detail screen.
- **Edge cases:** no-progress `404`; malformed JSON; auth/server failure; nil client.
- **TDD:** yes
  - RED: Write tests for `404` -> empty bookmarks and non-404/malformed payload -> error
  - Verify RED: `go test ./internal/abs/... -count=1` fails for the expected bookmark-fetch semantics
  - GREEN: Implement outcome classification in the ABS/app fetch path
  - Verify GREEN: `go test ./internal/abs/... -count=1` passes
  - REFACTOR: keep fetch semantics centralized so detail code doesn’t need to infer HTTP meaning
- **Verify:** `go test ./internal/abs/... -count=1`
- **Done:** Bookmark fetch produces outcomes that preserve the difference between empty and failed.

### 2. Carry bookmark-empty vs bookmark-error state into the detail screen [size: M]
- **Depends on:** Task 1
- **Goal:** Give the detail screen enough state to render bookmark list, empty state, or non-blocking error explicitly.
- **Files:** `internal/screens/detail/model.go`, `internal/app/model.go`, `internal/app/bookmarks.go`, related tests
- **Action:** Extend the message/state flow so detail updates can receive bookmark results together with failure information, without turning bookmark issues into a full-screen failure.
- **Edge cases:** navigating to detail repeatedly; refresh after create/delete; keeping bookmark focus valid when list becomes empty; item types that share the same fetch path.
- **TDD:** yes
  - RED: Write model tests for success, empty, and error bookmark updates
  - Verify RED: `go test ./internal/screens/detail/... -count=1` fails because the model only tracks bookmark slices today
  - GREEN: Add explicit detail state for bookmark load outcome and wire root/app updates into it
  - Verify GREEN: `go test ./internal/screens/detail/... -count=1` passes
  - REFACTOR: collapse repeated bookmark-state update logic if add/delete/fetch paths diverge
- **Verify:** `go test ./internal/screens/detail/... -count=1`
- **Done:** Detail state can represent all three bookmark outcomes without ambiguity.

### 3. Render explicit bookmark list, empty state, and load error in the detail view [size: M]
- **Depends on:** Task 2
- **Goal:** Make bookmark support visible and diagnosable on-screen.
- **Files:** `internal/screens/detail/view.go`, `internal/screens/detail/model_test.go`
- **Action:** Update bookmark section rendering and help text so supported items show:
  - bookmark rows when present,
  - a visible `No bookmarks yet` section for empty state,
  - a visible non-blocking bookmark error message for failures.
- **Edge cases:** empty/error state should not imply bookmark focusability; podcast episode section still coexists cleanly; narrow terminal widths.
- **TDD:** yes
  - RED: Add view tests for bookmark list, empty state, and error state visibility
  - Verify RED: `go test ./internal/screens/detail/... -count=1` fails because empty/error states are currently hidden
  - GREEN: Implement explicit bookmark section rendering and context-sensitive help
  - Verify GREEN: `go test ./internal/screens/detail/... -count=1` passes
  - REFACTOR: reuse existing empty-state styling pattern where possible
- **Verify:** `go test ./internal/screens/detail/... -count=1`
- **Done:** The detail screen no longer makes supported-empty and failed-to-load look the same.

### 4. Keep bookmark mutation flows consistent with the new state model [size: S]
- **Depends on:** Tasks 1-3
- **Goal:** Ensure add/delete refreshes still land in the correct list/empty/error state.
- **Files:** `internal/app/bookmarks.go`, `internal/screens/detail/model.go`, relevant tests
- **Action:** Update post-create/post-delete refresh handling so a refreshed empty result shows the empty section and a refresh failure shows the non-blocking bookmark error.
- **Edge cases:** deleting the last bookmark; create succeeds but refresh fails; bookmark focus after deletion.
- **TDD:** yes
  - RED: Add tests covering delete-last-bookmark and refresh-failure-after-mutation
  - Verify RED: targeted package tests fail for the expected state mismatch
  - GREEN: Align mutation refreshes with the new bookmark outcome flow
  - Verify GREEN: targeted tests pass
  - REFACTOR: share fetch-result mapping between initial load and mutation refresh
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** Bookmark mutations preserve the same explicit visibility semantics as initial load.

### 5. Add end-to-end coverage for visible bookmark states on detail entry [size: M]
- **Depends on:** Tasks 1-4
- **Goal:** Guard the real user flow across navigation, API responses, and rendering.
- **Files:** `internal/app/e2e_helpers_test.go`, `internal/app/e2e_test.go`
- **Action:** Add or update E2E cases for:
  - detail entry with existing bookmarks,
  - detail entry with `404`/no progress -> visible empty bookmark state,
  - detail entry with non-404 failure -> visible non-blocking bookmark error,
  - delete-last-bookmark -> visible empty state.
- **Edge cases:** shared behavior for books and other bookmark-capable items if covered by current fixtures.
- **TDD:** yes
  - RED: Add failing E2E expectations for empty/error bookmark visibility
  - Verify RED: `go test ./internal/app/... -count=1` fails for expected end-to-end behavior gaps
  - GREEN: Finish any missing wiring so the user-visible states appear correctly
  - Verify GREEN: `go test ./internal/app/... -count=1` passes
  - REFACTOR: keep mock ABS behavior aligned with real ABS `404` semantics
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** End-to-end flows prove bookmark visibility is no longer dependent on lucky API success.

### 6. Final verification [size: S]
- **Depends on:** Tasks 1-5
- **Goal:** Confirm the shipped behavior matches the product intent.
- **Files:** none unless follow-up test fixes are needed
- **Action:** Run repo build/test commands and a quick manual detail-screen sanity pass.
- **Verify:** `go build ./cmd/` and `go test ./... -count=1`
- **Done:** Bookmark detail view consistently shows list, empty, or error state as intended.

## Product-level verification criteria
- Entering detail for a bookmark-capable item never leaves bookmark support visually ambiguous.
- No-progress `404` from ABS yields a visible empty bookmark state, not an error.
- Non-404 failures and malformed bookmark responses are visible but non-blocking.
- Create/delete bookmark flows continue working and now preserve explicit empty/error semantics.
- Existing chapter and playback behavior remain unaffected.

## Open implementation questions
- None blocking implementation.
