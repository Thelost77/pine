package cache

import (
	"fmt"
	"strconv"
)

// The keys of the cached values. The store puts keyVersion before each key.
const (
	librariesKey         = "libraries"
	filteredLibrariesKey = "filtered-libraries"
)

// ItemKey is the key of a library item.
func ItemKey(itemID string) string { return "item:" + itemID }

// PersonalizedKey is the key of the home shelves of a library.
func PersonalizedKey(libraryID string) string { return "personalized:" + libraryID }

// SeriesContentsKey is the key of the contents of a series.
func SeriesContentsKey(seriesID string) string { return "series-contents:" + seriesID }

// BookmarksKey is the key of the bookmarks of a library item.
func BookmarksKey(itemID string) string { return "bookmarks:" + itemID }

// SearchSnapshotKey is the key of the search snapshot of a library, which the
// search cache writes.
func SearchSnapshotKey(libraryID string) string { return "search-snapshot:" + libraryID }

func recentEpisodesKey(libraryID string, limit int) string {
	return "recent-episodes:" + libraryID + ":" + strconv.Itoa(limit)
}

func libraryItemsKey(libraryID string, page, limit int) string {
	return fmt.Sprintf("items:%s:%d:%d", libraryID, limit, page)
}

func librarySeriesKey(libraryID string, page, limit int) string {
	return fmt.Sprintf("series:%s:%d:%d", libraryID, limit, page)
}
