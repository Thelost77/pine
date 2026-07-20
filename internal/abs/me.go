package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// MeResponse is the subset of GET /api/me used by pine.
type MeResponse struct {
	MediaProgress []MediaProgressEntry `json:"mediaProgress"`
}

// MediaProgressEntry is a single user progress record from /api/me.
type MediaProgressEntry struct {
	LibraryItemID string  `json:"libraryItemId"`
	EpisodeID     string  `json:"episodeId"`
	CurrentTime   float64 `json:"currentTime"`
	Progress      float64 `json:"progress"`
	IsFinished    bool    `json:"isFinished"`
}

// GetMe returns the current user, including all media progress records.
func (c *Client) GetMe(ctx context.Context) (*MeResponse, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/me", nil)
	if err != nil {
		return nil, fmt.Errorf("get me: %w", err)
	}
	var me MeResponse
	if err := json.Unmarshal(resp, &me); err != nil {
		return nil, fmt.Errorf("get me: unmarshal: %w", err)
	}
	return &me, nil
}
