package abs

import (
	"encoding/json"
	"testing"
)

func TestTotalDuration_MediaDuration(t *testing.T) {
	dur := 55908.2
	m := Media{Duration: &dur}
	if got := m.TotalDuration(); got != 55908.2 {
		t.Errorf("TotalDuration() = %f, want 55908.2", got)
	}
}

func TestTotalDuration_MetadataFallback(t *testing.T) {
	dur := 3600.0
	m := Media{Metadata: MediaMetadata{Duration: &dur}}
	if got := m.TotalDuration(); got != 3600.0 {
		t.Errorf("TotalDuration() = %f, want 3600.0", got)
	}
}

func TestTotalDuration_MediaTakesPrecedence(t *testing.T) {
	mediaDur := 100.0
	metaDur := 200.0
	m := Media{Duration: &mediaDur, Metadata: MediaMetadata{Duration: &metaDur}}
	if got := m.TotalDuration(); got != 100.0 {
		t.Errorf("TotalDuration() = %f, want 100.0 (media.duration should take precedence)", got)
	}
}

func TestTotalDuration_NeitherSet(t *testing.T) {
	m := Media{}
	if got := m.TotalDuration(); got != 0 {
		t.Errorf("TotalDuration() = %f, want 0", got)
	}
}

func TestHasDuration(t *testing.T) {
	dur := 100.0
	zero := 0.0

	tests := []struct {
		name string
		m    Media
		want bool
	}{
		{"media duration set", Media{Duration: &dur}, true},
		{"metadata duration set", Media{Metadata: MediaMetadata{Duration: &dur}}, true},
		{"zero media duration", Media{Duration: &zero}, false},
		{"no duration", Media{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.HasDuration(); got != tt.want {
				t.Errorf("HasDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaMetadataUnmarshalAuthors(t *testing.T) {
	var meta MediaMetadata
	err := json.Unmarshal([]byte(`{
		"title":"Book",
		"authorName":"Fallback Author",
		"authors":[
			{"id":"a1","name":"Author One"},
			{"id":"a2","name":"Author Two"}
		]
	}`), &meta)
	if err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if len(meta.Authors) != 2 {
		t.Fatalf("len(Authors) = %d, want 2", len(meta.Authors))
	}
	if meta.Authors[0].ID != "a1" || meta.Authors[0].Name != "Author One" {
		t.Fatalf("Authors[0] = %+v", meta.Authors[0])
	}
	if got := meta.DisplayAuthor(); got != "Author One, Author Two" {
		t.Fatalf("DisplayAuthor() = %q, want joined authors", got)
	}
	if !meta.HasMultipleAuthors() {
		t.Fatal("HasMultipleAuthors() = false, want true")
	}
}

func TestMediaMetadataUnmarshalPodcastAuthor(t *testing.T) {
	var meta MediaMetadata
	err := json.Unmarshal([]byte(`{
		"title":"Podcast",
		"author":"Podcast Author"
	}`), &meta)
	if err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.AuthorName == nil || *meta.AuthorName != "Podcast Author" {
		t.Fatalf("AuthorName = %v, want Podcast Author", meta.AuthorName)
	}
	if got := meta.DisplayAuthor(); got != "Podcast Author" {
		t.Fatalf("DisplayAuthor() = %q, want Podcast Author", got)
	}
}

func TestMediaMetadataUnmarshalAuthorNameTakesPrecedence(t *testing.T) {
	var meta MediaMetadata
	err := json.Unmarshal([]byte(`{
		"title":"Podcast",
		"author":"Podcast Author",
		"authorName":"Book Fallback"
	}`), &meta)
	if err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.AuthorName == nil || *meta.AuthorName != "Book Fallback" {
		t.Fatalf("AuthorName = %v, want Book Fallback", meta.AuthorName)
	}
}

func TestMediaMetadataDisplayAuthorFallback(t *testing.T) {
	fallback := "Fallback Author"
	meta := MediaMetadata{AuthorName: &fallback}
	if got := meta.DisplayAuthor(); got != fallback {
		t.Fatalf("DisplayAuthor() = %q, want %q", got, fallback)
	}

	empty := ""
	meta = MediaMetadata{AuthorName: &empty}
	if got := meta.DisplayAuthor(); got != "Unknown author" {
		t.Fatalf("DisplayAuthor() = %q, want Unknown author", got)
	}
}

func TestMediaMetadataUnmarshalSeriesObject(t *testing.T) {
	var meta MediaMetadata
	err := json.Unmarshal([]byte(`{
		"title":"Book",
		"series":{"id":"s1","name":"Series One","sequence":"2"}
	}`), &meta)
	if err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.Series == nil {
		t.Fatal("Series is nil, want primary series")
	}
	if meta.Series.ID != "s1" || meta.Series.Name != "Series One" || meta.Series.Sequence != "2" {
		t.Fatalf("Series = %+v", *meta.Series)
	}
	if len(meta.SeriesList) != 1 {
		t.Fatalf("len(SeriesList) = %d, want 1", len(meta.SeriesList))
	}
	if got := meta.PrimarySeries(); got == nil || got.ID != "s1" {
		t.Fatalf("PrimarySeries() = %+v, want s1", got)
	}
}

func TestMediaMetadataUnmarshalSeriesArray(t *testing.T) {
	var meta MediaMetadata
	err := json.Unmarshal([]byte(`{
		"title":"Book",
		"series":[
			{"id":"s1","name":"Series One","sequence":"2"},
			{"id":"s2","name":"Series Two","sequence":"1"}
		]
	}`), &meta)
	if err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.Series == nil {
		t.Fatal("Series is nil, want primary first series")
	}
	if meta.Series.ID != "s1" {
		t.Fatalf("Series.ID = %q, want s1", meta.Series.ID)
	}
	if len(meta.SeriesList) != 2 {
		t.Fatalf("len(SeriesList) = %d, want 2", len(meta.SeriesList))
	}
	if !meta.HasMultipleSeries() {
		t.Fatal("HasMultipleSeries() = false, want true")
	}
	if got := meta.SeriesByID("s2"); got == nil || got.Name != "Series Two" {
		t.Fatalf("SeriesByID(s2) = %+v, want Series Two", got)
	}
}
