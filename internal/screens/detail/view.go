package detail

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
)

// View renders the detail screen.
func (m Model) View() string {
	header := m.renderHeader()
	if !m.ready {
		return header
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
}

// renderHeader renders the fixed top section: title, author, duration, progress.
func (m Model) renderHeader() string {
	meta := m.item.Media.Metadata
	var sections []string

	// Title
	title := m.styles.Title.PaddingBottom(0).Render(meta.Title)
	sections = append(sections, title)

	// Author
	author := "Unknown author"
	if meta.AuthorName != nil {
		author = *meta.AuthorName
	}
	sections = append(sections, m.styles.Subtitle.Render(author))

	if m.item.Media.HasDuration() {
		dur := m.styles.Muted.Render("Duration: " + ui.FormatDuration(m.item.Media.TotalDuration()))
		sections = append(sections, dur)
	}

	// Chapter X/Y (if we have progress and chapters)
	if m.item.UserMediaProgress != nil && len(meta.Chapters) > 0 {
		currentTime := m.item.UserMediaProgress.CurrentTime
		chapterInfo := m.renderChapterInfo(currentTime, meta.Chapters)
		sections = append(sections, chapterInfo)
	}

	// Progress bar / Finished badge
	if m.item.UserMediaProgress != nil {
		if m.item.UserMediaProgress.IsFinished {
			sections = append(sections, m.styles.Accent.Render("Finished"))
		} else {
			progress := m.item.UserMediaProgress.Progress
			sections = append(sections, m.renderProgressBar(progress))
		}
	}

	// Blank separator
	sections = append(sections, "")

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderChapterInfo returns "Chapter X/Y" text based on current playback position.
func (m Model) renderChapterInfo(currentTime float64, chapters []abs.Chapter) string {
	chapterNum := 0
	totalChapters := len(chapters)
	for _, ch := range chapters {
		if currentTime >= ch.Start && currentTime < ch.End {
			chapterNum = ch.ID + 1
		}
	}
	chapterStr := m.styles.Muted.Render(fmt.Sprintf("Chapter %d/%d", chapterNum, totalChapters))
	return chapterStr
}

// renderProgressBar renders a lipgloss-styled progress bar.
func (m Model) renderProgressBar(progress float64) string {
	barWidth := m.width - 2
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}

	filled := int(float64(barWidth) * progress)
	if filled > barWidth {
		filled = barWidth
	}

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", barWidth-filled)

	bar := m.styles.Accent.Render(filledStr) + m.styles.Muted.Render(emptyStr)
	pct := fmt.Sprintf(" %d%%", int(progress*100))

	return bar + m.styles.Muted.Render(pct)
}

// buildContent builds the scrollable content: description + episodes (podcasts) / chapters (books) + bookmarks + help.
func (m Model) buildContent() string {
	meta := m.item.Media.Metadata
	var sections []string

	// Description
	if meta.Description != nil && *meta.Description != "" {
		descLabel := m.styles.Subtitle.Render("Description")
		desc := wordWrap(*meta.Description, m.width)
		sections = append(sections, descLabel, desc, "")
	}

	// Episodes (for podcasts)
	if m.item.MediaType == "podcast" && len(m.episodes) > 0 {
		epLabel := m.styles.Subtitle.Render("Episodes")
		if m.focusEpisodes {
			epLabel = m.styles.Accent.Render("▸ Episodes")
		}
		sections = append(sections, epLabel)
		for i, ep := range m.episodes {
			dur := ui.FormatDuration(ep.Duration)
			line := fmt.Sprintf("  %d. %s  %s", i+1, ep.Title, m.styles.Muted.Render("("+dur+")"))
			if m.focusEpisodes && i == m.selectedEpisode {
				line = m.styles.Selected.Render(fmt.Sprintf("▶ %d. %s  (%s)", i+1, ep.Title, dur))
			}
			sections = append(sections, line)
		}
		sections = append(sections, "")
	} else if m.item.MediaType == "podcast" && len(m.episodes) == 0 {
		sections = append(sections, m.styles.Subtitle.Render("Episodes"))
		sections = append(sections, m.styles.Muted.Render("  No episodes available"))
		sections = append(sections, "")
	}

	// Chapters (for books)
	if m.item.MediaType != "podcast" && len(meta.Chapters) > 0 {
		chapLabel := m.styles.Subtitle.Render("Chapters")
		if m.focusChapters {
			chapLabel = m.styles.Accent.Render("▸ Chapters")
		}
		sections = append(sections, chapLabel)
		for i, ch := range meta.Chapters {
			dur := ui.FormatDuration(ch.End - ch.Start)
			line := fmt.Sprintf("  %d. %s  %s",
				ch.ID+1, ch.Title, m.styles.Muted.Render("("+dur+")"))
			if m.focusChapters && i == m.selectedChapter {
				line = m.styles.Selected.Render(fmt.Sprintf("▶ %d. %s  (%s)", ch.ID+1, ch.Title, dur))
			}
			sections = append(sections, line)
		}
		sections = append(sections, "")
	}

	// Bookmarks
	if len(m.bookmarks) > 0 {
		bmLabel := m.styles.Subtitle.Render("Bookmarks")
		if m.focusBookmarks {
			bmLabel = m.styles.Accent.Render("▸ Bookmarks")
		}
		sections = append(sections, bmLabel)
		for i, bm := range m.bookmarks {
			ts := ui.FormatTimestamp(bm.Time)
			line := fmt.Sprintf("  🔖 %s  %s", ts, bm.Title)
			if m.focusBookmarks && i == m.selectedBookmark {
				line = m.styles.Selected.Render(fmt.Sprintf("🔖 %s  %s", ts, bm.Title))
			}
			sections = append(sections, line)
		}
		sections = append(sections, "")
	}

	// Help
	help := m.helpText()
	sections = append(sections, m.styles.Muted.Render(help))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// helpText returns context-sensitive help text based on current state.
func (m Model) helpText() string {
	if m.item.MediaType == "podcast" && len(m.episodes) > 0 {
		if m.focusEpisodes {
			return "enter play episode • space/p pause • j/k navigate • tab next section • q/h back"
		}
		if m.focusBookmarks {
			return "enter seek • d delete • j/k navigate • tab unfocus • q/h back"
		}
		return "space/p play • tab focus episodes/bookmarks • b bookmark • q/h back"
	}
	if m.item.MediaType == "podcast" && len(m.episodes) == 0 {
		return "b bookmark • tab focus bookmarks • q/h back"
	}
	if m.focusChapters {
		return "enter seek to chapter • j/k navigate • tab next section • q/h back"
	}
	if m.focusBookmarks {
		return "enter seek • d delete • j/k navigate • tab unfocus • b bookmark • q/h back"
	}
	chapters := m.item.Media.Metadata.Chapters
	if len(chapters) > 0 || len(m.bookmarks) > 0 {
		return "space/p play • b bookmark • tab focus chapters/bookmarks • j/k scroll • q/h back"
	}
	return "space/p play • b bookmark • f mark finished • j/k scroll • q/h back"
}


// wordWrap wraps text to the given width, breaking on spaces.
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		if result.Len() > 0 {
			result.WriteByte('\n')
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		lineLen := 0
		for i, word := range words {
			wLen := len(word)
			if i > 0 && lineLen+1+wLen > width {
				result.WriteByte('\n')
				lineLen = 0
			} else if i > 0 {
				result.WriteByte(' ')
				lineLen++
			}
			result.WriteString(word)
			lineLen += wLen
		}
	}
	return result.String()
}
