package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Thelost77/pine/internal/logger"
)

// GetLibraries returns all libraries on the server.
func (c *Client) GetLibraries(ctx context.Context) ([]Library, error) {
	data, err := c.do(ctx, http.MethodGet, "/api/libraries", nil)
	if err != nil {
		return nil, fmt.Errorf("get libraries: %w", err)
	}

	var resp struct {
		Libraries []Library `json:"libraries"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode libraries response: %w", err)
	}
	return resp.Libraries, nil
}

// GetPersonalized returns personalized shelves for a library (e.g. "continue-listening").
func (c *Client) GetPersonalized(ctx context.Context, libraryID string) ([]PersonalizedResponse, error) {
	path := fmt.Sprintf("/api/libraries/%s/personalized", libraryID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get personalized: %w", err)
	}

	var sections []PersonalizedResponse
	if err := json.Unmarshal(data, &sections); err != nil {
		return nil, fmt.Errorf("decode personalized response: %w", err)
	}
	return sections, nil
}

// GetLibraryItems returns a paginated list of items in a library.
func (c *Client) GetLibraryItems(ctx context.Context, libraryID string, page, limit int) (*LibraryItemsResponse, error) {
	path := fmt.Sprintf("/api/libraries/%s/items?page=%d&limit=%d", libraryID, page, limit)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get library items: %w", err)
	}

	var resp LibraryItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode library items response: %w", err)
	}
	return &resp, nil
}

// SearchLibrary searches a library by query string.
func (c *Client) SearchLibrary(ctx context.Context, libraryID, query string) (*SearchResult, error) {
	path := fmt.Sprintf("/api/libraries/%s/search?q=%s&limit=12", libraryID, url.QueryEscape(query))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("search library: %w", err)
	}

	var resp SearchResult
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return &resp, nil
}

// GetLibraryItem returns a single library item by ID with full details (including episodes for podcasts).
func (c *Client) GetLibraryItem(ctx context.Context, itemID string) (*LibraryItem, error) {
	path := fmt.Sprintf("/api/items/%s?expanded=1", itemID)
	logger.Debug("API request", "method", "GET", "path", path)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		logger.Error("get library item failed", "itemID", itemID, "err", err)
		return nil, fmt.Errorf("get library item: %w", err)
	}

	var item LibraryItem
	if err := json.Unmarshal(data, &item); err != nil {
		logger.Error("decode library item failed", "itemID", itemID, "err", err, "bodyLen", len(data))
		return nil, fmt.Errorf("decode library item: %w", err)
	}
	logger.Info("library item fetched", "itemID", item.ID, "mediaType", item.MediaType, "episodes", len(item.Media.Episodes))
	return &item, nil
}

const audioCheckSampleSize = 10

// FilterAudioLibraries filters out "book" type libraries that contain no audio content
// (ebooks without audio). Podcasts are always kept. For book libraries, it samples
// items and checks if any has Duration > 0 to determine if it's an audio library.
func (c *Client) FilterAudioLibraries(ctx context.Context, libs []Library) ([]Library, error) {
	if len(libs) == 0 {
		return libs, nil
	}

	result := make([]Library, 0, len(libs))
	for _, lib := range libs {
		// Always keep podcasts
		if lib.MediaType == "podcast" {
			result = append(result, lib)
			continue
		}

		// For "book" type libraries, check if any item has audio
		if lib.MediaType == "book" {
			hasAudio, err := c.libraryHasAudio(ctx, lib.ID)
			if err != nil {
				logger.Warn("failed to check library for audio", "libraryID", lib.ID, "err", err)
				// On error, include the library to be safe
				result = append(result, lib)
				continue
			}
			if hasAudio {
				result = append(result, lib)
			} else {
				logger.Info("excluding ebook-only library", "libraryID", lib.ID, "name", lib.Name)
			}
		}
	}

	return result, nil
}

// libraryHasAudio checks if a "book" type library contains any audio items
// by sampling items and checking if any has Duration > 0.
//
// This function assumes libraries contain homogeneous content (all audiobooks or all ebooks).
// If a library contains mixed content, the sampling may not detect audio content if it falls
// outside the sampled range. In practice, ABS typically separates audiobooks and ebooks
// into distinct libraries, so this is not expected to be an issue.
func (c *Client) libraryHasAudio(ctx context.Context, libraryID string) (bool, error) {
	resp, err := c.GetLibraryItems(ctx, libraryID, 0, audioCheckSampleSize)
	if err != nil {
		return false, err
	}

	for _, item := range resp.Results {
		if item.Media.Duration != nil && *item.Media.Duration > 0 {
			return true, nil
		}
		if item.Media.Metadata.Duration != nil && *item.Media.Metadata.Duration > 0 {
			return true, nil
		}
		if item.Media.NumAudioFiles != nil && *item.Media.NumAudioFiles > 0 {
			return true, nil
		}
		if item.Media.NumTracks != nil && *item.Media.NumTracks > 0 {
			return true, nil
		}
	}

	return false, nil
}
