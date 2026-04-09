package search

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/sahilm/fuzzy"
)

const (
	defaultCacheTTL   = 15 * time.Minute
	snapshotPageLimit = 100
)

// Cache keeps lightweight per-library search snapshots in memory.
type Cache struct {
	client *abs.Client
	ttl    time.Duration
	now    func() time.Time

	mu        sync.Mutex
	snapshots map[string]*librarySnapshot
	builds    map[string]*snapshotBuild
}

type snapshotBuild struct {
	done     chan struct{}
	snapshot *librarySnapshot
	err      error
}

type librarySnapshot struct {
	libraryID      string
	mediaType      string
	builtAt        time.Time
	lastAccessedAt time.Time
	entries        []snapshotEntry
}

type snapshotEntry struct {
	itemID    string
	libraryID string
	mediaType string
	title     string
	author    string
	duration  float64

	podcastTitle    string
	episodeID       string
	episodeTitle    string
	episodeDuration float64

	primarySearchText   string
	secondarySearchText string
	combinedSearchText  string
	fuzzySearchText     string
	primaryTokens       []string
	combinedTokens      []string
}

// NewCache creates a new in-memory search cache.
func NewCache(client *abs.Client) *Cache {
	return &Cache{
		client:    client,
		ttl:       defaultCacheTTL,
		now:       time.Now,
		snapshots: make(map[string]*librarySnapshot),
		builds:    make(map[string]*snapshotBuild),
	}
}

// Prepare ensures the library snapshot exists before the user starts searching.
func (c *Cache) Prepare(ctx context.Context, libraryID, libraryMediaType string) error {
	if _, err := c.ensureSnapshot(ctx, libraryID, libraryMediaType); err != nil {
		return err
	}
	return nil
}

// Search returns search results from a cached per-library snapshot.
func (c *Cache) Search(ctx context.Context, libraryID, libraryMediaType, query string) ([]abs.LibraryItem, error) {
	normalized := normalizeQuery(query)
	if normalized == "" {
		return nil, nil
	}

	snapshot, err := c.ensureSnapshot(ctx, libraryID, libraryMediaType)
	if err != nil {
		return nil, err
	}

	return snapshot.filter(normalized), nil
}

