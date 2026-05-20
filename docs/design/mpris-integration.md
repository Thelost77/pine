# MPRIS / D-Bus Integration

## Status

Draft — investigation complete, ready for implementation

## Context

MPRIS (Media Player Remote Interfacing Specification) is a D-Bus standard that allows external applications to discover, control, and query media players. Integrating MPRIS into pine would enable:

- **Desktop environment integration**: GNOME, KDE, and other DEs show playback controls in the system tray / media widget
- **Hardware media keys**: Keyboard play/pause/next/prev keys work out of the box via `playerctl` or DE media key daemon
- **Third-party controllers**: Tools like `playerctl`, `mpris-proxy`, `polybar` MPRIS modules, and phone remote apps can control pine
- **Observability**: Other apps can read current track, position, and playback status

jellyfin-tui has MPRIS integration, which is one of the features that makes it feel like a "real" desktop media player rather than just a terminal app.

## Investigation

### Library: `github.com/quarckster/go-mpris-server`

This is a Go implementation of the **server side** of the MPRIS 2.2 D-Bus interface. It depends on `github.com/godbus/dbus/v5` for D-Bus communication.

**Architecture:**

```
pkg/types/types.go      — Adapter interfaces you implement
pkg/server/server.go    — D-Bus server that registers your player on the bus
pkg/events/*.go         — Event handler that emits PropertiesChanged signals
```

**Usage pattern:**

1. Implement `OrgMprisMediaPlayer2Adapter` (root interface — identity, can-quit, etc.)
2. Implement `OrgMprisMediaPlayer2PlayerAdapter` (player interface — play, pause, seek, metadata, etc.)
3. Create `server.NewServer("pine", rootAdapter, playerAdapter)`
4. Create `events.NewEventHandler(server)`
5. Call `eventHandler.Player.OnPlayPause()` etc. when player state changes
6. Run `server.Listen()` in a goroutine

**Key interfaces:**

```go
// Root adapter — basic player identity
type OrgMprisMediaPlayer2Adapter interface {
    Raise() error
    Quit() error
    CanQuit() (bool, error)
    CanRaise() (bool, error)
    HasTrackList() (bool, error)
    Identity() (string, error)
    SupportedUriSchemes() ([]string, error)
    SupportedMimeTypes() ([]string, error)
}

// Player adapter — playback control + state
type OrgMprisMediaPlayer2PlayerAdapter interface {
    Next() error
    Previous() error
    Pause() error
    PlayPause() error
    Stop() error
    Play() error
    Seek(offset Microseconds) error
    SetPosition(trackId string, position Microseconds) error
    OpenUri(uri string) error
    PlaybackStatus() (PlaybackStatus, error)
    Rate() (float64, error)
    SetRate(float64) error
    Metadata() (Metadata, error)
    Volume() (float64, error)
    SetVolume(float64) error
    Position() (int64, error)
    MinimumRate() (float64, error)
    MaximumRate() (float64, error)
    CanGoNext() (bool, error)
    CanGoPrevious() (bool, error)
    CanPlay() (bool, error)
    CanPause() (bool, error)
    CanSeek() (bool, error)
    CanControl() (bool, error)
}
```

**Metadata struct:**

```go
type Metadata struct {
    TrackId        dbus.ObjectPath
    Length         Microseconds
    ArtUrl         string
    Album          string
    AlbumArtist    []string
    Artist         []string
    Title          string
    // ... and many more xesam fields
}
```

**Event handler methods:**

```go
eventHandler.Player.OnPlayPause()  // emit PlaybackStatus change
eventHandler.Player.OnTitle()      // emit Metadata change
eventHandler.Player.OnSeek(pos)    // emit Position change
eventHandler.Player.OnVolume()     // emit Volume change
eventHandler.Player.OnPlayback()   // emit PlaybackStatus + Metadata + Rate
eventHandler.Player.OnEnded()      // emit PlaybackStatus = Stopped
eventHandler.Player.OnOptions()    // emit CanGoNext, CanPlay, etc.
eventHandler.Player.OnAll()        // emit all properties
```

The event handler internally calls `dbus.Conn.Emit` with `PropertiesChanged` signals, which is what MPRIS clients listen for.

