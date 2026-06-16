# pine

A terminal user interface for [Audiobookshelf](https://www.audiobookshelf.org/) — listen to audiobooks and podcasts from your self-hosted server without leaving the terminal.

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [mpv](https://mpv.io/).

<!-- ![Home Screen](assets/home.png) -->

## Features

- Browse your audiobook and podcast libraries
- Play audiobooks with chapter navigation and multi-track support
- Play podcast episodes with progress tracking
- Persistent local API cache for instant startup and near-instant library switching
- Command palette (`Ctrl+P`) for global search and quick actions from anywhere
- Metadata editor for books and episodes
- MPRIS media-key support
- Bookmarks — create, edit, navigate to, and delete
- Sleep timer
- Automatic progress sync every 30 seconds
- Multiple library support with tab switching
- Delete books and podcast episodes from the server
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
pine                # start the TUI
pine --clear-cache  # clear local cache and exit
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
| `space` / `p` | Play / pause |
| `a` / `A` | Add to queue / play next |
| `>` | Play next queued item |
| `n` / `N` | Next / previous chapter |
| `+` / `-` | Speed up / down |
| `]` / `[` | Volume up / down |
| `c` | Open chapter list |
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

Optional config file: `~/.config/pine/config.toml`

See `internal/config/config.go` for the current fields and defaults.

## Releases

Create SemVer tags with a leading `v` so Go can resolve `@latest` and versioned installs correctly. Keep release notes in `docs/releases/` and publish the tag plus the GitHub Release together.

```sh
${EDITOR:-vi} docs/releases/v0.4.0.md
./scripts/release.sh v0.4.0
```

## Known issues

- **Ghostty rendering glitches.** On the Ghostty terminal, the home screen can show stale rows, duplicated item titles, or rows that appear to belong to a different library after pressing `tab`. This is a Ghostty rendering issue, not a pine bug. Pine renders correctly in other terminals such as [Alacritty](https://github.com/alacritty/alacritty), Kitty, WezTerm, or iTerm2. If you hit this, try a different terminal before reporting a bug.

## Architecture

```
main.go         Entry point
internal/
  abs/          Audiobookshelf API client
  app/          Root model, screen routing, playback lifecycle
  cache/        Persistent API cache store and cached client wrapper
  config/       TOML configuration
  db/           SQLite persistence (accounts, last session)
  logger/       File-based structured logging
  mpris/        MPRIS D-Bus adapter for media-key integration
  player/       mpv IPC wrapper
  screens/      Screen models (login, home, library, detail, metadataedit, series, serieslist)
  ui/           Shared styles, formatting, components (confirm, error, help, palette)
```

## Status

Active development. Core functionality is stable — audiobooks, podcasts, bookmarks, chapters, search, persistent cache, metadata editing, server-side deletion, and progress sync are all functional.

## Acknowledgements

Inspired by [Toutui](https://github.com/AlbanDAVID/Toutui).

## License

[MIT](LICENSE)
