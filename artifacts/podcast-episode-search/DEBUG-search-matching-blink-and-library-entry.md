# Debug: search matching gaps, empty-state blink, and missing library search entry

## Phase 1: Root Cause Investigation
- **Errors:**
  1. typing `100` does not match the cached episode `$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]`
  2. while a query is in-flight, Search can briefly render the list state with the `Results` title before switching to `No results found.`
  3. Search is available from Home with `/`, but not from the Library screen
- **Evidence:**
  - `internal/screens/search/cache.go` currently lowercases the query but does not normalize punctuation or tokenize it; podcast matching is `strings.HasPrefix(entry.normalizedEpisodeTitle, normalizedQuery)`
  - that means a title beginning with `$100M...` does **not** match query `100`, because the stored searchable string still begins with `$`
  - `internal/screens/search/view.go` falls through to `m.list.View()` whenever:
    - `m.query != ""`
    - `m.loading == false`
    - `m.err == nil`
    - `m.searched == false`
    - `len(m.items) == 0`
  - that transient state happens after typing but before the debounced search result arrives, so the Bubbles list briefly renders its own title (`Results`)
  - `internal/screens/library/model.go` has no search keybinding, no `/` handling, and no `NavigateSearchMsg`
  - `internal/app/render.go` also omits the search hint for `ScreenLibrary`
- **Data flow trace:**
  1. user types query
  2. `search.Model` updates `m.query` immediately and schedules a debounce tick
  3. before the search result message arrives, view rendering sees a non-empty query with no results and no completed search yet
  4. it renders `m.list.View()` and the list title appears briefly
  5. once the result arrives, `m.searched = true`; if zero matches, the UI flips to `No results found.`

## Phase 2: Pattern Analysis
- **Matching gap:** current book matching uses `strings.Contains` and podcast matching uses `HasPrefix`, so even the two media types already behave differently.
- **Normalization gap:** current normalization is only `strings.ToLower(strings.TrimSpace(query))`. It does not:
  - strip punctuation
  - collapse whitespace
  - expose token boundaries
  - support matching after punctuation or symbols
- **UI consistency gap:** Search has both:
  - a custom screen-level empty/loading/no-results renderer
  - a Bubbles list with its own visible title/state
  - those two rendering layers compete during the debounce gap
- **Navigation gap:** Home already routes to Search with enough context (`libraryID`, `libraryMediaType`), but Library does not expose the same route at all.

## Phase 3: Current-State Conclusions
- **Why `100` fails:** current podcast cache matching is punctuation-sensitive prefix matching against the raw episode title text.
- **Why `Results` blinks:** the search screen temporarily renders the list view before any search result has completed for the current query.
- **Why blank-ish queries can show `No results found.`:** a query containing only whitespace can be non-empty in the input model but normalize to empty in the cache search path, which can still flow through the "searched with zero matches" state.
- **Why Library has no search:** the feature simply is not wired there yet.

## Phase 4: Recommended Solution Direction

### A. Replace prefix/substring matching with normalized fuzzy search over the cached snapshot
- Keep the current **per-library in-memory cache** and local filtering flow.
- Change the cached search snapshot to store precomputed searchable forms per entry:
  - original render fields
  - normalized display text
  - tokenized terms
  - combined searchable text for ranking
- Normalize by:
  - lowercasing
  - replacing punctuation/symbol runs with spaces
  - collapsing repeated whitespace
  - trimming
- Example:
  - raw: `$100M+ Advice That'll Piss Off Every Business Guru (ft. DHH) [9xOaqIkaBZQ]`
  - normalized: `100m advice that ll piss off every business guru ft dhh 9xoaqikabzq`
  - query `100` now matches naturally without any special-case hack

### B. Use ranked multi-stage matching, not a single boolean contains/prefix check
- Recommended ranking order:
  1. exact normalized substring match
  2. token-prefix match
  3. fuzzy token/title match
  4. lower-priority cross-field matches (author, podcast title, identifier suffixes)
- Candidate filtering should require the query terms to be explainable by the candidate, then rank the results rather than just returning insertion order.
- This keeps exact matches feeling strong while still supporting forgiving fuzzy lookup.

### C. Reuse existing fuzzy capability instead of inventing a matcher from scratch
- `github.com/sahilm/fuzzy` is already present in `go.mod` as an indirect dependency.
- It is a good candidate for:
  - fuzzy ranking within already-cached entries
  - matching across normalized title strings or prebuilt token strings
- A good hybrid approach is:
  - normalize first
  - use exact/token substring checks as fast-path boosts
  - use fuzzy ranking for candidates that are not exact matches
- That gives better results than pure fuzzy-only scoring, especially for short numeric queries.

### D. Remove the `Results` title from Search entirely
- The simplest UI fix is to **never show the list title in Search**.
- Search already has enough context from the input itself; the `Results` label adds little value and creates visible blink when the list renderer appears transiently.
- Even with fuzzy search, the screen should explicitly render only:
  - `Type to search…`
  - `Searching…` / `Indexing…`
  - `No results found.`
  - result rows

### E. Add Search entry from Library
- Add the same `/` entrypoint to `internal/screens/library/model.go`.
- Emit a navigation message carrying the currently selected `libraryID` and `libraryMediaType`, matching the Home flow.
- Update `internal/app/render.go` hints so Library documents `/ search`.

## Proposed implementation order
1. finish Search/UI cleanup:
   - remove search list title
   - handle normalized-empty queries consistently
2. add Library → Search navigation
3. upgrade cache entries to include normalized/tokenized searchable text
4. introduce ranked fuzzy matching over cached entries
5. add regression tests for:
   - `$100M...` matched by `100`
   - punctuation-insensitive matching
   - no transient `Results` title
   - Library `/` opens Search

## Verification targets
- `100` should match `$100M...`
- short punctuation-insensitive queries should behave predictably
- Search should never flash `Results`
- Search should be accessible from both Home and Library
- fuzzy ranking should still preserve strong exact matches near the top
