package db

import (
	"database/sql"
	"fmt"
)

// ListeningSession is the item or the podcast episode that played last. At
// startup, pine loads it again at the position on the server.
type ListeningSession struct {
	ItemID    string
	EpisodeID string
}

const lastSessionID = "last"

// SaveListeningSession stores the item or the episode that plays. Only one
// session is kept at a time (the most recent), enabling crash recovery. It
// stores no session ID and no position: pine does not read them, and the ID
// of a play session is a secret.
func (s *Store) SaveListeningSession(session ListeningSession) error {
	_, err := s.DB.Exec(`
		INSERT INTO sessions (id, item_id, episode_id, session_id, "current_time", duration, created_at)
		VALUES (?, ?, ?, '', 0, 0, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			item_id        = excluded.item_id,
			episode_id     = excluded.episode_id,
			session_id     = excluded.session_id,
			"current_time" = excluded."current_time",
			duration       = excluded.duration,
			created_at     = excluded.created_at
	`, lastSessionID, session.ItemID, session.EpisodeID)
	if err != nil {
		return fmt.Errorf("saving listening session: %w", err)
	}
	return nil
}

// GetLastSession retrieves the last saved playback session.
// Returns an error if no session has been saved.
func (s *Store) GetLastSession() (ListeningSession, error) {
	var ls ListeningSession
	err := s.DB.QueryRow(`
		SELECT item_id, episode_id FROM sessions WHERE id = ?
	`, lastSessionID).Scan(&ls.ItemID, &ls.EpisodeID)
	if err == sql.ErrNoRows {
		return ListeningSession{}, fmt.Errorf("no saved session")
	}
	if err != nil {
		return ListeningSession{}, fmt.Errorf("getting last session: %w", err)
	}
	return ls, nil
}

// ClearSession removes all saved listening sessions.
func (s *Store) ClearSession() error {
	if _, err := s.DB.Exec(`DELETE FROM sessions`); err != nil {
		return fmt.Errorf("clearing sessions: %w", err)
	}
	return nil
}
