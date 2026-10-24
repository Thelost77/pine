package cache

import (
	"context"
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
	ttlBookmarks      = 1 * time.Minute
	ttlRecentEpisodes = 5 * time.Minute
	ttlLibraryItem    = 15 * time.Minute
)

// Client is a transparent caching wrapper around abs.Client.
type Client struct {
	*abs.Client
	store        *Store
	OnInvalidate func()

	// generation counts the calls of invalidate. cached does not store the
	// value of a fetch that started in an earlier generation, because the
	// value can be from before the change. invalidate holds mu while it
	// clears the store, and cached holds it while it checks the generation
	// and stores the value, so no old value comes after the clear.
	mu         sync.RWMutex
	generation uint64
}

// NewClient creates a caching wrapper around the given ABS client.
func NewClient(inner *abs.Client, store *Store) *Client {
	if inner == nil {
		panic("cache.NewClient: inner client is nil")
	}
	return &Client{
		Client: inner,
		store:  store,
	}
}

// cached returns the cached value of key, or calls fetch and caches its
// result for ttl. The cache only makes a request faster: a read or a write
// error of the cache goes to the log, and the caller gets the value of the
// server.
func cached[T any](c *Client, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	if c.store == nil {
		return fetch()
	}
	var v T
	if hit, err := c.store.Get(key, &v); err != nil {
		logger.Warn("cache read failed", "key", key, "err", err)
	} else if hit {
		return v, nil
	}

	c.mu.RLock()
	generation := c.generation
	c.mu.RUnlock()
	v, err := fetch()
	if err != nil {
		return v, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.generation != generation {
		return v, nil
	}
	if err := c.store.Put(key, v, ttl); err != nil {
		logger.Warn("cache write failed", "key", key, "err", err)
	}
	return v, nil
}

// GetLibraries returns libraries from cache or fetches from the API.
func (c *Client) GetLibraries(ctx context.Context) ([]abs.Library, error) {
	return cached(c, librariesKey, ttlLibraries, func() ([]abs.Library, error) {
		return c.Client.GetLibraries(ctx)
	})
}

// GetPersonalized returns personalized shelves from cache or fetches from the API.
func (c *Client) GetPersonalized(ctx context.Context, libraryID string) ([]abs.PersonalizedResponse, error) {
	return cached(c, PersonalizedKey(libraryID), ttlPersonalized, func() ([]abs.PersonalizedResponse, error) {
		return c.Client.GetPersonalized(ctx, libraryID)
	})
}

// GetRecentEpisodes returns recent podcast episodes from cache or fetches from the API.
func (c *Client) GetRecentEpisodes(ctx context.Context, libraryID string, limit int) ([]abs.LibraryItem, error) {
	return cached(c, recentEpisodesKey(libraryID, limit), ttlRecentEpisodes, func() ([]abs.LibraryItem, error) {
		return c.Client.GetRecentEpisodes(ctx, libraryID, limit)
	})
}

// GetLibraryItems returns a paginated list of library items from cache or
// fetches from the API. The response has the requested page and limit.
func (c *Client) GetLibraryItems(ctx context.Context, libraryID string, page, limit int) (*abs.LibraryItemsResponse, error) {
	return cached(c, libraryItemsKey(libraryID, page, limit), ttlLibraryItems, func() (*abs.LibraryItemsResponse, error) {
		resp, err := c.Client.GetLibraryItems(ctx, libraryID, page, limit)
		if err != nil {
			return nil, err
		}
		resp.Page, resp.Limit = page, limit
		return resp, nil
	})
}

// GetLibrarySeries returns a paginated list of series from cache or fetches
// from the API. The response has the requested page and limit.
func (c *Client) GetLibrarySeries(ctx context.Context, libraryID string, page, limit int) (*abs.LibrarySeriesResponse, error) {
	return cached(c, librarySeriesKey(libraryID, page, limit), ttlLibrarySeries, func() (*abs.LibrarySeriesResponse, error) {
		resp, err := c.Client.GetLibrarySeries(ctx, libraryID, page, limit)
		if err != nil {
			return nil, err
		}
		resp.Page, resp.Limit = page, limit
		return resp, nil
	})
}

// GetLibraryItem returns a single library item from cache or fetches from the API.
func (c *Client) GetLibraryItem(ctx context.Context, itemID string) (*abs.LibraryItem, error) {
	return cached(c, ItemKey(itemID), ttlLibraryItem, func() (*abs.LibraryItem, error) {
		return c.Client.GetLibraryItem(ctx, itemID)
	})
}

// GetSeriesContents returns series contents from cache or fetches from the API.
func (c *Client) GetSeriesContents(ctx context.Context, libraryID, seriesID string) (*abs.SeriesContents, error) {
	return cached(c, SeriesContentsKey(seriesID), ttlSeriesContents, func() (*abs.SeriesContents, error) {
		return c.Client.GetSeriesContents(ctx, libraryID, seriesID)
	})
}

// FilterAudioLibraries filters audio libraries using cache or fetches from the API.
func (c *Client) FilterAudioLibraries(ctx context.Context, libs []abs.Library) []abs.Library {
	v, _ := cached(c, filteredLibrariesKey, ttlLibraries, func() ([]abs.Library, error) {
		return c.Client.FilterAudioLibraries(ctx, libs), nil
	})
	return v
}

// GetBookmarks returns bookmarks from cache or fetches from the API.
func (c *Client) GetBookmarks(ctx context.Context, itemID string) ([]abs.Bookmark, error) {
	return cached(c, BookmarksKey(itemID), ttlBookmarks, func() ([]abs.Bookmark, error) {
		return c.Client.GetBookmarks(ctx, itemID)
	})
}

// invalidate wipes the persistent cache after a change on the server, and
// tells the in-memory caches (the search cache) to drop their snapshots. A
// change is rare, so it clears each key.
func (c *Client) invalidate() {
	if c.store != nil {
		c.mu.Lock()
		c.generation++
		if err := c.store.ClearAll(); err != nil {
			logger.Warn("cache clear failed", "err", err)
		}
		c.mu.Unlock()
	}
	if c.OnInvalidate != nil {
		c.OnInvalidate()
	}
}

// DeleteItem deletes a library item from the API and invalidates the caches.
func (c *Client) DeleteItem(ctx context.Context, itemID string, hardDelete bool) error {
	if err := c.Client.DeleteItem(ctx, itemID, hardDelete); err != nil {
		return err
	}
	c.invalidate()
	return nil
}

// DeleteEpisode deletes a podcast episode from the API and invalidates the caches.
func (c *Client) DeleteEpisode(ctx context.Context, podcastID string, episodeID string, hardDelete bool) error {
	if err := c.Client.DeleteEpisode(ctx, podcastID, episodeID, hardDelete); err != nil {
		return err
	}
	c.invalidate()
	return nil
}

// UpdateLibraryItemMedia updates media metadata and invalidates the caches.
func (c *Client) UpdateLibraryItemMedia(ctx context.Context, itemID string, req abs.UpdateMediaRequest) (*abs.LibraryItem, error) {
	updated, err := c.Client.UpdateLibraryItemMedia(ctx, itemID, req)
	if err != nil {
		return nil, err
	}
	c.invalidate()
	return updated, nil
}

// UpdatePodcastEpisode updates episode metadata and invalidates the caches.
func (c *Client) UpdatePodcastEpisode(ctx context.Context, itemID, episodeID string, req abs.UpdatePodcastEpisodeRequest) (*abs.LibraryItem, error) {
	updated, err := c.Client.UpdatePodcastEpisode(ctx, itemID, episodeID, req)
	if err != nil {
		return nil, err
	}
	c.invalidate()
	return updated, nil
}
