package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/spruce/buildinfo"
)

const pageLimit = 200

// HTTPStatusError wraps a non-2xx HTTP response status.
type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
}

// IsHTTPStatus reports whether err or one of its wrapped errors is an HTTPStatusError
// for the given status code.
func IsHTTPStatus(err error, statusCode int) bool {
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == statusCode
}

const maxResponseSize = 50 * 1024 * 1024 // 50 MB

// Client is an HTTP client for the Jellyfin API.
type Client struct {
	baseURL    string
	token      string
	userID     string
	deviceName string
	deviceID   string
	version    string
	httpClient *http.Client
}

// NewClient creates a new Jellyfin API client.
func NewClient(baseURL, token, userID, deviceName, deviceID string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		userID:     userID,
		deviceName: deviceName,
		deviceID:   deviceID,
		version:    buildinfo.Current(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// BaseURL returns the configured server URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Token returns the active auth token.
func (c *Client) Token() string {
	return c.token
}

// UserID returns the active user ID.
func (c *Client) UserID() string {
	return c.userID
}

// SetAuth updates the active auth credentials.
func (c *Client) SetAuth(token, userID string) {
	c.token = token
	c.userID = userID
}

// authHeader constructs the MediaBrowser Authorization header string.
func (c *Client) authHeader() string {
	header := `MediaBrowser Client="spruce", Device="` + c.deviceName + `", DeviceId="` + c.deviceID + `", Version="` + c.version + `"`
	if c.token != "" {
		header += `, Token="` + c.token + `"`
	}
	return header
}

// StreamURL constructs the direct stream URL for an audio item.
// The auth token is intentionally not embedded in the URL; callers must send
// StreamHeaders with the request so the token stays out of process args, logs,
// and MPRIS metadata.
func (c *Client) StreamURL(itemID, playSessionID string) string {
	q := url.Values{}
	q.Set("static", "true")
	q.Set("playSessionId", playSessionID)
	q.Set("deviceId", c.deviceID)
	return fmt.Sprintf("%s/Audio/%s/stream?%s", c.baseURL, itemID, q.Encode())
}

// StreamHeaders returns the HTTP headers needed by mpv to authenticate direct streaming.
func (c *Client) StreamHeaders() []string {
	return []string{"Authorization: " + c.authHeader()}
}

// DownloadAudio writes the original audio item to dst without applying an
// overall request timeout. Cancellation is controlled by ctx.
func (c *Client) DownloadAudio(ctx context.Context, itemID string, dst io.Writer) (int64, error) {
	if itemID == "" {
		return 0, errors.New("item ID is empty")
	}
	if dst == nil {
		return 0, errors.New("download destination is nil")
	}
	path := fmt.Sprintf("/Items/%s/Download", url.PathEscape(itemID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, fmt.Errorf("create audio download request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader())

	downloadClient := &http.Client{
		Transport:     c.httpClient.Transport,
		CheckRedirect: c.httpClient.CheckRedirect,
		Jar:           c.httpClient.Jar,
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download audio: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
		return 0, &HTTPStatusError{StatusCode: resp.StatusCode}
	}
	written, err := io.Copy(dst, resp.Body)
	if err != nil {
		return written, fmt.Errorf("write audio download: %w", err)
	}
	return written, nil
}

// do executes an authenticated HTTP request and returns the response body.
func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	start := time.Now()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("http request failed", "method", method, "path", path, "err", err, "duration", time.Since(start))
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limitedReader := io.LimitReader(resp.Body, maxResponseSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		logger.Error("http response read failed", "method", method, "path", path, "status", resp.StatusCode, "err", err, "duration", time.Since(start))
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if len(data) > maxResponseSize {
		logger.Error("http response body too large", "method", method, "path", path, "status", resp.StatusCode, "limit", maxResponseSize, "duration", time.Since(start))
		return nil, fmt.Errorf("response body exceeds %d byte limit", maxResponseSize)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Error("http request returned unexpected status", "method", method, "path", path, "status", resp.StatusCode, "body", string(data), "duration", time.Since(start))
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Body: string(data)}
	}

	logger.Debug("http request completed", "method", method, "path", path, "status", resp.StatusCode, "duration", time.Since(start))
	return data, nil
}

// Login authenticates a user against Jellyfin server.
func (c *Client) Login(ctx context.Context, username, password string) (*AuthResponse, error) {
	reqBody := AuthRequest{
		Username: username,
		Pw:       password,
	}
	data, err := c.do(ctx, http.MethodPost, "/Users/AuthenticateByName", reqBody)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	var resp AuthResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}
	c.SetAuth(resp.AccessToken, resp.User.ID)
	return &resp, nil
}

// fetchPaged repeatedly calls /Users/{userId}/Items with increasing StartIndex
// until TotalRecordCount is reached, accumulating Items into a slice.
// basePath must already include the userId segment (e.g. "/Users/{id}/Items").
// extra query values carry caller-specific params (IncludeItemTypes, Recursive,
// ParentId, SortBy, etc.).
func fetchPaged[T any](c *Client, ctx context.Context, basePath string, extra url.Values) ([]T, error) {
	var acc []T
	startIndex := 0
	total := -1
	for {
		q := url.Values{}
		for k, vs := range extra {
			q[k] = append(q[k], vs...)
		}
		q.Set("StartIndex", strconv.Itoa(startIndex))
		q.Set("Limit", strconv.Itoa(pageLimit))
		path := basePath + "?" + q.Encode()
		data, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var page itemsResponse[T]
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("decode paged response: %w", err)
		}
		acc = append(acc, page.Items...)
		if total < 0 {
			total = page.TotalRecordCount
		}
		startIndex += len(page.Items)
		// Safety: stop if a page returns no items, or if we've reached the
		// server-reported total, or if the server didn't report a total.
		if len(page.Items) == 0 {
			break
		}
		if total > 0 {
			if startIndex >= total {
				break
			}
		} else {
			break
		}
	}
	return acc, nil
}

// GetMusicLibraries fetches all user views and returns those related to music.
func (c *Client) GetMusicLibraries(ctx context.Context) ([]Library, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	path := fmt.Sprintf("/Users/%s/Views", url.PathEscape(c.userID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get views: %w", err)
	}
	var resp itemsResponse[Library]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode views: %w", err)
	}
	var musicLibs []Library
	for _, lib := range resp.Items {
		if strings.EqualFold(lib.CollectionType, "music") {
			musicLibs = append(musicLibs, lib)
		}
	}
	return musicLibs, nil
}

// GetArtists fetches music artists within a library.
func (c *Client) GetArtists(ctx context.Context, libraryID string) ([]Artist, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("ParentId", libraryID)
	params.Set("SortBy", "SortName")
	params.Set("SortOrder", "Ascending")
	params.Set("IncludeItemTypes", "MusicArtist,Artist")
	params.Set("Recursive", "true")

	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Artist](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get artists: %w", err)
	}
	return items, nil
}

// GetAlbums fetches music albums for a given artist.
func (c *Client) GetAlbums(ctx context.Context, artistID string) ([]Album, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("ParentId", artistID)
	params.Set("SortBy", "ProductionYear,SortName")
	params.Set("SortOrder", "Ascending")
	params.Set("IncludeItemTypes", "MusicAlbum")
	params.Set("Recursive", "true")
	params.Set("EnableUserData", "true")

	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Album](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get albums: %w", err)
	}
	return items, nil
}

