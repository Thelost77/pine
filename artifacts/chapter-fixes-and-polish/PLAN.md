# Plan: chapter-fixes-and-polish

## Tasks

### 1. Fix in-track boundary check [size: S]
- **Files:** `internal/app/bookmarks.go`
- **Action:** In `seekToBookGlobalPosition`, when `trackDuration == 0`, use `m.player.Duration` as the track end (treat entire book as one track). This makes the in-track path work for single-file books, podcasts, and any edge case.
- **TDD:** yes
  - RED: Write E2E test — set up playback with trackDuration=1800, seek to position within track → assert mock player Seek() called, no API calls for new session
  - Verify RED: `go test ./internal/app/... -run TestE2E_WithinTrackSeek` → fails
  - GREEN: Fix boundary check in `seekToBookGlobalPosition`
  - Verify GREEN: `go test ./internal/app/... -run TestE2E_WithinTrackSeek` → passes
- **Verify:** `go test ./... -count=1` — all tests pass including existing ones
- **Done:** Within-track seeks (h/l, same-track chapters) call mpv.Seek directly without restart

### 2. Fix cross-track restart to use target position [size: M]
- **Depends on:** Task 1
- **Files:** `internal/app/bookmarks.go`
- **Action:** Change cross-track path in `seekToBookGlobalPosition` to pass target book-global position into the closure. Use target position (not `session.CurrentTime`) for track selection from `session.AudioTracks`. Remove the `UpdateProgress` call — not needed since we pick the track ourselves. Keep CloseSession for cleanup.
- **TDD:** yes
  - RED: Write E2E test with multi-track mock (2 tracks: 0-1800, 1800-3600). Seek from position 500 to chapter at 2000 → assert new session started, trackStartOffset=1800, player position=2000
  - Verify RED: `go test ./internal/app/... -run TestE2E_CrossTrackChapterSeek` → fails
  - GREEN: Fix cross-track closure to use target position for track selection
  - Verify GREEN: `go test ./internal/app/... -run TestE2E_CrossTrackChapterSeek` → passes
- **Verify:** `go test ./... -count=1` — all pass
- **Done:** Cross-track chapter seek lands on correct position in correct track

### 3. Remove seek keys from player model [size: S]
- **Depends on:** Task 1
- **Files:** `internal/player/model.go`, `internal/player/model_test.go`
- **Action:** Remove SeekForward/SeekBack cases from `handleKey`. Remove key binding fields from `PlayerKeyMap`. Keep `SeekForwardKey()`/`SeekBackKey()` accessors for root model key matching.
- **TDD:** no
- **Verify:** `go test ./internal/player/... && go test ./internal/app/...` — all pass
- **Done:** No duplicate seek handling; root model owns all seek logic

### 4. Update E2E tests for multi-track coverage [size: M]
- **Depends on:** Task 1, 2
- **Files:** `internal/app/e2e_helpers_test.go`, `internal/app/e2e_test.go`
- **Action:** Add multi-track item + mock server handler. Add E2E tests:
  - Position tick → book-global (trackPos + offset)
  - h/l within track → mpv.Seek, no restart
  - Sync → sends book-global position
  - Existing single-track tests still pass
- **TDD:** no (these ARE the tests)
- **Verify:** `go test ./internal/app/... -v -run TestE2E_MultiTrack` — all pass
- **Done:** Multi-track playback has test coverage
