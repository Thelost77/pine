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
- **Status:** done
- **Commit:** 415f1b9 feat: advance queue on playback completion
- **Deviations:** none
- **Decisions:** Queue consumption is limited to true playback completion (closed mpv client at end-of-track/end-of-item); manual stop, sleep expiry, and generic playback errors keep the queue intact.

## Post-plan follow-up fixes
- **Home UX polish:** Home was refactored back to a single interactive list model so the Continue Listening header stays visible, Recently Added rows are selectable, and the subsection remains visually secondary.
- **Podcast Recently Added labeling:** Recently Added podcasts are hydrated to show a latest-episode-style row instead of only the podcast title when ABS payloads lack recent episode data.
- **Queue visibility:** App-level footer hints now advertise queue actions and show queue length.
- **Bookmark clarity:** New podcast bookmarks include the active episode title so they are meaningful inside podcast feeds.
- **ABS compatibility:** `mediaMetadata.series` decoding now accepts both object and array shapes from Audiobookshelf responses.
- **Queue entry points:** Queueing is available from both Detail and Home for books and podcast episodes.
- **Final queue semantics:**
  - `a` adds to queue
  - `A` moves the selected item to the front of the queue ("play next")
  - `>` immediately skips to the first queued item during active playback
  - when playback completes naturally, the queue advances automatically

## Metrics
- Tasks completed: 5/5
- Deviations: 3
