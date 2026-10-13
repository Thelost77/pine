package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// createBookmarkRequest is the body for POST /api/me/item/{id}/bookmark.
type createBookmarkRequest struct {
	Time  float64 `json:"time"`
	Title string  `json:"title"`
}

// bookmarksResponse is the body of GET /api/me/bookmarks/{id}. The body of
// GET /api/me has the same key, with the bookmarks of all items.
type bookmarksResponse struct {
	Bookmarks []Bookmark `json:"bookmarks"`
}

// GetBookmarks returns the bookmarks of the user for a library item. ABS
// before 2.36.0 has no route for the bookmarks of one item, and answers 404.
// GetBookmarks then reads the bookmarks of the item from GET /api/me, and the
// client does not ask for the route again. A later ABS answers 404 only for
// an item that it does not have. GET /api/me is correct for that server too.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]Bookmark, error) {
	var resp []byte
	var err error
	allItems := c.noBookmarkRoute.Load()
	if !allItems {
		resp, err = c.do(ctx, http.MethodGet, "/api/me/bookmarks/"+itemID, nil)
		allItems = IsHTTPStatus(err, http.StatusNotFound)
	}
	if allItems {
		resp, err = c.do(ctx, http.MethodGet, "/api/me", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get bookmarks: %w", err)
	}
	var body bookmarksResponse
	if err := json.Unmarshal(resp, &body); err != nil {
		return nil, fmt.Errorf("get bookmarks: unmarshal: %w", err)
	}
	if !allItems {
		return body.Bookmarks, nil
	}
	c.noBookmarkRoute.Store(true)
	bookmarks := []Bookmark{}
	for _, bookmark := range body.Bookmarks {
		if bookmark.LibraryItemID == itemID {
			bookmarks = append(bookmarks, bookmark)
		}
	}
	return bookmarks, nil
}

// CreateBookmark creates a bookmark on a library item at the given time.
func (c *Client) CreateBookmark(ctx context.Context, itemID string, time float64, title string) error {
	path := fmt.Sprintf("/api/me/item/%s/bookmark", itemID)
	body := createBookmarkRequest{
		Time:  time,
		Title: title,
	}
	_, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return fmt.Errorf("create bookmark: %w", err)
	}
	return nil
}

// UpdateBookmark updates the title of an existing bookmark.
func (c *Client) UpdateBookmark(ctx context.Context, itemID string, time float64, title string) error {
	path := fmt.Sprintf("/api/me/item/%s/bookmark", itemID)
	body := createBookmarkRequest{
		Time:  time,
		Title: title,
	}
	_, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return fmt.Errorf("update bookmark: %w", err)
	}
	return nil
}

// DeleteBookmark deletes a bookmark from a library item at the given time.
func (c *Client) DeleteBookmark(ctx context.Context, itemID string, time float64) error {
	timeStr := strconv.FormatFloat(time, 'f', -1, 64)
	path := fmt.Sprintf("/api/me/item/%s/bookmark/%s", itemID, timeStr)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}
