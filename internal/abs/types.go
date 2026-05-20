package abs

import (
	"cmp"
	"encoding/json"
	"slices"
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
	Title       string          `json:"title"`
	AuthorName  *string         `json:"authorName,omitempty"`
	Description *string         `json:"description,omitempty"`
	Duration    *float64        `json:"duration,omitempty"` // seconds
	Chapters    []Chapter       `json:"chapters,omitempty"`
	Series      *SeriesSequence `json:"series,omitempty"`
}

// UnmarshalJSON handles ABS returning series metadata as either an object or an array.
func (m *MediaMetadata) UnmarshalJSON(data []byte) error {
	type mediaMetadataAlias struct {
		Title       string          `json:"title"`
		AuthorName  *string         `json:"authorName,omitempty"`
		Description *string         `json:"description,omitempty"`
		Duration    *float64        `json:"duration,omitempty"`
		Chapters    []Chapter       `json:"chapters,omitempty"`
		Series      json.RawMessage `json:"series,omitempty"`
	}

	var aux mediaMetadataAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*m = MediaMetadata{
		Title:       aux.Title,
		AuthorName:  aux.AuthorName,
		Description: aux.Description,
		Duration:    aux.Duration,
		Chapters:    aux.Chapters,
	}

	if len(aux.Series) == 0 || string(aux.Series) == "null" {
		return nil
	}

	if aux.Series[0] == '[' {
		var seriesList []SeriesSequence
		if err := json.Unmarshal(aux.Series, &seriesList); err != nil {
			return err
		}
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
	return nil
}

// SeriesSequence is the minimal series context attached to a library item.
type SeriesSequence struct {
	ID       string `json:"id"`
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
