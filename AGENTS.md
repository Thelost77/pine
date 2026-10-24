# AGENTS.md

## Project Overview

pine is a TUI client for Audiobookshelf written in Go using the bubbletea framework (Elm architecture). It plays audiobooks and podcasts through libmpv, which it loads into its own process.

This repository also contains spruce, a TUI music client for Jellyfin. One Go module builds the two binaries. `main.go` is pine. `cmd/spruce/main.go` is spruce. See `docs/spruce/README.md` for spruce.

## Build & Test

```sh
go build -o pine .                   # build pine
go build -o spruce ./cmd/spruce      # build spruce
go install github.com/Thelost77/pine@latest
go test ./... -count=1             # run all tests
go test ./internal/app/... -v      # verbose app tests (includes E2E)
gofmt -l .                          # list the files that are not formatted
golangci-lint run                   # linter
go vet ./...                        # type-check
```

## Verify a Change

`gofmt` and `golangci-lint` are part of the development of this repository. Before you finish a change, do these checks. Each check must pass.

1. Use `gofmt -l .`. Make sure that it shows no file. Use `gofmt -w <file>` to format a file.
2. Use `go vet ./...`.
3. Use `golangci-lint run`. Make sure that it reports 0 issues. Use version 2.14.0, which is the version in `.github/workflows/ci.yml`.
4. Use `PINE_REQUIRE_LIBMPV=1 PINE_REQUIRE_DBUS=1 go test ./... -count=1`.

## Repository Layout

| Packages | Owner |
|----------|-------|
| `internal/logger`, `internal/ui`, `internal/ui/components`, `internal/mpris`, `internal/player` | Shared by pine and spruce |
| `internal/abs`, `internal/app`, `internal/cache`, `internal/captions`, `internal/config`, `internal/db`, `internal/screens` | pine |
| `internal/spruce/app`, `internal/spruce/buildinfo`, `internal/spruce/config`, `internal/spruce/jellyfin`, `internal/spruce/screens` | spruce |

Rules:

- A shared package must not import a pine package or a spruce package.
- A pine package must not import a spruce package. A spruce package must not import a pine package.
- A shared package gets the app name as an input: `logger.Init(dir, name)`, `mpris.NewBridge(program, name)`, and `player.SetAppName(name)`.
- A shared package must not contain app content. Each app supplies its help groups and its key names.
- `scripts/release.sh` checks that each binary links only the shared packages and its own packages.
- A change to a shared package must keep the tests of the two apps green: `go test ./... -count=1`.

The Architecture Map below describes pine and the shared packages.

## Architecture Map

### Core (internal/app/)

| File | Responsibility |
|------|---------------|
| `model.go` | Root Model struct, Update dispatcher, sleep timer, chapter overlay, command palette, 401 handling |
| `playback.go` | Playback lifecycle: handlePlayCmd → handlePlaySessionMsg → handlePlayerReady → handlePositionMsg → handleSyncTick → stopPlayback → Cleanup. Track rollover for multi-track books. Session restore on startup. |
| `navigation.go` | Screen routing via back stack, propagateSize (sets all 8 screen models), initScreen, updateScreen dispatch |
| `render.go` | View composition: header + error banner + screen + hints + player footer + chapter overlay modal + confirmation overlay |
| `bookmarks.go` | Bookmark CRUD, chapter next/prev, seek logic (in-track vs cross-track), fetchBookmarksCmd/fetchEpisodesCmd/fetchBookDetailCmd |
| `metadata.go` | Metadata editor save flow: server PATCH, cache eviction, search snapshot prewarm, detail refresh |
| `messages.go` | Screen enum, PlaySessionMsg, PlaySessionData, QueueEntry, all root-level message types |
| `keymap.go` | Global keybindings (quit, back, help, chapter overlay, queue, sleep, command palette). Key names for the player model. |
| `help.go` | Help overlay groups for pine |
| `sessions.go` | The `R` key and the palette entries for listening sessions: the target of the detail screen, the item that plays, or all items. The jump from a session: a seek, or a fetch of the item and `playItemAt`. |
| `queue.go` | Queue entry enqueue/dequeue with dedup, clone helpers |
| `mpris_state.go` | `MprisState`: the playback state that the MPRIS server reads through `mpris.ModelAccessor` |

### Player (internal/player/, shared)

