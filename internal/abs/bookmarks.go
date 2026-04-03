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

// GetBookmarks returns bookmarks for a library item, extracted from media progress.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]Bookmark, error) {
	path := fmt.Sprintf("/api/me/progress/%s", itemID)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		if IsHTTPStatus(err, http.StatusNotFound) {
			return []Bookmark{}, nil
		}
		return nil, fmt.Errorf("get bookmarks: %w", err)
	}

	var progress MediaProgressWithBookmarks
	if err := json.Unmarshal(resp, &progress); err != nil {
		return nil, fmt.Errorf("get bookmarks: unmarshal: %w", err)
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

// DeleteBookmark deletes a bookmark from a library item at the given time.
func (c *Client) DeleteBookmark(ctx context.Context, itemID string, time float64) error {
	timeStr := strconv.FormatFloat(time, 'f', 3, 64)
	path := fmt.Sprintf("/api/me/item/%s/bookmark/%s", itemID, timeStr)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}
