package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListeningSession is one playback session of the user. StartTime and
// CurrentTime are positions in the whole book or episode, in seconds.
// CurrentTime is the position of the last sync. UpdatedAt is the time of the
// last sync, in Unix milliseconds.
type ListeningSession struct {
	ID            string `json:"id"`
	LibraryItemID string `json:"libraryItemId"`
	EpisodeID     string `json:"episodeId"`
	MediaType     string `json:"mediaType"`
	// DisplayTitle is the title of the book, or the title of the episode.
	DisplayTitle  string `json:"displayTitle"`
	DisplayAuthor string `json:"displayAuthor"`
	// MediaMetadata.Title is the title of the book or of the podcast.
	MediaMetadata struct {
		Title string `json:"title"`
	} `json:"mediaMetadata"`
	StartTime     float64    `json:"startTime"`
	CurrentTime   float64    `json:"currentTime"`
	TimeListening float64    `json:"timeListening"`
	UpdatedAt     int64      `json:"updatedAt"`
	DeviceInfo    DeviceInfo `json:"deviceInfo"`
}

// ListeningSessionsResponse is one page of listening sessions, the newest
// first. Total is the number of sessions on all pages.
type ListeningSessionsResponse struct {
	Total    int                `json:"total"`
	Sessions []ListeningSession `json:"sessions"`
}

// GetListeningSessions returns the newest listening sessions of the user, for
// all items.
func (c *Client) GetListeningSessions(ctx context.Context, limit int) (*ListeningSessionsResponse, error) {
	return c.getListeningSessions(ctx, "/api/me/listening-sessions", limit)
}

// GetItemListeningSessions returns the newest listening sessions of the user
// for a book, or for one episode of a podcast. The server answers 404 for a
// podcast if episodeID is empty.
func (c *Client) GetItemListeningSessions(ctx context.Context, itemID, episodeID string, limit int) (*ListeningSessionsResponse, error) {
	path := "/api/me/item/listening-sessions/" + itemID
	if episodeID != "" {
		path += "/" + episodeID
	}
	return c.getListeningSessions(ctx, path, limit)
}

func (c *Client) getListeningSessions(ctx context.Context, path string, limit int) (*ListeningSessionsResponse, error) {
	query := url.Values{}
	query.Set("itemsPerPage", strconv.Itoa(limit))
	query.Set("page", "0")
	data, err := c.do(ctx, http.MethodGet, path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("get listening sessions: %w", err)
	}

	var resp ListeningSessionsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode listening sessions response: %w", err)
	}
	return &resp, nil
}
