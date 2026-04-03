# Clarify: chapters-and-bookmarks

## Scope Summary
Implement chapter navigation as a **root-owned playback overlay** that is available only during an active playback session with chapter data.

Bookmarks remain part of the detail screen and remain available independently of playback.

This replaces the earlier direction in `CONTEXT.md` that assumed chapters should be rendered inline on the detail screen. After brainstorming and clarification, the chosen UX is now:

- no visible chapter section before playback starts
- explicit chapter overlay opened by the user
- overlay belongs to playback state, not detail-screen state
- overlay closes immediately after a seek
- overlay closes immediately when the playback session is torn down
- pause does **not** close the overlay

## Resolved Product Decisions

### 1. Ownership
**Decision:** Root app model owns the chapter overlay state.

**Why:** Chapters are playback-scoped and the root model already owns playback lifecycle, active chapter data, and global overlays (`help`). This makes automatic teardown on session cleanup straightforward and avoids pushing playback state into the detail screen.

### 2. Entry Point
**Decision:** Open via dedicated keybinding `c`, only when playback is active and chapters exist.

**Why:** This keeps the action explicit and avoids overloading existing `n` / `N` chapter-skip behavior.

### 3. Back Behavior
**Decision:** `Esc` closes the chapter overlay first. Normal back navigation only applies when the overlay is not open.

**Why:** This preserves standard modal behavior and does not weaken the existing "esc goes back" model outside the overlay.

### 4. Playback Lifecycle
**Decision:** When the playback session ends and state is cleaned up, close the overlay immediately and clear its state.

**Clarification:** "Playback stops" here means session teardown such as quitting playback, switching item/episode, sleep-timer shutdown, launch failure cleanup, or mpv exit leading to cleanup. It does **not** mean pause via `space`.

### 5. Overlay Interaction
**Decision:** While open, the overlay captures:

- `j` / `k` to move selection
- `enter` to seek to the selected chapter
- `esc` to close

Existing global chapter skip keys `n` / `N` remain available outside the overlay.

### 6. Post-Selection Behavior
**Decision:** `enter` seeks and closes the overlay immediately.

**Why:** The overlay is a quick navigation tool, not a persistent browser.

## Brownfield Findings

### Existing chapter data flow
- `internal/app/playback.go`
  - `handlePlayCmd` and `handlePlayEpisodeCmd` populate `PlaySessionMsg.Session.Chapters` from ABS play-session responses.
  - `handlePlaySessionMsg` stores active chapters in `m.chapters`.
  - `stopPlayback` clears `m.chapters`.
- `internal/app/model.go`
  - global key handling already prioritizes playback keys before screen-local keys
  - `n` / `N` already operate on `m.chapters`
- `internal/app/render.go`
  - root view already supports full-screen overlay interception via `m.help.Visible()`

### Existing detail-screen assumptions that no longer match product direction
- `internal/screens/detail/view.go`
  - renders inline chapters from `m.item.Media.Metadata.Chapters`
  - help text mentions chapter focus in the detail body
- `internal/screens/detail/model.go`
  - maintains chapter focus and selection locally
  - cycles focus across chapters/bookmarks
  - emits `SeekToChapterCmd` from inline chapter selection
- `internal/screens/detail/model_test.go`
  - many tests currently assume inline chapter rendering/focus behavior

## Implementation Consequences

### Keep in root app
Add root-owned state for the chapter overlay, likely including:

- visible flag
- selected chapter index

The source of truth for displayed chapters should remain `m.chapters`, since that is already tied to the active playback session.

### Remove chapter interaction from detail screen
The detail screen should stop pretending chapters are part of static item metadata for books. In practice:

- no inline chapter list in the detail body
- no chapter focus cycle in detail
- no chapter-specific help text in detail
- `SeekToChapterCmd` may become unused and removable if root handles chapter selection directly

### Render strategy
The root `View()` should likely gain another early-return/interception branch similar to help:

- if help visible -> show help
- else if chapter overlay visible -> show chapter overlay
- else -> normal screen

This keeps the overlay modal and root-owned.

### Event routing strategy
Root key handling should intercept overlay keys before normal back navigation and before delegating to the active screen.

Likely order:

1. help toggle/close
2. chapter overlay open/close/interact
3. error banner dismissal
4. global quit/back
5. playback keys
6. screen-local update

The exact placement should preserve the clarified rule that `Esc` closes the overlay first.

### Cleanup points that must close the overlay
Overlay teardown should happen anywhere playback session state is cleared, including:

- `stopPlayback()`
- player launch failure cleanup in `model.go`
- any other branch that nils `m.chapters` / clears session state

Pause handling should not affect overlay visibility.

## Likely Files To Change

- `internal/app/model.go`
  - add overlay state
  - intercept open/close/navigation keys
  - close overlay during launch-failure cleanup
- `internal/app/playback.go`
  - ensure overlay state resets in `handlePlaySessionMsg` and `stopPlayback()`
- `internal/app/render.go`
  - render chapter overlay modally
  - update bottom hints to advertise `c` only when available
- `internal/app/keymap.go`
  - add chapter overlay keybinding
- `internal/ui/components/help.go`
  - document `c` in help, likely under Player or Detail/Player boundary
- `internal/screens/detail/model.go`
  - remove chapter focus/selection behavior
- `internal/screens/detail/view.go`
  - remove inline chapter rendering/help assumptions
- tests:
  - `internal/app/model_test.go`
  - `internal/app/playback_test.go`
  - `internal/app/e2e_test.go`
  - `internal/screens/detail/model_test.go`

## Testing Impact

### Add / update root-level tests
- chapter overlay opens only during active playback with chapters
- `Esc` closes overlay before navigating back
- `j/k` update overlay selection
- `enter` seeks selected chapter and closes overlay
- overlay closes on session teardown
- pause does not close overlay

### Update detail-screen tests
Current tests expecting inline chapters/focus cycling must be revised to match the new UX:

- chapter list should no longer render inline by default
- Tab should no longer cycle into chapters for books
- detail help text should stop advertising inline chapter navigation

### E2E expectations
Add or adapt E2E coverage for:

- open detail
- start playback
- press `c`
- navigate overlay
- select chapter
- verify seek command path / playback state transition

## Open Technical Questions
- None that block planning. The major product and ownership decisions are resolved.
