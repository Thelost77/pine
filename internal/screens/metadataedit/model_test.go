package metadataedit

import (
	"errors"
	"testing"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func testStyles() ui.Styles {
	return ui.NewStyles(config.Default().Theme)
}

func TestBuildRequestTitleAuthorAndSeries(t *testing.T) {
	item := abs.LibraryItem{
		ID:        "item-1",
		MediaType: "book",
		Media: abs.Media{Metadata: abs.MediaMetadata{
			Title:   "Old Title",
			Authors: []abs.Author{{ID: "old-author", Name: "Old Author"}},
			Series:  &abs.SeriesSequence{ID: "series-1", Name: "Old Series", Sequence: "1"},
			SeriesList: []abs.SeriesSequence{{
				ID:       "series-1",
				Name:     "Old Series",
				Sequence: "1",
			}},
		}},
	}
	m := New(testStyles(), item)
	m.inputs[fieldTitle].SetValue("New Title")
	m.inputs[fieldAuthor].SetValue("New Author")
	m.inputs[fieldSeries].SetValue("Old Series")
	m.inputs[fieldSequence].SetValue("2")

	req, changed, errText := m.buildRequest()
	if errText != "" {
		t.Fatalf("buildRequest error = %q", errText)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if req.Metadata.Title == nil || *req.Metadata.Title != "New Title" {
		t.Fatalf("Title = %v", req.Metadata.Title)
	}
	if req.Metadata.Authors == nil || len(*req.Metadata.Authors) != 1 {
		t.Fatalf("Authors = %#v", req.Metadata.Authors)
	}
	if (*req.Metadata.Authors)[0].ID != "" || (*req.Metadata.Authors)[0].Name != "New Author" {
		t.Fatalf("Authors[0] = %+v, want changed author without old ID", (*req.Metadata.Authors)[0])
	}
	if req.Metadata.Series == nil || len(*req.Metadata.Series) != 1 {
		t.Fatalf("Series = %#v", req.Metadata.Series)
	}
	if (*req.Metadata.Series)[0].ID != "series-1" || (*req.Metadata.Series)[0].Sequence != "2" {
		t.Fatalf("Series[0] = %+v, want preserved ID for sequence-only change", (*req.Metadata.Series)[0])
	}
}

func TestBuildRequestChangedSeriesNameOmitsOldID(t *testing.T) {
	item := abs.LibraryItem{
		ID:        "item-1",
		MediaType: "book",
		Media: abs.Media{Metadata: abs.MediaMetadata{
			Title:      "Title",
			SeriesList: []abs.SeriesSequence{{ID: "series-1", Name: "Old Series", Sequence: "1"}},
		}},
	}
	m := New(testStyles(), item)
	m.inputs[fieldSeries].SetValue("New Series")

	req, changed, errText := m.buildRequest()
	if errText != "" || !changed {
		t.Fatalf("buildRequest changed=%v err=%q", changed, errText)
	}
	if req.Metadata.Series == nil || (*req.Metadata.Series)[0].ID != "" {
		t.Fatalf("Series = %#v, want changed series without old ID", req.Metadata.Series)
	}
}

func TestBuildEpisodeRequest(t *testing.T) {
	item := abs.LibraryItem{
		ID:        "pod-1",
		MediaType: "podcast",
		Media: abs.Media{Metadata: abs.MediaMetadata{
			Title: "Podcast",
		}},
	}
	m := NewEpisode(testStyles(), item, abs.PodcastEpisode{
		ID:          "ep-1",
		Title:       "Old Episode",
		Description: "Old Description",
		Season:      "1",
		Episode:     "2",
		EpisodeType: "full",
	})
	m.inputs[fieldTitle].SetValue("New Episode")
	m.inputs[fieldDescription].SetValue("New Description")
	m.inputs[fieldSeason].SetValue("3")
	m.inputs[fieldEpisode].SetValue("4")
	m.inputs[fieldEpisodeType].SetValue("bonus")

	req, changed, errText := m.buildEpisodeRequest()
	if errText != "" {
		t.Fatalf("buildEpisodeRequest error = %q", errText)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if req.Title == nil || *req.Title != "New Episode" {
		t.Fatalf("Title = %v", req.Title)
	}
	if req.Description == nil || *req.Description != "New Description" {
		t.Fatalf("Description = %v", req.Description)
	}
	if req.Season == nil || *req.Season != "3" {
		t.Fatalf("Season = %v", req.Season)
	}
	if req.Episode == nil || *req.Episode != "4" {
		t.Fatalf("Episode = %v", req.Episode)
	}
	if req.EpisodeType == nil || *req.EpisodeType != "bonus" {
		t.Fatalf("EpisodeType = %v", req.EpisodeType)
	}
	if m.fieldVisible(fieldAuthor) || m.fieldVisible(fieldSeries) || m.fieldVisible(fieldSequence) {
		t.Fatal("episode editor shows show/book-only fields")
	}
}

func TestEpisodeFieldsAreEditableByKeyboard(t *testing.T) {
	m := NewEpisode(testStyles(), abs.LibraryItem{
		ID:        "pod-1",
		MediaType: "podcast",
		Media:     abs.Media{Metadata: abs.MediaMetadata{Title: "Podcast"}},
	}, abs.PodcastEpisode{ID: "ep-1", Title: "Episode"})

	type step struct {
		wantFocused int
		text        rune
	}
	steps := []step{
		{fieldDescription, 'D'},
		{fieldSeason, '1'},
		{fieldEpisode, '2'},
		{fieldEpisodeType, 'B'},
	}
	for _, step := range steps {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		if cmd == nil {
			t.Fatal("tab command = nil, want focus command")
		}
		if m.Focused() != step.wantFocused {
			t.Fatalf("Focused() = %d, want %d", m.Focused(), step.wantFocused)
		}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{step.text}})
	}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter command = nil, want SaveEpisodeCmd")
	}
	msg := cmd()
	save, ok := msg.(SaveEpisodeCmd)
	if !ok {
		t.Fatalf("cmd returned %T, want SaveEpisodeCmd", msg)
	}
	if save.Request.Description == nil || *save.Request.Description != "D" {
		t.Fatalf("Description = %v, want D", save.Request.Description)
	}
	if save.Request.Season == nil || *save.Request.Season != "1" {
		t.Fatalf("Season = %v, want 1", save.Request.Season)
	}
	if save.Request.Episode == nil || *save.Request.Episode != "2" {
		t.Fatalf("Episode = %v, want 2", save.Request.Episode)
	}
	if save.Request.EpisodeType == nil || *save.Request.EpisodeType != "B" {
		t.Fatalf("EpisodeType = %v, want B", save.Request.EpisodeType)
	}
}

