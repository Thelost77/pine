# Context: minimal-discovery-features

## Implementation Decisions
### Recently Added scope
- **Decision:** Keep **Continue Listening** as the primary home content and show a small **Recently Added** subsection beneath it for the **currently selected home library/context**.
- **Rationale:** This keeps home focused on the strongest next action while avoiding a separate Recently Added mode with ambiguous library context.
- **Details:** show **3 items**, and exclude titles already present in Continue Listening.
- **Options considered:** current selected library only; all libraries merged as a separate source; separate Recently Added per library in the normal tab cycle.

### Queue entry points
- **Decision:** Expose queue actions only from the **Detail** screen for books and podcast episodes.
- **Rationale:** This preserves Pine’s minimalism by limiting queue actions to intentional playback contexts rather than sprinkling enqueue affordances across every list.
- **Options considered:** detail only; detail plus home/library/search lists; everywhere playable items appear.

### Series presentation
- **Decision:** The compact series row in book detail opens a dedicated **Series** screen with its own list model.
- **Rationale:** Pine already uses screen-level routing at the root model. A dedicated screen keeps series navigation simple and avoids overloading Detail’s bookmark/episode focus logic.
- **Options considered:** dedicated screen; overlay/modal on top of detail; inline expansion inside detail.

### Series data loading
- **Decision:** When entering **book** detail, fetch enriched item data in the background so the series row appears automatically if series data exists.
- **Rationale:** This matches the current pattern where detail navigation triggers background fetches (bookmarks, podcast episodes). It lets the detail view stay clean while still surfacing series context without placeholder UI.
- **Options considered:** fetch on entering detail; fetch only after activating a placeholder row; fetch only when opening the series screen from a separate action.

### Queue continuity on manual play
- **Decision:** If the queue has items and the user manually starts something else, keep the remaining queue intact and continue from the newly chosen item.
- **Rationale:** This treats the queue as a passive “up next” list rather than a strict session script, which is more forgiving and lower-friction for a minimal client.
- **Options considered:** keep queue intact; clear queue on every manual start; replace only the head of the queue.

## Existing Code Insights
- This is a **brownfield** change in a Go + Bubble Tea app with strong existing patterns for screen routing, background API fetches, and root-level playback orchestration.
- `internal/screens/home/model.go` currently owns a single list model, cycles libraries with `tab`, caches items per library, and fetches `/personalized` but only surfaces the `continue-listening` section.
- `internal/screens/detail/model.go` and `view.go` already support focused subsections (episodes, bookmarks) and background data hydration, but only within the existing detail screen state machine.
- `internal/app/model.go` is the central integration point for navigation, follow-up fetches on entering detail, and dispatching playback/bookmark actions from screen models.
- `internal/app/playback.go` owns the playback lifecycle centrally; queue advancement will need to hook into playback stop/end handling here rather than in screen models.
- `internal/abs/libraries.go` already has `GetLibraryItem`, and Pine already uses it via `fetchEpisodesCmd` to hydrate detail with richer podcast data. That pattern can be reused for book-series enrichment.
- Audiobookshelf exposes library-specific series endpoints (`/api/libraries/:id/series` and `/api/libraries/:id/series/:seriesId`) and library personalized shelves (`/api/libraries/:id/personalized`), so Pine’s ABS client will need new types/endpoints for series and likely an explicit strategy for Recently Added.

## Integration Points
- **Home / Recently Added subsection:** `internal/screens/home/model.go`, `internal/screens/home/view.go`, and `internal/abs/libraries.go`
- **Detail / series row / queue actions:** `internal/screens/detail/model.go` and `internal/screens/detail/view.go`
- **New Series screen:** new screen model wired through `internal/app/messages.go`, `internal/app/navigation.go`, `internal/app/model.go`, and `internal/app/render.go`
- **Queue state and progression:** root app model plus playback lifecycle in `internal/app/model.go` and `internal/app/playback.go`
- **ABS client/types:** `internal/abs/types.go` and `internal/abs/libraries.go` for enriched item data, series data, and Recently Added retrieval

## Deferred Ideas
- Queue actions from home/library/search lists
- Persistent queue state across restarts
- Auto-enqueue podcast behavior
- Inline or overlay-based series browsing inside Detail
- General metadata-browsing surfaces beyond the compact series navigation flow