**Important finding:** The library does **not** auto-detect state changes. You must manually call the appropriate `On*` method whenever pine's player state changes. This means the integration point is in pine's playback lifecycle — specifically where `PositionMsg`, play/pause commands, and track changes happen.

### D-Bus Service Name

MPRIS clients discover players by watching D-Bus names matching `org.mpris.MediaPlayer2.*`. The library handles this:

```go
serviceName := "org.mpris.MediaPlayer2." + name  // e.g. "org.mpris.MediaPlayer2.pine"
conn.RequestName(serviceName, dbus.NameFlagReplaceExisting)
```

`playerctl` will then list it as `pine`.

### MPRIS Properties to Map

| MPRIS Property | Pine Source | Notes |
|----------------|-------------|-------|
| `Identity` | `"pine"` | Static |
| `CanQuit` | `true` | pine can always quit |
| `CanRaise` | `false` | TUI app, can't "raise" a window |
| `HasTrackList` | `false` | No track list interface planned |
| `PlaybackStatus` | `player.Playing` | `Playing` / `Paused` / `Stopped` |
| `Metadata` | `player.Title` + ABS item data | `xesam:title`, `xesam:artist`, `mpris:length` |
| `Position` | `player.Position` | In microseconds |
| `Volume` | `player.Volume` | 0.0–1.0 (pine uses 0–150, so divide by 100) |
| `Rate` | `player.Speed` | 1.0 = normal |
| `MinimumRate` | `0.1` | mpv min speed |
| `MaximumRate` | `4.0` | mpv max speed |
| `CanPlay` | `true` if item loaded | |
| `CanPause` | `true` if playing or paused | |
| `CanSeek` | `true` if playing or paused | |
| `CanGoNext` | `len(queue) > 0` | Queue-based next |
| `CanGoPrevious` | `false` | No previous-track feature yet |
| `CanControl` | `true` if item loaded | |

### MPRIS Methods to Map

| MPRIS Method | Pine Action | Notes |
|--------------|-------------|-------|
| `Play()` | Start playback / resume | If stopped, play current item. If paused, resume. |
| `Pause()` | Pause | |
| `PlayPause()` | Toggle | Same as `p` / space key |
| `Stop()` | Stop playback | Same as `q` during playback |
| `Next()` | Skip to next queued | Same as `>` key |
| `Previous()` | No-op | Not implemented in pine |
| `Seek(offset)` | Seek by offset | `offset` in microseconds. Positive = forward. |
| `SetPosition(trackId, pos)` | Seek to absolute position | `pos` in microseconds. Validate `trackId` matches current. |
| `OpenUri(uri)` | No-op / unsupported | pine doesn't support opening arbitrary URIs |
| `SetVolume(vol)` | Set volume | `vol` is 0.0–1.0. Map to pine's 0–150 scale. |
| `SetRate(rate)` | Set speed | Map to pine's speed. Validate within min/max. |

## Decision

Add MPRIS/D-Bus integration to pine using `github.com/quarckster/go-mpris-server`. The integration should:

1. Start a D-Bus server on app launch (when client is authenticated)
2. Expose `org.mpris.MediaPlayer2.pine` on the session bus
3. Emit `PropertiesChanged` signals when playback state, metadata, position, or volume changes
4. Accept remote control commands (Play, Pause, Next, Seek, SetVolume, etc.)
5. Stop the server on app quit to clean up the D-Bus name

## Architecture

### New Files

```
internal/mpris/
├── adapter.go       # RootAdapter + PlayerAdapter implementations
├── bridge.go        # Bridge between pine Model and adapters
├── server.go        # MPRIS server lifecycle (start/stop)
└── mpris_test.go    # Unit tests
```

### Modified Files

```
internal/app/model.go       # Start MPRIS server on login, emit events
internal/app/playback.go    # Emit MPRIS events on play/pause/seek/stop
internal/app/messages.go    # Add MprisEventMsg
main.go                     # Stop MPRIS server on shutdown
go.mod                      # Add go-mpris-server + godbus dependencies
```

### Component: `internal/mpris/adapter.go`

