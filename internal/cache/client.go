package cache

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Thelost77/pine/internal/abs"
)

const (
	ttlLibraries      = 24 * time.Hour
	ttlPersonalized   = 5 * time.Minute
	ttlLibraryItems   = 15 * time.Minute
	ttlLibrarySeries  = 15 * time.Minute
	ttlSeriesContents = 15 * time.Minute
	ttlMediaProgress  = 1 * time.Minute
	ttlBookmarks      = 1 * time.Minute
	ttlEpisodes       = 15 * time.Minute
	ttlRecentEpisodes = 5 * time.Minute
	ttlLibraryItem    = 15 * time.Minute
	ttlRecentlyAdded  = 5 * time.Minute
)

// Client is a transparent caching wrapper around abs.Client.
type Client struct {
	*abs.Client
	store         *Store
	inflightMutex sync.Mutex
	inflight      map[string]*inflightCall
}

type inflightCall struct {
	done chan struct{}
	err  error
}

// NewClient creates a caching wrapper around the given ABS client.
func NewClient(inner *abs.Client, store *Store) *Client {
	if inner == nil {
		panic("cache.NewClient: inner client is nil")
	}
	return &Client{
		Client:   inner,
		store:    store,
		inflight: make(map[string]*inflightCall),
	}
}

func (c *Client) getOrFetch(ctx context.Context, key string, fetch func() error) error {
	c.inflightMutex.Lock()
	if call, ok := c.inflight[key]; ok {
		c.inflightMutex.Unlock()
		select {
		case <-call.done:
			return call.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	call := &inflightCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.inflightMutex.Unlock()

	defer func() {
		c.inflightMutex.Lock()
		delete(c.inflight, key)
		c.inflightMutex.Unlock()
		close(call.done)
	}()

	call.err = fetch()
	return call.err
}

// GetLibraries returns libraries from cache or fetches from the API.
func (c *Client) GetLibraries(ctx context.Context) ([]abs.Library, error) {
	if c.store == nil {
		return c.Client.GetLibraries(ctx)
	}
	if cached, hit, _ := c.store.GetLibraries(); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "libraries", func() error {
		fetched, err := c.Client.GetLibraries(ctx)
		if err != nil {
			return err
		}
		_ = c.store.PutLibraries(fetched, ttlLibraries)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetLibraries()
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key libraries")
	}
	return cached, nil
}

// GetPersonalized returns personalized shelves from cache or fetches from the API.
func (c *Client) GetPersonalized(ctx context.Context, libraryID string) ([]abs.PersonalizedResponse, error) {
	if c.store == nil {
		return c.Client.GetPersonalized(ctx, libraryID)
	}
	if cached, hit, _ := c.store.GetPersonalized(libraryID); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "personalized:"+libraryID, func() error {
		fetched, err := c.Client.GetPersonalized(ctx, libraryID)
		if err != nil {
			return err
		}
		_ = c.store.PutPersonalized(libraryID, fetched, ttlPersonalized)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetPersonalized(libraryID)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key personalized:%s", libraryID)
	}
	return cached, nil
}

// GetRecentEpisodes returns recent podcast episodes from cache or fetches from the API.
func (c *Client) GetRecentEpisodes(ctx context.Context, libraryID string, limit int) ([]abs.LibraryItem, error) {
	if c.store == nil {
		return c.Client.GetRecentEpisodes(ctx, libraryID, limit)
	}
	if cached, hit, _ := c.store.GetRecentEpisodes(libraryID, limit); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "recent-episodes:"+libraryID+":"+strconv.Itoa(limit), func() error {
		fetched, err := c.Client.GetRecentEpisodes(ctx, libraryID, limit)
		if err != nil {
			return err
		}
		_ = c.store.PutRecentEpisodes(libraryID, limit, fetched, ttlRecentEpisodes)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetRecentEpisodes(libraryID, limit)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key recent-episodes:%s:%d", libraryID, limit)
	}
	return cached, nil
}

// GetRecentlyAdded returns the merged recently-added shelf from cache or fetches from the API.
func (c *Client) GetRecentlyAdded(ctx context.Context, libraries []abs.Library) ([]abs.LibraryItem, error) {
	if c.store == nil {
		return c.Client.GetRecentlyAdded(ctx, libraries)
	}
	if cached, hit, _ := c.store.GetRecentlyAdded(); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "recently-added", func() error {
		fetched, err := c.Client.GetRecentlyAdded(ctx, libraries)
		if err != nil {
			return err
		}
		_ = c.store.PutRecentlyAdded(fetched, ttlRecentlyAdded)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetRecentlyAdded()
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key recently-added")
	}
	return cached, nil
}

// GetLibraryItems returns a paginated list of library items from cache or fetches from the API.
func (c *Client) GetLibraryItems(ctx context.Context, libraryID string, page, limit int) (*abs.LibraryItemsResponse, error) {
	if c.store == nil {
		return c.Client.GetLibraryItems(ctx, libraryID, page, limit)
	}
	if items, total, hit, _ := c.store.GetLibraryItems(libraryID, page, limit); hit {
		return &abs.LibraryItemsResponse{
			Results: items,
			Total:   total,
			Limit:   limit,
			Page:    page,
		}, nil
	}

	err := c.getOrFetch(ctx, fmt.Sprintf("items:%s:%d:%d", libraryID, limit, page), func() error {
		resp, err := c.Client.GetLibraryItems(ctx, libraryID, page, limit)
		if err != nil {
			return err
		}
		_ = c.store.PutLibraryItems(libraryID, page, limit, resp.Results, resp.Total, ttlLibraryItems)
		return nil
	})
	if err != nil {
		return nil, err
	}

	items, total, hit, err := c.store.GetLibraryItems(libraryID, page, limit)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key items:%s:%d:%d", libraryID, limit, page)
	}
	return &abs.LibraryItemsResponse{
		Results: items,
		Total:   total,
		Limit:   limit,
		Page:    page,
	}, nil
}

// GetLibrarySeries returns a paginated list of series from cache or fetches from the API.
func (c *Client) GetLibrarySeries(ctx context.Context, libraryID string, page, limit int) (*abs.LibrarySeriesResponse, error) {
	if c.store == nil {
		return c.Client.GetLibrarySeries(ctx, libraryID, page, limit)
	}
	if items, total, hit, _ := c.store.GetLibrarySeries(libraryID, page, limit); hit {
		return &abs.LibrarySeriesResponse{
			Results: items,
			Total:   total,
			Limit:   limit,
			Page:    page,
		}, nil
	}

	err := c.getOrFetch(ctx, fmt.Sprintf("series:%s:%d:%d", libraryID, limit, page), func() error {
		resp, err := c.Client.GetLibrarySeries(ctx, libraryID, page, limit)
		if err != nil {
			return err
		}
		_ = c.store.PutLibrarySeries(libraryID, page, limit, resp.Results, resp.Total, ttlLibrarySeries)
		return nil
	})
	if err != nil {
		return nil, err
	}

	items, total, hit, err := c.store.GetLibrarySeries(libraryID, page, limit)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key series:%s:%d:%d", libraryID, limit, page)
	}
	return &abs.LibrarySeriesResponse{
		Results: items,
		Total:   total,
		Limit:   limit,
		Page:    page,
	}, nil
}

