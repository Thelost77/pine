package abs

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Thelost77/pine/internal/logger"
)

const batchFetchConcurrency = 10

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

// recentEpisodeResponse is the ABS response shape for GET /api/libraries/{id}/recent-episodes.
type recentEpisodeResponse struct {
	Episodes []recentEpisodeEntry `json:"episodes"`
	Total    int                  `json:"total"`
	Limit    int                  `json:"limit"`
	Page     int                  `json:"page"`
}

// recentEpisodeEntry represents a single episode in the recent-episodes response.
type recentEpisodeEntry struct {
	LibraryItemID string               `json:"libraryItemId"`
	ID            string               `json:"id"`
	Index         *int                 `json:"index"`
	Title         string               `json:"title"`
	Description   string               `json:"description,omitempty"`
	Duration      float64              `json:"duration"`
	PublishedAt   *int64               `json:"publishedAt,omitempty"`
	AddedAt       int64                `json:"addedAt,omitempty"`
	Podcast       recentEpisodePodcast `json:"podcast"`
}

// recentEpisodePodcast contains podcast metadata attached to a recent episode.
type recentEpisodePodcast struct {
	Metadata  recentEpisodePodcastMetadata `json:"metadata"`
	CoverPath string                       `json:"coverPath,omitempty"`
}

// recentEpisodePodcastMetadata contains podcast-level metadata.
type recentEpisodePodcastMetadata struct {
	Title  string `json:"title"`
	Author string `json:"author,omitempty"`
}

// GetRecentEpisodes fetches recently added podcast episodes for a library.
func (c *Client) GetRecentEpisodes(ctx context.Context, libraryID string, limit int) ([]LibraryItem, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	path := fmt.Sprintf("/api/libraries/%s/recent-episodes", libraryID)
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get recent episodes: %w", err)
	}

	var resp recentEpisodeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode recent episodes response: %w", err)
	}

	items := make([]LibraryItem, 0, len(resp.Episodes))
	for _, ep := range resp.Episodes {
		authorName := ep.Podcast.Metadata.Author
		var index *int
		if ep.Index != nil {
			index = ep.Index
		}
		var publishedAt *int64
		if ep.PublishedAt != nil {
			publishedAt = ep.PublishedAt
		}
		items = append(items, LibraryItem{
			ID:        ep.LibraryItemID,
			MediaType: "podcast",
			AddedAt:   ep.AddedAt,
			Media: Media{
				Metadata: MediaMetadata{
					Title:      ep.Podcast.Metadata.Title,
					AuthorName: &authorName,
				},
				CoverPath: strPtr(ep.Podcast.CoverPath),
			},
			RecentEpisode: &PodcastEpisode{
				ID:          ep.ID,
				Index:       index,
				Title:       ep.Title,
				Description: ep.Description,
				Duration:    ep.Duration,
				PublishedAt: publishedAt,
				AddedAt:     ep.AddedAt,
			},
		})
	}
	return items, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetRecentlyAdded fetches and merges the "recently-added" personalized shelf for the given libraries.
func (c *Client) GetRecentlyAdded(ctx context.Context, libraries []Library) ([]LibraryItem, error) {
	items := make([]LibraryItem, 0)
	for _, lib := range libraries {
		sections, err := c.GetPersonalized(ctx, lib.ID)
		if err != nil {
			return nil, fmt.Errorf("get recently added for library %s: %w", lib.ID, err)
		}
		for _, section := range sections {
			if section.ID != "recently-added" {
				continue
			}
			items = append(items, section.Entities...)
			break
		}
	}
	SortRecentlyAdded(items)
	return items, nil
}

// GetLibraryItems returns a paginated list of items in a library.
func (c *Client) GetLibraryItems(ctx context.Context, libraryID string, page, limit int) (*LibraryItemsResponse, error) {
	return c.getLibraryItems(ctx, libraryID, page, limit, "")
}

// GetLibrarySeries returns a paginated list of series in a library.
func (c *Client) GetLibrarySeries(ctx context.Context, libraryID string, page, limit int) (*LibrarySeriesResponse, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))
	path := fmt.Sprintf("/api/libraries/%s/series?%s", libraryID, query.Encode())
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get library series: %w", err)
	}

	var resp LibrarySeriesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode library series response: %w", err)
	}
	return &resp, nil
}