```go
package mpris

import (
    "github.com/godbus/dbus/v5"
    "github.com/quarckster/go-mpris-server/pkg/types"
)

type RootAdapter struct{}

func (r *RootAdapter) Raise() error                    { return nil }
func (r *RootAdapter) Quit() error                     { return nil }
func (r *RootAdapter) CanQuit() (bool, error)          { return true, nil }
func (r *RootAdapter) CanRaise() (bool, error)         { return false, nil }
func (r *RootAdapter) HasTrackList() (bool, error)     { return false, nil }
func (r *RootAdapter) Identity() (string, error)       { return "pine", nil }
func (r *RootAdapter) SupportedUriSchemes() ([]string, error) { return nil, nil }
func (r *RootAdapter) SupportedMimeTypes() ([]string, error)  { return nil, nil }

// PlayerAdapter holds a function-pointer bridge to pine's player state.
// It does not store state itself — it calls back into pine via closures.
type PlayerAdapter struct {
    GetPlaybackStatus func() types.PlaybackStatus
    GetMetadata       func() types.Metadata
    GetPosition       func() types.Microseconds
    GetVolume         func() float64
    GetRate           func() float64
    GetCanPlay        func() bool
    GetCanPause       func() bool
    GetCanSeek        func() bool
    GetCanGoNext      func() bool
    GetCanGoPrevious  func() bool
    GetCanControl     func() bool

    OnPlay       func() error
    OnPause      func() error
    OnPlayPause  func() error
    OnStop       func() error
    OnNext       func() error
    OnPrevious   func() error
    OnSeek       func(offset types.Microseconds) error
    OnSetPosition func(trackId string, pos types.Microseconds) error
    OnSetVolume  func(vol float64) error
    OnSetRate    func(rate float64) error
}

func (p *PlayerAdapter) Next() error              { return p.OnNext() }
func (p *PlayerAdapter) Previous() error          { return p.OnPrevious() }
func (p *PlayerAdapter) Pause() error             { return p.OnPause() }
func (p *PlayerAdapter) PlayPause() error         { return p.OnPlayPause() }
func (p *PlayerAdapter) Stop() error              { return p.OnStop() }
func (p *PlayerAdapter) Play() error              { return p.OnPlay() }
func (p *PlayerAdapter) Seek(offset types.Microseconds) error { return p.OnSeek(offset) }
func (p *PlayerAdapter) SetPosition(trackId string, pos types.Microseconds) error {
    return p.OnSetPosition(trackId, pos)
}
func (p *PlayerAdapter) OpenUri(uri string) error { return nil }
func (p *PlayerAdapter) PlaybackStatus() (types.PlaybackStatus, error) {
    return p.GetPlaybackStatus(), nil
}
func (p *PlayerAdapter) Rate() (float64, error)        { return p.GetRate(), nil }
func (p *PlayerAdapter) SetRate(r float64) error       { return p.OnSetRate(r) }
func (p *PlayerAdapter) Metadata() (types.Metadata, error) { return p.GetMetadata(), nil }
func (p *PlayerAdapter) Volume() (float64, error)      { return p.GetVolume(), nil }
func (p *PlayerAdapter) SetVolume(v float64) error     { return p.OnSetVolume(v) }
func (p *PlayerAdapter) Position() (int64, error)      { return int64(p.GetPosition()), nil }
func (p *PlayerAdapter) MinimumRate() (float64, error) { return 0.1, nil }
func (p *PlayerAdapter) MaximumRate() (float64, error) { return 4.0, nil }
func (p *PlayerAdapter) CanGoNext() (bool, error)      { return p.GetCanGoNext(), nil }
func (p *PlayerAdapter) CanGoPrevious() (bool, error)  { return p.GetCanGoPrevious(), nil }
func (p *PlayerAdapter) CanPlay() (bool, error)        { return p.GetCanPlay(), nil }
func (p *PlayerAdapter) CanPause() (bool, error)       { return p.GetCanPause(), nil }
func (p *PlayerAdapter) CanSeek() (bool, error)        { return p.GetCanSeek(), nil }
func (p *PlayerAdapter) CanControl() (bool, error)     { return p.GetCanControl(), nil }
```

### Component: `internal/mpris/server.go`

