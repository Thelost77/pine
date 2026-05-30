package abs

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
)

// LoginResponse is returned by POST /login.
type LoginResponse struct {
	User LoginUser `json:"user"`
}

// LoginUser contains user info including the auth token.
type LoginUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

// Library represents an Audiobookshelf library.
type Library struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"` // "book" or "podcast"
}

// LibraryItem represents a book or podcast in a library.
type LibraryItem struct {
	ID                string             `json:"id"`
	LibraryID         string             `json:"libraryId,omitempty"`
	AddedAt           int64              `json:"addedAt,omitempty"`
	MediaType         string             `json:"mediaType"` // "book" or "podcast"
	Media             Media              `json:"media"`
	UserMediaProgress *UserMediaProgress `json:"userMediaProgress,omitempty"`
	RecentEpisode     *PodcastEpisode    `json:"recentEpisode,omitempty"`
}

// SortRecentlyAdded sorts library items by descending addedAt and then title.
func SortRecentlyAdded(items []LibraryItem) {
	slices.SortFunc(items, func(a, b LibraryItem) int {
		if byAddedAt := cmp.Compare(b.AddedAt, a.AddedAt); byAddedAt != 0 {
			return byAddedAt
		}
		return cmp.Compare(a.Media.Metadata.Title, b.Media.Metadata.Title)
	})
}

// Media contains the media content and metadata of a library item.
type Media struct {
	Metadata      MediaMetadata    `json:"metadata"`
	Duration      *float64         `json:"duration,omitempty"`
	CoverPath     *string          `json:"coverPath,omitempty"`
	Episodes      []PodcastEpisode `json:"episodes,omitempty"`
	NumAudioFiles *int             `json:"numAudioFiles,omitempty"`
	NumTracks     *int             `json:"numTracks,omitempty"`
}

// TotalDuration returns the item duration, checking media.duration first
// then falling back to media.metadata.duration. Returns 0 if neither is set.
func (m Media) TotalDuration() float64 {
	if m.Duration != nil {
		return *m.Duration
	}
	if m.Metadata.Duration != nil {
		return *m.Metadata.Duration
	}
	return 0
}

// HasDuration returns true if a non-zero duration is available.
func (m Media) HasDuration() bool {
	return m.TotalDuration() > 0
}

// PodcastEpisode represents a single episode of a podcast.
type PodcastEpisode struct {
	ID          string     `json:"id"`
	Index       *int       `json:"index"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Season      string     `json:"season,omitempty"`
	Episode     string     `json:"episode,omitempty"`
	EpisodeType string     `json:"episodeType,omitempty"`
	Duration    float64    `json:"duration"`
	Size        int64      `json:"size,omitempty"`
	AudioTrack  AudioTrack `json:"audioTrack,omitempty"`
	PublishedAt *int64     `json:"publishedAt,omitempty"`
	AddedAt     int64      `json:"addedAt,omitempty"`
}

// MediaMetadata holds descriptive information about a media item.
type MediaMetadata struct {
	Title       string           `json:"title"`
	AuthorName  *string          `json:"authorName,omitempty"`
	Authors     []Author         `json:"authors,omitempty"`
	Description *string          `json:"description,omitempty"`
	Duration    *float64         `json:"duration,omitempty"` // seconds
	Chapters    []Chapter        `json:"chapters,omitempty"`
	Series      *SeriesSequence  `json:"series,omitempty"`
	SeriesList  []SeriesSequence `json:"-"`
}

// UnmarshalJSON handles ABS returning series metadata as either an object or an array.
func (m *MediaMetadata) UnmarshalJSON(data []byte) error {
	type mediaMetadataAlias struct {
		Title       string          `json:"title"`
		Author      *string         `json:"author,omitempty"`
		AuthorName  *string         `json:"authorName,omitempty"`
		Authors     []Author        `json:"authors,omitempty"`
		Description *string         `json:"description,omitempty"`
		Duration    *float64        `json:"duration,omitempty"`
		Chapters    []Chapter       `json:"chapters,omitempty"`
		Series      json.RawMessage `json:"series,omitempty"`
	}

	var aux mediaMetadataAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	authorName := aux.AuthorName
	if authorName == nil {
		authorName = aux.Author
	}

	*m = MediaMetadata{
		Title:       aux.Title,
		AuthorName:  authorName,
		Authors:     aux.Authors,
		Description: aux.Description,
		Duration:    aux.Duration,
		Chapters:    aux.Chapters,
	}

	seriesJSON := strings.TrimSpace(string(aux.Series))
	if seriesJSON == "" || seriesJSON == "null" {
		return nil
	}

	if seriesJSON[0] == '[' {
		var seriesList []SeriesSequence
		if err := json.Unmarshal(aux.Series, &seriesList); err != nil {
			return err
		}
		m.SeriesList = seriesList
		if len(seriesList) > 0 {
			series := seriesList[0]
			m.Series = &series
		}
		return nil
	}

	var series SeriesSequence
	if err := json.Unmarshal(aux.Series, &series); err != nil {
		return err
	}
	m.Series = &series
	m.SeriesList = []SeriesSequence{series}
	return nil
}

// DisplayAuthor returns the best author string for UI display and search.
func (m MediaMetadata) DisplayAuthor() string {
	names := make([]string, 0, len(m.Authors))
	for _, author := range m.Authors {
		name := strings.TrimSpace(author.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return strings.Join(names, ", ")
	}
	if m.AuthorName != nil {
		name := strings.TrimSpace(*m.AuthorName)
		if name != "" {
			return name
		}
	}
	return "Unknown author"
}

// PrimarySeries returns the first ABS series entry used by pine's current single-series behavior.
func (m MediaMetadata) PrimarySeries() *SeriesSequence {
	if len(m.SeriesList) > 0 {
		return &m.SeriesList[0]
	}
	return m.Series
}

// HasMultipleAuthors reports whether ABS returned more than one non-empty author.
func (m MediaMetadata) HasMultipleAuthors() bool {
	count := 0
	for _, author := range m.Authors {
		if strings.TrimSpace(author.Name) != "" {
			count++
		}
	}
	return count > 1
}

// HasMultipleSeries reports whether ABS returned more than one series entry.
func (m MediaMetadata) HasMultipleSeries() bool {
	return len(m.SeriesList) > 1
}

// SeriesByID returns the series entry with the given ABS series ID.
func (m MediaMetadata) SeriesByID(id string) *SeriesSequence {
	for i := range m.SeriesList {
		if m.SeriesList[i].ID == id {
			return &m.SeriesList[i]
		}
	}
	if m.Series != nil && m.Series.ID == id {
		return m.Series
	}
	return nil
}

// Author is an Audiobookshelf author reference attached to book metadata.
type Author struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

// SeriesSequence is the minimal series context attached to a library item.
type SeriesSequence struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Sequence string `json:"sequence,omitempty"`
}

// SeriesBook is a library item embedded in some ABS responses with sequence info.
type SeriesBook struct {
	LibraryItem
	Sequence string `json:"sequence,omitempty"`
}

// Series contains series metadata returned by ABS.
type Series struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	LibraryID   string       `json:"libraryId,omitempty"`
	AddedAt     int64        `json:"addedAt,omitempty"`
	UpdatedAt   int64        `json:"updatedAt,omitempty"`
	Books       []SeriesBook `json:"books"`
}

// SeriesContents combines series metadata with the library items in that series.
type SeriesContents struct {
	Series Series
	Items  []LibraryItem
}

// LibrarySeriesResponse is returned by GET /api/libraries/{id}/series.
type LibrarySeriesResponse struct {
	Results []Series `json:"results"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Page    int      `json:"page"`
}

