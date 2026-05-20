# Plan: mpris-integration

## Problem and target
Pine is invisible to the Linux desktop environment. Hardware media keys, DE widgets, and `playerctl` can't discover or control it. Adding MPRIS/D-Bus integration makes pine behave like a real desktop media player.

## Fixed decisions
- `*tea.Program.Send()` for D-Bus goroutine → bubbletea event loop messaging
- Closures over value copies for player state access (no mutex needed for simple types)
- Dedicated MPRIS message types (`MprisPlayPauseMsg`, `MprisSeekMsg`, `MprisSetVolumeMsg`, `MprisSetRateMsg`)
- `SetProgram(p *tea.Program)` pattern to inject program reference after construction
- MPRIS starts at app launch (auto-login assumed)
- `config.Player.SeekSeconds` for Next/Previous seek amount (same as `h`/`l` keys)
- Non-standard Next/Previous mapping to seek ±Ns (commented in code)
- Art URL omitted (ABS cover requires auth)
- Platform-agnostic `ModelAccessor` interface for future macOS support

## Assumptions
- D-Bus session bus is available on target systems (Linux desktop)
- Graceful degradation: if D-Bus unavailable, log and continue without MPRIS
- Player state fields (bool, float64, int) are safe to read from D-Bus goroutine without locking

## Non-goals
- macOS media control (future)
- Cover art via local proxy
- Chapter-level Next/Previous (currently seek-based)
- Track list interface (`HasTrackList = false`)

## Architecture / delivery shape

### New package: `internal/mpris/`
```
internal/mpris/
├── accessor.go    # ModelAccessor interface
├── adapter.go     # RootAdapter + PlayerAdapter (closure-based)
├── bridge.go      # Bridge: *tea.Program → MPRIS adapters via Send()
├── server.go      # Server lifecycle wrapper
└── mpris_test.go  # Unit tests
```

### Modified files
```
internal/app/model.go       # mprisBridge field, SetProgram, handle MPRIS messages
internal/app/messages.go    # MprisPlayPauseMsg, MprisSeekMsg, MprisSetVolumeMsg, MprisSetRateMsg
internal/app/playback.go    # Emit MPRIS events on play/pause/stop/seek
main.go                     # Call SetProgram between NewProgram and Run
go.mod / go.sum             # Add go-mpris-server + godbus/dbus/v5
```

### Key interfaces
```go
// internal/mpris/accessor.go
type ModelAccessor interface {
    IsPlaying() bool
    IsPaused() bool
    HasActiveItem() bool
    CurrentTitle() string
    CurrentAuthors() []string
    CurrentItemID() string
    PlayerPosition() float64
    PlayerDuration() float64
    PlayerVolume() int
    PlayerSpeed() float64
    QueueLength() int
}
```

### MPRIS → Pine message flow
```
D-Bus goroutine → PlayerAdapter.On*() → program.Send(MprisXxxMsg) → Update() → handleSeek/setVolume/setSpeed
```

### Pine → MPRIS event flow
```
Playback lifecycle (play/pause/stop/position) → bridge.Emit*() → eventHandler.Player.On*() → D-Bus PropertiesChanged
```

## Tasks / delivery order

### 1. Add dependencies [size: S]
- **Goal:** `go-mpris-server` and `godbus/dbus/v5` available in go.mod
- **Files:** `go.mod`, `go.sum`
- **Action:** `go get github.com/quarckster/go-mpris-server` and `go get github.com/godbus/dbus/v5`
- **Verify:** `go mod tidy && go build ./...`
- **Done:** Dependencies in go.mod, project builds

### 2. Create MPRIS adapters and server [size: M]
- **Goal:** Core MPRIS adapter implementations with unit tests
- **Files:** `internal/mpris/accessor.go`, `internal/mpris/adapter.go`, `internal/mpris/server.go`, `internal/mpris/mpris_test.go`
- **Action:**
  - Define `ModelAccessor` interface in `accessor.go`
  - Implement `RootAdapter` (static: Identity="pine", CanQuit=true, CanRaise=false, HasTrackList=false)
  - Implement `PlayerAdapter` with closure fields for all getters and action handlers
  - Implement `Server` wrapper (NewServer, Listen, Stop, EventHandler)
- **Edge cases:** D-Bus not available — `Listen()` returns error, handled gracefully at bridge level
- **TDD:** yes
  - RED: Write tests for RootAdapter identity, PlayerAdapter playback status mapping, volume conversion (0-150 → 0.0-1.0), metadata assembly
  - Verify RED: `go test ./internal/mpris/... -v` → fails (no implementation)
  - GREEN: Implement adapters and server
  - Verify GREEN: `go test ./internal/mpris/... -v` → passes
  - REFACTOR: Only if needed
