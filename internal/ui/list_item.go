package ui

import "github.com/Thelost77/pine/internal/abs"

// ListItem wraps a LibraryItem to implement the bubbles list.Item interface.
// Used by library and search screens for items without progress tracking.
type ListItem struct {
	Item abs.LibraryItem
}

func (i ListItem) Title() string {
	return i.Item.Media.Metadata.Title
}

func (i ListItem) Description() string {
	author := "Unknown author"
	if i.Item.Media.Metadata.AuthorName != nil {
		author = *i.Item.Media.Metadata.AuthorName
	}
	duration := ""
	if i.Item.Media.HasDuration() {
		duration = " • " + FormatDuration(i.Item.Media.TotalDuration())
	}
	return author + duration
}

func (i ListItem) FilterValue() string {
	return i.Item.Media.Metadata.Title
}
