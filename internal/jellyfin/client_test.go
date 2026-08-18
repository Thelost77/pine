package jellyfin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Thelost77/spruce/internal/buildinfo"
)

func TestHTTPStatusErrorDoesNotExposeResponseBody(t *testing.T) {
	err := (&HTTPStatusError{StatusCode: http.StatusInternalServerError, Body: "secret server body"}).Error()
	if strings.Contains(err, "secret server body") {
		t.Fatalf("HTTPStatusError exposed response body: %q", err)
	}
	if !strings.Contains(err, "500") {
		t.Fatalf("HTTPStatusError omitted status code: %q", err)
	}
}

func TestClient_Login(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/AuthenticateByName" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %q", r.Method)
		}
		if got := r.Header.Get("Authorization"); !strings.Contains(got, `Device="Manual device"`) || !strings.Contains(got, `DeviceId="manual-id"`) {
			t.Errorf("unexpected device identity: %q", got)
		}
		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		if req.Username != "alice" || req.Pw != "secret" {
			t.Errorf("unexpected creds: %+v", req)
		}

		resp := AuthResponse{
			User:        User{ID: "user-123", Name: "alice"},
			AccessToken: "token-xyz",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", "Manual device", "manual-id")
	if got := client.authHeader(); !strings.Contains(got, `Version="`+buildinfo.Current()+`"`) {
		t.Fatalf("authorization header has wrong client version: %q", got)
	}
	res, err := client.Login(context.Background(), "alice", "secret")
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if res.AccessToken != "token-xyz" || res.User.ID != "user-123" {
		t.Errorf("unexpected login result: %+v", res)
	}
	if client.Token() != "token-xyz" || client.UserID() != "user-123" {
		t.Errorf("client state not updated: token=%q, user=%q", client.Token(), client.UserID())
	}
}