func (c *Cache) ensureSnapshot(ctx context.Context, libraryID, libraryMediaType string) (*librarySnapshot, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	resolvedID, resolvedMediaType, err := c.resolveLibrary(ctx, libraryID, libraryMediaType)
	if err != nil {
		return nil, err
	}
	if resolvedID == "" {
		return &librarySnapshot{libraryID: "", mediaType: resolvedMediaType}, nil
	}

	now := c.now()

	c.mu.Lock()
	if snapshot, ok := c.snapshots[resolvedID]; ok && snapshot.mediaType == resolvedMediaType && now.Sub(snapshot.builtAt) < c.ttl {
		snapshot.lastAccessedAt = now
		c.mu.Unlock()
		return snapshot, nil
	}
	if build, ok := c.builds[resolvedID]; ok {
		c.mu.Unlock()
		select {
		case <-build.done:
			return build.snapshot, build.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	build := &snapshotBuild{done: make(chan struct{})}
	c.builds[resolvedID] = build
	c.mu.Unlock()

	snapshot, err := c.buildSnapshot(ctx, resolvedID, resolvedMediaType)
	if err == nil {
		snapshot.builtAt = now
		snapshot.lastAccessedAt = now
	}

	c.mu.Lock()
	delete(c.builds, resolvedID)
	if err == nil {
		c.snapshots[resolvedID] = snapshot
	}
	build.snapshot = snapshot
	build.err = err
	close(build.done)
	c.mu.Unlock()

	return snapshot, err
}

func (c *Cache) buildSnapshot(ctx context.Context, libraryID, libraryMediaType string) (*librarySnapshot, error) {
	switch libraryMediaType {
	case "podcast":
		return c.buildPodcastSnapshot(ctx, libraryID)
	default:
		return c.buildBookSnapshot(ctx, libraryID)
	}
}

func (c *Cache) buildBookSnapshot(ctx context.Context, libraryID string) (*librarySnapshot, error) {
	entries := make([]snapshotEntry, 0)
	page := 0

	for {
		resp, err := c.client.GetLibraryItems(ctx, libraryID, page, snapshotPageLimit)
		if err != nil {
			return nil, fmt.Errorf("list library items: %w", err)
		}

		for _, item := range resp.Results {
			author := ""
			if item.Media.Metadata.AuthorName != nil {
				author = *item.Media.Metadata.AuthorName
			}
			entries = append(entries, snapshotEntry{
				itemID:              item.ID,
				libraryID:           item.LibraryID,
				mediaType:           item.MediaType,
				title:               item.Media.Metadata.Title,
				author:              author,
				duration:            item.Media.TotalDuration(),
				primarySearchText:   normalizeSearchText(item.Media.Metadata.Title),
				secondarySearchText: normalizeSearchText(author),
				combinedSearchText:  combineSearchText(item.Media.Metadata.Title, author),
				fuzzySearchText:     compactNormalizedText(combineSearchText(item.Media.Metadata.Title, author)),
				primaryTokens:       tokenizeSearchText(normalizeSearchText(item.Media.Metadata.Title)),
				combinedTokens:      tokenizeSearchText(combineSearchText(item.Media.Metadata.Title, author)),
			})
		}

		if len(resp.Results) == 0 || len(resp.Results) < snapshotPageLimit {
			break
		}
		loaded := (page + 1) * snapshotPageLimit
		if resp.Total > 0 && loaded >= resp.Total {
			break
		}
		page++
	}

	return &librarySnapshot{
		libraryID: libraryID,
		mediaType: "book",
		entries:   entries,
	}, nil
}

func (c *Cache) buildPodcastSnapshot(ctx context.Context, libraryID string) (*librarySnapshot, error) {
	entries := make([]snapshotEntry, 0)
	page := 0

	for {
		resp, err := c.client.GetLibraryItems(ctx, libraryID, page, snapshotPageLimit)
		if err != nil {
			return nil, fmt.Errorf("list podcast library items: %w", err)
		}

		ids := make([]string, len(resp.Results))
		for i, item := range resp.Results {
			ids[i] = item.ID
		}
		fullItems, err := c.client.GetLibraryItemsBatch(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, fullItem := range fullItems {
			for _, episode := range fullItem.Media.Episodes {
				entries = append(entries, snapshotEntry{
					itemID:              fullItem.ID,
					libraryID:           fullItem.LibraryID,
					mediaType:           "podcast",
					podcastTitle:        fullItem.Media.Metadata.Title,
					episodeID:           episode.ID,
					episodeTitle:        episode.Title,
					episodeDuration:     episode.Duration,
					primarySearchText:   normalizeSearchText(episode.Title),
					secondarySearchText: normalizeSearchText(fullItem.Media.Metadata.Title),
					combinedSearchText:  combineSearchText(episode.Title, fullItem.Media.Metadata.Title),
					fuzzySearchText:     compactNormalizedText(combineSearchText(episode.Title, fullItem.Media.Metadata.Title)),
					primaryTokens:       tokenizeSearchText(normalizeSearchText(episode.Title)),
					combinedTokens:      tokenizeSearchText(combineSearchText(episode.Title, fullItem.Media.Metadata.Title)),
				})
			}
		}

		if len(resp.Results) == 0 || len(resp.Results) < snapshotPageLimit {
			break
		}
		loaded := (page + 1) * snapshotPageLimit
		if resp.Total > 0 && loaded >= resp.Total {
			break
		}
		page++
	}

	return &librarySnapshot{
		libraryID: libraryID,
		mediaType: "podcast",
		entries:   entries,
	}, nil
}

func (c *Cache) resolveLibrary(ctx context.Context, libraryID, libraryMediaType string) (string, string, error) {
	if libraryID != "" && libraryMediaType != "" {
		return libraryID, libraryMediaType, nil
	}

	libs, err := c.client.GetLibraries(ctx)
	if err != nil {
		return "", "", fmt.Errorf("fetch libraries: %w", err)
	}
	libs, _ = c.client.FilterAudioLibraries(ctx, libs)
	if len(libs) == 0 {
		return "", "", nil
	}
	if libraryID == "" {
		return libs[0].ID, libs[0].MediaType, nil
	}
	for _, lib := range libs {
		if lib.ID == libraryID {
			return lib.ID, lib.MediaType, nil
		}
	}
	return "", "", nil
}

func (s *librarySnapshot) filter(normalizedQuery string) []abs.LibraryItem {
	queryTokens := tokenizeSearchText(normalizedQuery)
	compactQuery := compactNormalizedText(normalizedQuery)
	fuzzyScores := make(map[int]int)
	fuzzyMatched := make(map[int]bool)
	if len(compactQuery) >= 2 {
		for _, match := range fuzzy.FindFromNoSort(compactQuery, snapshotSource{s.entries}) {
			fuzzyScores[match.Index] = match.Score
			fuzzyMatched[match.Index] = true
		}
	}

	candidates := make([]rankedSnapshotEntry, 0)
	for idx, entry := range s.entries {
		score, ok := entry.matchScore(normalizedQuery, queryTokens, fuzzyScores[idx], fuzzyMatched[idx])
		if !ok {
			continue
		}
		candidates = append(candidates, rankedSnapshotEntry{
			index: idx,
			score: score,
			entry: entry,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].index < candidates[j].index
		}
		return candidates[i].score > candidates[j].score
	})

	results := make([]abs.LibraryItem, 0, len(candidates))
	for _, candidate := range candidates {
		switch candidate.entry.mediaType {
		case "podcast":
			results = append(results, candidate.entry.podcastResult())
		default:
			results = append(results, candidate.entry.bookResult())
		}
	}
	return results
}

type snapshotSource struct {
	entries []snapshotEntry
}

func (s snapshotSource) Len() int {
	return len(s.entries)
}

func (s snapshotSource) String(i int) string {
	return s.entries[i].fuzzySearchText
}

type rankedSnapshotEntry struct {
	index int
	score int
	entry snapshotEntry
}

func (e snapshotEntry) matchScore(normalizedQuery string, queryTokens []string, fuzzyScore int, fuzzyHit bool) (int, bool) {
	score := 0
	matched := false

	if idx := strings.Index(e.primarySearchText, normalizedQuery); idx >= 0 {
		score += 100000 - idx
		matched = true
	}
	if idx := strings.Index(e.secondarySearchText, normalizedQuery); idx >= 0 {
		score += 70000 - idx
		matched = true
	}
	if idx := strings.Index(e.combinedSearchText, normalizedQuery); idx >= 0 {
		score += 85000 - idx
		matched = true
	}
	if allTokensPrefixMatch(queryTokens, e.primaryTokens) {
		score += 90000 + tokenPrefixBonus(queryTokens, e.primaryTokens)
		matched = true
	}
	if allTokensPrefixMatch(queryTokens, e.combinedTokens) {
		score += 80000 + tokenPrefixBonus(queryTokens, e.combinedTokens)
		matched = true
	}
	if fuzzyHit {
		score += 5000 + fuzzyScore
		matched = true
	}

	return score, matched
}

func allTokensPrefixMatch(queryTokens, candidateTokens []string) bool {
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return false
	}
	for _, queryToken := range queryTokens {
		found := false
		for _, candidateToken := range candidateTokens {
			if strings.HasPrefix(candidateToken, queryToken) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func tokenPrefixBonus(queryTokens, candidateTokens []string) int {
	bonus := 0
	for _, queryToken := range queryTokens {
		for _, candidateToken := range candidateTokens {
			if strings.HasPrefix(candidateToken, queryToken) {
				bonus += max(1, len(queryToken))*10 - (len(candidateToken) - len(queryToken))
				break
			}
		}
	}
	return bonus
}

func (e snapshotEntry) bookResult() abs.LibraryItem {
	var author *string
	if e.author != "" {
		authorValue := e.author
		author = &authorValue
	}
	var duration *float64
	if e.duration > 0 {
		durationValue := e.duration
		duration = &durationValue
	}
	return abs.LibraryItem{
		ID:        e.itemID,
		LibraryID: e.libraryID,
		MediaType: "book",
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title:      e.title,
				AuthorName: author,
				Duration:   duration,
			},
		},
	}
}

func (e snapshotEntry) podcastResult() abs.LibraryItem {
	episode := abs.PodcastEpisode{
		ID:       e.episodeID,
		Title:    e.episodeTitle,
		Duration: e.episodeDuration,
	}
	return abs.LibraryItem{
		ID:        e.itemID,
		LibraryID: e.libraryID,
		MediaType: "podcast",
		RecentEpisode: &abs.PodcastEpisode{
			ID:       episode.ID,
			Title:    episode.Title,
			Duration: episode.Duration,
		},
		Media: abs.Media{
			Metadata: abs.MediaMetadata{
				Title: e.podcastTitle,
			},
			Episodes: []abs.PodcastEpisode{episode},
		},
	}
}

func normalizeQuery(query string) string {
	return normalizeSearchText(query)
}

func normalizeSearchText(text string) string {
	var builder strings.Builder
	lastWasSpace := true
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastWasSpace = false
		case !lastWasSpace:
			builder.WriteByte(' ')
			lastWasSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func compactNormalizedText(text string) string {
	return strings.ReplaceAll(text, " ", "")
}

func combineSearchText(parts ...string) string {
	normalizedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := normalizeSearchText(part)
		if normalized == "" {
			continue
		}
		normalizedParts = append(normalizedParts, normalized)
	}
	return strings.Join(normalizedParts, " ")
}

func tokenizeSearchText(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Fields(text)
}
