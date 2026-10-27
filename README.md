# pine

A terminal user interface for [Audiobookshelf](https://www.audiobookshelf.org/) — listen to audiobooks and podcasts from your self-hosted server without leaving the terminal.

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [mpv](https://mpv.io/).

## Two programs

This repository contains two programs. One Go module builds them, and one release tag covers them. They share the player, MPRIS, logger, and UI packages.

| Program | Server | Install |
|---------|--------|---------|
| pine | [Audiobookshelf](https://www.audiobookshelf.org/): audiobooks and podcasts | `go install github.com/Thelost77/pine@latest` |
| [spruce](docs/spruce/README.md) | [Jellyfin](https://jellyfin.org/): music | `go install github.com/Thelost77/pine/cmd/spruce@latest` |

The remaining sections describe pine. For spruce, see [docs/spruce/README.md](docs/spruce/README.md).

## Features

- Browse your audiobook and podcast libraries
- Play audiobooks with chapter navigation and multi-track support
- Play podcast episodes with progress tracking
- Captions from VTT sidecar files
- Playback queue
- Series browsing, with marks for finished books
- Persistent local API cache for instant startup and near-instant library switching
- Fuzzy command-palette search (`Ctrl+P`) across books, podcasts, episodes, and series
- In-place filtering (`/`) on home, library, and series lists
- Metadata editor for books and episodes
- MPRIS media-key support on Linux
- Bookmarks — create, edit, navigate to, and delete
- Listening sessions (`R`) — see where each device stopped, and play from that position
- Sleep timer
- Automatic progress sync every 30 seconds
- Multiple library support with tab switching
- Delete books and podcast episodes from the server
- Keyboard-driven navigation
- Configurable colors and keys, with the Everforest dark theme as the default
- Session restore — pick up where you left off
- In-process playback through libmpv — no mpv subprocess

## Requirements

- Go 1.26.4+ for an installation from source
- libmpv, the library of [mpv](https://mpv.io/) 0.34 or later. The `mpv` program is not necessary.
  - Debian and Ubuntu: `sudo apt install libmpv2`. On Ubuntu 22.04 the package is `libmpv1`.
  - Arch: `sudo pacman -S mpv`
  - macOS: `brew install mpv`
- An [Audiobookshelf](https://www.audiobookshelf.org/) server

pine does not load the mpv config files. The options in `mpv.conf` do not apply.

Linux is the primary platform. macOS works with mpv from Homebrew. MPRIS needs a D-Bus session, so the media keys work on Linux only. On Windows, the tests pass with `libmpv-2.dll` in the directory of the program, but pine has not run on a Windows machine.

## Installation

```sh
go install github.com/Thelost77/pine@latest
```

Each release from v0.8.0 also has archives with a binary for Linux, macOS, and Windows. Download an archive from the [release page](https://github.com/Thelost77/pine/releases). The binary needs libmpv. See "Requirements".

## Usage

```sh
pine                # start the TUI
pine --clear-cache  # clear local cache and exit
```

On first run, you'll be prompted to log in to your Audiobookshelf server.

pine keeps its files in the user config directory. On Linux this is `~/.config/pine`.

| File | Content |
|------|---------|
| `pine.db` | The server URL, the username, the auth token, the last session, and the API cache |
| `config.toml` | The optional configuration |
| `pine.log` | The log. It shows why a stream did not load. |

## Common keybindings

Some keys are context-specific.

| Key | Action |
|-----|--------|
| `q` | Quit |
| `esc` / `←` | Go back |
| `enter` / `→` | Open / select |
| `j` / `k` | Navigate down / up |
| `/` | Filter current list |
| `h` / `l` | Seek backward / forward |
| `space` / `p` | Play / pause |
| `a` / `A` | Add to queue / play next |
| `>` | Play next queued item |
| `+` / `-` | Speed up / down |
| `]` / `[` | Volume up / down |
| `c` | Toggle captions during playback |
| `C` | Open chapter list |
| `R` | Listening sessions (detail: the book or episode; else: the item that plays, or all items) |
| `s` | Browse series (library) |
| `S` | Cycle sleep timer during playback |
| `b` | Add bookmark |
| `d` / `e` | Delete / edit bookmark |
| `f` | Mark finished |
| `m` | Edit metadata (detail view) |
| `tab` | Switch focus / library |
| `ctrl+p` | Command palette |
| `o` | Open library |
| `?` | Help overlay |

## Configuration

Optional config file: `config.toml` in the config directory. Each key is optional. This example shows the defaults:

```toml
[player]
speed = 1.0
seek_seconds = 10

[keybinds]
quit = "q"
back = "esc"
play_pause = " "
seek_forward = "l"
seek_backward = "h"
next_in_queue = ">"
speed_up = "+"
speed_down = "-"
volume_up = "]"
volume_down = "["
sleep_timer = "S"

[theme]
background = "#2b3339"
foreground = "#d3c6aa"
accent = "#a7c080"
error = "#e67e80"
muted = "#859289"
selected = "#475258"
border = "#4f585e"
info = "#7fbbb3"
```

`player.speed` must be from 0.1 to 100, and `player.seek_seconds` must be more than 0. For a different value, pine stops at startup with an error that names the key.

Pine creates a stable playback identity on first run:

```toml
device_name = "my-host (Pine)"
device_id = "pine-my-host-a41c29ef"
```

Audiobookshelf uses `device_id` to distinguish playback devices. Keep it stable
for one Pine installation. If you copy `config.toml` to another machine, delete
or replace its `device_id` there so Pine generates a new one. `device_name` is
display-only and can be changed without changing `device_id`.
Audiobookshelf displays this as `Pine <device_name>` (without Pine's default
`(Pine)` suffix), for example `Pine my-host`.

Pine writes a generated `device_name` and `device_id` to `config.toml`. A
read-only `config.toml`, for example a home-manager symlink, must set the two
keys, or Pine cannot start.

Pine sends its Go module build version as `clientVersion` to Audiobookshelf.
Local development builds report `dev`.

## Releases

Create SemVer tags with a leading `v` so Go can resolve `@latest` and versioned installs correctly. Keep release notes in `docs/releases/` and publish the tag plus the GitHub Release together.

One tag releases pine and spruce together. Write the release notes for the two programs in one file.
Commit the notes and push them to `main`. Then use the release script.

```sh
${EDITOR:-vi} docs/releases/v0.9.2.md
./scripts/release.sh v0.9.2
```

When the script publishes the GitHub Release, the `Release` workflow builds the two programs for Linux, macOS, and Windows. It attaches one archive for each program and each system to the release.

## Architecture

```
main.go         pine entry point
cmd/spruce/     spruce entry point
internal/
  abs/          Audiobookshelf API client
  app/          Root model, screen routing, playback lifecycle
  cache/        Persistent API cache store and cached client wrapper
  captions/     VTT caption parser
  config/       TOML configuration
  db/           SQLite persistence (accounts, last session)
  screens/      Screen models (login, home, library, detail, metadataedit, series, serieslist)
  spruce/       spruce packages (app, buildinfo, config, jellyfin, screens)
  logger/       Shared: file-based structured logging
  mpris/        Shared: MPRIS D-Bus adapter for media-key integration
  player/       Shared: libmpv player
  ui/           Shared: styles, formatting, components
```

## Development

```sh
go build -o pine .
go build -o spruce ./cmd/spruce
gofmt -l .
go vet ./...
golangci-lint run
go test ./... -count=1
```

A GitHub Actions workflow runs `go vet`, `golangci-lint`, and the tests on Linux, macOS, and Windows for each pull request. The player tests need libmpv. See [AGENTS.md](AGENTS.md) for the package map and the rules.

## Status

Active development. Core functionality is stable — audiobooks, podcasts, bookmarks, chapters, search, persistent cache, metadata editing, server-side deletion, and progress sync are all functional.

## Acknowledgements

Inspired by [Toutui](https://github.com/AlbanDAVID/Toutui).

## License

[MIT](LICENSE)
