package metadataedit

import (
	"strings"
	"sync/atomic"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var editorGeneration atomic.Uint64

const (
	fieldTitle = iota
	fieldAuthor
	fieldDescription
	fieldSeries
	fieldSequence
	fieldSeason
	fieldEpisode
	fieldEpisodeType
	numFields
)

// SaveCmd requests saving edited metadata to ABS.
type SaveCmd struct {
	ItemID     string
	Generation uint64
	Request    abs.UpdateMediaRequest
}

// SaveEpisodeCmd requests saving edited podcast episode metadata to ABS.
type SaveEpisodeCmd struct {
	ItemID     string
	EpisodeID  string
	Generation uint64
	Request    abs.UpdatePodcastEpisodeRequest
}

// SavedMsg reports the result of a metadata save.
type SavedMsg struct {
	ItemID     string
	Generation uint64
	Item       *abs.LibraryItem
	Err        error
}

// SavedEpisodeMsg reports the result of a podcast episode metadata save.
type SavedEpisodeMsg struct {
	ItemID     string
	EpisodeID  string
	Generation uint64
	Item       *abs.LibraryItem
	Err        error
}

// BackMsg requests returning to the previous screen.
type BackMsg struct{}

// Model is the bubbletea model for the metadata editor screen.
type Model struct {
	item       abs.LibraryItem
	episode    *abs.PodcastEpisode
	generation uint64

	inputs  [numFields]textinput.Model
	focused int

	authorEditable bool
	seriesEditable bool
	saving         bool
	validationErr  string
	saveErr        error

	width  int
	height int
	styles ui.Styles
}

// New creates a metadata editor for a library item.
func New(styles ui.Styles, item abs.LibraryItem) Model {
	meta := item.Media.Metadata
	var inputs [numFields]textinput.Model

	inputs[fieldTitle] = newInput("Title", meta.Title, 256)
	inputs[fieldAuthor] = newInput("Author", initialAuthorValue(meta), 256)

	series := meta.PrimarySeries()
	seriesName := ""
	seriesSequence := ""
	if series != nil {
		seriesName = series.Name
		seriesSequence = series.Sequence
	}
	inputs[fieldSeries] = newInput("Series", seriesName, 256)
	inputs[fieldSequence] = newInput("Sequence", seriesSequence, 32)

	m := Model{
		item:           item,
		generation:     editorGeneration.Add(1),
		inputs:         inputs,
		focused:        fieldTitle,
		authorEditable: item.MediaType != "book" || !meta.HasMultipleAuthors(),
		seriesEditable: item.MediaType == "book" && !meta.HasMultipleSeries(),
		styles:         styles,
	}
	_ = m.updateFocus()
	return m
}

// NewEpisode creates a metadata editor for a podcast episode.
func NewEpisode(styles ui.Styles, item abs.LibraryItem, episode abs.PodcastEpisode) Model {
	var inputs [numFields]textinput.Model
	inputs[fieldTitle] = newInput("Title", episode.Title, 256)
	inputs[fieldDescription] = newInput("Description", episode.Description, 2000)
	inputs[fieldSeason] = newInput("Season", episode.Season, 64)
	inputs[fieldEpisode] = newInput("Episode", episode.Episode, 64)
	inputs[fieldEpisodeType] = newInput("Episode Type", episode.EpisodeType, 64)

	m := Model{
		item:       item,
		episode:    &episode,
		generation: editorGeneration.Add(1),
		inputs:     inputs,
		focused:    fieldTitle,
		styles:     styles,
	}
	_ = m.updateFocus()
	return m
}

func newInput(placeholder, value string, limit int) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = limit
	input.Prompt = ""
	input.SetValue(value)
	return input
}

// Init returns the initial command.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// SetSize updates terminal dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	for i := range m.inputs {
		m.inputs[i].Width = max(16, min(64, width-20))
	}
}

