# Brainstorm: command-palette

## Problem Statement
The current application relies heavily on users remembering context-specific keybindings to navigate, perform actions, and control playback, which increases cognitive load. The goal is to provide a "Search Everywhere" unified command palette (triggered primarily by `Ctrl+P`) that serves as a single, intuitive interface for all possible actions (global navigation, playback controls, context-specific actions for the current screen/item) and content searches (audiobooks, podcasts, series, and bookmarks for the playing item). This removes the need for memorization and creates a highly responsive, JetBrains-IDE-like experience.

## Architecture
- **Frontend:** Go Bubble Tea framework with Lipgloss for rendering. The palette will be a centered modal overlay utilizing `normalizeOverlayCanvas`.
- **Backend:** Audiobookshelf Server (no new backend required).
- **Database:** Local SQLite database and in-memory caches (leveraging the active library cache).
- **External APIs:** Existing ABS API.
- **Deployment:** Local Go binary (TUI client).
- **Validation Strategy:** Validate cache readiness and ensure input sanitization during fuzzy search filtering.

## Chosen Approach
**Unified Palette with Cache-First Local Search**
A single unified palette (`Ctrl+P`) lists both actions and content items, organized with group headers (e.g., Actions, Books, Series, Bookmarks). Content is sourced from the active library's in-memory cache to ensure sub-millisecond, instant filtering as the user types using `sahilm/fuzzy`. This prioritizes the core requirement of extreme responsiveness. Selecting content navigates directly to its Detail page, and selecting a bookmark immediately jumps playback to that position and closes the palette.

## Rejected Alternatives
- **Separated Global vs Context Palettes**: Rejected because having two different triggers (`:` vs `;`) creates mode-confusion and adds unnecessary complexity for the user. A single unified palette is much simpler.
- **On-Demand Async DB/API Queries**: Rejected because it introduces network lag or database query latency, making the "Search Everywhere" feedback feel sluggish and breaking the requirement for an extremely fast UX.

## Test Strategy
- **Unit Tests**: Focus heavily on fuzzy filtering efficiency, grouping logic (headers), and `Palette` component state logic.
- **Manual UAT**: Rely on manual testing for visual rendering, overlay blending, and overall responsiveness.
- **Integration/E2E**: Add automated programmatic tests for the full flow (e.g., opening the palette with `Ctrl+P`, typing a search, selecting a book, and verifying the screen changes to Detail) where it provides high signal value.

## Open Questions
- None.
