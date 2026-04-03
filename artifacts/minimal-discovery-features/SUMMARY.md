# Summary: minimal-discovery-features

## Progress

### Task 1: Extend ABS client contracts for Recently Added and series
- **Status:** done
- **Commit:** c9af4bc feat: add ABS discovery contracts
- **Deviations:** Initial assumption of Recently Added as a standalone merged source changed later during Task 2 design.
- **Decisions:** Reused personalized shelves for Recently Added and extended library item decoding to include `libraryId`, `addedAt`, and compact series metadata.

### Task 2: Refactor home to support a Recently Added subsection
- **Status:** done
- **Commit:** 5f71a79 feat: add home recently added section
- **Deviations:** Replaced the earlier separate-source/tab plan with a smaller subsection design during implementation.
- **Decisions:** Home now keeps per-library Continue Listening as the main list capped at 5 items, plus a deduplicated Recently Added subsection capped at 3 items.

### Task 3: Add root queue state and detail-level queue actions
- **Status:** done
- **Commit:** c7365a5 feat: add detail queue actions
- **Deviations:** none
- **Decisions:** Queue actions are detail-only via `a`/`A`, queue is root-owned and in-memory, duplicates are normalized by item/episode identity, and manual play does not clear the queue.

### Task 4: Add enriched book detail and dedicated Series screen
- **Status:** done
- **Commit:** pending
- **Deviations:** none
- **Decisions:** Book detail now hydrates full ABS item data in the background for books, exposes a compact focusable series row when available, and opens a dedicated Series list that preselects the current book.

### Task 5: Hook queue advancement into playback completion and run integration coverage
- **Status:** pending
- **Commit:** —
- **Deviations:** —
- **Decisions:** —

## Metrics
- Tasks completed: 4/5
- Deviations: 2
