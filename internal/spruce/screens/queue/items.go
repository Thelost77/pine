package queue

import (
	"fmt"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/ui"
)

type queueItem struct {
	Track     jellyfin.Track
	Index     int
	IsCurrent bool
}

func (i queueItem) Title() string {
	prefix := fmt.Sprintf("%d. ", i.Index+1)
	if i.IsCurrent {
		prefix = "▶ " + prefix
	}
	return prefix + i.Track.Name
}

func (i queueItem) Description() string {
	dur := ui.FormatDuration(i.Track.Duration())
	status := ""
	if i.IsCurrent {
		status = " [Now Playing] • "
	} else {
		status = " • "
	}
	return fmt.Sprintf("%s%s%s", i.Track.DisplayArtist(), status, dur)
}

func (i queueItem) FilterValue() string { return i.Track.Name }
