# Debug: chapter overlay wrong index/current selection/high-jump behavior

## Phase 1: Root Cause Investigation
- **Error:** No crash. Observed runtime behavior:
  - overlay header showed `86 chapters` while chapter titles in the list went up to `98`
  - selecting the bottom item behaved like the 86th available entry rather than the book's expected last numbered chapter
  - opening the overlay did not start on the currently playing chapter
  - overlay had no `H/L` top/bottom jumps
- **Reproduction:**  
  1. Start playback for a book with many chapters.  
  2. Open the chapter overlay with `c`.  
  3. Observe chapter count vs numbered titles, selection starting point, and missing `H/L` navigation.  
  4. Navigate to the end and press `enter`.
- **Recent changes:** The chapter overlay was newly introduced in commits:
  - `b853322` root overlay state
  - `06505b7` overlay rendering
  - `b0de7f2` lifecycle reset
  - `77efd08` overlay seek
  - `df87daa` inline detail chapter removal
  - `18bb5f5` E2E coverage
- **Evidence:**  
  - `internal/abs/types.go` shows play sessions expose **two** possible chapter sources:
    - `PlaySession.Chapters`
    - `PlaySession.MediaMetadata.Chapters`
  - `internal/app/playback.go` and `internal/app/bookmarks.go` only propagated `session.Chapters`.
  - `internal/app/render.go` displayed `len(m.chapters)`, so any truncation at ingest would surface directly as a smaller count.
  - `internal/app/model.go` opened the overlay with the persisted/default index instead of deriving it from `m.player.Position`.
  - Live play-session inspection for the affected book showed:
    - `audioTracks = 98`
    - `chapters = 86`
    - `mediaMetadata.chapters = 0`
    - titles jump from `01`, `02`, `04`, ... to `95`, `97`, `98`
  - This proved the overlay header count (`86`) matched the actual ABS chapter payload; the apparent `98 chapters` came from raw track/file-style chapter titles rather than chapter ordinal positions.
- **Data flow trace:**  
  ABS play response → `abs.PlaySession` → `handlePlayCmd` / `handlePlayEpisodeCmd` / cross-track restart → `PlaySessionMsg.Session.Chapters` → `m.chapters` → overlay header count + selectable rows + seek target.  
  The bad value originated at the play-session ingest step, where only `session.Chapters` was used.

## Phase 2: Pattern Analysis
- **Working example:** `internal/abs/types.go` already uses a fallback/preference pattern for duration (`Media.TotalDuration()` prefers `media.duration`, falls back to `metadata.duration`).
- **Differences:**  
  - Duration logic already accounts for duplicated API fields.  
  - Chapter logic did **not**; it trusted only one field from the play-session response.  
  - Overlay opening logic also differed from next/prev chapter behavior: next/prev use current playback position, but opening the overlay did not.
- **Dependencies:**  
  - Correct chapter ingest depends on ABS play-session payload shape.  
  - Correct initial highlight depends on `m.player.Position` being book-global.  
  - `H/L` behavior depends on overlay-local selection clamping.

## Phase 3: Hypothesis
- **Hypothesis:** Two issues were mixed together:
  1. the missing current-chapter highlight and missing `H/L` behavior were genuine overlay behavior bugs,
  2. the `86` vs `98` mismatch was a presentation problem because ABS titles reflected track numbering, not chapter ordinals.
- **Test:**  
  1. Keep the failing tests for opening on current chapter and `H/L` top/bottom jumps.  
  2. Inspect the live play-session payload for the affected book and compare `audioTracks`, `chapters`, and `mediaMetadata.chapters`.  
  3. Add a render test that shows pine's own chapter ordinals alongside the raw ABS title.
- **Result:** confirmed
  - `TestCKeyOpensChapterOverlayOnlyWhenPlayingWithChapters` failed (`overlay index = 0, want 1`)
  - `TestHLJumpToOverlayExtremes` failed (`overlay index after L = 1, want 2`)
  - Live payload inspection showed ABS really returned `86` chapters and `98` audio tracks
  - New render test failed until ordinal prefixes were added

## Phase 4: Fix
- **Root cause:** Two separate issues:
  1. overlay opening and movement logic did not compute or jump selection from current playback state / explicit top-bottom commands
  2. overlay rows showed raw ABS titles only, and those titles used track numbering (`...98`) instead of pine chapter ordinals (`...86`)
- **Fix:**  
  - Changed `openChapterOverlay()` to initialize selection from the current playback position via `currentChapterOverlayIndex()`.  
  - Added `setChapterOverlaySelection()` and overlay key handling for uppercase `H`/`L` to jump to first/last chapter.  
  - Updated overlay help text to mention `H/L top/bottom`.
  - Updated overlay row rendering to prefix each entry with pine's chapter ordinal while keeping the raw ABS title (for example `86. 98-Poziom-smierci`).
- **Test:** Added/updated failing tests that now pass:
  - `TestCKeyOpensChapterOverlayOnlyWhenPlayingWithChapters`
  - `TestHLJumpToOverlayExtremes`
  - `TestViewRendersChapterOrdinalsAlongsideRawTitles`
- **Verification:**  
  - `go test ./internal/app/... -count=1`
  - `go build -o /tmp/pine-bugfix-build ./cmd/`
  - `go test ./... -count=1`

## Attempts
- Attempt 1: Hypothesis that overlay rendering/windowing was dropping rows → rejected after tracing showed the header count itself came from `len(m.chapters)`, so the truncation happened earlier.
- Attempt 2: Hypothesis that play-session ingest was using the wrong chapter source, plus overlay open/jump behavior lacked current-position logic → partially rejected after live payload inspection showed the affected book really had 86 chapters and 98 tracks.
- Attempt 3: Hypothesis that the remaining confusion was caused by showing raw ABS track-number titles without pine chapter ordinals, while open/jump behavior still needed current-position logic → confirmed and fixed.
