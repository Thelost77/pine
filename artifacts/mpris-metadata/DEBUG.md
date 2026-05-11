# MPRIS Integration — Debug & Investigation Log

## Problem Statement
Headphone controls (play/pause, seek) don't work with pine via MPRIS/D-Bus.

## Fixes Applied (all confirmed working)

### 1. DesktopEntry missing
`playerctld` ignores players without `DesktopEntry`. Added `"pine"` to `RootAdapter`.

### 2. Invalid D-Bus object path
UUIDs contain hyphens (`-`) which are illegal in D-Bus object paths (`[A-Za-z0-9_]` only). `TrackId` like `/org/pine/track/66b04003-09ef-...` caused "wire format error" and made the server drop all property replies. Fixed by replacing `-` with `_`.

### 3. Empty TrackId on stop
Zero-value `dbus.ObjectPath` is `""` — not a valid D-Bus object path (must start with `/`). When nothing is playing, `Metadata{}` was returned with this invalid path. Fixed by returning `/org/mpris/MediaPlayer2/NoTrack` (MPRIS convention).

### 4. CanPlay/CanPause/CanSeek always false
`playerctl` filters out players where `CanPlay=false`. These returned `accessor.HasActiveItem()` which was false when nothing was playing. Fixed to always return `true` (like VLC, mpv, etc.).

### 5. Metadata caching on stop
When playback stops, `clearPlaybackSessionState()` clears `m.player.Title` and `m.itemID`. Added `lastPlayedTitle`/`lastPlayedItemID` fields that are cached before clearing. Accessor methods fall back to cached values.

### 6. Metadata change signal not emitted on stop
`OnEnded()` only emits `PlaybackStatus` property change, not `Metadata`. Playerctl never re-reads metadata. Added `emitMprisTitle()` call after `emitMprisEnded()` in `stopPlayback()`.

### 7. Stale accessor (architectural root cause)
The MPRIS accessor held a `*Model` pointer to the `model` variable in `main()`. But bubbletea's `Run()` creates its own internal `model := p.initialModel` (line 684 of tea.go) and updates THAT on each `Update()` cycle. The pointer in `main()` is never updated — permanently stale.

**Fix:** Created shared `MprisState` struct (`internal/app/mpris_state.go`), heap-allocated. Model writes to it via `syncMprisState()` at end of `Update()`. MPRIS accessor reads from it via `func() ModelAccessor` closure. Both reference same heap object.

### 8. PropertiesChanged signal timing
`emitMprisPlayback()` was called BEFORE `syncMprisState()`, so the D-Bus signal carried stale data. System widget read `IsPlaying=false` from the signal. Fixed by calling `syncMprisState()` before every MPRIS signal emission.

### 9. PlaybackStatus change not emitted on play
`m.player.Playing` is set in `handlePositionMsg` when first tick arrives, but no `PlaybackStatus` change was emitted. System widget never knew playback started. Added check: if `wasPlaying != m.player.Playing`, emit `PlaybackStatus` change.

### 10. program.Send() blocking D-Bus handler
bubbletea's `msg` channel is unbuffered (`make(chan Msg)`). `program.Send()` blocks until program consumes message. D-Bus handler goroutine hung, blocking all subsequent headphone actions. Fixed by wrapping in `go b.program.Send(msg)`.

## Open Issue: Seek Performance

**Symptom:** Consecutive seeks via headphone buttons are terribly slow (several seconds between them).

**What we know:**
- The seek IPC call itself (`mpvipc.Call("seek", ...)`) is synchronous — holds a mutex lock and waits for mpv response
- Position tick (`TickCmd`) fires every 500ms and makes 3 sequential IPC calls (`GetPosition`, `GetDuration`, `GetPaused`), each acquiring the same lock
- The tick holds the lock ~300ms out of every 500ms cycle
- Seek goroutine has to wait for the lock between ticks
- `seekToBookGlobalPosition` was changed to fire-and-forget (`go mpvPlayer.Seek()`) — no longer blocks bubbletea event loop
- Added `seekPending` flag to prevent position ticks from overwriting optimistic position during seek

**What didn't work:**
- Fire-and-forget seek goroutine — still blocked on mpv IPC lock behind ticks
- Debouncing seeks — user correctly pointed out it's wrong (each seek is relative to current position)
- seekPending flag — prevents UI jumps but doesn't fix the slowness

**Suspected root cause:** The position tick's 3 sequential IPC calls (every 500ms) saturate the mpv IPC lock. Seek goroutine has to wait behind them. With IPC round-trip ~100ms per call, tick holds lock ~300ms/500ms = 60% of the time. Seek waits up to 300ms per attempt. With rapid seeks, they queue up.

**Possible solutions to investigate:**
1. **Batch IPC calls** — read position+duration+paused in a single `mpvipc.Call` (if mpv supports it)
2. **Reduce tick frequency** — change from 500ms to 2000ms (trade off: slower position updates)
3. **Skip tick during seek** — add atomic flag on `Mpv` struct, tick checks it and skips IPC if seek is pending
4. **Use mpv observe property** — instead of polling, use mpv's `observe_property` to get push-based position updates (eliminates tick IPC entirely)

**Key files:**
- `internal/player/mpv.go` — `Seek()`, `GetPosition()`, `GetDuration()`, `GetPaused()` all go through `conn.Call/Get` with shared lock
- `internal/player/commands.go` — `TickCmd` makes 3 sequential IPC calls every 500ms
- `internal/app/bookmarks.go:225` — `seekToBookGlobalPosition` fires seek goroutine
- `internal/app/playback.go:200` — `handlePositionMsg` processes ticks, chains next tick
- `internal/mpris/bridge.go` — D-Bus handler sends seek via `go program.Send()`

## playerctl / playerctld notes
- `playerctld` caches player capabilities at discovery time — must be killed and restarted after rebuilding pine with capability changes
- `playerctl -l` delegates to `playerctld` — if daemon is running with stale data, kill it first
- `playerctl metadata` returns "No player could handle this command" when `CanPlay=false`
- `playerctl -p pine status` works even without `playerctld`
