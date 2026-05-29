# Metadata Edit Analysis

Date: 2026-05-29

## Summary

Pine can support editing Audiobookshelf book metadata. The narrow feature
requested here is practical: fix incorrectly recognized audiobook metadata,
especially title and author, and possibly series.

The important distinction is between two different scopes:

1. A simple correction editor for title, author, and maybe one series.
2. Full app-wide support for multiple authors and multiple series everywhere.

The recommended path is not to start with a full multi-value UI refactor.
However, pine should still expand its ABS metadata model enough to preserve
multiple authors and multiple series before it allows editing author or series
fields. This prevents destructive saves where pine accidentally replaces a
multi-author or multi-series ABS item with only one value.

The best middle ground is:

- Add model support for ABS `authors[]` and `series[]`.
- Keep the existing single display author and primary series behavior for most
  screens.
- Add a small book-only metadata editor for `title`, `author`, `series`, and
  optionally `series sequence`.
- For the first version, keep multi-author and multi-series UI simple: either
  edit only the primary value, or disable author/series editing when multiple
  entries exist and show a message explaining why.

## User Problem

Audiobookshelf sometimes recognizes books incorrectly. The most common bad
metadata is title and author. Series is useful but secondary.

The desired workflow is:

- Open an audiobook in pine.
- Use a simple edit action.
- Correct title and author.
- Optionally correct series and series number.
- Save changes back to ABS.
- See corrected metadata in pine without restarting.

This does not require a full clone of ABS's web metadata editor. Fields like
subtitle, tags, genres, narrators, publisher, ISBN, ASIN, explicit, abridged,
cover, and description are out of scope for the first version unless explicitly
added later.

## ABS API Findings

Audiobookshelf supports editing library item media metadata.

The relevant ABS route is:

```http
PATCH /api/items/:id/media
```

In the ABS source this is wired in:

```text
audiobookshelf/server/routers/ApiRouter.js:111
```

The handler is:

```text
audiobookshelf/server/controllers/LibraryItemController.js:208-285
```

That handler:

- reads the request body as `mediaPayload`
- calls `req.libraryItem.media.updateFromRequest(mediaPayload)`
- handles book `metadata.series` separately when it is an array
- handles book `metadata.authors` separately when it is an array
- saves the library item
- writes the metadata file through `saveMetadataFile()`
- emits an `item_updated` socket event
- returns JSON containing `updated` and `libraryItem`

ABS protects this endpoint with update permissions. The middleware rejects
`PATCH` and `POST` when the user lacks update permission:

```text
audiobookshelf/server/controllers/LibraryItemController.js:1234-1241
```

The practical result is:

- users with ABS update permission can save edits
- read-only users will receive `403 Forbidden`

Pine currently stores the auth token, but it does not model the current user's
permissions in its login state. A first version can simply attempt the save and
show the `403` error. A more polished version should fetch `/api/me` and hide or
disable the edit action when `permissions.update` is false.

## ABS Payload Shape

ABS does not update a book author using a plain `authorName` string. It expects
arrays for book authors and series.

A minimal book update payload for this feature looks like:

```json
{
  "metadata": {
    "title": "The Player of Games",
    "authors": [{ "name": "Iain M. Banks" }],
    "series": [{ "name": "Culture", "sequence": "2" }]
  }
}
```

ABS book update handling accepts these relevant book metadata fields:

```text
audiobookshelf/server/models/Book.js:375-417
```

The specific fields useful for this feature are:

- `metadata.title`
- `metadata.authors`
- `metadata.series`

The ABS web UI uses the same route and payload pattern.

The details tab saves with:

```text
audiobookshelf/client/components/modals/item/tabs/Details.vue:162-177
```

The book edit form builds changed metadata with:

```text
audiobookshelf/client/components/widgets/BookDetailsEdit.vue:240-271
```

The ABS web form models authors and series as arrays, not as single strings.

## ABS Documentation Caveat

The cloned ABS repository contains `docs/openapi.json`, but the item media edit
route was not found there during inspection. The OpenAPI file appears incomplete
for this route.

