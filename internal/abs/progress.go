package abs

import (
	"context"
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
