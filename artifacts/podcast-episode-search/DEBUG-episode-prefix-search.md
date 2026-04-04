# Debug: episode prefix search misses longer prefixes

## Phase 1: Root Cause Investigation
- **Error:** In podcast-library search, a short prefix like `J` could find an episode, while a longer prefix like `Jas` could miss the same episode.
- **Reproduction:** Search inside a podcast library where the podcast title does not match the episode title prefix. Example: podcast `Joe Rogan`, episode `Jason...`; query `Jas` returned no result.
- **Recent changes:** Podcast-library search was implemented by expanding ABS podcast-show search results into episode rows.
- **Evidence:** `internal/abs/libraries.go` called `SearchLibrary(query)` first, then only expanded the podcast items returned by that show-level search. If ABS returned no podcast rows for `Jas`, no episode titles were ever checked.
- **Data flow trace:** search query -> `search.Model.searchCmd()` -> `abs.Client.SearchPodcastEpisodes()` -> `SearchLibrary(query)` -> returned podcast-show candidates -> `GetLibraryItem()` -> episode prefix filter. The candidate set was already wrong before episode filtering ran.

## Phase 2: Pattern Analysis
- **Working example:** Book-library search works because it searches the actual items being rendered instead of depending on a different higher-level entity first.
- **Differences:** Podcast search was effectively doing "search shows, then filter episodes", while the desired behavior is "search episodes in this library".
- **Dependencies:** Correct episode-prefix matching needs access to podcast library items and each podcast's episode list, not just ABS show-search output.

## Phase 3: Hypothesis
- **Hypothesis:** The bug happens because podcast episode search depends on show-level ABS search results, so episode-only prefixes can eliminate the correct show before episode filtering starts.
- **Test:** Replace show-search expansion with a library-wide podcast item scan, expand each podcast, and apply the prefix filter directly to episode titles. Add regression tests where the show title does not match the episode prefix.
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** `SearchPodcastEpisodes()` searched podcasts by show title first, so longer episode-only prefixes could remove the show from the candidate set before checking episodes.
- **Fix:** Reworked `SearchPodcastEpisodes()` to page through podcast library items, expand each podcast with `GetLibraryItem()`, and filter episodes directly by prefix.
- **Test:** Added regression coverage in `internal/abs/libraries_test.go` and `internal/screens/search/model_test.go` for `Joe Rogan` / `Jason...` style mismatches.
- **Verification:** `go test ./... -count=1 && go build -o /tmp/pine ./cmd/`

## Attempts
- Attempt 1: Hypothesis confirmed by replacing show-search candidate expansion with library-item scanning; the regression tests passed immediately with the new data source.
