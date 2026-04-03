package abs

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed testdata/play_session.json
var playSessionFixture []byte

// --- Deserialization tests ---

func TestPlaySessionDeserialization(t *testing.T) {
	var session PlaySession
	if err := json.Unmarshal(playSessionFixture, &session); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	if session.ID != "session-abc-123" {
		t.Errorf("ID = %q, want %q", session.ID, "session-abc-123")
	}
	if len(session.AudioTracks) != 2 {
		t.Fatalf("expected 2 audio tracks, got %d", len(session.AudioTracks))
	}
	track := session.AudioTracks[0]
	if track.Index != 0 {
		t.Errorf("track Index = %d, want 0", track.Index)
	}
	if track.Duration != 3600.5 {
		t.Errorf("track Duration = %f, want 3600.5", track.Duration)
	}
	if track.ContentURL != "/s/item/li-001/file-0.mp3" {
		t.Errorf("track ContentURL = %q, want %q", track.ContentURL, "/s/item/li-001/file-0.mp3")
	}
	if session.CurrentTime != 1234.56 {
		t.Errorf("CurrentTime = %f, want 1234.56", session.CurrentTime)
	}
	if session.MediaMetadata.Title != "The Great Adventure" {
		t.Errorf("Title = %q, want %q", session.MediaMetadata.Title, "The Great Adventure")
	}
	if len(session.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(session.Chapters))
	}
}

// --- Request body serialization tests ---

func TestStartPlaySessionRequestBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write(playSessionFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.StartPlaySession(context.Background(), "li-001", DeviceInfo{
		DeviceID:   "dev-1",
		ClientName: "pine",
	})
	if err != nil {
		t.Fatalf("StartPlaySession() error: %v", err)
	}

	var body struct {
		DeviceInfo      DeviceInfo `json:"deviceInfo"`
		ForceDirectPlay bool       `json:"forceDirectPlay"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.DeviceInfo.DeviceID != "dev-1" {
		t.Errorf("deviceId = %q, want %q", body.DeviceInfo.DeviceID, "dev-1")
	}
	if body.DeviceInfo.ClientName != "pine" {
		t.Errorf("clientName = %q, want %q", body.DeviceInfo.ClientName, "pine")
	}
	if body.ForceDirectPlay != true {
		t.Error("forceDirectPlay should be true")
	}
}

// --- HTTP integration tests ---

func TestStartPlaySessionHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/li-001/play" {
			t.Errorf("path = %q, want /api/items/li-001/play", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(playSessionFixture)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	session, err := c.StartPlaySession(context.Background(), "li-001", DeviceInfo{
		DeviceID:   "dev-1",
		ClientName: "pine",
	})
	if err != nil {
		t.Fatalf("StartPlaySession() error: %v", err)
	}
	if session.ID != "session-abc-123" {
		t.Errorf("session ID = %q, want %q", session.ID, "session-abc-123")
	}
	if len(session.AudioTracks) != 2 {
		t.Fatalf("expected 2 audio tracks, got %d", len(session.AudioTracks))
	}
}

func TestSyncSessionHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session/session-abc-123/sync" {
			t.Errorf("path = %q, want /api/session/session-abc-123/sync", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.SyncSession(context.Background(), "session-abc-123", 500.5, 120.0)
	if err != nil {
		t.Fatalf("SyncSession() error: %v", err)
	}

	var body struct {
		CurrentTime  float64 `json:"currentTime"`
		TimeListened float64 `json:"timeListened"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.CurrentTime != 500.5 {
		t.Errorf("currentTime = %f, want 500.5", body.CurrentTime)
	}
	if body.TimeListened != 120.0 {
		t.Errorf("timeListened = %f, want 120.0", body.TimeListened)
	}
}

func TestStartEpisodePlaySessionHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/li-pod-001/play/ep-001" {
			t.Errorf("path = %q, want /api/items/li-pod-001/play/ep-001", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "session-ep-001",
			"audioTracks": [{"contentUrl": "/s/item/li-pod-001/ep1.mp3", "duration": 1200.5}],
			"currentTime": 0,
			"episodeId": "ep-001"
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	session, err := c.StartEpisodePlaySession(context.Background(), "li-pod-001", "ep-001", DeviceInfo{
		DeviceID: "test-device",
		ClientName: "abs-cli-test",
	})
	if err != nil {
		t.Fatalf("StartEpisodePlaySession() error: %v", err)
	}
	if session.ID != "session-ep-001" {
		t.Errorf("session.ID = %q, want session-ep-001", session.ID)
	}
	if session.EpisodeID != "ep-001" {
		t.Errorf("session.EpisodeID = %q, want ep-001", session.EpisodeID)
	}

	var body struct {
		DeviceInfo      DeviceInfo `json:"deviceInfo"`
		ForceDirectPlay bool       `json:"forceDirectPlay"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.DeviceInfo.DeviceID != "test-device" {
		t.Errorf("deviceId = %q, want test-device", body.DeviceInfo.DeviceID)
	}
	if !body.ForceDirectPlay {
		t.Error("forceDirectPlay should be true")
	}
}

func TestCloseSessionHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session/session-abc-123/close" {
			t.Errorf("path = %q, want /api/session/session-abc-123/close", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.CloseSession(context.Background(), "session-abc-123", 600.0, 180.0)
	if err != nil {
		t.Fatalf("CloseSession() error: %v", err)
	}

	var body struct {
		CurrentTime  float64 `json:"currentTime"`
		TimeListened float64 `json:"timeListened"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.CurrentTime != 600.0 {
		t.Errorf("currentTime = %f, want 600.0", body.CurrentTime)
	}
	if body.TimeListened != 180.0 {
		t.Errorf("timeListened = %f, want 180.0", body.TimeListened)
	}
}