```go
package mpris

import (
    "github.com/quarckster/go-mpris-server/pkg/events"
    "github.com/quarckster/go-mpris-server/pkg/server"
    "github.com/quarckster/go-mpris-server/pkg/types"
)

type Server struct {
    srv     *server.Server
    handler *events.EventHandler
}

func NewServer(root types.OrgMprisMediaPlayer2Adapter, player types.OrgMprisMediaPlayer2PlayerAdapter) *Server {
    s := server.NewServer("pine", root, player)
    h := events.NewEventHandler(s)
    return &Server{srv: s, handler: h}
}

func (s *Server) Listen() error {
    return s.srv.Listen() // blocks, run in goroutine
}

func (s *Server) Stop() error {
    return s.srv.Stop()
}

func (s *Server) EventHandler() *events.EventHandler {
    return s.handler
}
```

### Component: `internal/mpris/bridge.go`

```go
package mpris

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/quarckster/go-mpris-server/pkg/types"
)

// Bridge connects pine's root Model to the MPRIS adapters.
// It holds function closures that read from / write to pine's state.
type Bridge struct {
    server   *Server
    adapter  *PlayerAdapter
}

func NewBridge() *Bridge {
    root := &RootAdapter{}
    player := &PlayerAdapter{}
    srv := NewServer(root, player)
    return &Bridge{server: srv, adapter: player}
}

// Start runs the D-Bus server in a goroutine.
func (b *Bridge) Start() {
    go func() {
        if err := b.server.Listen(); err != nil {
            // Log error but don't crash the app
            // D-Bus might not be available (e.g. SSH session, container)
        }
    }()
}

// Stop shuts down the D-Bus server.
func (b *Bridge) Stop() error {
    return b.server.Stop()
}

// Bind connects the adapter closures to pine's model.
// Call this after creating the bridge but before Start.
func (b *Bridge) Bind(getModel func() ModelAccessor) {
    b.adapter.GetPlaybackStatus = func() types.PlaybackStatus {
        m := getModel()
        if !m.IsPlaying() && !m.IsPaused() {
            return types.PlaybackStatusStopped
        }
        if m.IsPlaying() {
            return types.PlaybackStatusPlaying
        }
        return types.PlaybackStatusPaused
    }

    b.adapter.GetMetadata = func() types.Metadata {
        m := getModel()
        item := m.CurrentItem()
        if item == nil {
            return types.Metadata{}
        }
        md := types.Metadata{
            TrackId: dbus.ObjectPath("/org/mpris/MediaPlayer2/Track/" + item.ID),
            Title:   item.Media.Metadata.Title,
        }
        if m.PlayerDuration() > 0 {
            md.Length = types.Microseconds(m.PlayerDuration() * 1_000_000)
        }
        if len(item.Media.Metadata.Authors) > 0 {
            for _, a := range item.Media.Metadata.Authors {
                md.Artist = append(md.Artist, a.Name)
            }
        }
        // Cover art URL from ABS
        if item.Media.CoverPath != "" {
            md.ArtUrl = m.ServerURL() + "/api/items/" + item.ID + "/cover"
        }
        return md
    }

    b.adapter.GetPosition = func() types.Microseconds {
        return types.Microseconds(getModel().PlayerPosition() * 1_000_000)
    }

    b.adapter.GetVolume = func() float64 {
        return float64(getModel().PlayerVolume()) / 100.0
    }

    b.adapter.GetRate = func() float64 {
        return getModel().PlayerSpeed()
    }

    b.adapter.GetCanPlay = func() bool { return getModel().HasActiveItem() }
    b.adapter.GetCanPause = func() bool { return getModel().IsPlaying() || getModel().IsPaused() }
    b.adapter.GetCanSeek = func() bool { return getModel().IsPlaying() || getModel().IsPaused() }
    b.adapter.GetCanGoNext = func() bool { return getModel().QueueLength() > 0 }
    b.adapter.GetCanGoPrevious = func() bool { return false }
    b.adapter.GetCanControl = func() bool { return getModel().HasActiveItem() }

    // Action handlers emit tea.Msg back into pine's Update loop
    b.adapter.OnPlay = func() error {
        return getModel().EmitMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
    }
    b.adapter.OnPause = func() error {
        return getModel().EmitMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
    }
    b.adapter.OnPlayPause = func() error {
        return getModel().EmitMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
    }
    b.adapter.OnStop = func() error {
        return getModel().EmitMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
    }
    b.adapter.OnNext = func() error {
        return getModel().EmitMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
    }
    b.adapter.OnSeek = func(offset types.Microseconds) error {
        return getModel().EmitMsg(MprisSeekMsg{Offset: float64(offset) / 1_000_000})
    }
    b.adapter.OnSetPosition = func(trackId string, pos types.Microseconds) error {
        return getModel().EmitMsg(MprisSetPositionMsg{TrackID: trackId, Position: float64(pos) / 1_000_000})
    }
    b.adapter.OnSetVolume = func(vol float64) error {
        return getModel().EmitMsg(MprisSetVolumeMsg{Volume: int(vol * 100)})
    }
    b.adapter.OnSetRate = func(rate float64) error {
        return getModel().EmitMsg(MprisSetRateMsg{Rate: rate})
    }
}
```

