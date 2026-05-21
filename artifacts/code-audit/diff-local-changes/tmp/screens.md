# Cluster 3 Audit Notes: Screen Context Actions

## Target Files
- `internal/screens/detail/model.go`
- `internal/screens/serieslist/model.go`
- `internal/screens/home/model.go`
- `internal/screens/library/model.go`
- `internal/screens/search/model.go`
- `internal/screens/series/model.go`

## Findings

### 1. Detail Screen "Play" Action is Broken (High)
- **File/line:** `internal/screens/detail/model.go:689` and `internal/app/model.go:1156` (ActionOpenDetail)
- **Proof:** The "Play" action in the Detail screen's palette context actions is mapped to `components.ActionOpenDetail`. This action reload-navigates to the Detail screen instead of starting playback. `ActionPlayDirect` exists in `model.go` but requires `Data` to be populated to play, which is missing here.
- **Impact:** User is unable to play the book from the context menu using the "Play" action.
- **Suggested Fix:** Change the action to `components.ActionPlayDirect` and populate the `Data` field with the library item, e.g. `{Label: "Play", Action: components.ActionPlayDirect, LibraryID: item.LibraryID, ItemID: item.ID, Data: item}`.

### 2. Detail Screen Context Actions Target Stale/Incorrect Item (High)
- **File/line:** `internal/screens/detail/model.go:690, 693`, `internal/app/model.go:1220, 1227`
- **Proof:** The "Add Bookmark" and "Mark Finished" actions in `detail/model.go` do not attach the library item ID or data context to the palette item. In `model.go:1220` and `model.go:1227`, these actions fall back to using `m.itemID`, `m.playbackLibraryID`, and `m.episodeID` (which represent the *currently playing* item).
- **Impact:** If the user is viewing Book A on the Detail screen but Book B is currently playing, selecting "Add Bookmark" or "Mark Finished" from the command palette will target Book B instead of Book A. If no book is playing, selecting these actions does nothing because `m.isPlaying()` is false.
- **Suggested Fix:** Pass the target item ID and library ID context in `SelectedPaletteActions()`. Only show "Add Bookmark" if the item currently playing matches the item currently viewed. For "Mark Finished", update the handler to execute against the target item (which can be done via `client.UpdateProgress` without playing the book).

### 3. Series List "Open Selected" Action is Broken (High)
- **File/line:** `internal/screens/serieslist/model.go:241` and `internal/app/model.go:1156` (ActionOpenDetail)
- **Proof:** The "Open Selected" action in the Series List screen's context actions is mapped to `components.ActionOpenDetail` with `ItemID: sel.series.ID` and `Payload: "series"`. However, `ActionOpenDetail` in `model.go` does not check if the payload is `"series"`, and instead constructs a `LibraryItem` with `MediaType: "book"` using the series ID as the library item ID and navigates to the Detail screen.
- **Impact:** Selecting "Open Selected" on a series will attempt to open it as a book on the Detail screen, causing an API error/404 because a series ID is not a library item ID. It should instead navigate to `ScreenSeries` using `series.New(...)`.
- **Suggested Fix:** Update `handlePaletteAction` in `internal/app/model.go` under `case components.ActionOpenDetail` to check `if payload == "series"` and navigate to the Series screen:
  ```go
  if payload == "series" {
      m.series = series.New(m.styles, m.client, libraryID, itemID, "")
      return m.navigate(ScreenSeries)
  }
  ```
