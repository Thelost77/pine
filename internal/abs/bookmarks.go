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

type userBookmarksResponse struct {
	Bookmarks []Bookmark `json:"bookmarks"`
}

// GetBookmarks returns bookmarks for a library item from the authenticated user's bookmark collection.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]Bookmark, error) {
	path := "/api/me"
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get bookmarks: %w", err)
	}

	var user userBookmarksResponse
	if err := json.Unmarshal(resp, &user); err != nil {
		return nil, fmt.Errorf("get bookmarks: unmarshal: %w", err)
	}

	bookmarks := make([]Bookmark, 0, len(user.Bookmarks))
	for _, bookmark := range user.Bookmarks {
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
