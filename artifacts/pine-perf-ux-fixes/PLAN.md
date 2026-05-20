# Plan: pine-perf-ux-fixes

## Problem and target
- Fix 5 high-signal UX/perf issues and 4 production linter errors in the pine TUI Audiobookshelf client
- Target: each change is independent, small, and immediately verifiable

## Fixed decisions
- Skip the 15 low-signal improvements (micro-optimizations, edge cases, full features)
- Skip test-file linter fixes (noise)
- No new dependencies required

## Assumptions
- No refactoring of existing patterns unless strictly necessary
- Match existing code style (no comments unless documenting behavior, use existing patterns like tea.Cmd closures)

## Non-goals
- Queue viewer (full feature, not a fix)
- Detail view rebuild optimization (section caching — complex, defer until real user complaint)
- Search progress enrichment (consistency gap, low daily impact)

## Tasks / delivery order

### 1. Playback startup feedback [size: S]
- **Goal:** Show "Starting playback..." when user presses play, clear on PlayerReadyMsg or PlayerLaunchErrMsg
- **Files:** `internal/app/model.go`, `internal/app/render.go`, `internal/app/playback.go`
- **Action:**
  1. Add `isLaunching bool` field to Model struct
  2. In `handlePlayCmd()` (playback.go), set `m.isLaunching = true` before returning
  3. In `handlePlayerReady()` and `handlePlayerLaunchErr()`, set `m.isLaunching = false`
  4. In `viewPlayerFooter()` (render.go), add a muted "Starting playback..." line when `m.isLaunching && !m.isPlaying()`
- **Verify:** Run `go build -o pine . && go vet ./... && go test ./... -count=1`
- **Done:** User sees feedback immediately on play, disappears when mpv connects

### 2. Status bar hints: add speed/volume [size: S]
- **Goal:** Show speed/volume controls in the status bar so users can discover them
- **Files:** `internal/app/render.go`
- **Action:** In `viewHints()`, add two entries to the playing hints: `{"- / +", "speed"}` and `{"[ / ]", "volume"}`
- **Verify:** `go build -o pine . && go vet ./...`
- **Done:** Status bar shows speed/volume hints while playing

### 3. Sort podcast episodes by index [size: S]
- **Goal:** Episodes appear in consistent ascending index order
- **Files:** `internal/screens/detail/model.go`
- **Action:** In `SetEpisodes()`, sort `eps` by `Index` ascending using `slices.SortFunc` before storing
- **Verify:** `go build -o pine . && go vet ./... && go test ./... -count=1`
- **Done:** Episode list is ordered consistently regardless of API return order

### 4. Toggle mark finished / unfinished [size: M]
- **Goal:** `f` key toggles finished state; if already finished, marks as in-progress
- **Files:** `internal/screens/detail/model.go`
- **Action:**
  1. In `Update` handleKey `msg.String() == "f"` block: check if item is finished (via `m.item.UserMediaProgress.IsFinished`)
  2. If finished: dispatch a cmd that calls `UpdateProgress(..., isFinished=false)` and refreshes bookmarks
  3. If not finished or no progress: call existing `m.markFinishedCmd()` as today
- **Edge cases:** UserMediaProgress may be nil (no progress yet) → mark finished (current behavior)
- **Verify:** `go build -o pine . && go vet ./... && go test ./... -count=1`
- **Done:** Pressing `f` on a finished item toggles it back to in-progress

### 5. Strip markdown from descriptions [size: S]
- **Goal:** Remove raw markdown syntax from book/podcast descriptions for readability
- **Files:** `internal/screens/detail/view.go`
- **Action:**
  1. Add `stripMarkdown(s string) string` function that:
     - Replaces `[text](url)` → `text`
     - Strips `*` and `_` surrounding markers
  2. Apply to description string in `buildContent()` before calling `wordWrap()`
- **Verify:** `go build -o pine . && go vet ./... && go test ./... -count=1`
- **Done:** Descriptions render without `*`, `_`, `[]()` syntax

### 6. Fix production linter errors [size: S]
- **Goal:** Check err return values in 4 production locations
- **Files:** `internal/abs/client.go`, `internal/logger/logger.go`, `internal/player/commands.go`, `internal/screens/search/view.go`
- **Action:**
  1. `client.go:79`: wrap `defer resp.Body.Close()` in anonymous func with `_ =`
  2. `logger.go:47`: prefix `f.Close()` with `_ =`
  3. `player/commands.go:98`: prefix `os.Remove(socketPath)` with `_ =`
  4. `search/view.go:10`: remove dead assignment `body := m.styles.Muted.Render(...)` — it's immediately overwritten
- **Verify:** `~/go/bin/golangci-lint run` — production files should have zero issues
- **Done:** golangci-lint shows zero issues for the touched production files

## Product-level verification criteria
- `go build -o pine .` succeeds
- `go vet ./...` passes
- `go test ./... -count=1` passes
- `golangci-lint run` — no issues in production files (tests may still have issues)
