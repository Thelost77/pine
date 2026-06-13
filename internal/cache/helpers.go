package cache

import (
	"encoding/gob"
	"fmt"
	"strconv"
	"time"

	"github.com/Thelost77/pine/internal/abs"
)

func init() {
	// Register concrete types that will be stored so gob can encode/decode them.
	gob.Register([]abs.Library{})
	gob.Register([]abs.PersonalizedResponse{})
	gob.Register([]abs.LibraryItem{})
	gob.Register(&abs.LibraryItem{})
	gob.Register([]abs.Series{})
	gob.Register(&abs.Series{})
	gob.Register([]abs.PodcastEpisode{})
	gob.Register([]abs.Bookmark{})
	gob.Register(&abs.MediaProgressWithBookmarks{})
	gob.Register(paginatedItems{})
	gob.Register(paginatedSeries{})
}

type paginatedItems struct {
	Items []abs.LibraryItem
	Total int
}

type paginatedSeries struct {
	Items []abs.Series
	Total int
}

// Libraries -------------------------------------------------------------------

func (s *Store) GetLibraries() ([]abs.Library, bool, error) {
	var libraries []abs.Library
	hit, err := s.Get("libraries", &libraries)
	return libraries, hit, err
}

func (s *Store) PutLibraries(v []abs.Library, ttl time.Duration) error {
	return s.Put("libraries", v, ttl)
}

// Personalized ----------------------------------------------------------------

func (s *Store) GetPersonalized(libID string) ([]abs.PersonalizedResponse, bool, error) {
	var v []abs.PersonalizedResponse
	hit, err := s.Get("personalized:"+libID, &v)
	return v, hit, err
}

func (s *Store) PutPersonalized(libID string, v []abs.PersonalizedResponse, ttl time.Duration) error {
	return s.Put("personalized:"+libID, v, ttl)
}

// LibraryItems ---------------------------------------------------------------

func (s *Store) GetLibraryItems(libID string, page, limit int) ([]abs.LibraryItem, int, bool, error) {
	var v paginatedItems
	hit, err := s.Get(fmt.Sprintf("items:%s:%d:%d", libID, limit, page), &v)
	return v.Items, v.Total, hit, err
}

func (s *Store) PutLibraryItems(libID string, page, limit int, items []abs.LibraryItem, total int, ttl time.Duration) error {
	return s.Put(fmt.Sprintf("items:%s:%d:%d", libID, limit, page), paginatedItems{Items: items, Total: total}, ttl)
}

// LibrarySeries --------------------------------------------------------------

func (s *Store) GetLibrarySeries(libID string, page, limit int) ([]abs.Series, int, bool, error) {
	var v paginatedSeries
	hit, err := s.Get(fmt.Sprintf("series:%s:%d:%d", libID, limit, page), &v)
	return v.Items, v.Total, hit, err
}

func (s *Store) PutLibrarySeries(libID string, page, limit int, items []abs.Series, total int, ttl time.Duration) error {
	return s.Put(fmt.Sprintf("series:%s:%d:%d", libID, limit, page), paginatedSeries{Items: items, Total: total}, ttl)
}

// SeriesContents -------------------------------------------------------------

func (s *Store) GetSeriesContents(seriesID string) ([]abs.LibraryItem, bool, error) {
	var v []abs.LibraryItem
	hit, err := s.Get("series-contents:"+seriesID, &v)
	return v, hit, err
}

func (s *Store) PutSeriesContents(seriesID string, v []abs.LibraryItem, ttl time.Duration) error {
	return s.Put("series-contents:"+seriesID, v, ttl)
}

// MediaProgress --------------------------------------------------------------

func (s *Store) GetMediaProgress(itemID string) (*abs.MediaProgressWithBookmarks, bool, error) {
	var v abs.MediaProgressWithBookmarks
	hit, err := s.Get("progress:"+itemID, &v)
	if err != nil {
		return nil, false, err
	}
	if !hit {
		return nil, false, nil
	}
	return &v, true, nil
}

func (s *Store) PutMediaProgress(itemID string, v *abs.MediaProgressWithBookmarks, ttl time.Duration) error {
	return s.Put("progress:"+itemID, v, ttl)
}

// Bookmarks ------------------------------------------------------------------

func (s *Store) GetBookmarks(itemID string) ([]abs.Bookmark, bool, error) {
	var v []abs.Bookmark
	hit, err := s.Get("bookmarks:"+itemID, &v)
	return v, hit, err
}

func (s *Store) PutBookmarks(itemID string, v []abs.Bookmark, ttl time.Duration) error {
	return s.Put("bookmarks:"+itemID, v, ttl)
}

// Episodes -------------------------------------------------------------------

func (s *Store) GetEpisodes(itemID string) ([]abs.PodcastEpisode, bool, error) {
	var v []abs.PodcastEpisode
	hit, err := s.Get("episodes:"+itemID, &v)
	return v, hit, err
}

func (s *Store) PutEpisodes(itemID string, v []abs.PodcastEpisode, ttl time.Duration) error {
	return s.Put("episodes:"+itemID, v, ttl)
}

// RecentEpisodes -------------------------------------------------------------

func (s *Store) GetRecentEpisodes(libID string, limit int) ([]abs.LibraryItem, bool, error) {
	var v []abs.LibraryItem
	hit, err := s.Get("recent-episodes:"+libID+":"+strconv.Itoa(limit), &v)
	return v, hit, err
}

func (s *Store) PutRecentEpisodes(libID string, limit int, v []abs.LibraryItem, ttl time.Duration) error {
	return s.Put("recent-episodes:"+libID+":"+strconv.Itoa(limit), v, ttl)
}

// LibraryItem ----------------------------------------------------------------

func (s *Store) GetLibraryItem(itemID string) (*abs.LibraryItem, bool, error) {
	var v abs.LibraryItem
	hit, err := s.Get("item:"+itemID, &v)
	if err != nil {
		return nil, false, err
	}
	if !hit {
		return nil, false, nil
	}
	return &v, true, nil
}

func (s *Store) PutLibraryItem(itemID string, v *abs.LibraryItem, ttl time.Duration) error {
	return s.Put("item:"+itemID, v, ttl)
}

// RecentlyAdded --------------------------------------------------------------

func (s *Store) GetRecentlyAdded() ([]abs.LibraryItem, bool, error) {
	var v []abs.LibraryItem
	hit, err := s.Get("recently-added", &v)
	return v, hit, err
}

func (s *Store) PutRecentlyAdded(v []abs.LibraryItem, ttl time.Duration) error {
	return s.Put("recently-added", v, ttl)
}

// Series ---------------------------------------------------------------------

func (s *Store) GetSeries(seriesID string) (*abs.Series, bool, error) {
	var v abs.Series
	hit, err := s.Get("series-meta:"+seriesID, &v)
	if err != nil {
		return nil, false, err
	}
	if !hit {
		return nil, false, nil
	}
	return &v, true, nil
}

func (s *Store) PutSeries(seriesID string, v *abs.Series, ttl time.Duration) error {
	return s.Put("series-meta:"+seriesID, v, ttl)
}

// FilteredLibraries ----------------------------------------------------------

func (s *Store) GetFilteredLibraries() ([]abs.Library, bool, error) {
	var v []abs.Library
	hit, err := s.Get("filtered-libraries", &v)
	return v, hit, err
}

func (s *Store) PutFilteredLibraries(v []abs.Library, ttl time.Duration) error {
	return s.Put("filtered-libraries", v, ttl)
}

var _ = fmt.Sprintf // silence unused import if any