- **Verify:** `go test ./internal/mpris/... -v && go vet ./internal/mpris/...`
- **Done:** Adapters implement MPRIS interfaces, unit tests pass

### 3. Create bridge and app message types [size: M]
- **Depends on:** Task 2
- **Goal:** Bridge connects `*tea.Program` to MPRIS adapters. Message types defined for app-level handling.
- **Files:** `internal/mpris/bridge.go`, `internal/app/messages.go`
- **Action:**
  - Create `Bridge` struct with `*tea.Program`, `*Server`, `*PlayerAdapter`
  - `NewBridge(program *tea.Program)` — creates adapters, server
  - `Bind(accessor ModelAccessor)` — wires closures:
    - Getters read from accessor (state snapshots)
    - Action handlers call `program.Send(MprisXxxMsg)` for play/pause/seek/volume/rate
    - Next/Previous → seek ±config.Player.SeekSeconds (non-standard, commented)
  - `Start()` — runs `server.Listen()` in goroutine, logs error if D-Bus unavailable
  - `Stop()` — calls `server.Stop()`
  - Define message types in `messages.go`: `MprisPlayPauseMsg`, `MprisSeekMsg{Offset float64}`, `MprisSetVolumeMsg{Volume int}`, `MprisSetRateMsg{Rate float64}`
- **Edge cases:** D-Bus unavailable on Start — log at Warn level, bridge stays functional (just no external access)
- **Verify:** `go build ./...`
- **Done:** Bridge compiles, message types defined

### 4. Integrate MPRIS into Model [size: M]
- **Depends on:** Task 3
- **Goal:** Model owns MPRIS bridge, handles MPRIS messages, starts/stops MPRIS lifecycle
- **Files:** `internal/app/model.go`
- **Action:**
  - Add fields to `Model`: `mprisBridge *mpris.Bridge`, `program *tea.Program`
  - Add `SetProgram(p *tea.Program)` method (pointer receiver)
  - In `Init()`: if program is set, create bridge, bind accessor, start
  - In `Update()` tea.KeyMsg block: add cases for MPRIS messages:
    - `MprisPlayPauseMsg` → toggle pause via player model + mpv
    - `MprisSeekMsg` → call `handleSeek(msg.Offset)`
    - `MprisSetVolumeMsg` → set player.Volume, call mpv.SetVolume
    - `MprisSetRateMsg` → set player.Speed, call mpv.SetSpeed
  - Implement `ModelAccessor` methods on `Model` (simple field reads)
  - In `Cleanup()`: stop MPRIS bridge before mpv quit
- **Edge cases:** MPRIS seek when not playing → no-op. Volume/rate changes when not playing → no-op.
- **Verify:** `go build ./... && go vet ./...`
- **Done:** Model handles MPRIS messages, bridge starts/stops with app lifecycle

### 5. Emit MPRIS events from playback lifecycle [size: S]
- **Depends on:** Task 4
- **Goal:** MPRIS clients receive PropertiesChanged signals when pine's state changes
- **Files:** `internal/app/playback.go`, `internal/app/model.go`
- **Action:**
  - Add `emitMprisEvent` helper that checks bridge != nil before calling
  - In `handlePlaySessionMsg`: emit `OnPlayback()` (status + metadata)
  - In player pause toggle path: emit `OnPlayPause()`
  - In `stopPlayback`: emit `OnEnded()`
  - In `handlePositionMsg`: emit `OnSeek(position)` throttled to 1s
  - In volume/speed change paths: emit `OnVolume()` / `OnPlayback()`
  - Add `lastMprisEmit time.Time` field to Model for throttling
- **Edge cases:** Bridge nil (D-Bus unavailable) → all emit calls are no-ops. Position throttle → skip if <1s since last emit.
- **Verify:** `go build ./... && go test ./...`
- **Done:** State changes emit D-Bus signals

### 6. Wire SetProgram in main.go [size: S]
- **Depends on:** Task 4
- **Goal:** Program reference available to Model before Init runs
- **Files:** `main.go`
- **Action:** After `tea.NewProgram(model)`, call `model.SetProgram(p)` before `p.Run()`
- **Verify:** `go build -o pine . && go vet ./...`
- **Done:** App builds, MPRIS starts on launch

## Product-level verification criteria
- `playerctl -p pine status` returns Playing/Paused/Stopped correctly
- `playerctl -p pine play-pause` toggles playback
- `playerctl -p pine metadata` returns correct title and author
- Hardware media keys (play/pause) control pine via DE
- App starts and functions normally without D-Bus (SSH session, container)
- No goroutine leaks or crashes from MPRIS goroutine

## Open implementation questions
- None — all resolved in brainstorm and clarify phases
