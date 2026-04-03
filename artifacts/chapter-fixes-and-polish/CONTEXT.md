# Context: chapter-fixes-and-polish

## Implementation Decisions

### 1. In-track boundary check
- **Decision:** Add a defensive fallback — when `trackDuration == 0`, treat the entire book duration as one track (all seeks go to mpv directly). This handles single-file books, podcasts, and any edge case where track metadata is missing.
- **Rationale:** Even though `trackDuration` should always be set from ABS, a zero-value should never cause a full session restart.
- **Options considered:** (a) Assert trackDuration is non-zero (fragile), (b) Defensive fallback (chosen — safe and correct)

### 2. Cross-track restart: use target position for track selection
- **Decision:** When restarting playback for a cross-track seek, pass the target book-global position into the restart closure. Use that position (not `session.CurrentTime` from ABS) to select the correct track from `session.AudioTracks`. This eliminates the race condition with UpdateProgress.
- **Rationale:** We already know where we want to go. ABS gives us the track list and URLs. We just need to pick the right track for our target position, which is a simple loop we already have.
- **Options considered:** (a) Rely on UpdateProgress being processed before StartPlaySession (current, broken due to race), (b) Add delay/retry (hacky, unreliable), (c) Use our own target position (chosen — deterministic and correct)

### 3. Player model seek key handling
- **Decision:** Remove SeekForward/SeekBack key handling from the player model. The root model handles these keys with offset-aware logic. Keep the player model's pause, speed, and volume handling.
- **Rationale:** The root model must intercept seek keys to convert coordinates. Having duplicate handlers is confusing and fragile.
- **Options considered:** (a) Leave dead code in player model (confusing), (b) Remove from player model (chosen — clean)

### 4. Duration display in player footer
- **Decision:** Never let mpv's reported duration overwrite the book-global duration set from ABS. mpv only knows the current track's duration, which is meaningless for multi-track books.
- **Rationale:** Already partially implemented (the `m.player.Duration` line was removed from handlePositionMsg) but needs to be verified as complete.

## Existing Code Insights

- `seekToBookGlobalPosition` at `bookmarks.go:153` — the central seek function, has the broken boundary check and broken cross-track restart
- `handlePlayCmd` at `playback.go:60-71` — correct track selection logic that should be reused in the cross-track restart
- `handlePositionMsg` at `playback.go:211-212` — correctly converts track-relative to book-global via `msg.Position + m.trackStartOffset`
- `nextChapter`/`prevChapter` at `bookmarks.go:107-139` — chapter navigation using book-global positions, logic is correct after recent fix
- `handleSeek` at `bookmarks.go:237-250` — h/l seek handler, delegates to `seekToBookGlobalPosition`
- Mock server at `e2e_helpers_test.go:353-369` — single-track mock, needs multi-track variant for proper testing

## Integration Points

- `seekToBookGlobalPosition` is called from: `seekToChapter` (n/N), `handleSeekToBookmark` (enter on bookmark/chapter in detail), `handleSeek` (h/l)
- `handlePlaySessionMsg` is the single entry point for all new playback sessions — both initial play and cross-track restarts flow through it
- `handlePositionMsg` is the only place where mpv position ticks are processed

## Deferred Ideas

- Track end detection: when mpv reaches the end of a track file, automatically advance to the next track without user interaction. Currently mpv exits and playback stops. This is a separate feature.