**Note on the bridge pattern:** The bridge uses function closures rather than holding a reference to `app.Model` directly. This avoids a circular dependency between `internal/mpris` and `internal/app`. The `ModelAccessor` interface (defined in `internal/mpris`) exposes only the methods MPRIS needs:

```go
type ModelAccessor interface {
    IsPlaying() bool
    IsPaused() bool
    HasActiveItem() bool
    CurrentItem() *abs.LibraryItem
    PlayerPosition() float64
    PlayerDuration() float64
    PlayerVolume() int
    PlayerSpeed() float64
    QueueLength() int
    ServerURL() string
    EmitMsg(msg tea.Msg) error
}
```

The `EmitMsg` method is the critical bridge: MPRIS callbacks run on a D-Bus goroutine, so they cannot directly call `Model.Update()`. Instead, they queue a message that pine's bubbletea event loop will process on the next tick.

### Root Model Integration

Add to `Model`:
```go
mprisBridge *mpris.Bridge
```

In `NewWithPlayer`:
```go
m.mprisBridge = mpris.NewBridge()
m.mprisBridge.Bind(func() mpris.ModelAccessor { return &m })
m.mprisBridge.Start()
```

In `Cleanup`:
```go
func (m *Model) Cleanup() {
    if m.mprisBridge != nil {
        _ = m.mprisBridge.Stop()
    }
    // ... existing cleanup
}
```

In `Update`, handle MPRIS messages:
```go
case MprisSeekMsg:
    return m.handleSeek(msg.Offset)
case MprisSetPositionMsg:
    // Validate trackId matches current item, then seek
    return m.seekToPosition(msg.Position)
case MprisSetVolumeMsg:
    // Update player volume
    return m.setPlayerVolume(msg.Volume)
case MprisSetRateMsg:
    // Update player speed
    return m.setPlayerSpeed(msg.Rate)
```

### Event Emission

The library requires explicit event calls. Hook these into pine's existing playback lifecycle:

| Pine Event | MPRIS Event Call | Where |
|------------|------------------|-------|
| Play starts | `eventHandler.Player.OnPlayback()` | `handlePlaySessionMsg` |
| Pause toggled | `eventHandler.Player.OnPlayPause()` | `handleKey` (play/pause) |
| Stop | `eventHandler.Player.OnEnded()` | `stopPlayback` |
| Track changes | `eventHandler.Player.OnTitle()` | `handlePlaySessionMsg` |
| Position update | `eventHandler.Player.OnSeek(pos)` | `handlePositionMsg` (throttled?) |
| Volume change | `eventHandler.Player.OnVolume()` | `handleKey` (vol up/down) |
| Speed change | `eventHandler.Player.OnPlayback()` | `handleKey` (speed up/down) |
| Queue changes | `eventHandler.Player.OnOptions()` | `enqueueQueueEntry` |

**Position throttling:** `handlePositionMsg` fires every 500ms (mpvtick). Emitting `OnSeek` that frequently would spam D-Bus. Throttle to once per second, or only emit when the position change is > 5 seconds (indicating a user-initiated seek rather than normal playback).

```go
func (m Model) maybeEmitMprisPosition() {
    if m.mprisBridge == nil { return }
    now := time.Now()
    if now.Sub(m.lastMprisPositionEmit) < time.Second {
        return
    }
    m.lastMprisPositionEmit = now
    m.mprisBridge.EventHandler().Player.OnSeek(
        types.Microseconds(m.player.Position * 1_000_000),
    )
}
```

### `main.go` Changes

The D-Bus server should start **after** successful login, not at app startup. An unauthenticated pine has nothing to play. Modify the login success handler:

