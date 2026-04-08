# Pine deep code review

## Scope and approach

Reviewed the source and tests under:

- `cmd/`
- `internal/app/`
- `internal/abs/`
- `internal/db/`
- `internal/config/`
- `internal/player/`
- `internal/screens/`
- `internal/ui/`
- top-level docs (`README.md`, `CLAUDE.md`, `go.mod`)

I also ran the existing test suite and a build-equivalent command. Findings below are **verified against the codebase**, not copied blindly from agent output.

## Overall assessment

The repo is in good shape structurally:

- the Bubble Tea root model is clean and understandable
- playback lifecycle uses a solid generation-counter pattern
- tests are much stronger than average for a TUI app, especially around playback and E2E flows
- package boundaries are mostly sensible

The biggest issues are not style problems. They are:

1. **advertised features that are only partially implemented**
2. **async responses that are not correlated to the request that produced them**
3. **performance/scaling costs around podcast expansion and bookmarks**

## Strong parts worth preserving

1. **Playback generation counters** are a good design choice. `playGeneration` and `sleepGeneration` correctly guard against stale ticks and stale timer expirations (`internal/app/model.go:50,58`, `internal/app/playback.go:179-184`, `internal/app/model.go:377-386`).
2. **Message-driven screen architecture** is clean. Screen submodels emit typed messages and the root model owns cross-cutting concerns like navigation and playback (`internal/app/model.go`).
3. **Search stale-result rejection** is correctly implemented with query matching, which is exactly the kind of guard missing elsewhere (`internal/screens/search/model.go:133-150`).
4. **Test breadth** is strong: playback lifecycle, bookmarks, search cache, mpv wrapper, and many E2E flows are all covered.

## Findings

### High

#### 1. “Persistent sessions” are saved but not restored

**Why it matters:** the README promises “Persistent sessions — pick up where you left off”, but the app only writes session state; it never reads it back on startup.

**Evidence**

- README claim: `README.md:21`
- session saving exists: `internal/app/playback.go:447-452`, `internal/app/playback.go:538-543`, `internal/app/playback.go:612-617`
- restore path is missing: `GetLastSession` is only referenced in tests and the DB package, not in runtime code (`internal/db/sessions.go:41`, repo-wide search)
- the “session persistence” E2E test only checks that login saves an account, not that listening state is restored (`internal/app/e2e_test.go:1012-1038`)

**Impact**

- user-facing feature mismatch
- crash recovery / resume behavior is incomplete
- tests currently give false confidence because the named persistence E2E test does not exercise playback persistence

**Recommendation**

- either implement restore-on-startup from `GetLastSession`
- or downgrade/remove the README claim until it exists

#### 2. Default-account handling can create multiple defaults, and startup selection becomes nondeterministic

**Why it matters:** login always saves the logged-in account as default, but `SaveAccount` does not clear the flag from existing default accounts.

**Evidence**

- login writes `IsDefault: true` on every success: `internal/app/model.go:167-175`
- `SaveAccount` only upserts one row and never clears other defaults: `internal/db/accounts.go:21-48`
- startup reads `SELECT ... WHERE is_default = 1 LIMIT 1` with no ordering: `internal/db/accounts.go:53-58`

**Impact**

- logging into multiple servers/users can leave several rows marked default
- next startup may pick an arbitrary default account

**Recommendation**

- make “set this account default” transactional and clear other defaults in the same write path
- add a test that logs in twice through app/root behavior, not just isolated DB helpers

#### 3. Auth-expiry redirect logic is effectively not wired into most real API paths

**Why it matters:** the root model has special 401 handling, but production code rarely emits `components.ErrMsg`, so most unauthorized responses will just stay local to a screen instead of redirecting to login.

**Evidence**

- redirect logic exists only in root handling of `components.ErrMsg`: `internal/app/model.go:134-148`
- repo-wide usage shows `components.ErrMsg` is basically only referenced in tests and the root model
- screen/API error paths usually store the error locally instead:
  - home: `internal/screens/home/model.go:174-179`
  - library: `internal/screens/library/model.go:161-166`
  - search: `internal/screens/search/model.go:133-141`
  - detail bookmarks: `internal/screens/detail/model.go:356-373`
  - detail-load errors are logged and dropped by root: `internal/app/model.go:262-281`