// Update handles metadata edit messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SavedMsg:
		m.saving = false
		m.saveErr = msg.Err
		return m, nil
	case SavedEpisodeMsg:
		m.saving = false
		m.saveErr = msg.Err
		return m, nil

	case tea.KeyMsg:
		if m.saving && msg.String() != "esc" && msg.String() != "left" {
			return m, nil
		}
		switch msg.String() {
		case "esc", "left":
			return m, func() tea.Msg { return BackMsg{} }
		case "tab", "down":
			m.moveFocus(1)
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.moveFocus(-1)
			return m, m.updateFocus()
		case "enter":
			return m.save()
		}
	}

	if !m.fieldEditable(m.focused) {
		return m, nil
	}
	m.validationErr = ""
	m.saveErr = nil
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m Model) save() (Model, tea.Cmd) {
	m.validationErr = ""
	m.saveErr = nil
	if m.episode != nil {
		return m.saveEpisode()
	}

	request, changed, errText := m.buildRequest()
	if errText != "" {
		m.validationErr = errText
		return m, nil
	}
	if !changed {
		return m, func() tea.Msg { return BackMsg{} }
	}

	m.saving = true
	itemID := m.item.ID
	return m, func() tea.Msg {
		return SaveCmd{ItemID: itemID, Generation: m.generation, Request: request}
	}
}

func (m Model) saveEpisode() (Model, tea.Cmd) {
	request, changed, errText := m.buildEpisodeRequest()
	if errText != "" {
		m.validationErr = errText
		return m, nil
	}
	if !changed {
		return m, func() tea.Msg { return BackMsg{} }
	}

	m.saving = true
	itemID := m.item.ID
	episodeID := m.episode.ID
	return m, func() tea.Msg {
		return SaveEpisodeCmd{ItemID: itemID, EpisodeID: episodeID, Generation: m.generation, Request: request}
	}
}

func (m Model) buildRequest() (abs.UpdateMediaRequest, bool, string) {
	meta := m.item.Media.Metadata
	request := abs.UpdateMediaRequest{}
	changed := false

	title := strings.TrimSpace(m.inputs[fieldTitle].Value())
	if title == "" {
		return request, false, "Title cannot be empty"
	}
	if title != strings.TrimSpace(meta.Title) {
		request.Metadata.Title = &title
		changed = true
	}

	if m.authorEditable {
		authors, authorChanged := m.buildAuthors()
		if authorChanged {
			request.Metadata.Authors = &authors
			changed = true
		}
	}

	if m.item.MediaType == "book" && m.seriesEditable {
		series, seriesChanged := m.buildSeries()
		if seriesChanged {
			request.Metadata.Series = &series
			changed = true
		}
	}

	return request, changed, ""
}

func (m Model) buildEpisodeRequest() (abs.UpdatePodcastEpisodeRequest, bool, string) {
	request := abs.UpdatePodcastEpisodeRequest{}
	changed := false
	episode := m.episode
	if episode == nil {
		return request, false, "No episode selected"
	}

	title := strings.TrimSpace(m.inputs[fieldTitle].Value())
	if title == "" {
		return request, false, "Title cannot be empty"
	}
	if title != strings.TrimSpace(episode.Title) {
		request.Title = &title
		changed = true
	}

	description := strings.TrimSpace(m.inputs[fieldDescription].Value())
	if description != strings.TrimSpace(episode.Description) {
		request.Description = &description
		changed = true
	}
	season := strings.TrimSpace(m.inputs[fieldSeason].Value())
	if season != strings.TrimSpace(episode.Season) {
		request.Season = &season
		changed = true
	}
	episodeNumber := strings.TrimSpace(m.inputs[fieldEpisode].Value())
	if episodeNumber != strings.TrimSpace(episode.Episode) {
		request.Episode = &episodeNumber
		changed = true
	}
	episodeType := strings.TrimSpace(m.inputs[fieldEpisodeType].Value())
	if episodeType != strings.TrimSpace(episode.EpisodeType) {
		request.EpisodeType = &episodeType
		changed = true
	}

	return request, changed, ""
}

func (m Model) buildAuthors() ([]abs.Author, bool) {
	newName := strings.TrimSpace(m.inputs[fieldAuthor].Value())
	original, ok := singleAuthor(m.item.Media.Metadata)
	originalName := ""
	if ok {
		originalName = strings.TrimSpace(original.Name)
	}
	if newName == originalName {
		return nil, false
	}
	if newName == "" {
		return []abs.Author{}, true
	}
	return []abs.Author{{Name: newName}}, true
}

