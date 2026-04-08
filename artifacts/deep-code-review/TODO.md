# TODO

## Finding 4: Async responses lack request correlation — HIGH

Stale responses from previous screens can corrupt current UI state after quick navigation. Search already does this right (`SearchResultsMsg` carries `Query`, checked on receipt). Four other message types lack this pattern.

- `EpisodesLoadedMsg` (messages.go:93) — no `ItemID`; blindly applied via `m.detail.SetEpisodes()` (model.go:275-284)
- `BookDetailLoadedMsg` (messages.go:99) — no `ItemID`; blindly applied via `m.detail.SetItem()` (model.go:286-294)
- `PersonalizedMsg` (home/model.go:18) — no `LibraryID`; response cached under whatever library is currently selected (home/model.go:183)
- `LibraryItemsMsg` (library/model.go:26) — no `LibraryID`; response applied without checking which library it belongs to (library/model.go:173-188)

**Fix:** Add identity field (`ItemID` or `LibraryID`) to each message struct. Check it on receipt against current screen state. Discard if stale, like search does.

---

## Finding 3: Auth-expiry redirect is dead code in production — HIGH

No production code emits `components.ErrMsg`. All screen error paths store errors locally (`m.err = msg.Err`). The root model's 401 handler (`model.go:142-159`) is unreachable outside tests. `PlaybackErrorMsg` and `BookDetailLoadedMsg.Err` also don't check for 401.

- `components.ErrMsg` only created in tests (e2e_test.go, model_test.go)
- All screens: `home`, `library`, `search`, `serieslist`, `series`, `login` store errors locally, never emit `ErrMsg`
- `components.IsUnauthorized` uses string matching (`status 401`, `unauthorized`) on `err.Error()`

**Fix:** Add a `checkUnauthorized` helper in root model. In each screen's error handling, check if the error is a 401 and if so, emit `components.ErrMsg` instead of storing locally. Alternatively, add 401 checks to `PlaybackErrorMsg`, `BookDetailLoadedMsg.Err`, and `EpisodesLoadedMsg.Err` handlers in root model.

---

## Finding 10: Detail help text says `q/h back` — MEDIUM

All 13 help strings in `detail/view.go:170-213` say `q/h back`, but `q` quits the app and `h` seeks backward. The actual back keys are `esc` and `←`.

**Fix:** Replace `q/h back` with `esc/← back` throughout `helpText()` in `detail/view.go`.

---

## Finding 5: Podcast mark-finished uses item-level API — MEDIUM

`MarkFinishedCmd` (detail/model.go:73-76) carries only `Item` with no episode context. `handleMarkFinished` (playback.go:469-489) unconditionally calls `client.UpdateProgress` (item-level `PATCH /api/me/progress/{itemID}`) regardless of media type. Should call `client.UpdateEpisodeProgress` for podcasts with the selected episode.

The detail model already has the needed context — `m.selectedEpisode`, `m.episodes`, `m.focusEpisodes` (detail/model.go:180-182) and the `queueTarget()` helper (detail/model.go:568-577) shows the right pattern.

**Fix:** Add `Episode` field to `MarkFinishedCmd`. Emit it with the selected episode for podcasts. Branch in `handleMarkFinished` to call `UpdateEpisodeProgress` for podcasts.

---

## Finding 6: Bookmark reads hit full `/api/me` — MEDIUM

`GetBookmarks` (abs/bookmarks.go:22-41) calls `GET /api/me` and filters client-side by `libraryItemId`. This happens on every bookmark view, add, delete, rename, and detail load. The ABS API offers `GET /api/me/progress/{id}` which returns `MediaProgressWithBookmarks` (type already exists in abs/types.go:256-264) — an item-scoped response with just that item's bookmarks. No client method reads from this endpoint.

**Fix:** Add `GetMediaProgress(ctx, itemID)` method to abs client that calls `GET /api/me/progress/{id}` and returns `MediaProgressWithBookmarks`. Replace `GetBookmarks` calls in `bookmarks.go` with this new method, extracting just the bookmarks field.

---

## Finding 9: Keybind config has dead fields — MEDIUM

`config.KeybindsConfig` defines `Select`, `Back`, `Up`, `Down`, `Search` fields but no screen reads them. All 6 screen keymaps are hardcoded:
- home/model.go:76-111
- library/model.go:64-92
- detail/model.go:108-160
- search/model.go:47-59
- series/model.go:35-46
- serieslist/model.go:43-55

Root keymap also hardcodes `Quit` ("q"), `Back` ("esc","left"), `Help` ("?"), `ChapterOverlay` ("c") despite config having `Quit` and `Back` fields.

**Fix:** Either wire all config fields into screen and root keymaps, or remove the dead config fields and update the README to narrow the "fully configurable" claim.

---

## Finding 7: Podcast expansion is N+1 — MEDIUM

`hydrateRecentlyAddedPodcasts` (home/model.go:586-607) calls `GetLibraryItem` sequentially per podcast item in the recently-added shelf. `buildPodcastSnapshot` (search/cache.go:208-257) does the same per podcast per page. Both block the UI thread inside a `tea.Cmd`.

**Fix:** Add concurrency (goroutine pool with limit, e.g. 4 concurrent). Or defer podcast expansion — store minimal data for search, enrich only when the user opens the selected result.

---

## Finding 11: README Go version mismatch — LOW

README.md:26 says "Go 1.21+" but go.mod:3 requires `go 1.25.6`. Also CLAUDE.md:10 says `go build ./cmd/` which works but is unconventional — README already has the correct `go build -o pine ./cmd/`.

**Fix:** Update README Go version to `1.25+`. Optionally update CLAUDE.md build command to `go build -o pine ./cmd/`.

---

## Finding 8: Search cache prewarm and cancellation — LOW

- `Prepare` method exists (cache.go:78-84) but is never called — entering search doesn't prewarm
- `searchCmd` (search/model.go:228) uses `context.Background()` — no cancellation on screen exit
- `ensureSnapshot` (cache.go:122-126) blocks on `<-build.done` with no `ctx.Done()` select — hangs forever if build stalls

**Fix:** Call `cache.Prepare()` when entering search screen. Thread a cancelable context through snapshot building. Add `ctx.Done()` select in `ensureSnapshot`.

---

## Finding 12: Missing test coverage for risky cases — MEDIUM

No tests for:
1. Stale `EpisodesLoadedMsg` / `BookDetailLoadedMsg` after navigating to a different detail item
2. Stale `PersonalizedMsg` after rapid library switching
3. Stale `LibraryItemsMsg` after switching library tabs
4. 401 handling through real screen/API paths (only tested via injected `components.ErrMsg`)
5. Detail help text correctness (key descriptions vs actual bindings)