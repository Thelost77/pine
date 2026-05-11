# Brainstorm: mpris-integration

## Problem Statement

Pine is invisible to the Linux desktop environment. Hardware media keys (headphones, keyboard), DE media widgets (GNOME/KDE system tray), and tools like `playerctl` cannot discover or control pine. This makes pine feel like a terminal app rather than a desktop media player. Secondary: macOS support is desired eventually but not critical.

## Architecture

- **Platform abstraction:** Define a `MediaController` interface (play, pause, seek, get state, emit changes). MPRIS implementation for Linux. macOS implementation later. Pine's code only talks to the interface.
- **MPRIS library:** `github.com/quarckster/go-mpris-server` (server-side MPRIS 2.2, depends on `godbus/dbus/v5`)
- **Bridge pattern:** Closures from MPRIS adapters into pine's model state. `ModelAccessor` interface to avoid circular dependency. `EmitMsg` to send commands back into bubbletea's event loop from the D-Bus goroutine.
- **Lifecycle:** MPRIS server starts at app launch (credentials are always stored after first login). Gracefully degrades if D-Bus unavailable (log at debug level, continue).
- **Event emission:** Manual `On*` calls hooked into pine's playback lifecycle (play, pause, stop, seek, volume, speed, queue). Position emission throttled to 1s (pine already polls mpv every 500ms).
- **Position caching:** Cached on the model, updated on existing 500ms tick. MPRIS reads cache, not mpv IPC. Negligible staleness for audiobook use.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| MPRIS start timing | At app launch | Auto-login means always authenticated |
| Art URL | Omitted | ABS cover art requires auth; DE widgets may fail to load it. Revisit later. |
| Next/Previous mapping | Seek ±30s | Headphones have no seek button. Matches Audible-like behavior. **Non-standard MPRIS** — will be commented. |
| Track ID format | `/org/mpris/MediaPlayer2/Track/<itemID>` | ABS item IDs are UUIDs, safe for D-Bus object paths |
| Position source | Cached on model | Updated on existing 500ms tick. Zero extra IPC overhead. |
| Rate mapping | 1:1 with pine's speed | Direct mapping, no conversion needed |
| CanGoPrevious | false → Previous() = seek backward | Non-standard, documented |

## Chosen Approach

Platform-agnostic `MediaController` interface with MPRIS implementation using `go-mpris-server`. Bridge pattern with closures and `ModelAccessor` interface. Events emitted at key playback lifecycle points. Position cached on model. Next/Previous mapped to ±30s seek (non-standard, commented). Art URL omitted.

**Why this stack:** `go-mpris-server` is the only maintained Go MPRIS server library. It handles D-Bus name registration, property introspection, and `PropertiesChanged` signals. The bridge/closure pattern avoids circular imports and keeps MPRIS code isolated in `internal/mpris/`.

## Rejected Alternatives

- **No abstraction layer (MPRIS directly in app code):** Would make macOS support harder later. The interface costs little now and pays off when adding macOS.
- **Start MPRIS after login:** Unnecessary — pine auto-logs in. Starting at launch lets `playerctl` discover pine immediately.
- **Omit position caching:** Would cause IPC to mpv on every MPRIS poll (every 500ms per client). Unnecessary traffic.
- **Standard Next/Previous (chapter/queue skip):** User's headphones have no seek button. Seek ±30s is more useful for audiobooks.
- **Local HTTP proxy for art URL:** Over-engineering for a nice-to-have feature.

## Test Strategy

- **Unit tests** (`internal/mpris/mpris_test.go`): Test adapter implementations — playback status mapping, metadata assembly, volume conversion (0-150 → 0.0-1.0), seek message emission, position caching.
- **Integration with E2E tests** (`internal/app/e2e_test.go`): Verify MPRIS messages flow through the bubbletea Update loop — play/pause/seek via MPRIS commands, state changes emit correct events.
- **Manual testing** with `playerctl` on Linux: `playerctl -p pine status`, `playerctl -p pine play-pause`, `playerctl -p pine position`, etc.

## Open Questions

- None — all resolved.
