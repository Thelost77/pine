# pine

A terminal user interface for [Audiobookshelf](https://www.audiobookshelf.org/) — listen to audiobooks and podcasts from your self-hosted server without leaving the terminal.

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [mpv](https://mpv.io/).

<!-- ![Home Screen](assets/home.png) -->

## Features

- Browse your audiobook and podcast libraries
- Play audiobooks with chapter navigation and multi-track support
- Play podcast episodes with progress tracking
- Bookmarks — create, navigate to, and delete
- Sleep timer with configurable durations
- Automatic progress sync every 30 seconds
- Server-side search
- Multiple library support with tab switching
- Vim-style keybindings (fully configurable)
- Everforest dark theme (customizable via TOML)
- Persistent sessions — pick up where you left off
- Graceful cleanup — no orphaned mpv processes

## Requirements

- Go 1.21+
- [mpv](https://mpv.io/) installed and available in PATH
- An [Audiobookshelf](https://www.audiobookshelf.org/) server

## Installation

```sh
go install github.com/Thelost77/pine/cmd@latest
```

Or build from source:

```sh
git clone https://github.com/Thelost77/pine.git
cd pine
go build -o pine ./cmd/
```

## Usage

```sh
pine
```

On first run, you'll be prompted to log in to your Audiobookshelf server. Credentials are saved locally for future sessions.

## Keybindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `esc` / `←` | Go back |
| `enter` / `→` | Open / select |
| `j` / `k` | Navigate down / up |
| `h` / `l` | Seek backward / forward |
| `p` / `space` | Play / pause |
| `n` / `N` | Next / previous chapter |
| `+` / `-` | Speed up / down |
| `]` / `[` | Volume up / down |
| `s` | Cycle sleep timer |
| `b` | Add bookmark |
| `d` | Delete bookmark |
| `tab` | Switch focus / library |
| `/` | Search |
| `o` | Open library browser |
| `?` | Help overlay |

## Configuration

Config file: `~/.config/pine/config.toml`

```toml
[player]
speed = 1.0
seek_seconds = 10

[theme]
background = "#2b3339"
foreground = "#d3c6aa"
accent = "#a7c080"

[keybinds]
play_pause = " "
seek_forward = "l"
seek_backward = "h"
# See internal/config/config.go for all options
```

## Architecture

```
cmd/            Entry point
internal/
  abs/          Audiobookshelf API client
  app/          Root model, screen routing, playback lifecycle
  config/       TOML configuration
  db/           SQLite persistence (accounts, sessions)
  logger/       File-based structured logging
  player/       mpv IPC wrapper
  screens/      Screen models (login, home, library, detail, search)
  ui/           Shared styles, formatting, components
```

## Status

Active development. Core functionality works — audiobooks, podcasts, bookmarks, chapters, search, and progress sync are all functional.

## Acknowledgements

Inspired by [Toutui](https://github.com/AlbanDAVID/Toutui).

## License

[MIT](LICENSE)