func TestClient_GetMusicLibraries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), `Token="token-xyz"`) {
			t.Errorf("missing auth token header: %q", r.Header.Get("Authorization"))
		}
		resp := itemsResponse[Library]{
			Items: []Library{
				{ID: "lib-1", Name: "Music", CollectionType: "music"},
				{ID: "lib-2", Name: "Movies", CollectionType: "movies"},
				{ID: "lib-3", Name: "Songs", CollectionType: ""},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-xyz", "user-123", "Mac Mini (Spruce)", "spruce-mac-mini-a41c29ef")
	libs, err := client.GetMusicLibraries(context.Background())
	if err != nil {
		t.Fatalf("GetMusicLibraries error: %v", err)
	}
	if len(libs) != 1 {
		t.Fatalf("expected 1 music library (strict CollectionType==music), got %d", len(libs))
	}
	if libs[0].Name != "Music" {
		t.Errorf("unexpected library: %+v", libs)
	}
}

func TestClient_GetArtistsAlbumsTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentID := r.URL.Query().Get("ParentId")
		itemTypes := r.URL.Query().Get("IncludeItemTypes")
		if itemTypes == "Audio" && r.URL.Query().Get("EnableUserData") != "true" {
			t.Fatalf("EnableUserData = %q, want true", r.URL.Query().Get("EnableUserData"))
		}
		if itemTypes == "MusicAlbum" && r.URL.Query().Get("EnableUserData") != "true" {
			t.Fatalf("EnableUserData = %q, want true", r.URL.Query().Get("EnableUserData"))
		}

		switch {
		case itemTypes == "MusicArtist,Artist" && parentID == "lib-1":
			_ = json.NewEncoder(w).Encode(itemsResponse[Artist]{
				Items: []Artist{{ID: "art-1", Name: "Pink Floyd"}},
			})
		case itemTypes == "MusicAlbum" && parentID == "art-1":
			_ = json.NewEncoder(w).Encode(itemsResponse[Album]{
				Items: []Album{{ID: "alb-1", Name: "Dark Side of the Moon", ProductionYear: 1973}},
			})
		case itemTypes == "Audio" && parentID == "alb-1":
			_ = json.NewEncoder(w).Encode(itemsResponse[Track]{
				Items: []Track{
					{ID: "trk-12", Name: "Cemetery Drive", ParentIndexNumber: 1, IndexNumber: 12},
					{ID: "trk-2", Name: "Give 'Em Hell, Kid", ParentIndexNumber: 1, IndexNumber: 2},
					{ID: "trk-10", Name: "Hang 'Em High", ParentIndexNumber: 1, IndexNumber: 10},
					{ID: "trk-1", Name: "Helena", ParentIndexNumber: 1, IndexNumber: 1, RunTimeTicks: 4230000000, Artists: []string{"Pink Floyd"}},
				},
			})
		case itemTypes == "Audio" && parentID == "lib-1":
			_ = json.NewEncoder(w).Encode(itemsResponse[Track]{
				Items: []Track{{ID: "trk-1", Name: "Time", RunTimeTicks: 4230000000, Artists: []string{"Pink Floyd"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-xyz", "user-123", "Test device", "test-device-1")
	ctx := context.Background()

	artists, err := client.GetArtists(ctx, "lib-1")
	if err != nil || len(artists) != 1 || artists[0].Name != "Pink Floyd" {
		t.Fatalf("GetArtists failed: %+v, err=%v", artists, err)
	}

	allTracks, err := client.GetAllTracks(ctx, "lib-1")
	if err != nil || len(allTracks) != 1 || allTracks[0].Name != "Time" {
		t.Fatalf("GetAllTracks failed: %+v, err=%v", allTracks, err)
	}

	albums, err := client.GetAlbums(ctx, "art-1")
	if err != nil || len(albums) != 1 || albums[0].Name != "Dark Side of the Moon" {
		t.Fatalf("GetAlbums failed: %+v, err=%v", albums, err)
	}

	tracks, err := client.GetTracks(ctx, "alb-1")
	if err != nil || len(tracks) != 4 || tracks[0].Name != "Helena" || tracks[1].Name != "Give 'Em Hell, Kid" || tracks[2].Name != "Hang 'Em High" || tracks[3].Name != "Cemetery Drive" {
		t.Fatalf("GetTracks failed: %+v, err=%v", tracks, err)
	}
	if tracks[0].Duration() != 423.0 {
		t.Errorf("expected track duration 423s, got %f", tracks[0].Duration())
	}
	if tracks[0].DisplayArtist() != "Pink Floyd" {
		t.Errorf("expected DisplayArtist 'Pink Floyd', got %q", tracks[0].DisplayArtist())
	}
}

func TestClient_GetPlaylists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users/user-123/Items":
			if r.URL.Query().Get("IncludeItemTypes") != "Playlist" {
				t.Fatalf("IncludeItemTypes = %q, want Playlist", r.URL.Query().Get("IncludeItemTypes"))
			}
			_ = json.NewEncoder(w).Encode(itemsResponse[Playlist]{
				Items:            []Playlist{{ID: "pl-1", Name: "Favorites", Count: 2}},
				TotalRecordCount: 1,
			})
		case "/Playlists/pl-1/Items":
			if r.URL.Query().Get("userId") != "user-123" {
				t.Fatalf("userId = %q, want user-123", r.URL.Query().Get("userId"))
			}
			if r.URL.Query().Get("enableUserData") != "true" {
				t.Fatalf("enableUserData = %q, want true", r.URL.Query().Get("enableUserData"))
			}
			_ = json.NewEncoder(w).Encode(itemsResponse[Track]{
				Items:            []Track{{ID: "trk-1", Name: "One"}, {ID: "trk-2", Name: "Two"}},
				TotalRecordCount: 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-xyz", "user-123", "Test device", "test-device-2")
	playlists, err := client.GetPlaylists(context.Background())
	if err != nil {
		t.Fatalf("GetPlaylists error: %v", err)
	}
	if len(playlists) != 1 || playlists[0].Name != "Favorites" || playlists[0].Count != 2 {
		t.Fatalf("unexpected playlists: %+v", playlists)
	}

	tracks, err := client.GetPlaylistTracks(context.Background(), "pl-1")
	if err != nil {
		t.Fatalf("GetPlaylistTracks error: %v", err)
	}
	if len(tracks) != 2 || tracks[1].Name != "Two" {
		t.Fatalf("unexpected playlist tracks: %+v", tracks)
	}
}

func TestClient_SetFavorite(t *testing.T) {
	tests := []struct {
		name     string
		favorite bool
		method   string
	}{
		{name: "mark", favorite: true, method: http.MethodPost},
		{name: "unmark", favorite: false, method: http.MethodDelete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Fatalf("method = %q, want %q", r.Method, tt.method)
				}
				if r.URL.EscapedPath() != "/UserFavoriteItems/track%2F1" {
					t.Fatalf("path = %q, want escaped item path", r.URL.EscapedPath())
				}
				if r.URL.Query().Get("userId") != "user-123" {
					t.Fatalf("userId = %q, want user-123", r.URL.Query().Get("userId"))
				}
				_ = json.NewEncoder(w).Encode(UserItemData{IsFavorite: tt.favorite})
			}))
			defer server.Close()

			client := NewClient(server.URL, "token-xyz", "user-123", "Test device", "test-device")
			got, err := client.SetFavorite(context.Background(), "track/1", tt.favorite)
			if err != nil {
				t.Fatalf("SetFavorite error: %v", err)
			}
			if got.IsFavorite != tt.favorite {
				t.Fatalf("IsFavorite = %v, want %v", got.IsFavorite, tt.favorite)
			}
		})
	}
}

func TestClient_StreamHelpersAndProgress(t *testing.T) {
	progressCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Sessions/Playing/Progress" {
			progressCalled = true
			var req PlaybackProgressRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.ItemID != "trk-1" || req.PositionTicks != 120000000 {
				t.Errorf("unexpected progress req: %+v", req)
			}
			if req.PlayMethod != "DirectPlay" {
				t.Errorf("expected PlayMethod DirectPlay, got %q", req.PlayMethod)
			}
			if req.PlaySessionId != "sess-1" {
				t.Errorf("expected PlaySessionId sess-1, got %q", req.PlaySessionId)
			}
			if req.MediaSourceId != "trk-1" {
				t.Errorf("expected MediaSourceId trk-1, got %q", req.MediaSourceId)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token-xyz", "user-123", "Mac Mini (Spruce)", "spruce-mac-mini-a41c29ef")
	streamURL := client.StreamURL("trk-1", "test-session")
	wantQuery := "deviceId=spruce-mac-mini-a41c29ef&playSessionId=test-session&static=true"
	if streamURL != server.URL+"/Audio/trk-1/stream?"+wantQuery {
		t.Errorf("unexpected StreamURL: %q", streamURL)
	}
	if strings.Contains(streamURL, "token-xyz") {
		t.Errorf("StreamURL must not embed auth token: %q", streamURL)
	}
	headers := client.StreamHeaders()
	if len(headers) != 1 || !strings.HasPrefix(headers[0], "Authorization: MediaBrowser") {
		t.Errorf("unexpected StreamHeaders: %v", headers)
	}
	if !strings.Contains(headers[0], `Token="token-xyz"`) {
		t.Errorf("StreamHeaders missing auth token: %v", headers)
	}
	if !strings.Contains(headers[0], `Device="Mac Mini (Spruce)"`) || !strings.Contains(headers[0], `DeviceId="spruce-mac-mini-a41c29ef"`) {
		t.Errorf("unexpected device identity header: %v", headers)
	}

	err := client.ReportPlaybackProgress(context.Background(), "trk-1", 12.0, false, "sess-1")
	if err != nil {
		t.Fatalf("ReportPlaybackProgress error: %v", err)
	}
	if !progressCalled {
		t.Error("progress endpoint was not called")
	}
}
