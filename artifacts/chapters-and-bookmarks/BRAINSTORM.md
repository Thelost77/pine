# Brainstorm: chapters-and-bookmarks

## Problem Statement
The current experience mixes two different concepts:

- **Chapters** are a playback navigation tool. They matter when a book is actively playing and the user wants to jump around the current listening session.
- **Bookmarks** are persistent saved positions. They belong to the item more generally and remain useful even outside active playback.

The product goal is therefore not "show all structural metadata on the detail screen." The goal is to make playback navigation feel intentional and easy to access without conflating it with persistent item data.

For this task, the desired user experience is:

- Book detail can show bookmarks before playback starts.
- Chapters should not appear before playback starts.
- Chapters should be treated as playback-scoped controls, not passive metadata.
- Chapters should not live as a permanent inline section by default.
- Users should explicitly open chapter navigation through a dedicated interaction.

## Architecture
- **Frontend:** Go terminal UI using Bubble Tea, with playback-aware state and a user-invoked chapter navigation surface
- **Backend:** Not needed; existing Audiobookshelf APIs already provide playback and chapter data
- **Database:** Not needed for this change; no new persistence beyond current app behavior
- **External APIs:** Audiobookshelf playback/session APIs and bookmark/progress APIs
- **Deployment:** Existing local terminal app distribution; no deployment model changes
- **Validation Strategy:** Validate availability and lifecycle at the UI/state boundary so playback-only navigation is only exposed when active playback context exists, while bookmark actions remain item-scoped

## Chosen Approach
Use a **temporary chapter overlay/modal** that the user opens explicitly while listening.

Why this approach:

- It matches the product framing of chapters as an active playback tool rather than always-visible metadata.
- It keeps bookmarks conceptually separate instead of merging two different navigation concepts into one area.
- It avoids cluttering the detail screen with playback-specific UI when playback is not active.
- It creates a clear mental model: bookmarks live on the detail screen; chapters appear when the user asks for playback navigation.

Technology stack:

- **Frontend:** Go + Bubble Tea terminal UI
- **Backend:** none beyond existing Audiobookshelf API usage
- **Data source:** Audiobookshelf play-session chapter data plus existing bookmark/progress data

## Rejected Alternatives
- **Permanent dedicated playback panel on the detail screen:** Rejected because it makes the screen structurally about playback all the time, even though chapters are only meaningful during active listening. Stack: Go + Bubble Tea + existing Audiobookshelf APIs.
- **Single scrolling detail view with inline chapters:** Rejected because it continues to blur structural metadata with active playback controls and has weaker discoverability/control boundaries. Stack: Go + Bubble Tea + existing Audiobookshelf APIs.
- **Always-visible disabled chapter section before playback:** Rejected because it advertises unavailable functionality and adds interface weight without helping the primary task. Stack: Go + Bubble Tea + existing Audiobookshelf APIs.

## Test Strategy
Use a **hybrid strategy**:

- **Automated behavior tests:** Verify playback-scoped availability, opening/closing chapter navigation, selection behavior, and separation from bookmarks.
- **Automated integration-style TUI tests:** Verify the user flow around entering detail, starting playback, and interacting with chapter navigation in realistic state transitions.
- **Manual verification:** Quick terminal check for usability, visibility, and interaction feel, especially because modal/overlay behavior in a TUI can be correct logically but awkward ergonomically.

## Open Questions
- None at this stage.
