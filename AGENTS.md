# AGENTS.md

## Project Overview

pine is a TUI client for Audiobookshelf written in Go using the bubbletea framework (Elm architecture). It plays audiobooks and podcasts via mpv as a subprocess, controlled over Unix socket IPC.

## Build & Test

```sh
go build -o pine .                   # build
go install github.com/Thelost77/pine@latest
go test ./... -count=1             # run all tests
go test ./internal/app/... -v      # verbose app tests (includes E2E)
golangci-lint run                   # linter
go vet ./...                        # type-check
```

## Architecture Map

### Core (internal/app/)

| File | Responsibility |
|------|---------------|
| `model.go` | Root Model struct, Update dispatcher (400+ line switch), sleep timer, chapter overlay, 401 handling |
| `playback.go` | Playback lifecycle: handlePlayCmd → handlePlaySessionMsg → handlePlayerReady → handlePositionMsg → handleSyncTick → stopPlayback → Cleanup. Track rollover for multi-track books. Session restore on startup. |
| `navigation.go` | Screen routing via back stack, propagateSize (sets all 7 screen models), initScreen, updateScreen dispatch |
| `render.go` | View composition: header + error banner + screen + hints + player footer + chapter overlay modal |
| `bookmarks.go` | Bookmark CRUD, chapter next/prev, seek logic (in-track vs cross-track), fetchBookmarksCmd/fetchEpisodesCmd/fetchBookDetailCmd |
| `messages.go` | Screen enum, PlaySessionMsg, PlaySessionData, QueueEntry, all root-level message types |
| `keymap.go` | Global keybindings (quit, back, help, chapter overlay, queue, chapters, sleep) |
| `queue.go` | Queue entry enqueue/dequeue with dedup, clone helpers |

### Player (internal/player/)

| File | Responsibility |
|------|---------------|
| `mpv.go` | Mpv struct: Launch (spawns subprocess), Connect (IPC socket), GetPosition/GetDuration/GetPaused, Seek, SetSpeed/SetVolume, Quit. Snap socket dir detection. |
| `commands.go` | tea.Cmd factories: TickCmd (500ms poll), LaunchCmd (retry connect for 3s), TogglePauseCmd, SetSpeedCmd, SetVolumeCmd, QuitCmd |
| `model.go` | Player sub-model: Position/Duration/Speed/Volume state, key handling (play/pause, seek, speed, volume) |
| `view.go` | Player footer bar: icon + title + time + speed + optional volume/sleep |

### Screens (internal/screens/)

| Screen | Key files | Notes |
|--------|-----------|-------|
| `login` | model.go, view.go | 3-field form (server, username, password), tab navigation |
| `home` | model.go, view.go | Personalized shelves: continue-listening + recently-added. Library switching with tab. Item/recent caches per library. Skeleton loading with 500ms reveal delay. |
| `library` | model.go, view.go | Paginated item list (50/page), infinite scroll prefetch at 80%, per-library page cache. Library switching with tab. |
| `detail` | model.go, view.go | Viewport-scrollable content: description, series row, episodes list (podcasts), bookmarks list, inline bookmark editing. Tab cycles focus between sections. |
| `search` | model.go, view.go, cache.go | Text input + results list. 50ms debounce. Cache layer builds full library snapshots (all items fetched, normalized, tokenized) with 15min TTL. Fuzzy matching via sahilm/fuzzy. |
| `serieslist` | model.go | Paginated series browser (50/page), infinite scroll. |
| `series` | model.go | Books within a series, sorted by sequence number. Highlights current item. |

### API (internal/abs/)

| File | Responsibility |
|------|---------------|
| `client.go` | HTTP client with 10s timeout, 50MB response limit, auth header, request logging |
| `types.go` | All ABS data types. Custom UnmarshalJSON for MediaMetadata.Series (handles both object and array). |
| `libraries.go` | GetLibraries, GetPersonalized, GetLibraryItems (paginated), GetLibraryItemsBatch (10 concurrent), GetLibrarySeries, GetSeriesContents, FilterAudioLibraries (sample-based audio detection) |
| `playback.go` | StartPlaySession, StartEpisodePlaySession, SyncSession, CloseSession |
| `progress.go` | UpdateProgress, GetMediaProgress (with bookmarks), UpdateEpisodeProgress |
| `bookmarks.go` | CreateBookmark, UpdateBookmark, DeleteBookmark, GetBookmarks |

### Data (internal/db/)

SQLite store (WAL mode) for accounts and listening sessions. Single "last" session row for crash recovery.

### Config (internal/config/)

TOML config with defaults. Theme (Everforest Dark palette), player (speed, seek), keybinds, server address.

### UI (internal/ui/)

Styles from theme config. Format helpers (timestamps, durations). ListItem adapter for bubbles list. Components: HelpOverlay, ErrorBanner (5s auto-dismiss).

## Key Patterns to Follow

- Screen sub-models emit typed command messages (`PlayCmd`, `NavigateDetailMsg`) — root model intercepts in the big switch in `model.go:Update()`
- Playback ticks use generation counters (`playGeneration`) to ignore stale ticks from previous sessions
- All async work in `tea.Cmd` closures captures needed values by copy, never model references
- `tea.Batch` for concurrent async operations
- Skeleton loading with generation-gated reveal delays (home: 500ms, library: 150ms)
- Cache-first data loading: home screen caches per-library items, library screen caches per-library pages
- `q` always quits, `esc`/left goes back (never quits). `h`/`l` seek only, arrows navigate only
- Duration from ABS takes precedence over mpv-reported duration (streaming content reports 0)

## Testing

- Unit tests per package
- E2E tests in `internal/app/e2e_test.go` + `e2e_helpers_test.go` use mock HTTP server + mock player
- E2E tests drive the bubbletea Update loop programmatically (no real TUI)
- `internal/app/model_test.go`, `playback_test.go`, `bookmarks_test.go`, `render_test.go` for app-level tests
- Screen models have individual `*_test.go` files

## Logging

Logs to `~/.config/pine/pine.log` (slog-based, auto-rotated at 5MB). Token/credentials are NOT logged.
