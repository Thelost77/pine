# Brainstorm: bookmarks

## Problem Statement
The bookmark problem is not about mutation first; it is about **visibility**.

- Bookmarks are intended to be persistent saved positions for an item.
- For books, they should remain visible on the detail screen even before playback starts.
- The observed failure is that bookmarks **never appear** on the detail screen.
- Existing chapter work already separated playback-only navigation from persistent item data, so bookmark behavior should now be evaluated on its own terms instead of being conflated with chapters.

The product goal is therefore:

- preserve bookmarks as persistent item-level information
- make them visible from the detail view without requiring active playback
- determine whether the failure is caused by missing data from Audiobookshelf integration or by the terminal UI/state layer failing to present available bookmark data

## Architecture
- **Frontend:** Go terminal UI using Bubble Tea
- **Backend:** Not needed; this remains a client-side behavior issue unless investigation proves the external API contract is missing data
- **Database:** Not needed for this fix; no new persistence is implied
- **External APIs:** Audiobookshelf item, progress, and bookmark-related APIs already used by the client
- **Deployment:** Existing local terminal app distribution
- **Validation Strategy:** First validate the external-data contract versus client presentation, then validate item-level bookmark visibility at the UI/state boundary so bookmarks remain independent from playback-only chapter behavior

## Chosen Approach
Use an **API-contract-first investigation** within the existing stack.

Why this approach:

- The symptom is “bookmarks never appear,” which could come from either data acquisition or presentation.
- Fixing the presentation before proving data exists risks patching the wrong layer.
- Earlier artifacts already established that chapters and bookmarks have different product roles, so bookmark diagnosis should start by proving whether the client is receiving persistent bookmark data for the relevant item.

Technology stack:

- **Frontend:** Go + Bubble Tea terminal UI
- **Backend:** none beyond existing Audiobookshelf API usage
- **Data source:** existing Audiobookshelf bookmark/progress APIs

## Rejected Alternatives
- **Book-first restore:** Rejected for now because it assumes the bug is presentation-only for books before proving whether the client actually receives bookmark data. Stack: Go + Bubble Tea + existing Audiobookshelf APIs.
- **Shared bookmark model first:** Rejected for now because it broadens scope before root cause is known. Stack: Go + Bubble Tea + existing Audiobookshelf APIs.

## Test Strategy
Use a **hybrid strategy**:

- **Automated checks:** Verify bookmark data acquisition and UI state transitions once the source of truth is confirmed.
- **Automated integration-style TUI coverage:** Verify that a user entering a detail screen can see persistent bookmarks without playback.
- **Manual verification:** Confirm the terminal presentation is actually understandable and visible, since bookmark presence can be logically correct but still effectively hidden.

## Open Questions
- None blocking clarification. The remaining uncertainty is diagnostic: whether the bug is API/data-flow or presentation-layer, and whether it is book-specific or shared across bookmark-capable items.
