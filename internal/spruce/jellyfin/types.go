package jellyfin

import (
	"cmp"
	"slices"
)

// AuthRequest is the payload sent to POST /Users/AuthenticateByName.
type AuthRequest struct {
	Username string `json:"Username"`
	Pw       string `json:"Pw"`
}

// AuthResponse is returned by POST /Users/AuthenticateByName.
type AuthResponse struct {
	User        User   `json:"User"`
	AccessToken string `json:"AccessToken"`
}

// User represents a Jellyfin user account.
type User struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// Library represents a user view/collection in Jellyfin.
type Library struct {
	ID             string `json:"Id"`
	Name           string `json:"Name"`
	CollectionType string `json:"CollectionType,omitempty"` // e.g. "music"
}

// Artist represents a music artist item.
type Artist struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// Album represents a music album item.
type Album struct {
	ID             string       `json:"Id"`
	Name           string       `json:"Name"`
	ProductionYear int          `json:"ProductionYear,omitempty"`
	Artists        []string     `json:"Artists,omitempty"`
	UserData       UserItemData `json:"UserData,omitempty"`
}

// Playlist represents a Jellyfin playlist item.
type Playlist struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	Count int    `json:"ChildCount,omitempty"`
}

// UserItemData contains user-specific state returned with a Jellyfin item.
type UserItemData struct {
	IsFavorite bool `json:"IsFavorite"`
}

// Track represents a single audio track item.
type Track struct {
	ID                string       `json:"Id"`
	Name              string       `json:"Name"`
	RunTimeTicks      int64        `json:"RunTimeTicks,omitempty"` // 1 sec = 10,000,000 ticks
	IndexNumber       int          `json:"IndexNumber,omitempty"`
	ParentIndexNumber int          `json:"ParentIndexNumber,omitempty"` // Disc number
	AlbumID           string       `json:"AlbumId,omitempty"`
	Album             string       `json:"Album,omitempty"`
	Artists           []string     `json:"Artists,omitempty"`
	UserData          UserItemData `json:"UserData,omitempty"`
}

// SortAlbumTracks orders tracks by disc and track number while retaining source order for ties.
func SortAlbumTracks(tracks []Track) {
	slices.SortStableFunc(tracks, func(a, b Track) int {
		return cmp.Or(
			cmp.Compare(a.ParentIndexNumber, b.ParentIndexNumber),
			cmp.Compare(a.IndexNumber, b.IndexNumber),
		)
	})
}

// SortFavoritesFirst stably groups favorite tracks before other tracks.
func SortFavoritesFirst(tracks []Track) {
	slices.SortStableFunc(tracks, func(a, b Track) int {
		if a.UserData.IsFavorite == b.UserData.IsFavorite {
			return 0
		}
		if a.UserData.IsFavorite {
			return -1
		}
		return 1
	})
}

// Duration returns the duration of the track in seconds.
func (t Track) Duration() float64 {
	if t.RunTimeTicks <= 0 {
		return 0
	}
	return float64(t.RunTimeTicks) / 1e7
}

// DisplayArtist returns a formatted artist string for UI display.
func (t Track) DisplayArtist() string {
	if len(t.Artists) > 0 && t.Artists[0] != "" {
		return t.Artists[0]
	}
	return "Unknown Artist"
}

// itemsResponse is the generic container returned by GET /Users/{UserId}/Items and /Views.
type itemsResponse[T any] struct {
	Items            []T `json:"Items"`
	TotalRecordCount int `json:"TotalRecordCount,omitempty"`
}

// PlaybackProgressRequest is sent to POST /Sessions/Playing, /Sessions/Playing/Progress,
// and /Sessions/Playing/Stopped. Jellyfin correlates the play session via PlaySessionId.
type PlaybackProgressRequest struct {
	ItemID        string `json:"ItemId"`
	PositionTicks int64  `json:"PositionTicks"`
	IsPaused      bool   `json:"IsPaused"`
	PlayMethod    string `json:"PlayMethod"`
	CanSeek       bool   `json:"CanSeek"`
	PlaySessionId string `json:"PlaySessionId"`
	MediaSourceId string `json:"MediaSourceId"`
}

// SecondsToTicks converts seconds to Jellyfin 100ns ticks.
func SecondsToTicks(seconds float64) int64 {
	if seconds <= 0 {
		return 0
	}
	return int64(seconds * 1e7)
}

// UpdateItemRequest represents fields sent to POST /Items/{itemId} to update metadata.
type UpdateItemRequest struct {
	ID                string   `json:"Id"`
	Name              string   `json:"Name,omitempty"`
	Album             string   `json:"Album,omitempty"`
	Artists           []string `json:"Artists,omitempty"`
	IndexNumber       *int     `json:"IndexNumber,omitempty"`
	ParentIndexNumber *int     `json:"ParentIndexNumber,omitempty"`
	ProductionYear    *int     `json:"ProductionYear,omitempty"`
}
