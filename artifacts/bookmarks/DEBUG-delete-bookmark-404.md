# Debug: delete bookmark 404

## Phase 1: Root Cause Investigation
- **Error:** `delete bookmark: unexpected status 404: Not Found`
- **Reproduction:**  
  1. Create/select a bookmark with a fractional timestamp such as `4733.343044`.  
  2. Trigger bookmark deletion from the detail screen.  
  3. Observe `DELETE /api/me/item/{id}/bookmark/4733.343` returning `404`.
- **Recent changes:**  
  - `6f42d50` switched bookmark reads to the ABS user bookmark collection, which made add/list flows start working and exposed the remaining delete mismatch clearly.
- **Evidence:**  
  - Runtime log shows `DELETE path=/api/me/item/4e899c38-.../bookmark/4733.343 status=404 body="Not Found"`.  
  - The selected bookmark came from user bookmarks with the original time `4733.343044`.  
  - `internal/abs/bookmarks.go` formatted delete times with `strconv.FormatFloat(time, 'f', 3, 64)`.
- **Data flow trace:**  
  Selected bookmark time -> `DeleteBookmark()` truncates to 3 decimals -> ABS `removeBookmark()` parses the URL time and does exact `==` matching against stored bookmark time -> no exact match -> `404`.

## Phase 2: Pattern Analysis
- **Working example:** Bookmark creation sends the raw float time value, and ABS stores it unchanged in `user.bookmarks`.
- **Differences:**  
  - Create path preserved full precision.  
  - Delete path truncated precision to 3 decimals.
- **Dependencies:**  
  - ABS `User.findBookmark(libraryItemId, time)` and `removeBookmark()` use exact numeric time matching.

## Phase 3: Hypothesis
- **Hypothesis:** Delete fails because pine rounds bookmark times before sending them, and ABS requires the exact stored float to find the bookmark.
- **Test:** Add a unit test asserting `DeleteBookmark(4733.343044)` calls `/api/me/item/li-001/bookmark/4733.343044`.
- **Result:** confirmed  
  - failing test showed the client was sending `/bookmark/4733.343`

## Phase 4: Fix
- **Root cause:** Delete request serialization lost bookmark time precision required by ABS exact matching.
- **Fix:** Changed delete request formatting from fixed 3-decimal precision to shortest exact decimal (`strconv.FormatFloat(time, 'f', -1, 64)`) and updated tests to the corrected request shape.
- **Test:** Added/updated tests that now pass:
  - `TestDeleteBookmarkHTTP`
  - `TestDeleteBookmarkHTTPPreservesBookmarkPrecision`
  - `TestHandleDeleteBookmarkReturnsEmptyBookmarksWhenLastBookmarkRemoved`
- **Verification:**  
  - `go test ./internal/abs/... -count=1`  
  - `go test ./internal/app/... -count=1`  
  - `go test ./... -count=1`

## Attempts
- Attempt 1: Hypothesis that delete was failing for access or missing bookmark state -> rejected by evidence showing the bookmark existed and the request path time was truncated.
- Attempt 2: Hypothesis that 3-decimal rounding in the delete URL broke ABS exact-time matching -> confirmed and fixed.
