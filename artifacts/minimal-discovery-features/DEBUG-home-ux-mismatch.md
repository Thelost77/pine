# Debug: home UX mismatch

## Phase 1: Root Cause Investigation
- **Error:** Manual UAT found three mismatches on Home:
  1. Continue Listening header not visible
  2. Recently Added rows are not interactive
  3. Recently Added podcast rows show podcast/item titles rather than episode-style labels
- **Reproduction:** Open Home with empty or short Continue Listening plus Recently Added content. Observe that `internal/screens/home/view.go` replaces the main list with plain text when `len(m.items) == 0`, then appends a static Recently Added block rendered from raw strings.
- **Recent changes:** Commit `5f71a79 feat: add home recently added section` introduced the subsection via bespoke rendering in `internal/screens/home/view.go`.
- **Evidence:** Data flow splits at the view boundary:
  - Continue Listening goes through `setListItems()` → `listItem.Title()/Description()` → interactive `list.Model`
  - Recently Added bypasses that path and is rendered directly in `internal/screens/home/view.go:24`
  - `internal/abs/testdata/personalized.json` shows `recently-added` entities without `recentEpisode`, so episode-specific labels are not currently available from the fetched payload alone
- **Data flow trace:** `fetchPersonalizedCmd()` collects `continue-listening` and `recently-added` entities in `internal/screens/home/model.go`. The model stores both correctly, but the view renders `recentlyAdded` with `item.Media.Metadata.Title` directly instead of reusing the list item presenter. When Continue Listening is empty, the view drops the list entirely and replaces it with plain muted text, which also removes the header.

## Phase 2: Pattern Analysis
- **Working example:** Continue Listening rows in `internal/screens/home/model.go` use `listItem.Title()` and `listItem.Description()` and stay interactive through `list.Model`.
- **Differences:**
  - Recently Added is rendered as plain text, not `list.Item`
  - Empty-state handling replaces the whole main list/header with a string
  - Recently Added data currently lacks `recentEpisode`, unlike the continue-listening path
- **Dependencies:** To show episode-specific podcast labels in Recently Added, the UI needs either richer ABS payload data or an additional fetch/derivation step for podcast items.

## Phase 3: Hypothesis
- **Hypothesis:** The missing header and non-interactive subsection are caused by the custom string-based rendering in `internal/screens/home/view.go`, and the podcast-label mismatch is caused by relying on `recently-added` item payloads that do not include `recentEpisode`.
- **Test:** Read the rendering path and ABS fixture data, compare it with the working Continue Listening path, and confirm the missing fields / alternate rendering path.
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** Home has two rendering paths with different semantics; the new Recently Added path bypasses the existing interactive item presenter, and the current ABS data shape does not provide episode-level labels for Recently Added podcasts.
- **Fix:** Pending implementation. Clear fixes:
  - keep the Continue Listening header visible even in the empty state
  - move Recently Added onto an intentional interaction path instead of static strings
  - decide whether podcast rows should stay item-level or trigger extra data fetching for episode-level labels
- **Test:** Pending
- **Verification:** Pending

## Attempts
- Attempt 1: Investigated manual UAT result by tracing model → view → ABS payload flow — root cause confirmed, no code change yet
