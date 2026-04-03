# Debug: playback stopped during file swap

## Phase 1: Root Cause Investigation
- **Error:** `player position error, stopping playback` with `err="get time-pos: trying to send command on closed mpv client"` at `2026-04-03T20:41:26.279+02:00` in `~/.config/pine/pine.log`.
- **Reproduction:** For multi-track books, pine launches mpv with one track URL at a time. When the current file ends, the next 1-second poll hits a closed mpv IPC client and playback is stopped instead of advancing. The log shows this with item `4e899c38-1fb9-4a90-8e29-57904d6f9185` after ~12 minutes of uninterrupted playback.
- **Recent changes:** The current `HEAD` (`94a92aa`, `fix(app): finish chapter overlay debug`) added more playback logging and chapter-overlay work, but the failing sessions in the log all happened before that commit. There is no evidence of a bad merge or corrupted runtime state introduced by that change.
- **Evidence:**  
  1. `play cmd` starts book `REMOTE...` at `20:28:58`; ABS returns `currentTime=932.889259`, pine launches file `68003` at `currentTime=141.86143100000004`, so the track start offset is `791.027828`.  
  2. At `20:41:26`, pine logs the closed-mpv-client error and stops at global position `1679.262687`. That is exactly the end of the current track (`791.027828 + 888.2348589999999`).  
  3. At `20:42:18`, a manual `+10s` seek from `1679.262687` to `1689.262687` triggers `cross-track seek, restarting playback`, and pine successfully relaunches the next file (`68002`) at `9.488920999999891`. That proves the next-track data is valid and the rollover path works when explicitly invoked.
- **Data flow trace:**  
  1. `handlePlayCmd` / `handlePlayEpisodeCmd` chooses one `AudioTrack` from ABS and passes a single URL into `player.LaunchCmd` (`internal/app/playback.go`).  
  2. `player.LaunchCmd` starts mpv for that single file and polling begins (`internal/player/commands.go`).  
  3. When mpv reaches EOF, the IPC client closes.  
  4. The next poll returns `PositionMsg{Err: ...closed mpv client...}`.  
  5. `handlePositionMsg` treats any such error as fatal and calls `stopPlayback()` (`internal/app/playback.go:206-213`).  
  6. There is no automatic EOF handoff into the cross-track restart code.

## Phase 2: Pattern Analysis
- **Working example:** Explicit cross-track chapter/bookmark/seek jumps already work through `seekToBookGlobalPosition` in `internal/app/bookmarks.go:155-239`. That path closes the old session, starts a fresh ABS play session, selects the correct track for the target book position, and relaunches mpv.
- **Differences:**  
  1. **Manual cross-track path:** computes target book position first, then calls `StartPlaySession` again and selects the correct `AudioTrack`.  
  2. **Natural EOF path:** never enters `seekToBookGlobalPosition`; it only receives an mpv polling error and stops playback.  
  3. **Result:** manual boundary crossing restarts on the next file, natural boundary crossing dies at EOF.
- **Dependencies:** Correct rollover depends on ABS `AudioTracks` metadata (`StartOffset`, `Duration`, `ContentURL`), `trackStartOffset` / `trackDuration` state in the app model, and the mpv polling loop continuing to distinguish EOF from a genuine player failure.

## Phase 3: Hypothesis
- **Hypothesis:** Playback is not corrupted. The root cause is that pine has no automatic multi-track rollover path for natural EOF; it only knows how to change tracks when a user-triggered seek/chapter/bookmark action calls `seekToBookGlobalPosition`.
- **Test:** Compare the EOF stop log at `20:41:26` with the later manual `+10s` cross-track restart at `20:42:18`, then trace the corresponding code paths in `internal/app/playback.go` and `internal/app/bookmarks.go`.
- **Result:** **Confirmed.** The boundary math matches exactly, the manual restart succeeds with the next track, and the only passive EOF behavior in code is “log warning and stop playback.”

## Phase 4: Fix
- **Root cause:** Natural end-of-track closes mpv, and `handlePositionMsg` treats that closed-client poll error as terminal instead of using the existing cross-track restart mechanism.
- **Fix:** Not implemented in this investigation. The needed implementation is to detect EOF / closed-client-at-track-end and transition into the same restart logic used by `seekToBookGlobalPosition`, preserving book-global position instead of stopping playback.
- **Test:** No code change was made, so no new failing test was added in this pass.
- **Verification:** Existing app tests still pass; the investigation is based on log evidence plus source tracing.

## Attempts
- Attempt 1: Investigate whether the book or track data was corrupted → rejected. ABS repeatedly returns valid track/session data, and manual cross-track restarts succeed.
- Attempt 2: Investigate whether the “file swap” itself fails inside mpv → partially true but not root cause. mpv closes at EOF as expected for a single-file launch; the real bug is that pine treats that closure as a fatal stop instead of advancing.
- Attempt 3: Investigate whether explicit seeks were causing the stop → separated into a second pattern. Earlier log bursts are user-driven cross-track jumps to chapter-like positions; they are noisy but distinct from the passive EOF stop.
