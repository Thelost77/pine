package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/db"
)

// ---------------------------------------------------------------------------
// Test data fixtures
// ---------------------------------------------------------------------------

var testDuration = 3600.0
var testAuthor = "Test Author"
var testDescription = "A test audiobook for E2E testing"
var testPodcastAuthor = "Podcast Host"
var testEpisodeDuration = 1800.0

func testLibraryItem(id, title string) abs.LibraryItem {
	return abs.LibraryItem{
		ID:        id,
		MediaType: "book",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:       title,
				AuthorName:  &testAuthor,
				Description: &testDescription,
				Duration:    &testDuration,
				Chapters: []abs.Chapter{
					{ID: 0, Start: 0, End: 1800, Title: "Chapter 1"},
					{ID: 1, Start: 1800, End: 3600, Title: "Chapter 2"},
				},
			},
		},
		UserMediaProgress: &abs.UserMediaProgress{
			CurrentTime: 42.0,
			Progress:    0.0117,
			IsFinished:  false,
		},
	}
}

func testPodcastItem(id, title string) abs.LibraryItem {
	return abs.LibraryItem{
		ID:        id,
		MediaType: "podcast",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:      title,
				AuthorName: &testPodcastAuthor,
			},
			Episodes: []abs.PodcastEpisode{
				{
					ID:       id + "-ep-001",
					Index:    1,
					Title:    "Episode 1 - Pilot",
					Duration: testEpisodeDuration,
					AudioTrack: abs.AudioTrack{
						Index:       0,
						StartOffset: 0,
						Duration:    testEpisodeDuration,
						ContentURL:  fmt.Sprintf("/s/item/%s/ep1.mp3", id),
					},
				},
				{
					ID:       id + "-ep-002",
					Index:    2,
					Title:    "Episode 2 - Deep Dive",
					Duration: 2400.0,
					AudioTrack: abs.AudioTrack{
						Index:       0,
						StartOffset: 0,
						Duration:    2400.0,
						ContentURL:  fmt.Sprintf("/s/item/%s/ep2.mp3", id),
					},
				},
			},
		},
	}
}

func testBookmark(title string, seconds float64) abs.Bookmark {
	return abs.Bookmark{
		Title:     title,
		Time:      seconds,
		CreatedAt: 1700000000000,
	}
}

// ---------------------------------------------------------------------------
// Full mock ABS server
// ---------------------------------------------------------------------------

// e2eServerState holds mutable state for the mock server.
type e2eServerState struct {
	mu        sync.Mutex
	bookmarks map[string][]abs.Bookmark // itemID → bookmarks
	force401  bool                      // when true, next API call returns 401
}

func (s *e2eServerState) setForce401(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.force401 = v
}

func (s *e2eServerState) getForce401() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.force401 {
		s.force401 = false
		return true
	}
	return false
}

func (s *e2eServerState) getBookmarks(itemID string) []abs.Bookmark {
	s.mu.Lock()
	defer s.mu.Unlock()
	bms := s.bookmarks[itemID]
	cp := make([]abs.Bookmark, len(bms))
	copy(cp, bms)
	return cp
}

func (s *e2eServerState) addBookmark(itemID string, bm abs.Bookmark) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bookmarks[itemID] = append(s.bookmarks[itemID], bm)
}

func (s *e2eServerState) deleteBookmark(itemID string, bmTime float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bms := s.bookmarks[itemID]
	for i, b := range bms {
		if b.Time == bmTime {
			s.bookmarks[itemID] = append(bms[:i], bms[i+1:]...)
			return
		}
	}
}

