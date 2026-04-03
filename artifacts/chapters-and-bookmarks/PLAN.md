# Plan: chapters-and-bookmarks

## Problem and target
- Replace the broken inline chapter list on the detail screen with a **root-owned playback chapter overlay**.
- Chapters should only be available during an active playback session with chapter data.
- Bookmarks should remain on the detail screen and keep working independently of playback.
- Good looks like: while listening, pressing `c` opens a modal-like chapter picker; `j/k` moves, `enter` seeks and closes, `esc` closes first, and playback teardown clears the overlay immediately.

## Fixed decisions
- Chapter UI is **root app model owned**, not detail-owned.
- Chapters are **playback-scoped**, not general metadata.
- Open the overlay with `c`.
- `Esc` closes the overlay before back navigation.
- `Enter` seeks and closes the overlay.
- Playback pause does not close the overlay.
- Playback session teardown does close and clear the overlay.
- Keep existing `n` / `N` quick chapter navigation outside the overlay.
- Test strategy is hybrid: automated tests plus quick manual TUI verification.

## Assumptions
- Audiobookshelf chapter data from `PlaySessionMsg.Session.Chapters` is the sole source of truth for overlay content.
- Only one playback session is active at a time, matching the current root playback model.
- Full-screen modal rendering can reuse the same root interception pattern currently used by `help`.
- No config file changes are required for choosing the key; `c` can be added as a fixed keybinding in the app keymap/help surfaces.

## Non-goals
- No changes to ABS API integration shape beyond consuming existing play-session chapters.
- No redesign of bookmarks.
- No permanent chapter panel on the detail screen.
- No chapter support outside active playback.
- No generic modal framework extraction unless the implementation naturally stays tiny.

## Architecture / delivery shape
- `internal/app/`
  - root playback state remains source-of-truth for active chapters
  - add overlay state and modal event routing
  - render overlay above normal screen content
- `internal/screens/detail/`
  - remove inline chapter UI/state
  - keep bookmarks and podcast episode flows intact
- `internal/ui/components/`
  - likely only help text updates; chapter overlay can stay in root unless a tiny reusable renderer emerges
- Verification boundaries:
  - root-level behavior tests for modal lifecycle and key routing
  - detail tests updated to stop expecting inline chapter behavior
  - E2E coverage for opening overlay and seeking from it

## Tasks / delivery order

### 1. Add root chapter overlay state and keybinding [size: M]
- **Goal:** Establish playback-owned chapter overlay state and key routing in the root model.
- **Files:** `internal/app/model.go`, `internal/app/keymap.go`, possibly `internal/app/messages.go`
- **Action:** Add root state for overlay visibility and selected chapter index; add `c` keybinding; intercept `c`, `esc`, `j/k`, and `enter` while overlay is open; keep `n/N` behavior outside overlay.
- **Edge cases:** Ignore `c` when not playing or when `len(m.chapters)==0`; clamp selection when chapters change; `esc` must close overlay without navigating back.
- **TDD:** yes
  - RED: Write tests for opening overlay only during active playback with chapters, closing with `esc`, and moving selection with `j/k`
  - Verify RED: `go test ./internal/app/... -count=1` fails for expected missing overlay behavior
  - GREEN: Implement root overlay state/key handling
  - Verify GREEN: `go test ./internal/app/... -count=1` passes for targeted tests
  - REFACTOR: Extract tiny helpers if routing becomes noisy
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** Overlay can open/close/navigate entirely from root state and does not interfere with normal back behavior when closed.

### 2. Render the chapter overlay and advertise it in hints/help [size: M]
- **Depends on:** Task 1
- **Goal:** Make the overlay visible and discoverable.
- **Files:** `internal/app/render.go`, `internal/app/keymap.go`, `internal/ui/components/help.go`
- **Action:** Add root modal rendering branch similar to help; render chapter title list with current selection and playback context; show `c` in status hints only when available; document `c` in help overlay.
- **Edge cases:** Overlay should render sanely with narrow widths and many chapters; if no chapters exist, no hint should advertise `c`.
- **TDD:** yes
  - RED: Add view tests for chapter overlay visibility, hints, and help exposure
  - Verify RED: `go test ./internal/app/... -count=1` or package-specific tests fail because overlay/hints are absent
  - GREEN: Implement overlay rendering and hint/help updates
  - Verify GREEN: tests pass and existing help behavior remains green
  - REFACTOR: Keep rendering helper small and local to root if possible
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** Overlay renders as a modal surface; hints/help accurately describe `c` only in relevant states.

### 3. Wire overlay lifecycle to playback session setup and teardown [size: S]
- **Depends on:** Task 1
- **Goal:** Ensure overlay lifecycle matches playback lifecycle.
- **Files:** `internal/app/playback.go`, `internal/app/model.go`
- **Action:** Reset overlay selection on new play session; close and clear overlay during `stopPlayback()` and player launch failure cleanup; preserve overlay through pause-only state changes.
- **Edge cases:** Switching items while overlay is open should not leak old selection/state; launch failure should not leave overlay visible; stale ticks should not matter.
- **TDD:** yes
  - RED: Add tests for overlay clearing on stop, switch, and failure and persistence through pause
  - Verify RED: `go test ./internal/app/... -count=1` fails for expected lifecycle mismatch
  - GREEN: Implement lifecycle resets and cleanup in playback paths
  - Verify GREEN: tests pass
  - REFACTOR: Consolidate overlay reset helper if duplicated
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** Overlay state always matches active playback session boundaries.