func TestBuildEpisodeRequestCanClearOptionalFields(t *testing.T) {
	m := NewEpisode(testStyles(), abs.LibraryItem{
		ID:        "pod-1",
		MediaType: "podcast",
		Media: abs.Media{Metadata: abs.MediaMetadata{
			Title: "Podcast",
		}},
	}, abs.PodcastEpisode{
		ID:          "ep-1",
		Title:       "Episode",
		Description: "Description",
		Season:      "1",
		Episode:     "2",
		EpisodeType: "trailer",
	})
	m.inputs[fieldDescription].SetValue("   ")
	m.inputs[fieldSeason].SetValue("   ")
	m.inputs[fieldEpisode].SetValue("   ")
	m.inputs[fieldEpisodeType].SetValue("   ")

	req, changed, errText := m.buildEpisodeRequest()
	if errText != "" || !changed {
		t.Fatalf("buildEpisodeRequest changed=%v err=%q", changed, errText)
	}
	if req.Description == nil || *req.Description != "" {
		t.Fatalf("Description = %v, want empty string pointer", req.Description)
	}
	if req.Season == nil || *req.Season != "" {
		t.Fatalf("Season = %v, want empty string pointer", req.Season)
	}
	if req.Episode == nil || *req.Episode != "" {
		t.Fatalf("Episode = %v, want empty string pointer", req.Episode)
	}
	if req.EpisodeType == nil || *req.EpisodeType != "" {
		t.Fatalf("EpisodeType = %v, want empty string pointer", req.EpisodeType)
	}
}