```go
case login.LoginSuccessMsg:
    // ... existing login handling ...
    m.mprisBridge = mpris.NewBridge()
    m.mprisBridge.Bind(func() mpris.ModelAccessor { return &m })
    m.mprisBridge.Start()
    return m, cmd
```

And stop it in `Cleanup` before quitting mpv.

## Error Handling

D-Bus might not be available in all environments:

| Scenario | Behavior |
|----------|----------|
| **No session bus** (SSH without X11/D-Bus forwarding) | `server.Listen()` returns error. Log at Debug level, continue without MPRIS. |
| **Name already taken** | `dbus.NameFlagReplaceExisting` handles this — new instance replaces old. |
| **D-Bus daemon stops** | Server goroutine exits. Log error, pine continues normally. |
| **Permission denied** | Log error, continue without MPRIS. |

Never let MPRIS errors crash or block pine.

## Testing

### Unit Tests (`internal/mpris/mpris_test.go`)

```go
func TestRootAdapterIdentity(t *testing.T)
func TestPlayerAdapterPlaybackStatus(t *testing.T)
func TestPlayerAdapterMetadata(t *testing.T)
func TestPlayerAdapterVolumeMapping(t *testing.T)
func TestPlayerAdapterSeekCallsEmit(t *testing.T)
```

### E2E Tests (`internal/app/e2e_test.go`)

Use the mock player and mock HTTP server. Verify:
- After login, MPRIS server starts (check D-Bus name if session bus available)
- After play, `PlaybackStatus` returns `Playing`
- After pause, `PlaybackStatus` returns `Paused`
- After stop, `PlaybackStatus` returns `Stopped`
- `Metadata` returns correct title

### Manual Testing

```bash
# Check if pine appears in playerctl list
playerctl -l | grep pine

# Get playback status
playerctl -p pine status        # Playing / Paused / Stopped

# Get metadata
playerctl -p pine metadata      # title, artist, length

# Control playback
playerctl -p pine play
playerctl -p pine pause
playerctl -p pine play-pause
playerctl -p pine stop
playerctl -p pine next
playerctl -p pine position 120  # seek to 2:00
playerctl -p pine volume 0.5

# Listen for property changes
playerctl -p pine -F metadata
```

## Dependencies

Add to `go.mod`:

```
github.com/quarckster/go-mpris-server v0.x.x
github.com/godbus/dbus/v5 v5.x.x  // transitive, but good to pin
```

The library is GPL-3.0 licensed. Pine is MIT licensed. Using a GPL library as a dependency does **not** infect pine's MIT license (the GPL "viral" clause applies to derivative works, not independent programs that link against a library). However, distributing a combined binary that includes GPL code requires compliance with GPL terms for that binary. This is acceptable for an open-source project.

## Open Questions

1. **Should MPRIS start before login?** Currently proposed to start after login since there's no content to play before authentication. But starting early would let `playerctl` discover pine immediately and show "Stopped" status.

2. **Track ID format:** The MPRIS spec requires `mpris:trackid` to be a D-Bus object path. Proposal: `/org/mpris/MediaPlayer2/Track/<itemID>`. The `<itemID>` is pine's ABS item ID.

3. **Art URL:** ABS cover art requires authentication. MPRIS clients may fail to load it. Should pine expose a local proxy, or omit `mpris:artUrl`?

4. **Position accuracy:** `Position()` is polled every 500ms by MPRIS clients. Should pine cache the last known position to avoid IPC overhead?

5. **Rate/speed mapping:** MPRIS `Rate` is playback speed. Pine's `player.Speed` is the same concept. Direct 1:1 mapping.

6. **Previous track:** MPRIS `Previous()` is not implemented in pine. Return `CanGoPrevious = false` and make `Previous()` a no-op.

7. **Chapter navigation:** Should `Next()` skip to next chapter when chapters exist, or always next queued item? Current pine behavior (`>` key) is "next queued". MPRIS `Next` should match this.

## References

- MPRIS 2.2 Specification: https://specifications.freedesktop.org/mpris-spec/2.2/
- `go-mpris-server`: https://github.com/quarckster/go-mpris-server
- `godbus/dbus`: https://github.com/godbus/dbus
- `playerctl`: https://github.com/altdesktop/playerctl
- jellyfin-tui MPRIS: `src/mpris.rs` (Rust reference)
