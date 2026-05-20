# Context: mpris-integration

## Implementation Decisions

### Message passing from D-Bus goroutine
- **Decision:** Use `*tea.Program.Send()` to inject messages from the D-Bus goroutine into bubbletea's event loop.
- **Rationale:** Standard bubbletea pattern for external goroutines. Thread-safe. No custom infrastructure needed.
- **Options considered:** Custom `EmitMsg` wrapper — rejected as it would just wrap `Program.Send()` anyway.

### Player state access from D-Bus goroutine
- **Decision:** Closures over value copies. Each getter reads the relevant field at call time.
- **Rationale:** Player state fields are simple types (bool, float64, int). No slices/maps that could be mid-mutation. Race-free in practice without explicit locking.
- **Options considered:** Mutex-protected accessor — rejected as unnecessary complexity for simple field reads.

### MPRIS command message types
- **Decision:** Define dedicated message types: `MprisPlayPauseMsg`, `MprisSeekMsg`, `MprisSetVolumeMsg`, `MprisSetRateMsg`. Handle in `Update()`.
- **Rationale:** Decouples MPRIS from keyboard handling. Routes through proper playback logic (`handleSeek`, player volume/speed setters).
- **Options considered:** Synthesizing `tea.KeyMsg` — rejected as fragile, bypasses proper playback logic.

### MPRIS lifecycle / program injection
- **Decision:** Add `SetProgram(p *tea.Program)` on the Model. Call it between `tea.NewProgram(model)` and `p.Run()`. MPRIS bridge starts on the first update tick or in `Init()`.
- **Rationale:** Bridge needs `*tea.Program` for `Send()`. Program doesn't exist at model construction time. This is the cleanest injection point.
- **Options considered:** Start in `Init()` without program reference (impossible), start after `p.Run()` in main (blocking).

### Seek amount for Next/Previous
- **Decision:** Use `config.Player.SeekSeconds` — same value as `h`/`l` keyboard seek.
- **Rationale:** Consistent behavior between keyboard and headphone controls. User configures once, applies everywhere.
- **Options considered:** Hardcode 30s — rejected as it would diverge from keyboard seek.

## Existing Code Insights

- **Seek logic** in `bookmarks.go:handleSeek(seconds float64)` handles both in-track and cross-track seeks. MPRIS seek calls should route through this.
- **Play/pause toggle** happens in `player.handleKey` → `TogglePauseCmd`. MPRIS play/pause should call the same path via a dedicated message.
- **Volume** stored as `int` (0-150) in `player.Model.Volume`. MPRIS uses `float64` (0.0-1.0). Conversion: `vol / 100.0` for MPRIS, `vol * 100` from MPRIS.
- **Speed** stored as `float64` in `player.Model.Speed`. Direct 1:1 mapping with MPRIS `Rate`.
- **Model is value-receiver** (`func (m Model)`) in most methods. `SetProgram` will need a pointer receiver or store the program pointer in the struct.
- **Cleanup()** in `playback.go` is synchronous, called after `p.Run()` returns. MPRIS bridge stop goes here.
- **Key bindings** defined in `keymap.go`. `NextInQueue` maps to `>` key. Chapter navigation maps to `n`/`N`. MPRIS `Next`/`Previous` will be separate (seek ±N seconds).

## Integration Points

- `internal/app/model.go` — add `mprisBridge` field, `SetProgram` method, handle new MPRIS messages in `Update()`
- `internal/app/playback.go` — emit MPRIS events on play start, pause, stop, position updates
- `internal/app/messages.go` — add `MprisPlayPauseMsg`, `MprisSeekMsg`, `MprisSetVolumeMsg`, `MprisSetRateMsg`
- `main.go` — call `SetProgram` between program creation and `p.Run()`
- `go.mod` — add `go-mpris-server` and `godbus/dbus/v5`

## Deferred Ideas

- macOS media control implementation (future)
- Cover art via local proxy or `file://` path
- Chapter-level Next/Previous (currently queue-based)
- Configurable seek amount specifically for MPRIS (use existing config)
