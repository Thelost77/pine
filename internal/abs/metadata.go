package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// UpdateMediaRequest is the body for PATCH /api/items/{id}/media.
type UpdateMediaRequest struct {
	Metadata UpdateMediaMetadata `json:"metadata"`
}

// UpdateMediaMetadata contains only metadata fields pine intentionally edits.
type UpdateMediaMetadata struct {
	Title   *string           `json:"title,omitempty"`
	Authors *[]Author         `json:"authors,omitempty"`
	Series  *[]SeriesSequence `json:"series,omitempty"`
}

// UpdatePodcastEpisodeRequest is the body for PATCH /api/podcasts/{id}/episode/{episodeId}.
type UpdatePodcastEpisodeRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Season      *string `json:"season,omitempty"`
	Episode     *string `json:"episode,omitempty"`
	EpisodeType *string `json:"episodeType,omitempty"`
}

// UpdateLibraryItemMedia updates an ABS library item's media metadata.
func (c *Client) UpdateLibraryItemMedia(ctx context.Context, itemID string, req UpdateMediaRequest) (*LibraryItem, error) {
	path := fmt.Sprintf("/api/items/%s/media", itemID)
	data, err := c.do(ctx, http.MethodPatch, path, req)
	if err != nil {
		return nil, fmt.Errorf("update library item media: %w", err)
	}

	var resp struct {
		LibraryItem *LibraryItem `json:"libraryItem"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode update media response: %w", err)
	}
	return resp.LibraryItem, nil
}

// UpdatePodcastEpisode updates ABS metadata for a single podcast episode.
func (c *Client) UpdatePodcastEpisode(ctx context.Context, itemID, episodeID string, req UpdatePodcastEpisodeRequest) (*LibraryItem, error) {
	path := fmt.Sprintf("/api/podcasts/%s/episode/%s", itemID, episodeID)
	data, err := c.do(ctx, http.MethodPatch, path, req)
	if err != nil {
		return nil, fmt.Errorf("update podcast episode: %w", err)
	}

	var item LibraryItem
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("decode update podcast episode response: %w", err)
	}
	return &item, nil
}
