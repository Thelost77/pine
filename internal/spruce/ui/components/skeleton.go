package components

import (
	"strings"

	"github.com/Thelost77/pine/internal/spruce/ui"
	"github.com/charmbracelet/bubbles/list"
)

// SkeletonItem implements list.Item for skeleton loading placeholder rows.
type SkeletonItem struct {
	TitleStr       string
	DescriptionStr string
}

func (i SkeletonItem) Title() string       { return i.TitleStr }
func (i SkeletonItem) Description() string { return i.DescriptionStr }
func (i SkeletonItem) FilterValue() string { return "" }

// BuildSkeletonRows returns a list of placeholder items mimicking list rows during loading.
func BuildSkeletonRows(styles ui.Styles) []list.Item {
	placeholder := func(width int) string {
		return styles.Muted.Render(strings.Repeat("-", width))
	}

	return []list.Item{
		SkeletonItem{TitleStr: placeholder(22), DescriptionStr: placeholder(14) + " • " + placeholder(5)},
		SkeletonItem{TitleStr: placeholder(18), DescriptionStr: placeholder(12) + " • " + placeholder(6)},
		SkeletonItem{TitleStr: placeholder(24), DescriptionStr: placeholder(15) + " • " + placeholder(4)},
		SkeletonItem{TitleStr: placeholder(19), DescriptionStr: placeholder(13) + " • " + placeholder(5)},
		SkeletonItem{TitleStr: placeholder(21), DescriptionStr: placeholder(11) + " • " + placeholder(6)},
	}
}
