package abs

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
	MediaType         string             `json:"mediaType"` // "book" or "podcast"
	Media             Media              `json:"media"`
	UserMediaProgress *UserMediaProgress `json:"userMediaProgress,omitempty"`
	RecentEpisode     *PodcastEpisode    `json:"recentEpisode,omitempty"`
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
	Index       int        `json:"index"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Season      string     `json:"season,omitempty"`
	Episode     string     `json:"episode,omitempty"`
	EpisodeType string     `json:"episodeType,omitempty"`
	Duration    float64    `json:"duration"`
	Size        int64      `json:"size,omitempty"`
	AudioTrack  AudioTrack `json:"audioTrack,omitempty"`
	PublishedAt int64      `json:"publishedAt,omitempty"`
	AddedAt     int64      `json:"addedAt,omitempty"`
}

// MediaMetadata holds descriptive information about a media item.
type MediaMetadata struct {
	Title       string    `json:"title"`
	AuthorName  *string   `json:"authorName,omitempty"`
	Description *string   `json:"description,omitempty"`
	Duration    *float64  `json:"duration,omitempty"` // seconds
	Chapters    []Chapter `json:"chapters,omitempty"`
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
	Title     string  `json:"title"`
	Time      float64 `json:"time"`      // seconds
	CreatedAt int64   `json:"createdAt"` // unix timestamp ms
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