// GetLibraryItem returns a single library item from cache or fetches from the API.
func (c *Client) GetLibraryItem(ctx context.Context, itemID string) (*abs.LibraryItem, error) {
	if c.store == nil {
		return c.Client.GetLibraryItem(ctx, itemID)
	}
	if cached, hit, _ := c.store.GetLibraryItem(itemID); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "item:"+itemID, func() error {
		fetched, err := c.Client.GetLibraryItem(ctx, itemID)
		if err != nil {
			return err
		}
		_ = c.store.PutLibraryItem(itemID, fetched, ttlLibraryItem)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetLibraryItem(itemID)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key item:%s", itemID)
	}
	return cached, nil
}

// GetSeries returns series metadata from cache or fetches from the API.
func (c *Client) GetSeries(ctx context.Context, libraryID, seriesID string) (*abs.Series, error) {
	if c.store == nil {
		return c.Client.GetSeries(ctx, libraryID, seriesID)
	}
	if cached, hit, _ := c.store.GetSeries(seriesID); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "series-meta:"+seriesID, func() error {
		fetched, err := c.Client.GetSeries(ctx, libraryID, seriesID)
		if err != nil {
			return err
		}
		_ = c.store.PutSeries(seriesID, fetched, ttlSeriesContents)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetSeries(seriesID)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key series-meta:%s", seriesID)
	}
	return cached, nil
}

// GetSeriesContents returns series contents from cache or fetches from the API.
func (c *Client) GetSeriesContents(ctx context.Context, libraryID, seriesID string) (*abs.SeriesContents, error) {
	if c.store == nil {
		return c.Client.GetSeriesContents(ctx, libraryID, seriesID)
	}
	if items, hit, _ := c.store.GetSeriesContents(seriesID); hit {
		series, err := c.GetSeries(ctx, libraryID, seriesID)
		if err != nil {
			return nil, err
		}
		return &abs.SeriesContents{
			Series: *series,
			Items:  items,
		}, nil
	}

	err := c.getOrFetch(ctx, "series-contents:"+seriesID, func() error {
		resp, err := c.Client.GetSeriesContents(ctx, libraryID, seriesID)
		if err != nil {
			return err
		}
		_ = c.store.PutSeriesContents(seriesID, resp.Items, ttlSeriesContents)
		_ = c.store.PutSeries(seriesID, &resp.Series, ttlSeriesContents)
		return nil
	})
	if err != nil {
		return nil, err
	}

	items, hit, err := c.store.GetSeriesContents(seriesID)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key series-contents:%s", seriesID)
	}
	series, err := c.GetSeries(ctx, libraryID, seriesID)
	if err != nil {
		return nil, err
	}
	return &abs.SeriesContents{
		Series: *series,
		Items:  items,
	}, nil
}