The reliable references are the ABS server route, the controller, the book
model, and the ABS web client implementation.

## Current Pine Model State

Pine currently models only a small subset of ABS metadata.

The relevant type is:

```text
internal/abs/types.go:92-100
```

Current fields:

```go
type MediaMetadata struct {
    Title       string
    AuthorName  *string
    Description *string
    Duration    *float64
    Chapters    []Chapter
    Series      *SeriesSequence
}
```

The custom JSON unmarshal currently handles `series` as either an object or an
array, but when ABS returns an array it keeps only the first entry:

```text
internal/abs/types.go:102-147
```

Important consequences:

- Pine keeps `authorName`, but not `authors[]`.
- Pine keeps one `Series`, but not `series[]`.
- Pine can display a simple author string.
- Pine can display and navigate one series.
- Pine cannot currently preserve all ABS author and series data during an edit.

This is acceptable for read-only display in many cases, but it is risky for
write operations.

## Current Pine Usage Of Author

Author display is spread across simple UI helpers and cache construction.

Representative locations:

- `internal/screens/detail/view.go:31-35`
- `internal/screens/home/model.go:640-660`
- `internal/screens/library/model.go:546-569`
- `internal/ui/list_item.go:18-31`
- `internal/screens/search/cache.go:198-215`
- `internal/screens/search/cache.go:485-507`

These locations expect one display author string. They do not need a full
multi-author UI to keep working. They can be updated to use a helper such as
`DisplayAuthor()` that joins multiple authors or falls back to `AuthorName`.

Recommended display behavior:

- If `authors[]` has values, join names with `, `.
- Else if `authorName` exists, use it.
- Else show `Unknown author`.

This keeps current UI behavior while making the model safer.

## Current Pine Usage Of Series

Series handling is more behaviorally significant than author handling.

Representative locations:

- `internal/screens/detail/view.go:94-107`
- `internal/screens/detail/model.go:462-472`
- `internal/screens/detail/model.go:587-590`
- `internal/screens/detail/model.go:711-720`
- `internal/screens/series/model.go:266-270`
- `internal/abs/libraries.go:385-438`
- `internal/app/playback.go:939-946`

The main issue is that pine treats an item as belonging to one current series
for navigation and playback context.

Notably, playback auto-continue stores one series id:

```text
internal/app/playback.go:939-946
```

That means full multi-series support is not just a type change. Pine would need
a rule for which series context is active when a book belongs to more than one
series.

For the simple metadata editor, this does not need to be solved fully. Pine can
continue using a primary series for display/navigation while still preserving the
full `series[]` list in the model.

Recommended primary series behavior:

- Use the first `series[]` entry as the primary series.
- Keep the existing `Series` field or expose a helper returning the first entry.
- Do not redesign series navigation in the first metadata-edit iteration.

## Why A Partial Model Update Is Worth Doing

If pine only edits title, no model expansion is required.

If pine edits author or series, a partial model expansion is worth doing because
ABS uses arrays for these fields. Without retaining those arrays, pine risks
overwriting data it did not display.

Example risky scenario:

- ABS item has authors `Author A` and `Author B`.
- Pine only knows `authorName = "Author A, Author B"` or another display string.
- User edits author to `Author A`.
- Pine sends `authors: [{"name":"Author A"}]`.
- ABS removes `Author B` from the book.

Another risky scenario:

- ABS item belongs to two series.
- Pine keeps only the first series.
- User edits series name.
- Pine sends a one-entry `series` array.
- ABS removes the second series association.

For a metadata correction feature, silent destructive edits would be worse than
not supporting the edge case. That is why the model should preserve arrays even
if the first UI only edits a single value.

## Recommended Model Shape

The smallest safe model expansion is to add array fields while preserving the
existing display-friendly fields.

Suggested additions:

```go
type Author struct {
    ID   string `json:"id,omitempty"`
    Name string `json:"name"`
}

type MediaMetadata struct {
    Title       string           `json:"title"`
    AuthorName  *string          `json:"authorName,omitempty"`
    Authors     []Author         `json:"authors,omitempty"`
    Description *string          `json:"description,omitempty"`
    Duration    *float64         `json:"duration,omitempty"`
    Chapters    []Chapter        `json:"chapters,omitempty"`
    Series      *SeriesSequence  `json:"series,omitempty"`
    SeriesList  []SeriesSequence `json:"-"`
}
```

The exact JSON tags for `Series` and `SeriesList` need care because ABS can
return `series` as either an object or an array. The current custom unmarshal is
already handling this. It can be adjusted to populate both:

- `Series` as the primary first series for current app behavior
- `SeriesList` as all series for preservation and editing safety

An alternative is to replace `Series` with `SeriesList`, but that would force a
larger refactor across the app. Keeping `Series` as a primary-series compatibility
field is the smaller and safer change.

Recommended helper methods:

```go
func (m MediaMetadata) DisplayAuthor() string
func (m MediaMetadata) PrimarySeries() *SeriesSequence
func (m MediaMetadata) SeriesByID(id string) *SeriesSequence
```

These helpers would reduce repeated logic across screens.

## Recommended Editor Behavior

The first editor should be intentionally narrow.

Fields:

- Title
- Author
- Series
- Series number

Scope:

- Books only
- No podcast metadata editing in v1
- No cover editing in v1
- No tags, genres, narrators, ISBN, ASIN, publisher, subtitle, explicit, or
  abridged in v1

Save behavior:

- Build a `PATCH /api/items/:id/media` request.
- Include `metadata.title` when changed.
- Include `metadata.authors` when author changed.
- Include `metadata.series` when series or sequence changed.
- Refetch the item after save with `GET /api/items/:id?expanded=1`.
- Update the detail model with the returned item.
- Invalidate or refresh cached list/search data.

Multi-value behavior for v1:

- If there are zero or one authors, allow editing the single author field.
- If there are multiple authors, show them joined for display but either disable
  author editing or require explicit overwrite confirmation.
- If there are zero or one series entries, allow editing series and sequence.
- If there are multiple series entries, show the primary one but either disable
  series editing or require explicit overwrite confirmation.

The safest first version is to disable editing of author or series when multiple
entries exist. This avoids destructive saves while keeping the UI simple.

## Pine Integration Points

The existing mutation flow for bookmarks is a good template.

Existing bookmark edit pieces:

- detail screen emits typed commands in `internal/screens/detail/model.go`
- root app dispatches commands in `internal/app/model.go`
- async ABS calls live in `internal/app/bookmarks.go`
- the detail screen receives update messages and refreshes content

The metadata edit implementation can follow the same pattern.

Likely new pieces:

- `internal/abs/metadata.go` for update request types and API method
- `internal/screens/detail` additions for an edit command or navigation command
- possibly a new `internal/screens/metadataedit` package for the form
- root app message handling in `internal/app/model.go`
- cache invalidation helpers for home, library, and search

The current detail screen already uses `e` for bookmark editing. A metadata edit
key should avoid that conflict. Options:

- use uppercase `E`
- use `m` for metadata
- expose it through the command palette only
- add it to detail help text only when the item is a book

## Cache And Refresh Requirements

After saving metadata, updating only the detail view is not enough. Pine has
several in-memory caches that can retain stale titles and authors.

Relevant caches:

- home per-library item/recent caches in `internal/screens/home/model.go`
- library page cache in `internal/screens/library/model.go`
- search snapshots with a 15 minute TTL in `internal/screens/search/cache.go`

Minimum acceptable v1 behavior:

- Refetch and update the current detail item immediately.
- Clear or update the relevant library cache entry.
- Clear the relevant search snapshot.

If cache invalidation is not implemented, the user may save metadata correctly
but still see old title/author in home, library, or search until those views
refresh or the search cache expires.

## Testing Requirements

Recommended tests for the model expansion:

- Decode ABS metadata with one author.
- Decode ABS metadata with multiple authors.
- Decode ABS metadata with `series` as an object.
- Decode ABS metadata with `series` as an array.
- Preserve primary-series behavior for existing code.
- Ensure `DisplayAuthor()` falls back to `AuthorName`.
- Ensure `DisplayAuthor()` returns `Unknown author` only when no author data
  exists.

Recommended tests for the API client:

- `UpdateLibraryItemMedia` sends `PATCH /api/items/{id}/media`.
- Title-only payload is encoded correctly.
- Author payload is encoded as `metadata.authors` array.
- Series payload is encoded as `metadata.series` array.
- HTTP errors are surfaced, especially `403`.

Recommended tests for the UI/app flow:

- Editing title updates the detail header after save.
- Editing author sends the expected ABS payload.
- Editing series sends the expected ABS payload.
- Multiple authors disable or warn in the simple editor.
- Multiple series disable or warn in the simple editor.
- Save failure shows an error banner and does not mutate local state as saved.
- Search/home/library stale cache behavior is covered if invalidation is added.

## Effort Estimate

### Option 1: Title Only

This is the smallest useful edit feature.

Work:

- Add ABS update method.
- Add simple title edit UI.
- Save and refetch detail.
- Add basic tests.

Estimated effort:

- Half a day to one day.

Risk:

- Low.
- No author/series array problems.

Downside:

- Does not solve the user's most important correction fully because author is
  often wrong too.

### Option 2: Recommended Safe Simple Editor

This supports title, one author, and one primary series while preserving ABS
multi-value data.

Work:

- Expand ABS metadata model with `Authors` and full series list.
- Keep current display behavior through helpers.
- Add update payload structs.
- Add book-only edit form.
- Save through `PATCH /api/items/:id/media`.
- Refetch detail after save.
- Add targeted cache invalidation.
- Add unit and app-flow tests.

Estimated effort:

- One to two days.

Risk:

- Medium-low.
- Main risk is series semantics and cache invalidation, not the ABS API call.

Why this is recommended:

- It fixes the actual user problem.
- It avoids data loss for multi-author or multi-series books.
- It avoids a large UI redesign.

### Option 3: Full Multi-Author And Multi-Series UX

This means pine fully exposes multiple authors and multiple series in display,
search, detail navigation, and editing.

Work:

- Replace most single-value assumptions with multi-value UI.
- Add multi-select or repeatable fields for authors and series.
- Decide how series navigation works when a book belongs to several series.
- Decide how playback auto-continue chooses a series context.
- Update search and list display to handle multiple values intentionally.
- Expand tests across detail, series, playback, search, and cache behavior.

Estimated effort:

- Two to three days minimum.
- More if the editor needs polished multi-select interactions.

Risk:

- Medium.
- Product decisions matter more than raw coding difficulty.

This is not recommended as the first implementation unless full multi-series
behavior is a core goal.

## Recommendation

Implement Option 2.

Do not do a full app-wide multi-author and multi-series UX refactor first.
However, do update the ABS metadata model enough to retain arrays before adding
author or series editing.

Recommended first milestone:

- Add model support for `authors[]` and `series[]`.
- Add display helper methods.
- Keep current single-display behavior.
- Add a book-only editor for title and author.
- Add series and series number if the current item has zero or one series.
- Disable or warn for multi-author or multi-series cases in v1.

This approach keeps the implementation proportional to the problem while
protecting user metadata from accidental loss.

## Implementation Sketch

Suggested sequence:

1. Update `internal/abs/types.go`.
2. Add tests for decoding authors and multiple series.
3. Add metadata update request types and `UpdateLibraryItemMedia`.
4. Add tests for the PATCH request payload.
5. Add display helpers and update author display call sites.
6. Keep existing primary-series behavior working.
7. Add the simple edit screen or edit mode.
8. Wire detail screen action to root app handling.
9. Save, refetch detail, and invalidate stale caches.
10. Add app-level tests for successful save and `403` failure.

The biggest design decision before coding is how to present multi-author and
multi-series items in the simple editor. The safest answer for v1 is to display
them but not allow editing them unless the user explicitly agrees to overwrite
the full list.