| File | Responsibility |
|------|---------------|
| `mpv.go` | Mpv struct: Launch (creates a libmpv handle and loads the file), GetPosition/GetDuration/GetPaused/GetIdle, Seek, SetSpeed/SetVolume, Quit. `ErrClosed` and `ErrPropertyUnavailable`. Writes the warnings and errors of mpv to the app log, without the query, the user, or the password of a URL. Runtime directory for the media cache. |
| `libmpv.go`, `libmpv_unix.go`, `libmpv_windows.go` | Binding to libmpv through purego, with no cgo. The library loads on the first Launch. Library search order for each system. |
| `commands.go` | tea.Cmd factories: TickCmd (500ms poll), TickAfterCmd (poll after an interval), LaunchCmd, SetPauseCmd, SetSpeedCmd, SetVolumeCmd, QuitCmd. A command for the pause state, the speed, or the volume returns CommandErrMsg if the player does not accept the value. WatchEvents returns PlayerEndMsg at the end of a playback. |
| `model.go` | Player sub-model: Position/Duration/Speed/Volume state, key handling (play/pause, seek, speed, volume) |
| `view.go` | Player footer bar: icon + title + time + speed + optional volume/sleep |
| `cache.go`, `group.go` | Session media cache and player group for simultaneous playback. Only spruce uses them. |

### Screens (internal/screens/)

| Screen | Key files | Notes |
|--------|-----------|-------|
| `login` | model.go, view.go | 3-field form (server, username, password), tab navigation |
| `home` | model.go, view.go | Personalized shelves: continue-listening + recently-added. Library switching with tab. Item/recent caches per library. Skeleton loading with 500ms reveal delay. |
| `library` | model.go, view.go | Paginated item list (50/page), infinite scroll prefetch at 80%, per-library page cache shared with search cache. Library switching with tab. |
| `detail` | model.go, view.go | Viewport-scrollable content: description, series row, episodes list (podcasts), bookmarks list, inline bookmark editing, item deletion. Tab cycles focus between sections. |
| `metadataedit` | model.go, view.go | Inline metadata editor for books and podcast episodes. |
| `serieslist` | model.go | Paginated series browser (50/page), infinite scroll. |
| `series` | model.go | Books within a series, sorted by sequence number. Highlights current item, prefixes finished books with ✓. |
| `sessions` | model.go | Listening sessions of a book, a podcast episode, or all items (50 newest). Each row shows the end position, the device, and the time of the last sync. Enter sends `PlayFromMsg`. |
| `search` | cache.go | No longer a standalone screen. Fuzzy-search cache used by the command palette and shared with the library view. Builds full library snapshots (all items fetched, normalized, tokenized) with background pre-warm. `Search` answers only from a ready snapshot in memory or in SQLite. On a miss, it returns no results and builds the snapshot in the background, so the palette does not wait for the network in `Update`. When the palette opens and the snapshot of its library is not ready, `openGlobalPalette` returns `PrebuildCmd`. On `CacheReadyMsg` for that library, `Palette.Refresh` applies the query again and shows the results. An invalidation removes the builds in flight, so a build that started before a change stores nothing. |

### Cache (internal/cache/)

| File | Responsibility |
|------|---------------|
| `client.go` | Cached wrapper around `abs.Client`: serves cache hits instantly. On a miss, it fetches from the API in the command of the caller and stores the result. Each cached method calls `cached(c, key, ttl, fetch)`, which returns the data of the server. A cache read or write error goes to the log and does not fail the request. A fetch that started before an invalidation does not store its value (`generation`, under `mu`). `invalidate` calls `OnInvalidate` also when the store is nil. |
| `keys.go` | The names of all cache keys. Code outside the package uses `ItemKey`, `PersonalizedKey`, `SeriesContentsKey`, `BookmarksKey`, and `SearchSnapshotKey`, not key strings. A key has no account, so a login (`LoginSuccessMsg`) clears the persistent cache with `ClearAll`. |
| `store.go` | SQLite-backed cache persistence and `ClearAll`. Each key in the database has the prefix `keyVersion` (`v1-`). Raise it when the type of a cached value changes. |

### MPRIS (internal/mpris/, shared)

| File | Responsibility |
|------|---------------|
| `adapter.go` | `RootAdapter` (`CanQuit` and `CanRaise` are false), the playback status and the track metadata from the app state, `trackPath`, and `PlayerActions`. |
| `bridge.go` | Bubbletea message bridge between the MPRIS server and root model. A rate of 0 pauses. A rate is limited to `MinimumRate`..`MaximumRate`, and a volume to 0..1.5. `SetPosition` ignores a position outside the track. With skip seek, `CanGoNext` and `CanGoPrevious` are `HasActiveItem` and `CanSeek`. |
| `accessor.go` | Read-only MPRIS property accessor. |
| `server.go` | MPRIS server on `godbus/dbus`: the root and player interfaces, the D-Bus properties interface, introspection, and the change signals. `NewServer(name, root, accessor, actions)` reads the player properties from `ModelAccessor` and calls `PlayerActions`. It rejects a Rate or a Volume value that is not finite. `CanPlay` and `CanPause` are `HasActiveItem`, and `StatusChanged` and `PlaybackChanged` send them. `StatusChanged` and `PlaybackChanged` also send `CanGoNext` and `CanGoPrevious`. |