// FilterAudioLibraries filters audio libraries using cache or fetches from the API.
func (c *Client) FilterAudioLibraries(ctx context.Context, libs []abs.Library) ([]abs.Library, error) {
	if c.store == nil {
		return c.Client.FilterAudioLibraries(ctx, libs)
	}
	if cached, hit, _ := c.store.GetFilteredLibraries(); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "filtered-libraries", func() error {
		fetched, err := c.Client.FilterAudioLibraries(ctx, libs)
		if err != nil {
			return err
		}
		_ = c.store.PutFilteredLibraries(fetched, ttlLibraries)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetFilteredLibraries()
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key filtered-libraries")
	}
	return cached, nil
}

// GetMediaProgress returns media progress from cache or fetches from the API.
func (c *Client) GetMediaProgress(ctx context.Context, itemID string) (*abs.MediaProgressWithBookmarks, error) {
	if c.store == nil {
		return c.Client.GetMediaProgress(ctx, itemID)
	}
	if cached, hit, _ := c.store.GetMediaProgress(itemID); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "progress:"+itemID, func() error {
		fetched, err := c.Client.GetMediaProgress(ctx, itemID)
		if err != nil {
			return err
		}
		_ = c.store.PutMediaProgress(itemID, fetched, ttlMediaProgress)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetMediaProgress(itemID)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key progress:%s", itemID)
	}
	return cached, nil
}

// GetBookmarks returns bookmarks from cache or fetches from the API.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]abs.Bookmark, error) {
	if c.store == nil {
		return c.Client.GetBookmarks(ctx, itemID)
	}
	if cached, hit, _ := c.store.GetBookmarks(itemID); hit {
		return cached, nil
	}

	err := c.getOrFetch(ctx, "bookmarks:"+itemID, func() error {
		fetched, err := c.Client.GetBookmarks(ctx, itemID)
		if err != nil {
			return err
		}
		_ = c.store.PutBookmarks(itemID, fetched, ttlBookmarks)
		return nil
	})
	if err != nil {
		return nil, err
	}

	cached, hit, err := c.store.GetBookmarks(itemID)
	if err != nil {
		return nil, err
	}
	if !hit {
		return nil, fmt.Errorf("unexpected cache miss after fetch for key bookmarks:%s", itemID)
	}
	return cached, nil
}

// DeleteItem deletes a library item from the API and invalidates related caches.
func (c *Client) DeleteItem(ctx context.Context, itemID string, hardDelete bool) error {
	err := c.Client.DeleteItem(ctx, itemID, hardDelete)
	if err != nil {
		return err
	}
	if c.store != nil {
		_ = c.store.Delete("item:" + itemID)
		// Ideally we would invalidate items, recently-added, etc.
		// But for now, we just wipe the entire cache to be safe since delete is rare.
		_, _ = c.store.db.DB.Exec(`DELETE FROM api_cache`)
	}
	return nil
}

// DeleteEpisode deletes a podcast episode from the API and invalidates related caches.
func (c *Client) DeleteEpisode(ctx context.Context, podcastID string, episodeID string, hardDelete bool) error {
	err := c.Client.DeleteEpisode(ctx, podcastID, episodeID, hardDelete)
	if err != nil {
		return err
	}
	if c.store != nil {
		_ = c.store.Delete("item:" + podcastID)
		// Wipe entire cache to be safe since delete is rare.
		_, _ = c.store.db.DB.Exec(`DELETE FROM api_cache`)
	}
	return nil
}

// UpdateLibraryItemMedia updates media metadata and invalidates caches.
func (c *Client) UpdateLibraryItemMedia(ctx context.Context, itemID string, req abs.UpdateMediaRequest) (*abs.LibraryItem, error) {
	updated, err := c.Client.UpdateLibraryItemMedia(ctx, itemID, req)
	if err != nil {
		return nil, err
	}
	if c.store != nil {
		_, _ = c.store.db.DB.Exec(`DELETE FROM api_cache`)
	}
	return updated, nil
}

// UpdatePodcastEpisode updates episode metadata and invalidates caches.
func (c *Client) UpdatePodcastEpisode(ctx context.Context, itemID, episodeID string, req abs.UpdatePodcastEpisodeRequest) (*abs.LibraryItem, error) {
	updated, err := c.Client.UpdatePodcastEpisode(ctx, itemID, episodeID, req)
	if err != nil {
		return nil, err
	}
	if c.store != nil {
		_, _ = c.store.db.DB.Exec(`DELETE FROM api_cache`)
	}
	return updated, nil
}