// newFullMockABSServer creates a comprehensive mock ABS server handling all endpoints.
func newFullMockABSServer(log *apiLog, state *e2eServerState) *httptest.Server {
	items := []abs.LibraryItem{
		testLibraryItem("item-001", "The Great Gatsby"),
		testLibraryItem("item-002", "1984"),
		testLibraryItem("item-003", "Brave New World"),
	}

	podcastItems := []abs.LibraryItem{
		testPodcastItem("pod-001", "Tech Talk Daily"),
		testPodcastItem("pod-002", "Science Hour"),
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		log.record(r.Method, r.URL.Path, body)

		w.Header().Set("Content-Type", "application/json")

		// Force 401 for testing unauthorized redirect
		if state.getForce401() {
			http.Error(w, `{"error": "unauthorized status 401"}`, http.StatusUnauthorized)
			return
		}

		switch {
		// --- Auth ---
		case r.Method == http.MethodPost && r.URL.Path == "/login":
			resp := abs.LoginResponse{User: abs.LoginUser{
				ID: "usr-001", Username: "alice", Token: "jwt-token-e2e",
			}}
			json.NewEncoder(w).Encode(resp)

		// --- Libraries ---
		case r.Method == http.MethodGet && r.URL.Path == "/api/libraries":
			resp := struct {
				Libraries []abs.Library `json:"libraries"`
			}{
				Libraries: []abs.Library{
					{ID: "lib-001", Name: "Audiobooks", MediaType: "book"},
					{ID: "lib-pods", Name: "Podcasts", MediaType: "podcast"},
				},
			}
			json.NewEncoder(w).Encode(resp)

		// --- Personalized (book library) ---
		case r.Method == http.MethodGet && r.URL.Path == "/api/libraries/lib-001/personalized":
			resp := []abs.PersonalizedResponse{
				{
					ID:       "continue-listening",
					Entities: items[:2],
				},
			}
			json.NewEncoder(w).Encode(resp)

		// --- Personalized (podcast library) ---
		case r.Method == http.MethodGet && r.URL.Path == "/api/libraries/lib-pods/personalized":
			resp := []abs.PersonalizedResponse{
				{
					ID:       "continue-listening",
					Entities: podcastItems[:1],
				},
			}
			json.NewEncoder(w).Encode(resp)

		// --- Library items (books) ---
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/libraries/lib-001/items"):
			resp := abs.LibraryItemsResponse{
				Results: items,
				Total:   len(items),
				Limit:   20,
				Page:    0,
			}
			json.NewEncoder(w).Encode(resp)

		// --- Library items (podcasts) ---
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/libraries/lib-pods/items"):
			resp := abs.LibraryItemsResponse{
				Results: podcastItems,
				Total:   len(podcastItems),
				Limit:   20,
				Page:    0,
			}
			json.NewEncoder(w).Encode(resp)

		// --- Get single item (expanded, for podcast episodes) ---
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/items/"):
			itemID := strings.TrimPrefix(r.URL.Path, "/api/items/")
			// Remove query params
			if idx := strings.Index(itemID, "?"); idx >= 0 {
				itemID = itemID[:idx]
			}
			// Check if this is a play request (handled below)
			if strings.Contains(itemID, "/play/") {
				break
			}
			// Find the item
			for _, item := range append(items, podcastItems...) {
				if item.ID == itemID {
					json.NewEncoder(w).Encode(item)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"item not found"}`))

		// --- Search (book library) ---
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/libraries/lib-001/search"):
			q := r.URL.Query().Get("q")
			var matched []abs.SearchResultEntry
			for _, item := range items {
				if strings.Contains(strings.ToLower(item.Media.Metadata.Title), strings.ToLower(q)) {
					matched = append(matched, abs.SearchResultEntry{LibraryItem: item, MatchKey: "title", MatchText: item.Media.Metadata.Title})
				}
			}
			resp := abs.SearchResult{Book: matched}
			json.NewEncoder(w).Encode(resp)

		// --- Search (podcast library) ---
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/libraries/lib-pods/search"):
			q := r.URL.Query().Get("q")
			var matched []abs.SearchResultEntry
			for _, item := range podcastItems {
				if strings.Contains(strings.ToLower(item.Media.Metadata.Title), strings.ToLower(q)) {
					matched = append(matched, abs.SearchResultEntry{LibraryItem: item, MatchKey: "title", MatchText: item.Media.Metadata.Title})
				}
			}
			resp := abs.SearchResult{Podcast: matched}
			json.NewEncoder(w).Encode(resp)

		// --- Bookmarks (via progress endpoint) ---
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/me/progress/"):
			itemID := strings.TrimPrefix(r.URL.Path, "/api/me/progress/")
			bms := state.getBookmarks(itemID)
			resp := abs.MediaProgressWithBookmarks{
				LibraryItemID: itemID,
				CurrentTime:   42.0,
				Progress:      0.0117,
				Bookmarks:     bms,
			}
			json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/me/progress/"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		// --- Create bookmark ---
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/bookmark"):
			// Path: /api/me/item/{id}/bookmark
			parts := strings.Split(r.URL.Path, "/")
			// /api/me/item/{id}/bookmark → parts[4] is the item ID
			if len(parts) >= 5 {
				itemID := parts[4]
				bmTime, _ := body["time"].(float64)
				bmTitle, _ := body["title"].(string)
				state.addBookmark(itemID, abs.Bookmark{
					Title:     bmTitle,
					Time:      bmTime,
					CreatedAt: time.Now().UnixMilli(),
				})
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		// --- Delete bookmark ---
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/bookmark/"):
			parts := strings.Split(r.URL.Path, "/")
			if len(parts) >= 6 {
				itemID := parts[4]
				// Parse the time from URL
				var bmTime float64
				fmt.Sscanf(parts[6], "%f", &bmTime)
				state.deleteBookmark(itemID, bmTime)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		// --- Episode playback: POST /api/items/{id}/play/{episodeId} ---
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/play/"):
			parts := strings.Split(r.URL.Path, "/")
			// /api/items/{id}/play/{episodeId} → parts: ["", "api", "items", "{id}", "play", "{epId}"]
			if len(parts) >= 6 {
				itemID := parts[3]
				epID := parts[5]
				resp := abs.PlaySession{
					ID: "sess-ep-e2e",
					AudioTracks: []abs.AudioTrack{
						{Index: 0, ContentURL: fmt.Sprintf("/s/item/%s/%s.mp3", itemID, epID), Duration: 1800},
					},
					CurrentTime: 0,
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}

		// --- Book playback: POST /api/items/{id}/play ---
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/play"):
			// /api/items/{id}/play
			parts := strings.Split(r.URL.Path, "/")
			itemID := parts[3]
			var resp abs.PlaySession
			if itemID == "item-multitrack" {
				resp = abs.PlaySession{
					ID: "sess-mt-e2e",
					AudioTracks: []abs.AudioTrack{
						{Index: 0, StartOffset: 0, ContentURL: "/s/item/item-multitrack/track0.mp3", Duration: 1800},
						{Index: 1, StartOffset: 1800, ContentURL: "/s/item/item-multitrack/track1.mp3", Duration: 1800},
					},
					CurrentTime: 500.0,
					Chapters: []abs.Chapter{
						{ID: 0, Start: 0, End: 1800, Title: "Chapter 1"},
						{ID: 1, Start: 1800, End: 3600, Title: "Chapter 2"},
					},
				}
			} else {
				resp = abs.PlaySession{
					ID: "sess-e2e",
					AudioTracks: []abs.AudioTrack{
						{Index: 0, ContentURL: fmt.Sprintf("/s/item/%s/audio.mp3", itemID), Duration: 3600},
					},
					CurrentTime: 42.0,
					Chapters: []abs.Chapter{
						{ID: 0, Start: 0, End: 1800, Title: "Chapter 1"},
						{ID: 1, Start: 1800, End: 3600, Title: "Chapter 2"},
					},
				}
			}
			json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sync"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/close"):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))

		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
}

// ---------------------------------------------------------------------------
// E2E model constructors
// ---------------------------------------------------------------------------

// newE2EModel creates an unauthenticated model pointed at the mock server.
func newE2EModel(srv *httptest.Server, mp *mockPlayer) Model {
	cfg := config.Default()
	return NewWithPlayer(cfg, nil, nil, mp)
}

// newE2EModelAuthenticated creates an authenticated model pointed at the mock server.
func newE2EModelAuthenticated(srv *httptest.Server, mp *mockPlayer) Model {
	cfg := config.Default()
	client := abs.NewClient(srv.URL, "jwt-token-e2e")
	return NewWithPlayer(cfg, nil, client, mp)
}

// newE2EModelWithDB creates an authenticated model with a real database.
func newE2EModelWithDB(t *testing.T, srv *httptest.Server, mp *mockPlayer) (Model, *db.Store) {
	t.Helper()
	store, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := config.Default()
	client := abs.NewClient(srv.URL, "jwt-token-e2e")
	return NewWithPlayer(cfg, store, client, mp), store
}

// newE2EModelUnauthWithDB creates an unauthenticated model with a real database.
func newE2EModelUnauthWithDB(t *testing.T, srv *httptest.Server, mp *mockPlayer) (Model, *db.Store) {
	t.Helper()
	store, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := config.Default()
	return NewWithPlayer(cfg, store, nil, mp), store
}

// ---------------------------------------------------------------------------
// E2E helpers — driving the Update→Cmd→Msg loop
// ---------------------------------------------------------------------------

// feedCmd executes a tea.Cmd, feeds the resulting message back into Update,
// and returns the new model + the cmd returned by Update.
// Commands that block longer than 200ms (e.g. tea.Tick) are skipped.
func feedCmd(m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	if cmd == nil {
		return m, nil
	}

	type result struct{ msg tea.Msg }
	ch := make(chan result, 1)
	done := make(chan struct{})
	go func() {
		msg := cmd()
		ch <- result{msg: msg}
		close(done)
	}()

	select {
	case r := <-ch:
		// Wait for the goroutine to fully complete to avoid data races
		<-done
		if r.msg == nil {
			return m, nil
		}
		// If it's a batch, execute each sub-command and feed results
		if batch, ok := r.msg.(tea.BatchMsg); ok {
			var lastCmd tea.Cmd
			for _, c := range batch {
				m, lastCmd = feedCmd(m, c)
			}
			return m, lastCmd
		}
		res, nextCmd := m.Update(r.msg)
		return res.(Model), nextCmd
	case <-time.After(200 * time.Millisecond):
		// Drain the channel in case the goroutine completes after timeout
		go func() { <-ch; <-done }()
		return m, nil
	}
}

// feedCmdChain repeatedly executes feedCmd up to maxDepth times, resolving
// command chains automatically. Useful for commands that produce further commands.
func feedCmdChain(m Model, cmd tea.Cmd, maxDepth int) Model {
	for i := 0; i < maxDepth && cmd != nil; i++ {
		m, cmd = feedCmd(m, cmd)
	}
	return m
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

func e2ePressKey(m Model, key rune) (Model, tea.Cmd) {
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
	return res.(Model), cmd
}

func e2ePressSpecial(m Model, keyType tea.KeyType) (Model, tea.Cmd) {
	res, cmd := m.Update(tea.KeyMsg{Type: keyType})
	return res.(Model), cmd
}

func e2eTypeString(m Model, s string) Model {
	for _, r := range s {
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = res.(Model)
	}
	return m
}

// e2eSetSize sends a window size message to ensure rendering works.
func e2eSetSize(m Model, w, h int) Model {
	res, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return res.(Model)
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

func assertScreen(t *testing.T, m Model, expected Screen) {
	t.Helper()
	if m.ActiveScreen() != expected {
		t.Errorf("screen = %v, want %v", m.ActiveScreen(), expected)
	}
}

func assertViewContains(t *testing.T, m Model, substr string) {
	t.Helper()
	v := m.View()
	if !strings.Contains(v, substr) {
		t.Errorf("View() does not contain %q\n--- View output (first 500 chars) ---\n%.500s", substr, v)
	}
}

func assertViewNotContains(t *testing.T, m Model, substr string) {
	t.Helper()
	v := m.View()
	if strings.Contains(v, substr) {
		t.Errorf("View() should not contain %q", substr)
	}
}

func assertBackStack(t *testing.T, m Model, expected []Screen) {
	t.Helper()
	stack := m.BackStack()
	if len(stack) != len(expected) {
		t.Errorf("back stack length = %d, want %d; stack = %v", len(stack), len(expected), stack)
		return
	}
	for i, s := range expected {
		if stack[i] != s {
			t.Errorf("back stack[%d] = %v, want %v", i, stack[i], s)
		}
	}
}

func assertAPICallMade(t *testing.T, log *apiLog, method, pathPrefix string) {
	t.Helper()
	reqs := log.get()
	for _, r := range reqs {
		if r.Method == method && strings.HasPrefix(r.Path, pathPrefix) {
			return
		}
	}
	t.Errorf("expected API call %s %s*, got %d requests: %v", method, pathPrefix, len(reqs), summarizeRequests(reqs))
}

func assertNoAPICall(t *testing.T, log *apiLog, method, pathPrefix string) {
	t.Helper()
	reqs := log.get()
	for _, r := range reqs {
		if r.Method == method && strings.HasPrefix(r.Path, pathPrefix) {
			t.Errorf("unexpected API call %s %s", r.Method, r.Path)
			return
		}
	}
}

func summarizeRequests(reqs []apiRequest) []string {
	s := make([]string, len(reqs))
	for i, r := range reqs {
		s[i] = r.Method + " " + r.Path
	}
	return s
}