### 4. Route overlay selection to seeking [size: S]
- **Depends on:** Task 1, Task 3
- **Goal:** Make `enter` in the overlay seek to the selected chapter and close the overlay.
- **Files:** `internal/app/model.go`, `internal/app/bookmarks.go` or `internal/app/playback.go` depending on helper reuse, possibly `internal/app/messages.go`
- **Action:** On overlay `enter`, use selected `m.chapters` entry to call existing seek flow (`seekToChapter` / `seekToBookGlobalPosition`) and close the overlay immediately.
- **Edge cases:** Cross-track seeks must still restart playback correctly via existing logic; invalid or clamped selection should not panic.
- **TDD:** yes
  - RED: Add root tests covering in-track and chapter-based selection dispatch from overlay
  - Verify RED: `go test ./internal/app/... -count=1` fails because `enter` does not seek and close
  - GREEN: Wire overlay `enter` to existing seek path
  - Verify GREEN: tests pass and existing playback seek tests remain green
  - REFACTOR: Remove redundant command or message indirection if root can seek directly
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** Selecting a chapter from the overlay seeks correctly and closes the overlay.

### 5. Remove inline chapter behavior from the detail screen [size: M]
- **Depends on:** Task 1
- **Goal:** Align detail screen behavior with the new product model.
- **Files:** `internal/screens/detail/model.go`, `internal/screens/detail/view.go`, `internal/screens/detail/model_test.go`
- **Action:** Remove inline chapter rendering, chapter focus state, and chapter-specific help text from detail; keep bookmarks and podcast episode navigation intact; remove dead `SeekToChapterCmd` plumbing if unused after root implementation.
- **Edge cases:** Tab cycling for podcasts and bookmarks must still work; book detail without chapters should still show bookmarks and help correctly.
- **TDD:** yes
  - RED: Update detail tests to assert chapters are not rendered inline and Tab no longer cycles through chapter focus for books
  - Verify RED: `go test ./internal/screens/detail/... -count=1` fails due to old chapter behavior
  - GREEN: Simplify detail model and view to match new UX
  - Verify GREEN: `go test ./internal/screens/detail/... -count=1` passes
  - REFACTOR: Remove dead fields, methods, and messages left by inline chapters
- **Verify:** `go test ./internal/screens/detail/... -count=1`
- **Done:** Detail screen no longer exposes stale or broken chapter UI, while bookmarks continue to work.

### 6. Add end-to-end coverage for real user flow [size: M]
- **Depends on:** Tasks 1-5
- **Goal:** Validate the interaction through the root Bubble Tea flow.
- **Files:** `internal/app/e2e_test.go`, maybe `internal/app/e2e_helpers_test.go`
- **Action:** Add or adapt E2E tests for entering detail, starting playback, pressing `c`, moving selection, pressing `enter`, and confirming the seek path and overlay close; add teardown coverage if practical.
- **Edge cases:** Ensure `esc` closes overlay before back; ensure `c` does nothing before playback.
- **TDD:** yes
  - RED: Add E2E expectations for overlay flow
  - Verify RED: `go test ./internal/app/... -count=1` fails because the flow does not exist end-to-end yet
  - GREEN: Finish any missing wiring for the flow
  - Verify GREEN: `go test ./internal/app/... -count=1` passes
  - REFACTOR: Keep helpers reusable and avoid test-only branching in production
- **Verify:** `go test ./internal/app/... -count=1`
- **Done:** End-to-end tests cover the intended user flow and guard against regressions across root, detail, and playback boundaries.

### 7. Full verification and manual sanity check [size: S]
- **Depends on:** Tasks 1-6
- **Goal:** Confirm the shipped behavior matches the product intent.
- **Files:** none or minor test touch-ups only
- **Action:** Run repo-standard build and test commands and perform a quick manual check of the TUI flow.
- **Verify:** `go build ./cmd/ && go test ./... -count=1`
- **Done:** Build passes, tests pass, and manual flow confirms:
  - detail shows bookmarks but no inline chapters before playback
  - `c` opens chapter overlay only during active playback with chapters
  - `j/k` navigate, `enter` seeks and closes, `esc` closes first
  - quitting or switching playback clears the overlay
  - pause does not close the overlay

## Product-level verification criteria
- Users never see misleading inline chapters sourced from empty library metadata.
- Chapter navigation is only exposed when a playback session actually provides chapter data.
- The overlay is discoverable via hints/help but does not clutter the detail screen when unavailable.
- Overlay interaction does not break existing global playback controls, back behavior, bookmarks, or podcast episode flows.
- Playback teardown always removes chapter overlay state so no stale chapter UI persists.

## Open implementation questions
- None blocking implementation.
