package cache

import (
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/abs"
)

func TestHelpers_Libraries(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := []abs.Library{{ID: "lib1", Name: "Books", MediaType: "book"}}
	if err := s.PutLibraries(want, 5*time.Minute); err != nil {
		t.Fatalf("PutLibraries error: %v", err)
	}

	got, hit, err := s.GetLibraries()
	if err != nil {
		t.Fatalf("GetLibraries error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].ID != "lib1" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHelpers_Personalized(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := []abs.PersonalizedResponse{
		{ID: "continue-listening", Entities: []abs.LibraryItem{{ID: "li1"}}},
	}
	if err := s.PutPersonalized("lib1", want, 5*time.Minute); err != nil {
		t.Fatalf("PutPersonalized error: %v", err)
	}

	got, hit, err := s.GetPersonalized("lib1")
	if err != nil {
		t.Fatalf("GetPersonalized error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].ID != "continue-listening" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHelpers_LibraryItems(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	wantItems := []abs.LibraryItem{{ID: "li1"}, {ID: "li2"}}
	wantTotal := 42
	if err := s.PutLibraryItems("lib1", 2, 50, wantItems, wantTotal, 5*time.Minute); err != nil {
		t.Fatalf("PutLibraryItems error: %v", err)
	}

	gotItems, gotTotal, hit, err := s.GetLibraryItems("lib1", 2, 50)
	if err != nil {
		t.Fatalf("GetLibraryItems error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(gotItems) != 2 || gotTotal != 42 {
		t.Fatalf("got %d items, total %d; want %d items, total %d", len(gotItems), gotTotal, 2, 42)
	}
}

func TestHelpers_LibrarySeries(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	wantItems := []abs.Series{{ID: "s1", Name: "Series A"}}
	wantTotal := 10
	if err := s.PutLibrarySeries("lib1", 1, 50, wantItems, wantTotal, 5*time.Minute); err != nil {
		t.Fatalf("PutLibrarySeries error: %v", err)
	}

	gotItems, gotTotal, hit, err := s.GetLibrarySeries("lib1", 1, 50)
	if err != nil {
		t.Fatalf("GetLibrarySeries error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(gotItems) != 1 || gotTotal != 10 {
		t.Fatalf("got %d items, total %d; want %d items, total %d", len(gotItems), gotTotal, 1, 10)
	}
}

func TestHelpers_SeriesContents(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := []abs.LibraryItem{{ID: "li1"}}
	if err := s.PutSeriesContents("s1", want, 5*time.Minute); err != nil {
		t.Fatalf("PutSeriesContents error: %v", err)
	}

	got, hit, err := s.GetSeriesContents("s1")
	if err != nil {
		t.Fatalf("GetSeriesContents error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].ID != "li1" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHelpers_MediaProgress(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := &abs.MediaProgressWithBookmarks{
		LibraryItemID: "li1",
		CurrentTime:   120,
		Progress:      0.5,
		IsFinished:    false,
		Bookmarks:     []abs.Bookmark{{Title: "bm1", Time: 30}},
	}
	if err := s.PutMediaProgress("li1", want, 5*time.Minute); err != nil {
		t.Fatalf("PutMediaProgress error: %v", err)
	}

	got, hit, err := s.GetMediaProgress("li1")
	if err != nil {
		t.Fatalf("GetMediaProgress error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if got.LibraryItemID != "li1" || got.CurrentTime != 120 || len(got.Bookmarks) != 1 {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHelpers_Bookmarks(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := []abs.Bookmark{{Title: "mark1", Time: 45}}
	if err := s.PutBookmarks("li1", want, 5*time.Minute); err != nil {
		t.Fatalf("PutBookmarks error: %v", err)
	}

	got, hit, err := s.GetBookmarks("li1")
	if err != nil {
		t.Fatalf("GetBookmarks error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].Time != 45 {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHelpers_Episodes(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := []abs.PodcastEpisode{{ID: "ep1", Title: "Pilot"}}
	if err := s.PutEpisodes("li1", want, 5*time.Minute); err != nil {
		t.Fatalf("PutEpisodes error: %v", err)
	}

	got, hit, err := s.GetEpisodes("li1")
	if err != nil {
		t.Fatalf("GetEpisodes error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].Title != "Pilot" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHelpers_RecentEpisodes(t *testing.T) {
	dbStore := openTestDB(t)
	s := NewStore(dbStore)

	want := []abs.LibraryItem{{ID: "li1", MediaType: "podcast"}}
	if err := s.PutRecentEpisodes("lib1", 10, want, 5*time.Minute); err != nil {
		t.Fatalf("PutRecentEpisodes error: %v", err)
	}

	got, hit, err := s.GetRecentEpisodes("lib1", 10)
	if err != nil {
		t.Fatalf("GetRecentEpisodes error: %v", err)
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].ID != "li1" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
