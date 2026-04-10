# pine

A terminal user interface for [Audiobookshelf](https://www.audiobookshelf.org/) — listen to audiobooks and podcasts from your self-hosted server without leaving the terminal.

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [mpv](https://mpv.io/).

<!-- ![Home Screen](assets/home.png) -->

## Features

- Browse your audiobook and podcast libraries
- Play audiobooks with chapter navigation and multi-track support
- Play podcast episodes with progress tracking
- Bookmarks — create, edit, navigate to, and delete
- Sleep timer
- Automatic progress sync every 30 seconds
- Library search
- Multiple library support with tab switching
- Keyboard-driven navigation
- Everforest dark theme
- Session restore — pick up where you left off
- Graceful cleanup — no orphaned mpv processes

## Requirements

- Go 1.25+
- [mpv](https://mpv.io/) installed and available in PATH
- An [Audiobookshelf](https://www.audiobookshelf.org/) server

## Installation

```sh
go install github.com/Thelost77/pine@latest
```

## Usage

```sh
pine
```

On first run, you'll be prompted to log in to your Audiobookshelf server. pine stores the server URL, username, and auth token locally in `~/.config/pine/pine.db` for future sessions.

## Common keybindings

Some keys are context-specific.

| Key | Action |
|-----|--------|
| `q` | Quit |
| `esc` / `←` | Go back |
| `enter` / `→` | Open / select |
| `j` / `k` | Navigate down / up |
| `h` / `l` | Seek backward / forward |
| `p` / `space` | Play / pause |
| `a` / `A` | Add to queue / play next |
| `n` / `N` | Next / previous chapter |
| `+` / `-` | Speed up / down |
| `]` / `[` | Volume up / down |
| `c` | Open chapter list |
| `s` / `S` | Browse series in library / cycle sleep timer during playback |
| `b` | Add bookmark |
| `d` / `e` | Delete / edit bookmark |
| `f` | Mark finished |
| `tab` | Switch focus / library |
| `/` | Search |
| `o` | Open library |
| `?` | Help overlay |

## Configuration

Optional config file: `~/.config/pine/config.toml`

See `internal/config/config.go` for the current fields and defaults.

## Releases

Create SemVer tags with a leading `v` so Go can resolve `@latest` and versioned installs correctly. Keep release notes in `docs/releases/` and publish the tag plus the GitHub Release together.

```sh
${EDITOR:-vi} docs/releases/v0.1.0.md
./scripts/release.sh v0.1.0
```

## Architecture

```
main.go         Entry point
internal/
  abs/          Audiobookshelf API client
  app/          Root model, screen routing, playback lifecycle
  config/       TOML configuration
  db/           SQLite persistence (accounts, last session)
  logger/       File-based structured logging
  player/       mpv IPC wrapper
  screens/      Screen models (login, home, library, detail, search, series, serieslist)
  ui/           Shared styles, formatting, components
```

## Status

Active development. Core functionality works — audiobooks, podcasts, bookmarks, chapters, search, and progress sync are all functional.

## Acknowledgements

Inspired by [Toutui](https://github.com/AlbanDAVID/Toutui).

## License

[MIT](LICENSE)
