# Code Audit: Command Palette & Search Cache Series Indexing

Base: `e0b0bcbbde4ea1be566089d144aefd378994d7e2`
Head: `e37927d2c70014a51e60f065a7d76ee6f6df6df0` (plus local changes)
Reviewed on: `2026-05-21`

## Verdict

- **Recommendation:** do not merge
- **Counts:** Critical 0, High 3, Medium 2, Low 1, Nit 1

While the overall feature is well-designed and the test suite passes, there are three high-severity correctness issues affecting key command palette context actions that make the feature dysfunctional or incorrect in production usage. These issues should be resolved before merging.

---

## Merge-Blocking Findings

### High: Detail Screen "Play" Action is Broken

- **File/line:** `internal/screens/detail/model.go:689` and `internal/app/model.go:1156`
- **Proof:**
  The "Play" action in the Detail screen's palette context actions is mapped to `components.ActionOpenDetail`:
  ```go
  {Label: "Play", Action: components.ActionOpenDetail, LibraryID: item.LibraryID, ItemID: item.ID}
  ```
  In `internal/app/model.go:1156`, `ActionOpenDetail` triggers detail view navigation/load:
  ```go
  case components.ActionOpenDetail:
      ...
      if itemID != "" && libraryID != "" {
          m.detail = detail.New(m.styles, abs.LibraryItem{ID: itemID, LibraryID: libraryID, MediaType: "book"})
          m2, navCmd := m.navigate(ScreenDetail)
          return m2, tea.Batch(m.detailLoadCmds(abs.LibraryItem{ID: itemID, LibraryID: libraryID, MediaType: "book"}, navCmd)...)
      }
  ```
  This reload-navigates to the Detail screen instead of initiating playback.
- **Impact:** Selecting "Play" on the Detail screen from the command palette does not start playing; it only performs a useless screen reload.
- **Suggested fix:** Change the action to `components.ActionPlayDirect` and pass the item as `Data` context:
  ```diff
  -{Label: "Play", Action: components.ActionOpenDetail, LibraryID: item.LibraryID, ItemID: item.ID},
  +{Label: "Play", Action: components.ActionPlayDirect, LibraryID: item.LibraryID, ItemID: item.ID, Data: item},
  ```

### High: Detail Screen Context Actions Target Incorrect Item

- **File/line:** `internal/screens/detail/model.go:690, 693` and `internal/app/model.go:1220, 1227`
- **Proof:**
  The "Add Bookmark" and "Mark Finished" actions in the Detail screen context actions do not supply any payload/data context:
  ```go
  {Label: "Add Bookmark", Action: components.ActionAddBookmark},
  {Label: "Mark Finished", Action: components.ActionMarkFinished},
  ```
  In `internal/app/model.go:1220` and `model.go:1227`, these actions fall back to using `m.itemID`, `m.playbackLibraryID`, and `m.episodeID`, which represent the *currently playing* item:
  ```go
  case components.ActionAddBookmark:
      if m.isPlaying() {
          return m.handleAddBookmark(detail.AddBookmarkCmd{
              Item: abs.LibraryItem{ID: m.itemID, LibraryID: m.playbackLibraryID},
          })
      }
  ```
- **Impact:** If the user is viewing Book A on the Detail screen but Book B is currently playing, selecting "Add Bookmark" or "Mark Finished" from the command palette will target Book B instead of Book A. If no book is playing, selecting these actions does nothing because `m.isPlaying()` evaluates to false.
- **Suggested fix:** Pass the viewed item's ID and library ID context in `SelectedPaletteActions()`. Only show "Add Bookmark" if the item currently playing matches the item currently viewed. For "Mark Finished", update the handler to execute against the target item (which can be done via `client.UpdateProgress` without playing the book).

### High: Series List Screen "Open Selected" Action is Broken

