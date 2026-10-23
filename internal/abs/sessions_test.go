package abs

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed testdata/listening_sessions.json
var listeningSessionsFixture []byte

// TestListeningSessionsRoutes verifies the path and the query of each route,
// and the decode of a podcast session and of two book sessions. One book
// session has the listening time null, as some sessions on a server have.
func TestListeningSessionsRoutes(t *testing.T) {
	tests := []struct {
		name  string
		call  func(c *Client) (*ListeningSessionsResponse, error)
		route string
	}{
		{
			name: "all items",
			call: func(c *Client) (*ListeningSessionsResponse, error) {
				return c.GetListeningSessions(context.Background(), 50)
			},
			route: "/api/me/listening-sessions",
		},
		{
			name: "book",
			call: func(c *Client) (*ListeningSessionsResponse, error) {
				return c.GetItemListeningSessions(context.Background(), "li-001", "", 50)
			},
			route: "/api/me/item/listening-sessions/li-001",
		},
		{
			name: "episode",
			call: func(c *Client) (*ListeningSessionsResponse, error) {
				return c.GetItemListeningSessions(context.Background(), "pod-001", "ep-001", 50)
			},
			route: "/api/me/item/listening-sessions/pod-001/ep-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %q, want GET", r.Method)
				}
				if r.URL.Path != tt.route {
					t.Errorf("path = %q, want %q", r.URL.Path, tt.route)
				}
				if got := r.URL.Query().Get("itemsPerPage"); got != "50" {
					t.Errorf("itemsPerPage = %q, want 50", got)
				}
				if got := r.URL.Query().Get("page"); got != "0" {
					t.Errorf("page = %q, want 0", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(listeningSessionsFixture)
			}))
			defer srv.Close()

			resp, err := tt.call(NewClient(srv.URL, "tok"))
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if resp.Total != 955 || len(resp.Sessions) != 3 {
				t.Fatalf("total = %d, sessions = %d, want 955 and 3", resp.Total, len(resp.Sessions))
			}

			episode := resp.Sessions[0]
			if episode.LibraryItemID != "pod-001" || episode.EpisodeID != "ep-001" {
				t.Errorf("episode session item = %q/%q, want pod-001/ep-001", episode.LibraryItemID, episode.EpisodeID)
			}
			if episode.MediaMetadata.Title != "JBlow" || episode.DisplayTitle != "Braid and The Witness" {
				t.Errorf("episode session titles = %q, %q", episode.MediaMetadata.Title, episode.DisplayTitle)
			}
			if episode.StartTime != 2799.0 || episode.CurrentTime != 6227.6 || episode.TimeListening != 1416.79 {
				t.Errorf("episode session times = %v, %v, %v", episode.StartTime, episode.CurrentTime, episode.TimeListening)
			}
			if episode.UpdatedAt != 1792573341300 {
				t.Errorf("episode session updatedAt = %d", episode.UpdatedAt)
			}
			device := episode.DeviceInfo
			if device.DeviceID != "pine-laptop-0001" || device.ClientName != "pine" || device.ClientVersion != "v0.8.3" || device.DeviceName != "Pine laptop" {
				t.Errorf("episode session device = %+v", device)
			}

			book := resp.Sessions[1]
			if book.EpisodeID != "" || book.MediaType != "book" || book.DisplayAuthor != "Wiesław Myśliwski" {
				t.Errorf("book session = %+v", book)
			}
			if resp.Sessions[2].TimeListening != 0 {
				t.Errorf("null timeListening = %v, want 0", resp.Sessions[2].TimeListening)
			}
		})
	}
}

// TestGetItemListeningSessionsNotFound verifies that a 404 of the server is
// an HTTP status error. The server answers 404 for an unknown item, and for a
// podcast with no episode.
func TestGetItemListeningSessionsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "tok").GetItemListeningSessions(context.Background(), "pod-001", "", 50)
	if !IsHTTPStatus(err, http.StatusNotFound) {
		t.Fatalf("error = %v, want HTTP 404", err)
	}
}
