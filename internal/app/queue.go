package app

import "github.com/Thelost77/pine/internal/abs"

func (m *Model) enqueueQueueEntry(entry QueueEntry, front bool) {
	if entry.Item.ID == "" {
		return
	}

	filtered := make([]QueueEntry, 0, len(m.queue))
	for _, existing := range m.queue {
		if queueEntryKey(existing) == queueEntryKey(entry) {
			continue
		}
		filtered = append(filtered, existing)
	}

	if front {
		m.queue = append([]QueueEntry{cloneQueueEntry(entry)}, filtered...)
		return
	}
	m.queue = append(filtered, cloneQueueEntry(entry))
}

func queueEntryKey(entry QueueEntry) string {
	if entry.Episode != nil {
		return entry.Item.ID + "::" + entry.Episode.ID
	}
	return entry.Item.ID
}

func cloneQueueEntry(entry QueueEntry) QueueEntry {
	return QueueEntry{
		Item:    entry.Item,
		Episode: cloneEpisodePtr(entry.Episode),
	}
}

func cloneEpisodePtr(episode *abs.PodcastEpisode) *abs.PodcastEpisode {
	if episode == nil {
		return nil
	}
	cp := *episode
	return &cp
}
