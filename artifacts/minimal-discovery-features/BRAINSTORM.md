# Brainstorm: minimal-discovery-features

## Problem Statement
Pine works well because it stays narrow: the user opens the app, finds one strong next action, and starts listening without wading through metadata or dashboard clutter.

The opportunity is not to make Pine “feature complete” with Audiobookshelf. It is to add a few discovery and navigation tools that preserve the current signal-to-noise ratio while reducing friction in common listening flows.

The three candidate features are:

- **Recently Added** on the home screen
- **Queue** for manual “play next” / “add to queue”
- **Series navigation** from a book detail screen

The product goal is therefore:

- keep Pine visually minimal
- avoid introducing management-heavy surfaces
- improve “what should I listen to now?” and “what comes next?” flows
- prefer features that shorten the path to playback over features that expose more metadata

## Architecture
- **Frontend:** Go terminal UI using Bubble Tea, extending existing home/detail/playback flows
- **Backend:** none beyond existing Audiobookshelf APIs already used by the client
- **Database:** no new persistence required for first pass; queue should stay session-only and in memory
- **External APIs:** Audiobookshelf personalized/library/item/playback APIs, plus whatever item metadata is needed to resolve series relationships
- **Deployment:** existing single-binary local terminal app
- **Validation Strategy:** verify each feature preserves the one-list / one-primary-action feel while behaving correctly across home, detail, and playback transitions

## Chosen Approach
Implement the three features with deliberately narrow product boundaries.

### 1. Recently Added
Keep **Continue Listening** as the main home surface and add a **small Recently Added subsection** beneath it.

Behavior:

- Home remains focused on **Continue Listening**
- add a small **Recently Added** subsection under the main list
- show **3 items**
- exclude any title already present in Continue Listening
- do not turn Recently Added into its own browsing mode or a separate home tab

Why this approach:

- preserves Continue Listening as the primary home intent
- avoids a dead-end without introducing a new navigation mode
- sidesteps awkward cross-library behavior for library/search actions
- keeps Recently Added clearly secondary and low-noise

### 2. Queue
Add a **session-only in-memory queue** with **manual enqueue only**.

Behavior:

- first version supports only:
  - **play next**
  - **add to queue**
- no persistence across restarts
- no auto-enqueue heuristics
- no heavyweight queue-management workflow required for v1

Why this approach:

- queue is useful as a listening aid without becoming a content-management feature
- keeping it ephemeral avoids scope creep into playlist-like behavior
- it adds utility with very little visual overhead

### 3. Series navigation
Expose series as a **compact navigational affordance**, not as expanded metadata.

Behavior:

- in book detail, show a single compact row such as:
  - `Series: The Expanse #2`
- selecting that row opens a simple ordered series list
- the series list focuses on:
  - ordered books
  - current-book context
  - opening another entry

Why this approach:

- solves the concrete audiobook problem of “what comes before/after this?”
- keeps the detail screen compact
- frames series as navigation context rather than metadata browsing

## Rejected Alternatives
- **Home dashboard with multiple permanent shelves:** Rejected because it lowers signal-to-noise and changes Pine’s identity from focused player to browsing surface.
- **Recently Added as a standalone tab/source:** Rejected because it creates extra navigation state and awkward library-context questions for `o` and `/`.
- **Persistent queue / playlist-like queue:** Rejected because it turns a lightweight playback helper into a managed object with storage, recovery, and more UX overhead.
- **Auto-enqueue next podcast episode in v1:** Rejected because it adds behavior users did not explicitly request and weakens the predictability of the queue.
- **Large inline series section on detail:** Rejected because it expands metadata density instead of offering a compact path to adjacent books.
- **General metadata browsing features (authors, narrators, tags, filters, history pages):** Rejected for now because they risk becoming metadata tourism rather than helping the user start listening faster.

## Test Strategy
Use a **hybrid strategy**:

- **Automated behavior tests:** verify the home screen shows a small Recently Added subsection, excludes duplicates already in Continue Listening, and caps the section at 3 items
- **Automated integration-style TUI tests:** verify realistic flows such as empty or short Continue Listening lists with Recently Added underneath, enqueuing from detail/episodes, and opening a series list from a detail view
- **Manual verification:** confirm the UI remains compact and obvious in-terminal, especially for queue visibility and the series affordance

## Open Questions
- How Audiobookshelf exposes the series relationship needed for the compact detail row and ordered series list, and whether Pine’s current ABS client needs additional item-expansion calls or new response types to support it cleanly
