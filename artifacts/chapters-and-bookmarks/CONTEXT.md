# Context: chapters-and-bookmarks

## Problem

The detail screen's chapter list and Tab focus cycling don't work because chapters are read from `item.Media.Metadata.Chapters` (library item metadata), but ABS only returns chapters in the **play session response** (`POST /api/items/{id}/play`), not in library item endpoints.

The root model stores play session chapters in `m.chapters` and uses them for n/N navigation — that works. But the detail screen never receives them, so:
- Chapter list doesn't render in the detail view
- Tab cycling skips chapters (thinks there are none)
- Enter on a chapter doesn't work (no chapters to select)

## Findings

### Where chapters are used in the detail screen
All reference `m.item.Media.Metadata.Chapters` (always empty from library endpoints):
- `detail/view.go:137` — renders chapter list (guarded by `len(meta.Chapters) > 0`)
- `detail/view.go:200` — help text checks for chapters
- `detail/model.go:307-308` — Enter on focused chapter
- `detail/model.go:343, 371-372` — j/k navigation in chapter list
- `detail/model.go:402` — cycleFocus checks chapter count

### Where chapters come from
- ABS `POST /api/items/{id}/play` response has top-level `chapters` field
- Root model stores them in `m.chapters` after `handlePlaySessionMsg`
- Root model uses them for n/N (nextChapter/prevChapter) — works correctly

### What needs to happen
1. Add `SetChapters([]abs.Chapter)` to detail model (like existing `SetBookmarks`)
2. Store chapters separately from `item.Media.Metadata.Chapters` in the detail model
3. Update all references in the detail model/view to use the stored chapters
4. Call `SetChapters` from root model when play session starts (`handlePlaySessionMsg`)
5. Clear chapters when playback stops

### UX consideration (from user)
- Chapters should be clearly tied to the currently playing item
- Bookmarks can be more separated (they persist in ABS regardless of playback)
- The UI needs to make it clear what's available — currently there's no indication that Tab/chapters exist

### Bookmarks
Bookmarks already work via `SetBookmarks` and `fetchBookmarksCmd`. They're fetched when entering the detail screen (independent of playback). The bookmark model is a good pattern to follow for chapters.