The server is written from the MPRIS 2.2 specification. The Go MPRIS server library that exists is GPL-3.0, and a GPL module makes the release binaries GPL. Do not add a module with a copyleft license.

### API (internal/abs/)

| File | Responsibility |
|------|---------------|
| `client.go` | HTTP client with 10s timeout, 50MB response limit, auth header, request logging. `NewClient` removes the spaces and the trailing slashes of the server address. `HTTPStatusError` keeps at most 500 bytes of the response body. A log line or a request error shows `SessionTag` (a short hash) in place of a play session ID. |
| `types.go` | All ABS data types. Custom UnmarshalJSON for MediaMetadata.Series (handles both object and array). |
| `libraries.go` | GetLibraries, GetPersonalized, GetLibraryItems (paginated), GetLibraryItemsBatch (10 concurrent), GetLibrarySeries, GetSeriesContents, FilterAudioLibraries (sample-based audio detection) |
| `playback.go` | StartPlaySession, StartEpisodePlaySession, SyncSession, CloseSession, CloseSessionNoSync (sends no position) |
| `progress.go`, `me.go` | UpdateProgress, UpdatePosition (no `isFinished` key), GetMediaProgress (ABS gives no bookmarks with the progress), GetMe (user + all media progress). UpdateProgress and UpdatePosition take an episode ID for a podcast episode. |
| `sessions.go` | GetListeningSessions (`GET /api/me/listening-sessions`), GetItemListeningSessions (`GET /api/me/item/listening-sessions/{id}[/{episodeId}]`) |
| `bookmarks.go` | CreateBookmark, UpdateBookmark, DeleteBookmark, GetBookmarks (`GET /api/me/bookmarks/{id}`, or `GET /api/me` for ABS before 2.36.0) |

### Data (internal/db/)

SQLite store (WAL mode) for accounts and listening sessions. `busy_timeout` is 5000 ms, so a write waits while a different connection writes. Single "last" session row for crash recovery. It keeps only the item ID and the episode ID. The restore gets the position from the server. The accounts table keeps only the account of the last login, because `SaveAccount` deletes the other accounts.

### Config (internal/config/)

TOML config with defaults. Theme (Everforest Dark palette), player (speed, seek), keybinds, server address. The theme type and its default colors are in `internal/ui`.

### UI (internal/ui/, shared)

Styles from theme config. Format helpers (timestamps, durations). `RunProgram` runs a Bubble Tea program and quits it at SIGHUP. Components: HelpOverlay (the app supplies the groups), ErrorBanner (5s auto-dismiss), ConfirmOverlay, CommandPalette, and the filter, flash, and skeleton helpers.

## Key Patterns to Follow

