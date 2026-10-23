package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/logger"
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
	OnInvalidate  func()
}

type inflightCall struct {
	done chan struct{}
	val  any
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

// ClearAll wipes the entire persistent cache and notifies registered
// in-memory caches (e.g. the search cache) to drop their snapshots.
func (c *Client) ClearAll() error {
	if c.store == nil {
		if c.OnInvalidate != nil {
			c.OnInvalidate()
		}
		return nil
	}
	if err := c.store.ClearAll(); err != nil {
		return err
	}
	if c.OnInvalidate != nil {
		c.OnInvalidate()
	}
	return nil
}

func (c *Client) getOrFetch(ctx context.Context, key string, fetch func() (any, error)) (any, error) {
	c.inflightMutex.Lock()
	if call, ok := c.inflight[key]; ok {
		c.inflightMutex.Unlock()
		select {
		case <-call.done:
			if call.err != nil &&
				(errors.Is(call.err, context.Canceled) || errors.Is(call.err, context.DeadlineExceeded)) &&
				ctx.Err() == nil {
				return c.getOrFetch(ctx, key, fetch)
			}
			return call.val, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
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

	call.val, call.err = fetch()
	return call.val, call.err
}

// fetchCached returns the cached value of key, or fetches the value and
// writes it to the cache. The cache only makes a request faster: a read or a
// write error of the cache goes to the log, and the caller gets the value of
// the server.
func fetchCached[T any](c *Client, ctx context.Context, key string, get func() (T, bool, error), put func(T) error, fetch func() (T, error)) (T, error) {
	cached, hit, err := get()
	if err != nil {
		logger.Warn("cache read failed", "key", key, "err", err)
	} else if hit {
		return cached, nil
	}

	v, err := c.getOrFetch(ctx, key, func() (any, error) {
		fetched, err := fetch()
		if err != nil {
			return nil, err
		}
		if err := put(fetched); err != nil {
			logger.Warn("cache write failed", "key", key, "err", err)
		}
		return fetched, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}

// GetLibraries returns libraries from cache or fetches from the API.
func (c *Client) GetLibraries(ctx context.Context) ([]abs.Library, error) {
	if c.store == nil {
		return c.Client.GetLibraries(ctx)
	}
	return fetchCached(c, ctx, "libraries",
		c.store.GetLibraries,
		func(v []abs.Library) error { return c.store.PutLibraries(v, ttlLibraries) },
		func() ([]abs.Library, error) { return c.Client.GetLibraries(ctx) },
	)
}

// GetPersonalized returns personalized shelves from cache or fetches from the API.
func (c *Client) GetPersonalized(ctx context.Context, libraryID string) ([]abs.PersonalizedResponse, error) {
	if c.store == nil {
		return c.Client.GetPersonalized(ctx, libraryID)
	}
	return fetchCached(c, ctx, "personalized:"+libraryID,
		func() ([]abs.PersonalizedResponse, bool, error) { return c.store.GetPersonalized(libraryID) },
		func(v []abs.PersonalizedResponse) error {
			return c.store.PutPersonalized(libraryID, v, ttlPersonalized)
		},
		func() ([]abs.PersonalizedResponse, error) { return c.Client.GetPersonalized(ctx, libraryID) },
	)
}

// GetRecentEpisodes returns recent podcast episodes from cache or fetches from the API.
func (c *Client) GetRecentEpisodes(ctx context.Context, libraryID string, limit int) ([]abs.LibraryItem, error) {
	if c.store == nil {
		return c.Client.GetRecentEpisodes(ctx, libraryID, limit)
	}
	return fetchCached(c, ctx, "recent-episodes:"+libraryID+":"+strconv.Itoa(limit),
		func() ([]abs.LibraryItem, bool, error) { return c.store.GetRecentEpisodes(libraryID, limit) },
		func(v []abs.LibraryItem) error {
			return c.store.PutRecentEpisodes(libraryID, limit, v, ttlRecentEpisodes)
		},
		func() ([]abs.LibraryItem, error) { return c.Client.GetRecentEpisodes(ctx, libraryID, limit) },
	)
}

// GetRecentlyAdded returns the merged recently-added shelf from cache or fetches from the API.
func (c *Client) GetRecentlyAdded(ctx context.Context, libraries []abs.Library) ([]abs.LibraryItem, error) {
	if c.store == nil {
		return c.Client.GetRecentlyAdded(ctx, libraries)
	}
	return fetchCached(c, ctx, "recently-added",
		c.store.GetRecentlyAdded,
		func(v []abs.LibraryItem) error { return c.store.PutRecentlyAdded(v, ttlRecentlyAdded) },
		func() ([]abs.LibraryItem, error) { return c.Client.GetRecentlyAdded(ctx, libraries) },
	)
}

// GetLibraryItems returns a paginated list of library items from cache or fetches from the API.
func (c *Client) GetLibraryItems(ctx context.Context, libraryID string, page, limit int) (*abs.LibraryItemsResponse, error) {
	if c.store == nil {
		return c.Client.GetLibraryItems(ctx, libraryID, page, limit)
	}
	return fetchCached(c, ctx, fmt.Sprintf("items:%s:%d:%d", libraryID, limit, page),
		func() (*abs.LibraryItemsResponse, bool, error) {
			items, total, hit, err := c.store.GetLibraryItems(libraryID, page, limit)
			return &abs.LibraryItemsResponse{Results: items, Total: total, Limit: limit, Page: page}, hit, err
		},
		func(v *abs.LibraryItemsResponse) error {
			return c.store.PutLibraryItems(libraryID, page, limit, v.Results, v.Total, ttlLibraryItems)
		},
		func() (*abs.LibraryItemsResponse, error) {
			resp, err := c.Client.GetLibraryItems(ctx, libraryID, page, limit)
			if err != nil {
				return nil, err
			}
			return &abs.LibraryItemsResponse{Results: resp.Results, Total: resp.Total, Limit: limit, Page: page}, nil
		},
	)
}

// GetLibrarySeries returns a paginated list of series from cache or fetches from the API.
func (c *Client) GetLibrarySeries(ctx context.Context, libraryID string, page, limit int) (*abs.LibrarySeriesResponse, error) {
	if c.store == nil {
		return c.Client.GetLibrarySeries(ctx, libraryID, page, limit)
	}
	return fetchCached(c, ctx, fmt.Sprintf("series:%s:%d:%d", libraryID, limit, page),
		func() (*abs.LibrarySeriesResponse, bool, error) {
			items, total, hit, err := c.store.GetLibrarySeries(libraryID, page, limit)
			return &abs.LibrarySeriesResponse{Results: items, Total: total, Limit: limit, Page: page}, hit, err
		},
		func(v *abs.LibrarySeriesResponse) error {
			return c.store.PutLibrarySeries(libraryID, page, limit, v.Results, v.Total, ttlLibrarySeries)
		},
		func() (*abs.LibrarySeriesResponse, error) {
			resp, err := c.Client.GetLibrarySeries(ctx, libraryID, page, limit)
			if err != nil {
				return nil, err
			}
			return &abs.LibrarySeriesResponse{Results: resp.Results, Total: resp.Total, Limit: limit, Page: page}, nil
		},
	)
}

// GetLibraryItem returns a single library item from cache or fetches from the API.
func (c *Client) GetLibraryItem(ctx context.Context, itemID string) (*abs.LibraryItem, error) {
	if c.store == nil {
		return c.Client.GetLibraryItem(ctx, itemID)
	}
	return fetchCached(c, ctx, "item:"+itemID,
		func() (*abs.LibraryItem, bool, error) { return c.store.GetLibraryItem(itemID) },
		func(v *abs.LibraryItem) error { return c.store.PutLibraryItem(itemID, v, ttlLibraryItem) },
		func() (*abs.LibraryItem, error) { return c.Client.GetLibraryItem(ctx, itemID) },
	)
}

// GetSeries returns series metadata from cache or fetches from the API.
func (c *Client) GetSeries(ctx context.Context, libraryID, seriesID string) (*abs.Series, error) {
	if c.store == nil {
		return c.Client.GetSeries(ctx, libraryID, seriesID)
	}
	return fetchCached(c, ctx, "series-meta:"+seriesID,
		func() (*abs.Series, bool, error) { return c.store.GetSeries(seriesID) },
		func(v *abs.Series) error { return c.store.PutSeries(seriesID, v, ttlSeriesContents) },
		func() (*abs.Series, error) { return c.Client.GetSeries(ctx, libraryID, seriesID) },
	)
}

// GetSeriesContents returns series contents from cache or fetches from the API.
// The cache keeps the items and the series metadata under two keys.
func (c *Client) GetSeriesContents(ctx context.Context, libraryID, seriesID string) (*abs.SeriesContents, error) {
	if c.store == nil {
		return c.Client.GetSeriesContents(ctx, libraryID, seriesID)
	}
	key := "series-contents:" + seriesID
	items, hit, err := c.store.GetSeriesContents(seriesID)
	if err != nil {
		logger.Warn("cache read failed", "key", key, "err", err)
	} else if hit {
		series, err := c.GetSeries(ctx, libraryID, seriesID)
		if err != nil {
			return nil, err
		}
		return &abs.SeriesContents{
			Series: *series,
			Items:  items,
		}, nil
	}

	v, err := c.getOrFetch(ctx, key, func() (any, error) {
		resp, err := c.Client.GetSeriesContents(ctx, libraryID, seriesID)
		if err != nil || resp == nil {
			return resp, err
		}
		if err := c.store.PutSeriesContents(seriesID, resp.Items, ttlSeriesContents); err != nil {
			logger.Warn("cache write failed", "key", key, "err", err)
		}
		if err := c.store.PutSeries(seriesID, &resp.Series, ttlSeriesContents); err != nil {
			logger.Warn("cache write failed", "key", "series-meta:"+seriesID, "err", err)
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*abs.SeriesContents), nil
}

// FilterAudioLibraries filters audio libraries using cache or fetches from the API.
func (c *Client) FilterAudioLibraries(ctx context.Context, libs []abs.Library) ([]abs.Library, error) {
	if c.store == nil {
		return c.Client.FilterAudioLibraries(ctx, libs)
	}
	return fetchCached(c, ctx, "filtered-libraries",
		c.store.GetFilteredLibraries,
		func(v []abs.Library) error { return c.store.PutFilteredLibraries(v, ttlLibraries) },
		func() ([]abs.Library, error) { return c.Client.FilterAudioLibraries(ctx, libs) },
	)
}

// GetMediaProgress returns media progress from cache or fetches from the API.
func (c *Client) GetMediaProgress(ctx context.Context, itemID string) (*abs.MediaProgressWithBookmarks, error) {
	if c.store == nil {
		return c.Client.GetMediaProgress(ctx, itemID)
	}
	return fetchCached(c, ctx, "progress:"+itemID,
		func() (*abs.MediaProgressWithBookmarks, bool, error) { return c.store.GetMediaProgress(itemID) },
		func(v *abs.MediaProgressWithBookmarks) error {
			return c.store.PutMediaProgress(itemID, v, ttlMediaProgress)
		},
		func() (*abs.MediaProgressWithBookmarks, error) { return c.Client.GetMediaProgress(ctx, itemID) },
	)
}

// GetBookmarks returns bookmarks from cache or fetches from the API.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]abs.Bookmark, error) {
	if c.store == nil {
		return c.Client.GetBookmarks(ctx, itemID)
	}
	return fetchCached(c, ctx, "bookmarks:"+itemID,
		func() ([]abs.Bookmark, bool, error) { return c.store.GetBookmarks(itemID) },
		func(v []abs.Bookmark) error { return c.store.PutBookmarks(itemID, v, ttlBookmarks) },
		func() ([]abs.Bookmark, error) { return c.Client.GetBookmarks(ctx, itemID) },
	)
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
		if c.OnInvalidate != nil {
			c.OnInvalidate()
		}
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
		if c.OnInvalidate != nil {
			c.OnInvalidate()
		}
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
		if c.OnInvalidate != nil {
			c.OnInvalidate()
		}
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
		if c.OnInvalidate != nil {
			c.OnInvalidate()
		}
	}
	return updated, nil
}
