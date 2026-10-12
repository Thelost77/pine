# spruce

A terminal music client for [Jellyfin](https://jellyfin.org/). Browse albums and
playlists, build a queue, and control playback without leaving the terminal.

Spruce uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the
interface and [mpv](https://mpv.io/) for audio playback.

## Features

- Browse albums and tracks across Jellyfin music libraries
- Browse Jellyfin playlists
- Play individual tracks or queue whole albums and playlists
- Shuffle, repeat, filter, remove, and jump within the playback queue
- Cache repeated media locally for the current Spruce session
- Play up to eight queued tracks together in subliminal mode
- Fuzzy command-palette search across loaded albums, playlists, and tracks
- Edit album and track metadata
- Report playback state and progress to Jellyfin
- MPRIS media controls through the D-Bus session bus
- Configurable playback speed, seek interval, keybindings, and colors
- Sleep timer

## Requirements

- Go 1.26.4 or newer for source installation
- libmpv, the library of mpv 0.34 or later. The `mpv` program is not necessary.
  - Debian and Ubuntu: `sudo apt install libmpv2`. On Ubuntu 22.04 the package
    is `libmpv1`.
  - Arch: `sudo pacman -S mpv`
  - macOS: `brew install mpv`
- A Jellyfin account with at least one music library
- A Unix-like operating system

spruce does not load the mpv config files. The options in `mpv.conf` do not
apply.

Linux is the primary platform. macOS cross-compiles, but MPRIS requires a D-Bus
session and is primarily useful on Linux. Windows is not currently supported.

## Installation

```sh
go install github.com/Thelost77/pine/cmd/spruce@latest
```

Ensure the Go binary directory, normally `~/go/bin`, is in `PATH`.

To build the current checkout:

```sh
go build -o spruce ./cmd/spruce
```

## Usage

```sh
spruce
spruce --version
```

The first run opens the login screen. Enter the Jellyfin server address,
username, and password. If the address has no scheme, Spruce uses `http://`.

After a successful login, Spruce stores the server address, username, user ID,
access token, and stable device identity in the operating system's Spruce config
directory. On Linux this is normally `~/.config/spruce/config.toml`.

The access token is stored as plaintext. The config directory uses mode `0700`
and the file uses mode `0600`; local file permissions are the security boundary.
See [Auth token storage](../design/spruce/auth-token-storage.md) for the decision and
operational details.

Logs are written to the same config directory as `spruce.log` and rotated after
5 MB.

## Common keybindings

Keys can be context-specific. Press `?` in Spruce for the complete active help.

| Key | Action |
|-----|--------|
| `q` | Quit |
| `esc` / `←` | Go back or clear an active filter |
| `enter` / `→` | Open or select |
| `j` / `k` | Move down or up |
| `/` | Filter the current list |
| `space` | Play or pause |
| `h` / `l` | Seek backward or forward |
| `n` / `>` | Next track |
| `N` / `p` / `<` | Previous track |
| `a` / `A` | Add selected track or collection to the queue |
| `S` | Shuffle selected collection or cycle the sleep timer |
| `s` | Shuffle the current queue |
| `r` / `R` | Repeat current track or queue |
| `b` | Toggle subliminal mode from the queue |
| `+` / `-` | Increase or decrease playback speed |
| `]` / `[` | Increase or decrease volume |
| `m` | Edit selected album or track metadata |
| `t` | Toggle album-order/title track sorting |
| `o` | Open playlists |
| `tab` | Switch between content and queue |
| `ctrl+p` | Open the command palette |
| `?` | Toggle help |

## Repeat cache and subliminal mode

When track or queue repeat is active, Spruce downloads repeated media into a
private session cache. Later repeats play the local files instead of transferring
the media again. If repeat is enabled after playback starts, the current item
may transfer once more while the cache is prepared. The cache is limited to 2
GiB and removed when Spruce exits. Jellyfin playback progress reports still use
a small amount of network traffic.

Subliminal mode is available from the queue with `b` or the command palette. It
first caches every queued track, then plays each queue entry at the same time in
an independent loop. Each track starts at 20% volume. The normal volume controls
change every track to the same value. Subliminal mode supports up to eight queue
entries and locks queue changes until the mode stops. Jellyfin does not receive
playback progress for a simultaneous subliminal mix because its session model
represents only one current item.

## Configuration

Spruce writes all defaults to `config.toml` after the first successful login.
Common editable settings include:

```toml
[player]
speed = 1.0
seek_seconds = 10

[keybinds]
quit = "q"
play_pause = " "
seek_forward = "l"
seek_backward = "h"
next_track = "n"
prev_track = "N"
speed_up = "+"
speed_down = "-"
volume_up = "]"
volume_down = "["
sleep_timer = "S"
back = "esc"
```

The generated `[theme]` section contains the editable Everforest color values.
See [Theme configuration](../design/spruce/theme-configuration.md) for the supported
scope.

Jellyfin identifies each Spruce installation through `server.device_id`. Keep
that value stable on one installation. Remove it when copying the config to a
different machine so Spruce generates a separate identity.

## Development

```sh
go mod verify
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
```

Main packages:

```text
cmd/spruce/main.go         process startup and shutdown
internal/spruce/app/       root model, navigation, queue, playback lifecycle
internal/spruce/jellyfin/  Jellyfin API client and data types
internal/spruce/screens/   login, library, playlist, queue, metadata editor
internal/mpris/            shared D-Bus MPRIS adapter and bridge
internal/player/           shared libmpv player
internal/ui/               shared styles and components
```

## Releases

Spruce and pine are in one Go module. One SemVer tag with a leading `v`
releases the two programs. Release notes are in `docs/releases/`. The notes for
spruce v0.1.0 and v0.2.0 are in `docs/releases/spruce/`.

Commit the notes and all release changes. Push them to `main`. Then use the
release script:

```sh
./scripts/release.sh v0.7.0
```

The script verifies the repository, creates and pushes the exact annotated tag,
and publishes a GitHub Release. It can safely retry release creation after a
partial failure.

## Related project

[pine](../../README.md) is a terminal client for Audiobookshelf. It is in the
same repository.

## License

[MIT](../../LICENSE)
