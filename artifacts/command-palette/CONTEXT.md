# Context: command-palette

## Implementation Decisions

### Series Search in Palette
- **Decision:** Extend `search.Cache` to also fetch and snapshot series data via `GetLibrarySeries`.
- **Rationale:** Ensures series are treated as first-class searchable entities just like audiobooks and podcasts. Relying on the `seriesList` screen's memory would be fragile since it requires the user to have visited that screen first.
- **Options considered:** Extend `search.Cache`, Read from `seriesList` screen state, Skip series.

### Rendering Grouped Section Headers
- **Decision:** Use `bubbles/list` with "header" pseudo-items that render distinctly and are skipped over during keyboard navigation.
- **Rationale:** Leverages the existing, robust pagination and scrolling of `bubbles/list` while faking section dividers, avoiding the need to build a custom scrollable list component from scratch.
- **Options considered:** Pseudo-items in `bubbles/list`, Custom minimal scrollable list, Visual separators without explicit headers.

### Context Actions Data Flow
- **Decision:** Each screen model will implement a method (e.g., `SelectedPaletteActions()`) to return contextual actions for its currently selected item. The root model queries the active screen before passing data to the palette.
- **Rationale:** Maintains strong encapsulation. The root model doesn't need to know the internal data structures of each screen to figure out what item is highlighted.
- **Options considered:** Screens provide actions vs Root model reads internal screen state directly.

### Search Cache Cold-Start Behavior
- **Decision:** Trigger the `search.Cache` build preemptively in the background whenever a library is loaded or the app starts.
- **Rationale:** "Search Everywhere" needs to be instant. If we wait for the user to press `Ctrl+P` to start indexing, the first experience will be sluggish.
- **Options considered:** Preemptive build on library load, Show "Indexing..." placeholder, Skip content until ready.

## Existing Code Insights
- **`search.Cache`**: Already handles fuzzy matching via `sahilm/fuzzy`, tokenization, debouncing, and background snapshot building. We just need to extend `snapshotEntry` and `ensureSnapshot` to handle series.
- **`normalizeOverlayCanvas`**: Exists in `internal/app/render.go` and is perfectly suited to render the command palette as a centered modal.
- **Key Interception**: `Update()` in `internal/app/model.go` has a clear precedence order. We'll insert the palette intercept right after the help overlay.

## Integration Points
- **`internal/app/model.go`**: Add `palette` component, intercept `Ctrl+P`.
- **`internal/app/render.go`**: Render palette overlay.
- **`internal/screens/search/cache.go`**: Add series fetching and indexing.
- **`internal/app/messages.go`**: Add new palette action messages.
