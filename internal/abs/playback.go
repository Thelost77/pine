package abs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Thelost77/pine/internal/logger"
)

// DeviceInfo identifies the client device for a play session.
type DeviceInfo struct {
	DeviceID   string `json:"deviceId"`
	ClientName string `json:"clientName"`
}

// startPlayRequest is the body for POST /api/items/{id}/play.
type startPlayRequest struct {
	DeviceInfo      DeviceInfo `json:"deviceInfo"`
	ForceDirectPlay bool       `json:"forceDirectPlay"`
}

// syncSessionRequest is the body for POST /api/session/{id}/sync and /close.
type syncSessionRequest struct {
	CurrentTime  float64 `json:"currentTime"`
	TimeListened float64 `json:"timeListened"`
}

// StartPlaySession starts a new playback session for a library item.
func (c *Client) StartPlaySession(ctx context.Context, itemID string, device DeviceInfo) (*PlaySession, error) {
	path := fmt.Sprintf("/api/items/%s/play", itemID)
	logger.Debug("API request", "method", "POST", "path", path, "itemID", itemID)
	body := startPlayRequest{
		DeviceInfo:      device,
		ForceDirectPlay: true,
	}
	data, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		logger.Error("start play session failed", "itemID", itemID, "err", err)
		return nil, fmt.Errorf("start play session: %w", err)
	}
	var session PlaySession
	if err := json.Unmarshal(data, &session); err != nil {
		logger.Error("decode play session failed", "itemID", itemID, "err", err, "body", string(data))
		return nil, fmt.Errorf("decode play session: %w", err)
	}
	logger.Info("play session created", "sessionID", session.ID, "tracks", len(session.AudioTracks), "currentTime", session.CurrentTime)
	return &session, nil
}

// StartEpisodePlaySession starts a new playback session for a podcast episode.
func (c *Client) StartEpisodePlaySession(ctx context.Context, itemID, episodeID string, device DeviceInfo) (*PlaySession, error) {
	path := fmt.Sprintf("/api/items/%s/play/%s", itemID, episodeID)
	logger.Debug("API request", "method", "POST", "path", path, "itemID", itemID, "episodeID", episodeID)
	body := startPlayRequest{
		DeviceInfo:      device,
		ForceDirectPlay: true,
	}
	data, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		logger.Error("start episode play session failed", "itemID", itemID, "episodeID", episodeID, "err", err)
		return nil, fmt.Errorf("start episode play session: %w", err)
	}
	var session PlaySession
	if err := json.Unmarshal(data, &session); err != nil {
		logger.Error("decode episode play session failed", "itemID", itemID, "episodeID", episodeID, "err", err, "body", string(data))
		return nil, fmt.Errorf("decode episode play session: %w", err)
	}
	logger.Info("episode play session created", "sessionID", session.ID, "tracks", len(session.AudioTracks), "currentTime", session.CurrentTime, "episodeID", session.EpisodeID)
	return &session, nil
}

// SyncSession syncs the current playback position for an active session.
func (c *Client) SyncSession(ctx context.Context, sessionID string, currentTime, timeListened float64) error {
	path := fmt.Sprintf("/api/session/%s/sync", sessionID)
	body := syncSessionRequest{
		CurrentTime:  currentTime,
		TimeListened: timeListened,
	}
	_, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return fmt.Errorf("sync session: %w", err)
	}
	return nil
}

// CloseSession closes an active playback session.
func (c *Client) CloseSession(ctx context.Context, sessionID string, currentTime, timeListened float64) error {
	path := fmt.Sprintf("/api/session/%s/close", sessionID)
	body := syncSessionRequest{
		CurrentTime:  currentTime,
		TimeListened: timeListened,
	}
	_, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	return nil
}