func (m Model) buildSeries() ([]abs.SeriesSequence, bool) {
	newName := strings.TrimSpace(m.inputs[fieldSeries].Value())
	newSequence := strings.TrimSpace(m.inputs[fieldSequence].Value())
	original, ok := singleSeries(m.item.Media.Metadata)
	if ok && newName == strings.TrimSpace(original.Name) && newSequence == strings.TrimSpace(original.Sequence) {
		return nil, false
	}
	if !ok && newName == "" && newSequence == "" {
		return nil, false
	}
	if newName == "" {
		return []abs.SeriesSequence{}, true
	}
	series := abs.SeriesSequence{Name: newName, Sequence: newSequence}
	if ok && newName == strings.TrimSpace(original.Name) {
		series.ID = original.ID
	}
	return []abs.SeriesSequence{series}, true
}

func (m *Model) moveFocus(delta int) {
	for range numFields {
		m.focused = (m.focused + delta + numFields) % numFields
		if m.fieldEditable(m.focused) {
			return
		}
	}
}

func (m Model) fieldEditable(field int) bool {
	if !m.fieldVisible(field) {
		return false
	}
	if m.episode != nil {
		return true
	}
	switch field {
	case fieldTitle:
		return true
	case fieldAuthor:
		return m.authorEditable
	case fieldSeries, fieldSequence:
		return m.seriesEditable
	default:
		return false
	}
}

// MatchesBookSave reports whether a save result belongs to this editor instance.
func (m Model) MatchesBookSave(itemID string, generation uint64) bool {
	return m.episode == nil && m.item.ID == itemID && (generation == 0 || m.generation == generation)
}

// MatchesEpisodeSave reports whether a save result belongs to this editor instance.
func (m Model) MatchesEpisodeSave(itemID, episodeID string, generation uint64) bool {
	return m.episode != nil && m.item.ID == itemID && m.episode.ID == episodeID && (generation == 0 || m.generation == generation)
}

func (m Model) fieldVisible(field int) bool {
	if m.episode != nil {
		switch field {
		case fieldTitle, fieldDescription, fieldSeason, fieldEpisode, fieldEpisodeType:
			return true
		default:
			return false
		}
	}
	switch field {
	case fieldTitle, fieldAuthor:
		return true
	case fieldSeries, fieldSequence:
		return m.item.MediaType == "book"
	default:
		return false
	}
}

func (m *Model) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, numFields)
	for i := range m.inputs {
		if i == m.focused && m.fieldEditable(i) {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

func initialAuthorValue(meta abs.MediaMetadata) string {
	if meta.HasMultipleAuthors() {
		return meta.DisplayAuthor()
	}
	author, ok := singleAuthor(meta)
	if ok {
		return author.Name
	}
	if meta.AuthorName != nil {
		return strings.TrimSpace(*meta.AuthorName)
	}
	return ""
}

func singleAuthor(meta abs.MediaMetadata) (abs.Author, bool) {
	var single abs.Author
	count := 0
	for _, author := range meta.Authors {
		if strings.TrimSpace(author.Name) == "" {
			continue
		}
		single = author
		count++
	}
	if count == 1 {
		return single, true
	}
	if count > 1 {
		return abs.Author{}, false
	}
	if meta.AuthorName != nil && strings.TrimSpace(*meta.AuthorName) != "" {
		return abs.Author{Name: strings.TrimSpace(*meta.AuthorName)}, true
	}
	return abs.Author{}, false
}

func singleSeries(meta abs.MediaMetadata) (abs.SeriesSequence, bool) {
	if len(meta.SeriesList) == 1 {
		return meta.SeriesList[0], true
	}
	if len(meta.SeriesList) > 1 {
		return abs.SeriesSequence{}, false
	}
	if meta.Series != nil && strings.TrimSpace(meta.Series.Name) != "" {
		return *meta.Series, true
	}
	return abs.SeriesSequence{}, false
}

// Focused returns the focused field index for tests.
func (m Model) Focused() int {
	return m.focused
}

// Saving reports whether a save is in progress.
func (m Model) Saving() bool {
	return m.saving
}

// ValidationError returns the current validation error.
func (m Model) ValidationError() string {
	return m.validationErr
}

// SaveError returns the last save error.
func (m Model) SaveError() error {
	return m.saveErr
}
