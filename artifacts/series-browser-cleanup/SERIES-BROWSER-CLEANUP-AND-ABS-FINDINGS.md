# Series browser cleanup and Audiobookshelf API findings

## Why this artifact exists

This document captures the full context behind the series work in `pine`, including:

- what originally broke
- what was fixed correctly
- which workaround path turned into poor UX
- why that workaround was removed again
- what the codebase now does instead
- what an upstream Audiobookshelf improvement could look like

The goal is to leave a durable record so the series topic can be revisited later without re-discovering the same constraints from scratch.

---

## User-visible problem statement

The original user goals were:

1. opening a book's series from book detail should show the actual books in that series
2. library rows should somehow expose series membership/position

The first part was a real correctness bug.

The second part turned into a UX/performance design problem because Audiobookshelf does not expose reliable per-book series metadata on the normal paged library item endpoint.

---

## What was actually wrong

### 1. The original series-detail integration assumed the wrong ABS response shape

`pine` originally treated:

- `GET /api/libraries/{libraryId}/series/{seriesId}`

as if it returned a series object plus the books in that series.

That is not how Audiobookshelf behaves. Upstream server code shows that this route returns series metadata only. The actual books must be loaded separately.

### 2. The correct way to load books in a series is through the library items endpoint

The working path is:

- fetch series metadata from `GET /api/libraries/{libraryId}/series/{seriesId}`
- fetch books from `GET /api/libraries/{libraryId}/items?filter=series.<encoded-id>`

Two upstream behaviors mattered here:

1. the series route is metadata-only
2. the library filter value must be base64-encoded, not passed raw

So the real series-detail fix was:

- keep detail-screen entry into a series
- fetch series contents via filtered library items
- sort the resulting books by sequence

That part is valid and should stay.

---

## What made row-level series rendering awkward

### Audiobookshelf list payloads are minified

Upstream Audiobookshelf code builds normal paged library results from:

- `LibraryItem.getByFilterAndSort(...)`
- `LibraryItem.toOldJSONMinified()`

The important consequence is that the standard library list payload is intentionally lightweight.

For books, the reliable full series object comes from:

- `GET /api/items/{id}?expanded=1`

not from the normal paged library response.

### Why that matters

To render:

- `The Expanse #2`

for each visible book row, the client needs:

- series id
- series name
- series sequence

Those are not reliably present on the generic library page response.

So in the worst case, showing series info for `N` books means:

- `1` call for the library page
- `N` additional expanded item calls

This is not a `pine` bug. It is a consequence of the Audiobookshelf API shape.

---

## The workaround path that was tried

To satisfy the row-level display goal, the client previously added a workaround:

1. load the library page normally
2. detect books missing series metadata
3. fetch expanded book payloads in the background
4. merge the expanded metadata back into rows
5. cache those expanded items for reuse

This was technically correct, but it produced several UX problems.

### Problem A: delayed row updates

Rows appeared immediately without series information, then changed after background fetches completed.

### Problem B: effectively sequential feeling

Even after caching and batching improvements, the enrichment strategy still felt serialized because it stayed tied to the visible cursor window.

### Problem C: engineering complexity disproportionate to the feature

Once row-level series rendering was introduced, it forced extra complexity into:

- library rows
- home rows
- shared cache ownership
- stale request handling
- prefetch heuristics

That complexity existed mainly to compensate for missing metadata in the upstream list endpoint.

---

## Design conclusion

The core conclusion was:

> A dedicated series browser fits the ABS API much better than trying to make generic book rows double as a rich series browser.

That led to the cleanup direction implemented here:

1. keep the **book detail -> series detail** path
2. add an **all-series screen** for the current library
3. remove series labels from general-purpose rows
4. remove row-level series enrichment from library and home

---

## What was kept

### 1. Series information inside book detail

Book detail still fetches expanded item data. That means:

- a book detail screen still shows its series name/sequence when present
- selecting the series row from detail still opens the series-detail screen

This is the right place to show series metadata because detail already justifies a richer item fetch.

### 2. Series-detail screen

The existing series-detail screen remains and still works by:

- loading series metadata
- loading that series's books via filtered library items
- sorting books by sequence

This is the correct ABS-aligned implementation.

---

## What was removed

### 1. Library row series labels

Library rows no longer append series name/sequence into the row description.

Reason:

- the data is not cheap to obtain from ABS list payloads
- background enrichment created laggy and noisy rendering
- the row list should stay fast and stable

### 2. Home / Continue Listening series labels

Home rows no longer enrich books just to display series metadata.

