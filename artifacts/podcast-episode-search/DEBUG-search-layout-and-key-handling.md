# Debug: search layout jump and search key interception

## Phase 1: Root Cause Investigation
- **Error:** Podcast episode search had two follow-up regressions:
  1. the empty/loading/no-results search states rendered as a tiny 3-line view, so the shared app layout vertically centered the whole screen until list results appeared
  2. typing `c` in search during active playback opened the global chapter overlay instead of entering text
- **Reproduction:**
  1. open Search
  2. observe the initial hint rendered in the vertical middle of the screen
  3. type a query that returns results and the content jumps to the top because `list.View()` fills the available height
  4. clear the query or search for something with no hits and the short view is centered again
  5. while playback is active and chapters exist, type `c` in Search and observe that the chapter overlay opens instead of the query changing
- **Recent changes:** The podcast episode search work added search-specific follow-up fixes in `internal/screens/search/` and `internal/app/`, but the empty-state rendering still returned short strings and the root app still handled chapter/playback keys before forwarding input to Search.
- **Evidence:** `internal/screens/search/view.go` returned only `input + blank + message` for empty/loading/error/no-results states, while result states returned `m.list.View()`, which occupies the configured height. `internal/app/model.go` matched `m.keys.ChapterOverlay` and other playback shortcuts before calling `updateScreen(msg)`, so Search never saw those keys.
- **Data flow trace:**
  1. `app.Model.Update(tea.KeyMsg)` received the key
  2. for `c`, the root app handled `m.keys.ChapterOverlay` first and returned early
  3. `search.Model.Update()` never received the rune, so `textinput` could not append `c`
  4. for layout, `search.Model.View()` returned a short message-only string
  5. `app.Model.View()` wrapped the whole app content in `lipgloss.Place(..., Center, Center, ...)`, so short search states were centered while full-height result states stayed anchored

## Phase 2: Pattern Analysis
- **Working example:** Search results were stable because `list.View()` renders within a fixed-height area. Other text-input screens such as Login also manage their own placement explicitly instead of relying on the app shell to guess.
- **Differences:** Empty search states did not reserve the same body height as result states. Search input also differed from other screens because it needs focused text entry, but the root app treated it like any other screen and let global playback bindings win first.
- **Dependencies:** A correct fix needs:
  1. a stable search body height for every search state
  2. the Search screen to receive typed keys before global playback/chapter shortcuts can intercept them

## Phase 3: Hypothesis
- **Hypothesis:** The vertical jump happens because short search states do not fill the search pane, and `c` is blocked because root-level playback bindings run before Search input handling.
- **Test:** Add regressions that assert:
  1. empty and no-results search views fill the configured search height
  2. typing `c` on `ScreenSearch` updates the query instead of opening the chapter overlay
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** Search used inconsistent body heights across states, and the root app intercepted playback/chapter keys before forwarding key events to Search.
- **Fix:**
  1. `internal/screens/search/view.go` now always renders the search body inside a fixed-height, top-aligned area
  2. `internal/screens/search/model.go` reserves the correct body height for the input + body layout
  3. `internal/app/model.go` now lets `ScreenSearch` handle typed keys before global playback/chapter shortcuts run, while still preserving global quit/help behavior
- **Test:** Added regressions in `internal/screens/search/model_test.go` and `internal/app/model_test.go`
- **Verification:** `go test ./... -count=1` and `go build -o /tmp/pine ./cmd/`

## Attempts
- Attempt 1: Hypothesis confirmed immediately by adding fixed-height search view tests and an app-level `c` typing test; the focused fix passed without requiring further redesign.
