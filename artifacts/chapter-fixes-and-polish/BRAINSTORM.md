# Brainstorm: chapter-fixes-and-polish

## Problem Statement

Multi-track audiobook playback is fundamentally broken. Pine plays one audio file at a time via mpv, but ABS uses book-global coordinates for everything (position, chapters, bookmarks, sync). The attempted fix added `trackStartOffset`/`trackDuration` to convert between coordinate systems, but introduced three compounding bugs:

### Bug 1: trackDuration is always 0 in the in-track check
`seekToBookGlobalPosition` checks `bookPos >= trackStartOffset && bookPos < trackStartOffset + trackDuration`. But `trackDuration` comes from the `PlaySessionData` message, and the `handlePlaySessionMsg` stores it. Looking at logs: the session IS being created with tracks, but the in-track boundary check fails because `trackEnd = offset + 0 = offset`, so `bookPos < offset` is false for any position past the track start. **Every seek — including within the same track — falls through to the cross-track restart path.**

**Root cause confirmed by logs:** Even pressing N to seek to position 0 (clearly within track 0 which covers 0-1538s) triggers "cross-track seek, restarting playback".

### Bug 2: UpdateProgress + StartPlaySession race condition
The cross-track restart does: UpdateProgress(targetPos) → CloseSession → StartPlaySession. But these run sequentially in a single goroutine, and ABS returns `currentTime=1542` (the old position) from StartPlaySession, meaning UpdateProgress hasn't taken effect by the time the new session starts. The seek lands on the wrong spot.

### Bug 3: h/l seek always triggers full session restart
Same root cause as Bug 1 — `trackDuration == 0` means every seek goes through the restart path. A 10-second skip triggers a network round-trip to ABS.

### Secondary issues
- Player model still has seek key handlers (SeekForward/SeekBack) that would use book-global positions with mpv — dead code now but fragile
- E2E tests don't set `trackDuration`, so they paper over the bug

## Architecture

This is a single-binary Go TUI app using bubbletea. No backend, no database changes, no new dependencies. The fix is entirely within the internal position tracking logic.

- **Frontend:** bubbletea TUI (no change)
- **Backend:** N/A — Pine is a client, ABS is the server
- **Database:** sqlite for local session persistence (no schema change)
- **External APIs:** ABS REST API (no new endpoints needed)
- **Deployment:** single binary (no change)
- **Validation Strategy:** coordinate conversion correctness verified through E2E tests with multi-track mock data

## Chosen Approach

**Fix the coordinate conversion properly, eliminate the restart-for-every-seek pattern.**

The core insight: for most seeks (h/l, within-track chapter jumps), we should just do `mpv.Seek(trackRelative)`. The cross-track restart should ONLY happen when the target position is genuinely outside the current track's range. And when it does happen, we need to handle the race condition.

Three changes:

1. **Fix the in-track boundary check** — ensure `trackDuration` is always set correctly. When it's 0 (single-file books, podcasts), treat the entire duration as one track.

2. **Fix cross-track restart** — instead of UpdateProgress → StartPlaySession (which races), we should close the old session, then start a new session, and in the session response, ignore `session.CurrentTime` and use our target position for track selection instead. We already have `session.AudioTracks` with all the offsets — we can find the right track ourselves without relying on ABS to have processed our progress update.

3. **Clean up the seek key interception** — the root model handles h/l with offset conversion. The player model's seek handlers are now dead code for the root model flow. Either remove them from player model or make the player model offset-aware. Removing is cleaner since the root model should own all offset logic.

## Rejected Alternatives

- **Always restart playback for every seek:** Simpler but terrible UX — network round-trip for every 10s skip. Also creates race conditions with ABS.
- **Use mpv playlist with all 98 tracks:** mpv supports playlists, could load all tracks at once. But mpv playlist seeks are complex, would need to track which playlist entry is active, and streaming 98 HTTP URLs through a playlist is unreliable.
- **Pre-calculate all track boundaries and never call ABS for restarts:** Would avoid the race condition, but we'd need to store all 98 track URLs and manage track switching ourselves. Over-engineering for what should be a rare operation.

## Test Strategy

- **E2E tests** with multi-track mock data (2-3 tracks with different offsets) to verify:
  - Within-track seek (h/l, chapter within same track) → mpv.Seek called, no restart
  - Cross-track chapter navigation → restart with correct position
  - Position ticks report book-global position correctly
  - Sync sends book-global position
- **Existing tests** must continue passing (single-track = trackStartOffset 0 case)
- Manual testing with a real multi-track audiobook on ABS

## Open Questions

None — the problem, root causes, and fix approach are clear.