- **File/line:** `internal/screens/serieslist/model.go:241` and `internal/app/model.go:1156`
- **Proof:**
  The action returns `components.ActionOpenDetail` with `ItemID: sel.series.ID` and `Payload: "series"`:
  ```go
  {Label: "Open Selected", Action: components.ActionOpenDetail, LibraryID: m.libraryID, ItemID: sel.series.ID, Payload: "series"}
  ```
  However, `ActionOpenDetail` handling in `model.go` does not check the payload, treating the series ID as a book ID and routing to the book Detail screen:
  ```go
  case components.ActionOpenDetail:
      ...
      if itemID != "" && libraryID != "" {
          m.detail = detail.New(m.styles, abs.LibraryItem{ID: itemID, LibraryID: libraryID, MediaType: "book"})
          m2, navCmd := m.navigate(ScreenDetail)
          ...
  ```
- **Impact:** Selecting "Open Selected" on a series in the Series List screen via the command palette attempts to load the series ID on the book Detail screen, causing an API error/404 because a series ID does not correspond to a library item.
- **Suggested fix:** Add `if payload == "series"` handling under `case components.ActionOpenDetail` in `model.go` to instantiate `series.New(...)` and navigate to `ScreenSeries`:
  ```go
  if payload == "series" {
      m.series = series.New(m.styles, m.client, libraryID, itemID, "")
      return m.navigate(ScreenSeries)
  }
  ```

---

## Non-Blocking Findings

### Medium: "Show Queue" Action is a Dead UX Action

- **File/line:** `internal/app/model.go:1034`, `internal/app/model.go:1151`
- **Proof:**
  When `len(m.queue) > 0`, the palette offers a "Show Queue" action:
  ```go
  components.PaletteItem{Label: "Show Queue", Action: components.ActionShowQueue}
  ```
  This action triggers `ActionShowQueue` which is mapped to returning `m, nil` (does nothing).
- **Impact:** Confuses the user since there is no separate Queue screen in the application.
- **Suggested fix:** Remove "Show Queue" from the command palette list since there is no UI to show the queue.

### Medium: Chapter Overlay Bypasses restoreFooter Layout Loop

- **File/line:** `internal/app/render.go:42-53`
- **Proof:**
  If `m.chapterOverlayVisible` is true, the `View()` function returns early:
  ```go
  if m.chapterOverlayVisible {
      return m.overlayChapterModal(content)
  }
  ```
  This bypasses `restoreFooter` which is called later:
  ```go
  if m.width > 0 && m.height > 0 {
      content = m.restoreFooter(content, hints, footer)
  }
  ```
- **Impact:** When the chapter overlay is active, the player footer and hint line at the bottom of the screen may be truncated or completely missing (especially on short terminals) because `restoreFooter` is not run.
- **Suggested fix:** Call `restoreFooter` before rendering the chapter overlay modal, or ensure footer restoration happens at the end of `View()` for all rendering paths.

### Low: Unused openContextPalette Method

- **File/line:** `internal/app/model.go:986:17`
- **Proof:**
  `golangci-lint` warning:
  ```
  internal/app/model.go:986:17: func (*Model).openContextPalette is unused (unused)
  ```
  A code search reveals no callers of `openContextPalette`.
- **Impact:** Dead code.
- **Suggested fix:** Delete `openContextPalette` from `internal/app/model.go`.

### Nit: Lipgloss Style Copy Deprecations

- **File/line:** `internal/ui/components/palette.go:167, 168, 171, 172, 173`
- **Proof:**
  `golangci-lint` output:
  ```
  internal/ui/components/palette.go:167:45: SA1019: styles.Selected.Copy is deprecated: to copy just use assignment (i.e. a := b).
  ```
- **Impact:** Minor style deprecation.
- **Suggested fix:** Replace `.Copy()` calls with direct style assignment.

---

## Residual Risks

- **Silent Series Search Failures:** If fetching series from ABS fails or times out, it fails silently in the cache build without bubbling up a warning to the user, returning only book search results. This is acceptable for search but should be monitored.

---

## Verification Notes

- `go test ./...` in `/home/thelost/Projekty/pine`: **PASS** (534 passed in 17 packages).
- `golangci-lint run` in `/home/thelost/Projekty/pine`: **FAIL** (42 issues, including the unused method and lipgloss style deprecations).
