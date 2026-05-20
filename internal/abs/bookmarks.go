package abs

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// createBookmarkRequest is the body for POST /api/me/item/{id}/bookmark.
type createBookmarkRequest struct {
	Time  float64 `json:"time"`
	Title string  `json:"title"`
}

// GetBookmarks returns bookmarks for a library item via the per-item progress endpoint.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]Bookmark, error) {
	progress, err := c.GetMediaProgress(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("get bookmarks: %w", err)
	}
	return progress.Bookmarks, nil
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
