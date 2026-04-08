package abs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateProgressHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/progress/li-001" {
			t.Errorf("path = %q, want /api/me/progress/li-001", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.UpdateProgress(context.Background(), "li-001", 500.5, 0.45, false)
	if err != nil {
		t.Fatalf("UpdateProgress() error: %v", err)
	}

	var body struct {
		CurrentTime float64 `json:"currentTime"`
		Progress    float64 `json:"progress"`
		IsFinished  bool    `json:"isFinished"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.CurrentTime != 500.5 {
		t.Errorf("currentTime = %f, want 500.5", body.CurrentTime)
	}
	if body.Progress != 0.45 {
		t.Errorf("progress = %f, want 0.45", body.Progress)
	}
	if body.IsFinished != false {
		t.Error("isFinished should be false")
	}
}

func TestUpdateProgressFinishedHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.UpdateProgress(context.Background(), "li-002", 3600.0, 1.0, true)
	if err != nil {
		t.Fatalf("UpdateProgress() error: %v", err)
	}

	var body struct {
		CurrentTime float64 `json:"currentTime"`
		Progress    float64 `json:"progress"`
		IsFinished  bool    `json:"isFinished"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.IsFinished != true {
		t.Error("isFinished should be true")
	}
	if body.Progress != 1.0 {
		t.Errorf("progress = %f, want 1.0", body.Progress)
	}
}

func TestGetMediaProgressHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/progress/li-001" {
			t.Errorf("path = %q, want /api/me/progress/li-001", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"libraryItemId": "li-001",
			"currentTime": 120.5,
			"progress": 0.03,
			"isFinished": false,
			"bookmarks": [
				{"libraryItemId": "li-001", "title": "Great passage", "time": 300.5, "createdAt": 1700000000000}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	progress, err := c.GetMediaProgress(context.Background(), "li-001")
	if err != nil {
		t.Fatalf("GetMediaProgress() error: %v", err)
	}
	if progress.LibraryItemID != "li-001" {
		t.Errorf("libraryItemId = %q, want li-001", progress.LibraryItemID)
	}
	if progress.CurrentTime != 120.5 {
		t.Errorf("currentTime = %f, want 120.5", progress.CurrentTime)
	}
	if len(progress.Bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(progress.Bookmarks))
	}
	if progress.Bookmarks[0].Title != "Great passage" {
		t.Errorf("bookmark title = %q, want Great passage", progress.Bookmarks[0].Title)
	}
}

func TestGetMediaProgressNotFoundReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, err := c.GetMediaProgress(context.Background(), "li-404")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestUpdateEpisodeProgressHTTP(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me/progress/li-pod-001/ep-001" {
			t.Errorf("path = %q, want /api/me/progress/li-pod-001/ep-001", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.UpdateEpisodeProgress(context.Background(), "li-pod-001", "ep-001", 300.0, 0.5, false)
	if err != nil {
		t.Fatalf("UpdateEpisodeProgress() error: %v", err)
	}

	var body struct {
		CurrentTime float64 `json:"currentTime"`
		Progress    float64 `json:"progress"`
		IsFinished  bool    `json:"isFinished"`
	}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if body.CurrentTime != 300.0 {
		t.Errorf("currentTime = %f, want 300.0", body.CurrentTime)
	}
	if body.Progress != 0.5 {
		t.Errorf("progress = %f, want 0.5", body.Progress)
	}
}