func (c *Client) getLibraryItems(ctx context.Context, libraryID string, page, limit int, filter string) (*LibraryItemsResponse, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))
	if filter != "" {
		query.Set("filter", filter)
	}
	path := fmt.Sprintf("/api/libraries/%s/items?%s", libraryID, query.Encode())
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

const podcastSearchPageLimit = 100

// SearchPodcastEpisodes scans podcast library items and returns episode-level prefix hits.
func (c *Client) SearchPodcastEpisodes(ctx context.Context, libraryID, query string) ([]LibraryItem, error) {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return nil, nil
	}

	page := 0
	items := make([]LibraryItem, 0)
	for {
		resp, err := c.GetLibraryItems(ctx, libraryID, page, podcastSearchPageLimit)
		if err != nil {
			return nil, fmt.Errorf("list podcast library items: %w", err)
		}

		ids := make([]string, len(resp.Results))
		for i, li := range resp.Results {
			ids[i] = li.ID
		}
		fullItems, err := c.GetLibraryItemsBatch(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, item := range fullItems {
			for _, episode := range item.Media.Episodes {
				if !strings.HasPrefix(strings.ToLower(episode.Title), normalized) {
					continue
				}
				resultItem := *item
				ep := episode
				resultItem.RecentEpisode = &ep
				resultItem.Media.Episodes = []PodcastEpisode{ep}
				items = append(items, resultItem)
			}
		}

		if len(resp.Results) == 0 || len(resp.Results) < podcastSearchPageLimit {
			break
		}
		loaded := (page + 1) * podcastSearchPageLimit
		if resp.Total > 0 && loaded >= resp.Total {
			break
		}
		page++
	}

	return items, nil
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

// GetLibraryItemsBatch fetches multiple library items concurrently.
func (c *Client) GetLibraryItemsBatch(ctx context.Context, ids []string) ([]*LibraryItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]*LibraryItem, len(ids))
	var firstErr error
	var errOnce sync.Once
	sem := make(chan struct{}, batchFetchConcurrency)
	var wg sync.WaitGroup

	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			item, err := c.GetLibraryItem(ctx, id)
			if err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("expand item %s: %w", id, err)
					cancel()
				})
				return
			}
			results[i] = item
		}()
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// GetSeries returns series metadata scoped to a library.
func (c *Client) GetSeries(ctx context.Context, libraryID, seriesID string) (*Series, error) {
	path := fmt.Sprintf("/api/libraries/%s/series/%s", libraryID, seriesID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get series: %w", err)
	}

	var series Series
	if err := json.Unmarshal(data, &series); err != nil {
		return nil, fmt.Errorf("decode series response: %w", err)
	}
	return &series, nil
}

const seriesPageLimit = 50

// GetSeriesContents returns series metadata plus all library items in that series.
func (c *Client) GetSeriesContents(ctx context.Context, libraryID, seriesID string) (*SeriesContents, error) {
	series, err := c.GetSeries(ctx, libraryID, seriesID)
	if err != nil {
		return nil, err
	}
	if series == nil {
		return nil, nil
	}

	items := make([]LibraryItem, 0)
	for page := 0; ; page++ {
		resp, err := c.getLibraryItems(ctx, libraryID, page, seriesPageLimit, "series."+encodeLibraryFilterValue(seriesID))
		if err != nil {
			return nil, fmt.Errorf("get series items: %w", err)
		}
		items = append(items, resp.Results...)

		if len(resp.Results) == 0 || len(resp.Results) < seriesPageLimit {
			break
		}
		loaded := (page + 1) * seriesPageLimit
		if resp.Total > 0 && loaded >= resp.Total {
			break
		}
	}

	sortSeriesItems(items, seriesID)
	if series.Name == "" {
		for _, item := range items {
			if item.Media.Metadata.Series != nil && item.Media.Metadata.Series.ID == seriesID && item.Media.Metadata.Series.Name != "" {
				series.Name = item.Media.Metadata.Series.Name
				break
			}
		}
	}

	return &SeriesContents{Series: *series, Items: items}, nil
}