// Chapter represents a chapter within a media item.
type Chapter struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Title string  `json:"title"`
}

// UserMediaProgress tracks a user's progress on a media item.
type UserMediaProgress struct {
	CurrentTime float64 `json:"currentTime"` // seconds
	Progress    float64 `json:"progress"`    // 0.0 - 1.0
	IsFinished  bool    `json:"isFinished"`
}

// PlaySession is returned by POST /api/items/{id}/play or /api/items/{id}/play/{episodeId}.
type PlaySession struct {
	ID            string        `json:"id"`
	AudioTracks   []AudioTrack  `json:"audioTracks"`
	CurrentTime   float64       `json:"currentTime"`
	EpisodeID     string        `json:"episodeId,omitempty"`
	MediaMetadata MediaMetadata `json:"mediaMetadata"`
	Chapters      []Chapter     `json:"chapters,omitempty"`
}

// AudioTrack represents a single audio track in a play session.
type AudioTrack struct {
	Index       int     `json:"index"`
	StartOffset float64 `json:"startOffset"`
	Duration    float64 `json:"duration"`
	ContentURL  string  `json:"contentUrl"`
}

// LibraryItemsResponse is returned by GET /api/libraries/{id}/items.
type LibraryItemsResponse struct {
	Results []LibraryItem `json:"results"`
	Total   int           `json:"total"`
	Limit   int           `json:"limit"`
	Page    int           `json:"page"`
}

// SearchResult is returned by GET /api/libraries/{id}/search.
type SearchResult struct {
	Book    []SearchResultEntry `json:"book"`
	Podcast []SearchResultEntry `json:"podcast"`
}

// SearchResultEntry wraps a LibraryItem in a search result with match context.
type SearchResultEntry struct {
	LibraryItem LibraryItem `json:"libraryItem"`
	MatchKey    string      `json:"matchKey,omitempty"`
	MatchText   string      `json:"matchText,omitempty"`
}

// PersonalizedResponse is a section returned by GET /api/libraries/{id}/personalized.
type PersonalizedResponse struct {
	ID       string        `json:"id"` // e.g. "continue-listening"
	Entities []LibraryItem `json:"entities"`
}

// Bookmark represents a user bookmark within a media item.
type Bookmark struct {
	LibraryItemID string  `json:"libraryItemId,omitempty"`
	Title         string  `json:"title"`
	Time          float64 `json:"time"`      // seconds
	CreatedAt     int64   `json:"createdAt"` // unix timestamp ms
}

// MediaProgressWithBookmarks extends progress with bookmark data.
type MediaProgressWithBookmarks struct {
	LibraryItemID string     `json:"libraryItemId"`
	EpisodeID     string     `json:"episodeId,omitempty"`
	CurrentTime   float64    `json:"currentTime"`
	Progress      float64    `json:"progress"`
	IsFinished    bool       `json:"isFinished"`
	Bookmarks     []Bookmark `json:"bookmarks"`
}