Reason:

- personalized shelves are also minified
- background enrichment there existed mainly to continue the row-label workaround
- it added complexity without being the best UX for browsing series

### 3. Library/home row enrichment pipeline

The extra client logic that expanded books in the background just to decorate list rows was removed from those screens.

That cleanup intentionally reduces:

- network chatter
- staggered row mutation
- cache coordination burden
- row-level feature coupling

---

## What was added

## All-series screen

An all-series browser was added for the current audiobook library.

### Entry point

From the library screen:

- press `s`

### Behavior

- fetches `GET /api/libraries/{libraryId}/series`
- renders a list of series names
- pressing `enter` opens the existing series-detail screen for the selected series

### Why this is better

This aligns with the upstream API:

- Audiobookshelf already exposes a library-scoped series list endpoint
- the client no longer needs to guess series membership from generic book rows
- the user still gets explicit series browsing, but in a place where the API supports it directly

---

## Current architecture after cleanup

### Library screen

- fast paged item list
- no row-level series enrichment
- `s` opens the library's series browser

### Series browser screen

- lists all series in the current library
- selecting one opens series-detail

### Series-detail screen

- shows ordered books in that series

### Book detail screen

- still shows the current book's series membership
- still allows direct navigation into the series-detail view

---

## Why this is a cleaner engineering boundary

The removed workaround was trying to answer a **series-centric question** from a **generic book-list endpoint**.

That is the wrong abstraction boundary.

The new structure instead maps screens to the data the API naturally provides:

- generic book browsing -> library items endpoint
- series browsing -> series endpoint
- series contents -> series-filtered library items
- rich single-book context -> expanded item endpoint

That makes the code simpler and the UX more predictable.

---

## Important upstream evidence

These upstream Audiobookshelf behaviors were confirmed:

### 1. `/api/libraries/{id}/series/{seriesId}` does not return the series books

It returns series metadata only.

### 2. `/api/libraries/{id}/items` is built from minified list serialization

Normal library pages are intentionally light and do not reliably carry full `media.metadata.series`.

### 3. `/api/items/{id}?expanded=1` is the reliable source of full book series metadata

If a client wants full series membership/sequence for an arbitrary book, this is the dependable route.

### 4. Series filtering on library items requires encoded filter values

Correct client format:

- `series.<base64(seriesId)>`

not:

- `series.<raw-series-id>`

---

## Files in `pine` most relevant to this topic

### ABS client

- `internal/abs/libraries.go`
- `internal/abs/types.go`

### Series detail

- `internal/screens/series/model.go`

### New series browser

- `internal/screens/serieslist/model.go`

### Library cleanup

- `internal/screens/library/model.go`

### Home cleanup

- `internal/screens/home/model.go`

### Detail-level series preservation

- `internal/app/bookmarks.go`
- `internal/screens/detail/model.go`
- `internal/screens/detail/view.go`

---

## Artifact cleanup note

Earlier debug artifacts for:

- broken series loading
- library row-level series enrichment
- shared metadata cache for row-level series labels

were intentionally removed once this document was written. They described intermediate states that are no longer the intended direction. This artifact now serves as the single durable reference for the full series investigation and cleanup.

---

## Possible future Audiobookshelf upstream improvements

If an upstream PR is considered, the most useful improvements would likely be one of these:

### Option A: allow richer paged library item responses

For example:

- `GET /api/libraries/{id}/items?expanded=1`
- or `GET /api/libraries/{id}/items?include=series`

This would let clients request full series metadata for a paged result without making `N` per-item calls.

### Option B: guarantee series metadata on book list rows

If the generic book list always returned at least:

- `series.id`
- `series.name`
- `series.sequence`

then clients could show row-level series info cheaply without expanded item requests.

### Option C: expose a dedicated bulk book expansion endpoint

For example:

- `POST /api/items/bulk`
- body with item ids and requested include fields

This would still keep list responses light while avoiding `N` follow-up requests.

---

## Recommendation if revisiting this later

Unless Audiobookshelf changes upstream, the current direction should remain:

1. keep detail-screen series metadata
2. keep the series-detail screen
3. keep the dedicated all-series browser
4. avoid reintroducing row-level series enrichment in generic lists

If row-level series labels are ever reconsidered, they should only come back if one of these becomes true:

- Audiobookshelf adds a richer bulk endpoint
- Audiobookshelf guarantees series metadata on normal list rows
- the UX requirement becomes strong enough to justify deliberately slower page hydration

Without one of those conditions, the dedicated series browser is the better tradeoff.
