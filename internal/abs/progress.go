package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// updateProgressRequest is the body for PATCH /api/me/progress/{id}.
// The server reads "isFinished": false as "mark as not finished": it removes
// the finished mark and sets the position to 0. With no isFinished key, the
// server sets or keeps the mark by its own rule for the position.
type updateProgressRequest struct {
	CurrentTime float64 `json:"currentTime"`
	Progress    float64 `json:"progress"`
	IsFinished  *bool   `json:"isFinished,omitempty"`
}

// progressPath is the path of the progress of a library item, or of a
// podcast episode if episodeID is not empty.
func progressPath(itemID, episodeID string) string {
	path := "/api/me/progress/" + itemID
	if episodeID != "" {
		path += "/" + episodeID
	}
	return path
}

// UpdateProgress updates the user's progress on a library item, or on a
// podcast episode if episodeID is not empty.
func (c *Client) UpdateProgress(ctx context.Context, itemID, episodeID string, currentTime, progress float64, isFinished bool) error {
	body := updateProgressRequest{
		CurrentTime: currentTime,
		Progress:    progress,
		IsFinished:  &isFinished,
	}
	_, err := c.do(ctx, http.MethodPatch, progressPath(itemID, episodeID), body)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	return nil
}

// UpdatePosition updates the user's position in a library item, or in a
// podcast episode if episodeID is not empty. It sends no isFinished key.
func (c *Client) UpdatePosition(ctx context.Context, itemID, episodeID string, currentTime, progress float64) error {
	body := updateProgressRequest{
		CurrentTime: currentTime,
		Progress:    progress,
	}
	_, err := c.do(ctx, http.MethodPatch, progressPath(itemID, episodeID), body)
	if err != nil {
		return fmt.Errorf("update position: %w", err)
	}
	return nil
}

// GetMediaProgress returns the user's media progress for a specific library
// item. ABS gives no bookmarks in this response: GetBookmarks reads them.
func (c *Client) GetMediaProgress(ctx context.Context, itemID string) (*MediaProgressWithBookmarks, error) {
	path := fmt.Sprintf("/api/me/progress/%s", itemID)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get media progress: %w", err)
	}
	var progress MediaProgressWithBookmarks
	if err := json.Unmarshal(resp, &progress); err != nil {
		return nil, fmt.Errorf("get media progress: unmarshal: %w", err)
	}
	return &progress, nil
}