- Screen sub-models emit typed command messages (`PlayCmd`, `NavigateDetailMsg`) — root model intercepts in the big switch in `model.go:Update()`
- Playback ticks use generation counters (`playGeneration`) to ignore stale ticks from previous sessions
- All async work in `tea.Cmd` closures captures needed values by copy, never model references
- `tea.Batch` for concurrent async operations
- Skeleton loading with generation-gated reveal delays (home: 500ms, library: 150ms)
- Cache-first data loading through `internal/cache.Client`: home and library views hit the persistent cache first. A miss fetches from the server in the command of the caller and stores the result.
- `q` quits, and `esc`/left goes back (never quits). `h`/`l` seek only, arrows navigate only. These keys have no global function while a text input or a dialog gets the keys (`capturesKeys`).
- A quit (`q` or ctrl+c) returns only `tea.Quit` and keeps the session in the model. `Cleanup` sends the last position, and `main` waits for it. Bubble Tea does not wait for a command at a quit, so a quit must not send the position in a command.
- The `main` of pine and the `main` of spruce run the program with `ui.RunProgram`, which quits the program at SIGHUP, the close of the terminal, so that `Cleanup` runs. Bubble Tea quits the program at SIGTERM.
- `ctrl+p` opens the global command palette from each screen, but not while a text input or a dialog gets the keys
- A text input or a confirmation dialog of a screen gets each key, except ctrl+c. `capturesKeys` in `playback.go` is true for the login screen, the metadata editor, a list filter, and the bookmark title editor. It is also true for the confirmation dialog of the detail screen. `Update` in `model.go` then gives the key to the screen, before the global keys, the chapter overlay, the error banner, and the playback keys. The help overlay and the command palette get the keys first while they are open. If you add a text input or a dialog to a screen, add it to `capturesKeys`.
- Duration from ABS takes precedence over mpv-reported duration (streaming content reports 0)
- Each launch makes a new mpv handle, which plays at the speed 1.0. `handlePlayerReady` sets the speed of the model in the new player.
- The listening time (`timeListened`) is the time of the clock between two position ticks of a player that plays (`trackListeningTime`). It is not the change of the position: at the speed 2.0, the position moves two seconds in each second. A player that is paused or idle does not play. mpv reports no pause while it waits for data, so a position tick also reads the mpv property `core-idle` (`GetIdle`, `PositionMsg.Idle`). A test moves the clock with `Model.now`.
- pine finds the end of a track through `errors.Is(err, player.ErrClosed)` on a position tick. spruce uses `PlayerEndMsg`, and a tick with `ErrClosed` can arrive before it. Do not match the text of a player error.
- Enter on a bookmark (`handleSeekToBookmark`) seeks the player only if the book or the episode of the bookmark plays. If not, it stops the playback and starts that book or that episode at the time of the bookmark.
- A bookmark of ABS has no episode ID. `handleAddBookmark` puts the episode title and `episodeTitleSeparator` before the title of a podcast bookmark, and `bookmarkEpisode` finds the episode from that title. The bookmarks on a server have that format, so do not change it.
- The copies of the root model share the MPRIS bridge through `mprisLink`, because `main` calls `SetProgram` after `tea.NewProgram` copies the model. Reach the bridge through `mprisBridge()`.
- pine writes the MPRIS position to `MprisState` on each position tick. It sends the MPRIS signal Seeked only for a seek (`seekToBookGlobalPosition`), for a playback that starts at a position after 0, and for a restart of the same item (`restartPlaybackAt`) at each position, because the track ID does not change. It does not send Seeked as a periodic position report.
- `isUnauthorized` finds a 401 with `abs.IsHTTPStatus(err, http.StatusUnauthorized)`. A test that needs a 401 uses `&abs.HTTPStatusError{StatusCode: http.StatusUnauthorized}`, not an error text.

## Testing

- Unit tests per package
- E2E tests in `internal/app/e2e_test.go` + `e2e_helpers_test.go` use mock HTTP server + mock player
- E2E tests drive the bubbletea Update loop programmatically (no real TUI)
- `feedCmd` in `e2e_helpers_test.go` skips a command that needs more than 200 ms, so that a test does not wait for a `tea.Tick`. A request to an address with no server, or a load that writes to a database file, can need more on Windows in CI. For such a command, use `feedCmdWithin` with `slowCmdWait`.
- `internal/app/model_test.go`, `playback_test.go`, `bookmarks_test.go`, `render_test.go` for app-level tests
- `internal/ui/program_unix_test.go` starts the test program again as a child process, sends SIGHUP to it, and checks that it gets to the place where a `main` calls `Cleanup`. The child runs its program with `ui.RunProgram`, as each `main` does. It does not run on Windows.
- `internal/cache/*_test.go` for cache store and helper behavior
- `internal/mpris/mpris_test.go` for `RootAdapter` and the server properties, and `bridge_test.go` for the bridge
- `internal/mpris/server_linux_test.go` starts the MPRIS server on a private session bus and checks it as an MPRIS client. It skips if `dbus-daemon` is absent. With `PINE_REQUIRE_DBUS=1` it fails instead. CI and `scripts/release.sh` set that variable.
- `internal/app/mpris_linux_test.go` starts the MPRIS server of the pine model on a private session bus and checks the Seeked signals. It skips if `dbus-daemon` is absent. With `PINE_REQUIRE_DBUS=1` it fails instead.
- `internal/screens/metadataedit/model_test.go` for the metadata editor
- Screen models have individual `*_test.go` files
- `internal/player/libmpv_test.go`, `mpv_test.go`, and `group_test.go` play audio through the real libmpv with `ao=null`. They skip if libmpv is absent. With `PINE_REQUIRE_LIBMPV=1` they fail instead. `scripts/release.sh` sets that variable.
- `.github/workflows/ci.yml` runs `go vet` and the tests on Ubuntu 22.04, 24.04, and 26.04, on macOS (Apple Silicon and Intel), and on Windows. It runs `golangci-lint` v2.14.0 on Ubuntu 24.04, and the job fails on each issue. A test that asserts a Unix file mode must not make that assertion on Windows.

## Logging

Logs to `~/.config/pine/pine.log` (slog-based). At start, `Init` renames `<name>.log` to `<name>.log.old` when the file is over 5 MB, and only while it holds the lock of the file. An instance that cannot lock the log writes to `<name>-<pid>.log`. Token/credentials are NOT logged. A play session ID gives the audio of the session with no login, so pine logs only `abs.SessionTag(id)` in the field `session`, never the full ID. spruce logs to `spruce.log` in its config directory.
