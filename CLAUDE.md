# CLAUDE.md

## Project

pine is a TUI client for Audiobookshelf written in Go with bubbletea.

## Build & Test

```sh
go build ./cmd/                    # build
go test ./... -count=1             # run all tests
go test ./internal/app/... -v      # verbose app tests (includes E2E)
```

Do not run linter or type-check — the maintainer verifies manually.

## Architecture

Bubbletea Elm-architecture: Model -> Update -> View. Root model (`internal/app/`) dispatches messages to screen sub-models.

**Key files:**
- `internal/app/model.go` — Root model, Update dispatcher, struct definition
- `internal/app/playback.go` — Playback lifecycle (launch -> connect -> tick -> sync -> stop -> cleanup)
- `internal/app/navigation.go` — Screen routing, back stack
- `internal/app/render.go` — View composition (header, screen, hints, footer)
- `internal/app/bookmarks.go` — Bookmark CRUD, chapter navigation, episode fetching
- `internal/player/mpv.go` — mpv IPC wrapper with snap detection

**Screen models** (`internal/screens/`): login, home, library, detail, search. Each owns its own state, Update, and View.

**Player**: mpv launched as subprocess, controlled via Unix socket IPC (JSON protocol). Socket path auto-detected for snap vs native mpv installations.

## Patterns

- Screen sub-models emit command messages (e.g., `PlayCmd`, `NavigateDetailMsg`) — root model intercepts and handles them
- Playback ticks use generation counters to ignore stale ticks from previous sessions
- `tea.Batch` for concurrent async operations (API calls, mpv commands)
- All API calls in tea.Cmd closures capture needed values (not model references)

## Key decisions

- `q` always quits, `esc`/left arrow goes back (never quits)
- `h`/`l` seek only (never navigate), arrows navigate only (never seek)
- Sleep timer uses generation counter to handle cycling (15m->30m doesn't fire stale 15m tick)
- Snap mpv detection: resolves symlink, checks if binary is `/usr/bin/snap`, uses `~/snap/mpv/common/` for socket
- Duration from ABS takes precedence over mpv-reported duration (streaming content reports 0)

## Testing

- Unit tests per package
- E2E tests in `internal/app/e2e_test.go` use mock HTTP server + mock player
- E2E tests drive the bubbletea Update loop programmatically (no real TUI)
- Tests don't catch environment-specific issues (e.g., snap mpv socket isolation)

## Logging

Logs to `~/.config/pine/pine.log`. Uses `internal/logger` package (slog-based).
Token/credentials are NOT logged.