**Impact**

- expired tokens likely surface as local errors instead of a coherent re-auth flow
- the 401 redirect behavior tested in app tests is not representative of normal runtime traffic

**Recommendation**

- standardize API error propagation so unauthorized errors reach one shared handler
- prefer structured checks using `abs.IsHTTPStatus(err, 401)` over string matching in `components.IsUnauthorized`

#### 4. Several async responses are not tied to the request/library/item that produced them

**Why it matters:** stale responses can overwrite current UI state after quick navigation or library switching.

**Evidence**

- detail async loads carry no item identity:
  - emitted from `detailLoadCmds`: `internal/app/bookmarks.go:153-161`
  - consumed without correlation:
    - `EpisodesLoadedMsg`: `internal/app/model.go:262-271`
    - `BookDetailLoadedMsg`: `internal/app/model.go:273-281`
- home personalized fetch carries no library ID:
  - message shape: `internal/screens/home/model.go:18-24`
  - fetch captures selected index but response is applied to whichever library is currently selected when it arrives: `internal/screens/home/model.go:274-327`, `internal/screens/home/model.go:174-191`
- library paging fetch carries page but no library ID:
  - message shape: `internal/screens/library/model.go:26-31`
  - tab switching resets library and immediately refetches: `internal/screens/library/model.go:202-210`
  - page responses are applied without checking the active library: `internal/screens/library/model.go:161-180`

**Concrete bad cases**

- open detail A, quickly open detail B, then A’s delayed episode/bookmark/detail payload mutates B’s screen
- switch libraries quickly and get old library results rendered under the new library title

**Recommendation**

- include `ItemID`, `LibraryID`, or request generation in async messages
- ignore messages that do not match current screen state, like search already does

### Medium

#### 5. Podcast “mark finished” uses the item-level path, not the episode-level path

**Why it matters:** on podcasts, `f` appears to act on the current detail view, but the command only knows about the item and always calls `UpdateProgress`, never `UpdateEpisodeProgress`.

**Evidence**

- detail emits only `MarkFinishedCmd{Item: item}`: `internal/screens/detail/model.go:418-422`
- app handles it with item-level progress only: `internal/app/playback.go:465-484`
- episode-specific progress API exists but is not used here: `internal/abs/progress.go:31-44`

**Impact**

- podcast “mark finished” behavior is likely wrong or at least ambiguous
- can mark the whole podcast item instead of the selected episode

**Recommendation**

- either disable/hide mark-finished for podcasts until semantics are clear
- or pass selected episode context and use `UpdateEpisodeProgress`

#### 6. Bookmark reads are implemented as full `/api/me` fetches plus client-side filtering

**Why it matters:** every bookmark view, add, delete, and rename pulls the whole bookmark collection and filters locally.

**Evidence**

- `GetBookmarks` calls `/api/me` and filters in memory: `internal/abs/bookmarks.go:21-40`
- add/delete/update all re-fetch through that same path immediately after mutation: `internal/app/bookmarks.go:36-47`, `internal/app/bookmarks.go:70-81`, `internal/app/bookmarks.go:94-105`

**Impact**

- poor scaling if a user has many bookmarks across many items
- unnecessary network and JSON cost on a common workflow

**Recommendation**

- prefer an item-scoped bookmark endpoint if ABS has one
- otherwise cache the `/api/me` bookmark set for the current item or batch refresh less aggressively

#### 7. Podcast expansion is N+1 and mostly sequential in both Home and Search

**Why it matters:** podcast-heavy libraries will feel slower than they need to.

**Evidence**

- home “recently added” hydration loops and calls `GetLibraryItem` per podcast item: `internal/screens/home/model.go:580-600`
- search podcast snapshot loops library pages and then expands every podcast item individually: `internal/screens/search/cache.go:208-257`
- both use synchronous sequential expansion inside one command path

**Impact**

- slower home load for podcast shelves
- first search on a large podcast library may be very expensive

**Recommendation**

- add concurrency limits for item expansion or defer expansion until needed
- consider storing only enough data for search first, then enriching the selected result later