func TestBuildRequestClearsSeries(t *testing.T) {
	item := abs.LibraryItem{
		ID:        "item-1",
		MediaType: "book",
		Media: abs.Media{Metadata: abs.MediaMetadata{
			Title:      "Title",
			SeriesList: []abs.SeriesSequence{{ID: "series-1", Name: "Old Series", Sequence: "1"}},
		}},
	}
	m := New(testStyles(), item)
	m.inputs[fieldSeries].SetValue("")
	m.inputs[fieldSequence].SetValue("")

	req, changed, errText := m.buildRequest()
	if errText != "" || !changed {
		t.Fatalf("buildRequest changed=%v err=%q", changed, errText)
	}
	if req.Metadata.Series == nil || len(*req.Metadata.Series) != 0 {
		t.Fatalf("Series = %#v, want empty slice", req.Metadata.Series)
	}
}

func TestMultiAuthorAndSeriesFieldsAreReadOnly(t *testing.T) {
	item := abs.LibraryItem{
		ID:        "item-1",
		MediaType: "book",
		Media: abs.Media{Metadata: abs.MediaMetadata{
			Title:      "Title",
			Authors:    []abs.Author{{Name: "One"}, {Name: "Two"}},
			SeriesList: []abs.SeriesSequence{{Name: "One"}, {Name: "Two"}},
		}},
	}
	m := New(testStyles(), item)
	if m.authorEditable {
		t.Fatal("authorEditable = true, want false")
	}
	if m.seriesEditable {
		t.Fatal("seriesEditable = true, want false")
	}

	m.inputs[fieldAuthor].SetValue("Changed Author")
	m.inputs[fieldSeries].SetValue("Changed Series")
	req, changed, errText := m.buildRequest()
	if errText != "" || changed {
		t.Fatalf("buildRequest changed=%v err=%q req=%+v, want no changes", changed, errText, req)
	}
}

func TestSaveRejectsEmptyTitle(t *testing.T) {
	m := New(testStyles(), abs.LibraryItem{ID: "item-1", MediaType: "book", Media: abs.Media{Metadata: abs.MediaMetadata{Title: "Title"}}})
	m.inputs[fieldTitle].SetValue("   ")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("cmd != nil, want no command")
	}
	if m.ValidationError() == "" {
		t.Fatal("ValidationError() empty, want title error")
	}
}

func TestSaveNoChangesReturnsBackMsg(t *testing.T) {
	m := New(testStyles(), abs.LibraryItem{ID: "item-1", MediaType: "book", Media: abs.Media{Metadata: abs.MediaMetadata{Title: "Title"}}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd = nil, want BackMsg command")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Fatalf("cmd returned %T, want BackMsg", msg)
	}
}

func TestLeftCancels(t *testing.T) {
	m := New(testStyles(), abs.LibraryItem{ID: "item-1", MediaType: "book", Media: abs.Media{Metadata: abs.MediaMetadata{Title: "Title"}}})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if cmd == nil {
		t.Fatal("cmd = nil, want BackMsg command")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Fatalf("cmd returned %T, want BackMsg", msg)
	}
}

func TestSavedMsgStoresSaveError(t *testing.T) {
	m := New(testStyles(), abs.LibraryItem{ID: "item-1", MediaType: "book", Media: abs.Media{Metadata: abs.MediaMetadata{Title: "Title"}}})
	m.saving = true
	err := errors.New("boom")

	m, _ = m.Update(SavedMsg{ItemID: "item-1", Err: err})
	if m.Saving() {
		t.Fatal("Saving() = true, want false")
	}
	if m.SaveError() != err {
		t.Fatalf("SaveError() = %v, want %v", m.SaveError(), err)
	}
}