func sortSeriesItems(items []LibraryItem, seriesID string) {
	slices.SortStableFunc(items, func(a, b LibraryItem) int {
		aseq, aok := seriesSequenceValue(a, seriesID)
		bseq, bok := seriesSequenceValue(b, seriesID)
		switch {
		case aok && bok:
			if aseq < bseq {
				return -1
			}
			if aseq > bseq {
				return 1
			}
		case aok:
			return -1
		case bok:
			return 1
		}

		atitle := a.Media.Metadata.Title
		btitle := b.Media.Metadata.Title
		if atitle != btitle {
			return cmp.Compare(atitle, btitle)
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func seriesSequenceValue(item LibraryItem, seriesID string) (float64, bool) {
	if item.Media.Metadata.Series == nil || item.Media.Metadata.Series.ID != seriesID {
		return 0, false
	}
	sequence := strings.TrimSpace(item.Media.Metadata.Series.Sequence)
	if sequence == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(sequence, 64)
	if err != nil || math.IsNaN(value) {
		return 0, false
	}
	return value, true
}

func encodeLibraryFilterValue(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

const audioCheckSampleSize = 10

// FilterAudioLibraries filters out "book" type libraries that contain no audio content
// (ebooks without audio). Podcasts are always kept. For book libraries, it samples
// items and checks if any has Duration > 0 to determine if it's an audio library.
// Audio checks for book libraries run in parallel.
func (c *Client) FilterAudioLibraries(ctx context.Context, libs []Library) ([]Library, error) {
	if len(libs) == 0 {
		return libs, nil
	}

	logger.Debug("filtering audio libraries", "inputCount", len(libs))

	type checkResult struct {
		include bool
		err     error
	}

	// Run audio checks in parallel for book libraries.
	// Each goroutine writes to its own index — no mutex needed.
	checks := make([]checkResult, len(libs))
	var wg sync.WaitGroup
	for i, lib := range libs {
		if lib.MediaType != "book" {
			continue
		}
		wg.Add(1)
		go func(i int, lib Library) {
			defer wg.Done()

			hasAudio, err := c.libraryHasAudio(ctx, lib.ID)
			checks[i] = checkResult{include: hasAudio, err: err}
			if err != nil {
				logger.Warn("failed to check library for audio", "libraryID", lib.ID, "err", err)
				return
			}
			if !hasAudio {
				logger.Info("excluding ebook-only library", "libraryID", lib.ID, "name", lib.Name)
			}
		}(i, lib)
	}
	wg.Wait()

	result := make([]Library, 0, len(libs))
	for i, lib := range libs {
		if lib.MediaType == "podcast" {
			result = append(result, lib)
			continue
		}
		if lib.MediaType == "book" {
			if checks[i].err != nil || checks[i].include {
				result = append(result, lib)
			}
			continue
		}
	}

	logger.Info("audio libraries filtered", "inputCount", len(libs), "outputCount", len(result))
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

	for i, item := range resp.Results {
		if item.Media.Duration != nil && *item.Media.Duration > 0 {
			logger.Debug("library audio detected", "libraryID", libraryID, "sampleSize", len(resp.Results), "sampleIndex", i, "signal", "media.duration")
			return true, nil
		}
		if item.Media.Metadata.Duration != nil && *item.Media.Metadata.Duration > 0 {
			logger.Debug("library audio detected", "libraryID", libraryID, "sampleSize", len(resp.Results), "sampleIndex", i, "signal", "metadata.duration")
			return true, nil
		}
		if item.Media.NumAudioFiles != nil && *item.Media.NumAudioFiles > 0 {
			logger.Debug("library audio detected", "libraryID", libraryID, "sampleSize", len(resp.Results), "sampleIndex", i, "signal", "numAudioFiles")
			return true, nil
		}
		if item.Media.NumTracks != nil && *item.Media.NumTracks > 0 {
			logger.Debug("library audio detected", "libraryID", libraryID, "sampleSize", len(resp.Results), "sampleIndex", i, "signal", "numTracks")
			return true, nil
		}
	}

	logger.Debug("library audio not detected in sample", "libraryID", libraryID, "sampleSize", len(resp.Results))
	return false, nil
}