// GetAllAlbums fetches all music albums within a library.
func (c *Client) GetAllAlbums(ctx context.Context, libraryID string) ([]Album, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("ParentId", libraryID)
	params.Set("SortBy", "SortName")
	params.Set("SortOrder", "Ascending")
	params.Set("IncludeItemTypes", "MusicAlbum")
	params.Set("Recursive", "true")
	params.Set("EnableUserData", "true")

	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Album](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get all albums: %w", err)
	}
	return items, nil
}

// GetTracks fetches audio tracks for a given album.
func (c *Client) GetTracks(ctx context.Context, albumID string) ([]Track, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("ParentId", albumID)
	params.Set("SortBy", "ParentIndexNumber,IndexNumber")
	params.Set("SortOrder", "Ascending")
	params.Set("IncludeItemTypes", "Audio")
	params.Set("Recursive", "true")
	params.Set("EnableUserData", "true")

	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Track](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get tracks: %w", err)
	}
	SortAlbumTracks(items)
	return items, nil
}

// GetTrack fetches a single track by item ID.
func (c *Client) GetTrack(ctx context.Context, itemID string) (*Track, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("Ids", itemID)
	params.Set("EnableUserData", "true")
	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Track](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get track: %w", err)
	}
	if len(items) == 0 {
		return nil, errors.New("track not found")
	}
	return &items[0], nil
}

// GetAllTracks fetches all audio tracks within a library.
func (c *Client) GetAllTracks(ctx context.Context, libraryID string) ([]Track, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("ParentId", libraryID)
	params.Set("SortBy", "SortName")
	params.Set("SortOrder", "Ascending")
	params.Set("IncludeItemTypes", "Audio")
	params.Set("Recursive", "true")
	params.Set("EnableUserData", "true")

	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Track](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get all tracks: %w", err)
	}
	return items, nil
}

// GetPlaylists fetches user playlists.
func (c *Client) GetPlaylists(ctx context.Context) ([]Playlist, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	params := url.Values{}
	params.Set("SortBy", "SortName")
	params.Set("SortOrder", "Ascending")
	params.Set("IncludeItemTypes", "Playlist")
	params.Set("Recursive", "true")

	basePath := fmt.Sprintf("/Users/%s/Items", url.PathEscape(c.userID))
	items, err := fetchPaged[Playlist](c, ctx, basePath, params)
	if err != nil {
		return nil, fmt.Errorf("get playlists: %w", err)
	}
	return items, nil
}