#### 8. Search cache has good matching logic but poor cancellation/prewarm behavior

**Why it matters:** first-search latency can be high, and work continues even if the user leaves the screen.

**Evidence**

- `Prepare` exists but is unused: `internal/screens/search/cache.go:78-84`
- searches use `context.Background()` from `searchCmd`, so snapshot builds are not cancelable from screen lifecycle: `internal/screens/search/model.go:227-233`
- `ensureSnapshot` waits on in-progress builds without a cancellable select on context: `internal/screens/search/cache.go:122-126`

**Impact**

- expensive snapshot work can continue after the user backs out
- first keystroke may pay the full snapshot-build cost

**Recommendation**

- prewarm via `Prepare` when entering search
- thread cancelable contexts through snapshot building

#### 9. Keybinding configurability is only partial, but the product/docs present it as comprehensive

**Why it matters:** the repo claims keybindings are “fully configurable”, but most screen keymaps and help text are hardcoded.

**Evidence**

- README claim: `README.md:19`
- config exposes many keybind options: `internal/config/config.go:32-57`
- root/player use some config keybinds, but many bindings remain hardcoded:
  - app keymap hardcodes quit/back/help/chapter overlay: `internal/app/keymap.go:21-55`
  - home keymap hardcoded: `internal/screens/home/model.go:75-111`
  - library keymap hardcoded: `internal/screens/library/model.go:56-80`
  - detail keymap hardcoded: `internal/screens/detail/model.go:108-160`
  - search keymap hardcoded: `internal/screens/search/model.go:47-59`
  - login behavior hardcoded: `internal/screens/login/model.go:86-103`
- view hints and help overlay are also hardcoded:
  - status hints: `internal/app/render.go:84-127`
  - help overlay groups: `internal/ui/components/help.go:39-101`

**Impact**

- customized bindings and visible help can disagree
- “fully configurable” is currently overstated

**Recommendation**

- either narrow the docs claim
- or drive all visible key help and screen bindings from config

### Low

#### 10. Detail-screen help text is factually wrong in several states

**Why it matters:** the help text repeatedly says `q/h back`, but `q` quits and `h` is a seek key during playback, not “back”.

**Evidence**

- repeated incorrect strings in `internal/screens/detail/view.go:180-213`
- actual global back behavior is `esc`/left: `internal/app/keymap.go:27-30`, `README.md:57-60`

**Impact**

- misleading UX during the most interaction-heavy screen

#### 11. README / code / local guidance are out of sync in a few places

**Evidence**

- README says `Go 1.21+`: `README.md:26`
- `go.mod` currently declares `go 1.25.6`: `go.mod:3`
- `CLAUDE.md` suggests `go build ./cmd/`, but the command collides with the existing `cmd` directory in this repo; a working variant is `go build -o <binary> ./cmd/`

**Impact**

- onboarding friction
- confusing build expectations for contributors and agents

#### 12. Test suite is strong overall, but it misses the riskiest stale-state cases

**Missing coverage that would pay off most**

1. stale `EpisodesLoadedMsg` / `BookDetailLoadedMsg` after navigating to a different detail item
2. stale `PersonalizedMsg` after rapid library switching
3. stale `LibraryItemsMsg` arriving after switching library tabs
4. multiple-default-account behavior through actual login flow
5. real playback-session restore, if the feature is meant to exist
6. unauthorized API handling through real screen/API code paths, not only injected `components.ErrMsg`

## Suggested priority order

1. Fix **session restore mismatch** or remove the claim.
2. Fix **default-account exclusivity**.
3. Add **request correlation/generation guards** to home, library, and detail async loads.
4. Decide the intended semantics for **podcast mark finished**.
5. Tackle **bookmark and podcast expansion performance**.
6. Reconcile **keybinding configurability** with docs/help/hints.

## Short conclusion

The repo is already well-structured and unusually well-tested for a TUI app, but it has a few high-value correctness gaps hiding behind otherwise clean code. The most important theme is consistency: several features are implemented halfway (session persistence, configurable keybinds, auth-expiry handling), and several async flows need the same stale-message defense that search already has.
