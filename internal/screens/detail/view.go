package detail

import (
	"fmt"
	"strings"

	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/lipgloss"
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

// buildContent builds the scrollable content: description + episodes (podcasts) + bookmarks + help.
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

	// Bookmarks
	if m.bookmarksLoaded {
		bmLabel := m.styles.Subtitle.Render("Bookmarks")
		if m.focusBookmarks {
			bmLabel = m.styles.Accent.Render("▸ Bookmarks")
		}
		sections = append(sections, bmLabel)
		switch {
		case m.bookmarkLoadErr != nil:
			sections = append(sections, m.styles.Muted.Render("  "+m.bookmarkLoadErr.Error()))
		case len(m.bookmarks) == 0:
			sections = append(sections, m.styles.Muted.Render("  No bookmarks yet"))
		default:
			for i, bm := range m.bookmarks {
				ts := ui.FormatTimestamp(bm.Time)
				if m.editingBookmark && i == m.selectedBookmark {
					line := fmt.Sprintf("  ✎ %s  %s", ts, m.bookmarkInput.View())
					sections = append(sections, line)
					if m.bookmarkEditErr != "" {
						sections = append(sections, "  "+m.styles.Error.Render(m.bookmarkEditErr))
					}
					continue
				}
				line := fmt.Sprintf("  🔖 %s  %s", ts, bm.Title)
				if m.focusBookmarks && i == m.selectedBookmark {
					line = m.styles.Selected.Render(fmt.Sprintf("🔖 %s  %s", ts, bm.Title))
				}
				sections = append(sections, line)
			}
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
	hasBookmarkFocus := m.bookmarkLoadErr == nil && len(m.bookmarks) > 0
	queueHints := " • a queue • A next"
	if m.editingBookmark {
		return "enter save • esc cancel • type bookmark title"
	}
	if m.item.MediaType == "podcast" && len(m.episodes) > 0 {
		if m.focusEpisodes {
			if hasBookmarkFocus {
				return "enter play episode" + queueHints + " • space/p pause • j/k navigate • tab next section • q/h back"
			}
			return "enter play episode" + queueHints + " • space/p pause • j/k navigate • tab unfocus • q/h back"
		}
		if m.focusBookmarks && hasBookmarkFocus {
			return "enter seek • e edit • d delete • j/k navigate • tab unfocus • q/h back"
		}
		if hasBookmarkFocus {
			return "space/p play • tab focus episodes/bookmarks • b bookmark • q/h back"
		}
		return "space/p play • tab focus episodes • b bookmark • q/h back"
	}
	if m.item.MediaType == "podcast" && len(m.episodes) == 0 {
		if hasBookmarkFocus {
			return "b bookmark • tab focus bookmarks • q/h back"
		}
		return "b bookmark • q/h back"
	}
	if m.focusBookmarks && hasBookmarkFocus {
		return "enter seek • e edit • d delete • j/k navigate • tab unfocus • b bookmark" + queueHints + " • q/h back"
	}
	if hasBookmarkFocus {
		return "space/p play • b bookmark" + queueHints + " • tab focus bookmarks • j/k scroll • q/h back"
	}
	return "space/p play • b bookmark • f mark finished" + queueHints + " • j/k scroll • q/h back"
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