// GetPlaylistTracks fetches audio tracks in a playlist.
func (c *Client) GetPlaylistTracks(ctx context.Context, playlistID string) ([]Track, error) {
	if c.userID == "" {
		return nil, errors.New("user ID not set")
	}
	var acc []Track
	startIndex := 0
	total := -1
	for {
		q := url.Values{}
		q.Set("userId", c.userID)
		q.Set("startIndex", strconv.Itoa(startIndex))
		q.Set("limit", strconv.Itoa(pageLimit))
		q.Set("enableUserData", "true")
		path := fmt.Sprintf("/Playlists/%s/Items?%s", url.PathEscape(playlistID), q.Encode())
		data, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("get playlist tracks: %w", err)
		}
		var page itemsResponse[Track]
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("decode playlist tracks: %w", err)
		}
		acc = append(acc, page.Items...)
		if total < 0 {
			total = page.TotalRecordCount
		}
		startIndex += len(page.Items)
		if len(page.Items) == 0 {
			break
		}
		if total > 0 {
			if startIndex >= total {
				break
			}
		} else {
			break
		}
	}
	return acc, nil
}

// SetFavorite updates the current user's favorite state for an item.
func (c *Client) SetFavorite(ctx context.Context, itemID string, favorite bool) (UserItemData, error) {
	if c.userID == "" {
		return UserItemData{}, errors.New("user ID not set")
	}
	q := url.Values{}
	q.Set("userId", c.userID)
	path := fmt.Sprintf("/UserFavoriteItems/%s?%s", url.PathEscape(itemID), q.Encode())
	method := http.MethodDelete
	if favorite {
		method = http.MethodPost
	}
	data, err := c.do(ctx, method, path, nil)
	if err != nil {
		return UserItemData{}, fmt.Errorf("set favorite: %w", err)
	}
	var userData UserItemData
	if err := json.Unmarshal(data, &userData); err != nil {
		return UserItemData{}, fmt.Errorf("decode favorite response: %w", err)
	}
	return userData, nil
}

// ReportPlaybackStart reports to Jellyfin that playback has begun.
func (c *Client) ReportPlaybackStart(ctx context.Context, itemID, playSessionID string) error {
	body := PlaybackProgressRequest{
		ItemID:        itemID,
		PlayMethod:    "DirectPlay",
		CanSeek:       true,
		PlaySessionId: playSessionID,
		MediaSourceId: itemID,
	}
	_, err := c.do(ctx, http.MethodPost, "/Sessions/Playing", body)
	return err
}

// ReportPlaybackProgress reports active playback position heartbeats.
func (c *Client) ReportPlaybackProgress(ctx context.Context, itemID string, positionSeconds float64, paused bool, playSessionID string) error {
	body := PlaybackProgressRequest{
		ItemID:        itemID,
		PositionTicks: SecondsToTicks(positionSeconds),
		IsPaused:      paused,
		PlayMethod:    "DirectPlay",
		CanSeek:       true,
		PlaySessionId: playSessionID,
		MediaSourceId: itemID,
	}
	_, err := c.do(ctx, http.MethodPost, "/Sessions/Playing/Progress", body)
	return err
}

// ReportPlaybackStopped reports that playback has stopped.
func (c *Client) ReportPlaybackStopped(ctx context.Context, itemID string, positionSeconds float64, playSessionID string) error {
	body := PlaybackProgressRequest{
		ItemID:        itemID,
		PositionTicks: SecondsToTicks(positionSeconds),
		PlayMethod:    "DirectPlay",
		CanSeek:       true,
		PlaySessionId: playSessionID,
		MediaSourceId: itemID,
	}
	_, err := c.do(ctx, http.MethodPost, "/Sessions/Playing/Stopped", body)
	return err
}

// UpdateItem updates item metadata in Jellyfin via POST /Items/{itemId}.
func (c *Client) UpdateItem(ctx context.Context, itemID string, req UpdateItemRequest) error {
	if c.userID == "" {
		return errors.New("user ID not set")
	}
	getPath := fmt.Sprintf("/Users/%s/Items/%s", url.PathEscape(c.userID), url.PathEscape(itemID))
	data, err := c.do(ctx, http.MethodGet, getPath, nil)
	if err != nil {
		return fmt.Errorf("fetch existing item: %w", err)
	}

	var itemData map[string]any
	if err := json.Unmarshal(data, &itemData); err != nil {
		return fmt.Errorf("decode existing item: %w", err)
	}

	if req.Name != "" {
		itemData["Name"] = req.Name
	}
	if req.Album != "" {
		itemData["Album"] = req.Album
	}
	if req.Artists != nil {
		itemData["Artists"] = req.Artists
	}
	if req.IndexNumber != nil {
		itemData["IndexNumber"] = *req.IndexNumber
	}
	if req.ParentIndexNumber != nil {
		itemData["ParentIndexNumber"] = *req.ParentIndexNumber
	}
	if req.ProductionYear != nil {
		itemData["ProductionYear"] = *req.ProductionYear
	}

	postPath := fmt.Sprintf("/Items/%s", url.PathEscape(itemID))
	_, err = c.do(ctx, http.MethodPost, postPath, itemData)
	return err
}
