package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// updateProgressRequest is the body for PATCH /api/me/progress/{id}.
type updateProgressRequest struct {
	CurrentTime float64 `json:"currentTime"`
	Progress    float64 `json:"progress"`
	IsFinished  bool    `json:"isFinished"`
}

// UpdateProgress updates the user's progress on a library item.
func (c *Client) UpdateProgress(ctx context.Context, itemID string, currentTime, progress float64, isFinished bool) error {
	path := fmt.Sprintf("/api/me/progress/%s", itemID)
	body := updateProgressRequest{
		CurrentTime: currentTime,
		Progress:    progress,
		IsFinished:  isFinished,
	}
	_, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	return nil
}

// GetMediaProgress returns the user's media progress (including bookmarks)
// for a specific library item.
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

// UpdateEpisodeProgress updates the user's progress on a podcast episode.
func (c *Client) UpdateEpisodeProgress(ctx context.Context, itemID, episodeID string, currentTime, progress float64, isFinished bool) error {
	path := fmt.Sprintf("/api/me/progress/%s/%s", itemID, episodeID)
	body := updateProgressRequest{
		CurrentTime: currentTime,
		Progress:    progress,
		IsFinished:  isFinished,
	}
	_, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return fmt.Errorf("update episode progress: %w", err)
	}
	return nil
}
