package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ListeningSession represents the last active playback state for crash recovery.
type ListeningSession struct {
	ItemID      string
	EpisodeID   string
	SessionID   string
	CurrentTime float64
	Duration    float64
	CreatedAt   time.Time
}

const lastSessionID = "last"

// SaveListeningSession stores the current playback state. Only one session is
// kept at a time (the most recent), enabling crash recovery.
func (s *Store) SaveListeningSession(session ListeningSession) error {
	_, err := s.DB.Exec(`
		INSERT INTO sessions (id, item_id, episode_id, session_id, "current_time", duration, created_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			item_id        = excluded.item_id,
			episode_id     = excluded.episode_id,
			session_id     = excluded.session_id,
			"current_time" = excluded."current_time",
			duration       = excluded.duration,
			created_at     = excluded.created_at
	`, lastSessionID, session.ItemID, session.EpisodeID, session.SessionID, session.CurrentTime, session.Duration)
	if err != nil {
		return fmt.Errorf("saving listening session: %w", err)
	}
	return nil
}

// GetLastSession retrieves the last saved playback session.
// Returns an error if no session has been saved.
func (s *Store) GetLastSession() (ListeningSession, error) {
	var ls ListeningSession
	var createdAt string
	err := s.DB.QueryRow(`
		SELECT item_id, episode_id, session_id, "current_time", duration, created_at
		FROM sessions WHERE id = ?
	`, lastSessionID).Scan(&ls.ItemID, &ls.EpisodeID, &ls.SessionID, &ls.CurrentTime, &ls.Duration, &createdAt)
	if err == sql.ErrNoRows {
		return ListeningSession{}, fmt.Errorf("no saved session")
	}
	if err != nil {
		return ListeningSession{}, fmt.Errorf("getting last session: %w", err)
	}
	ls.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return ls, nil
}

// ClearSession removes all saved listening sessions.
func (s *Store) ClearSession() error {
	if _, err := s.DB.Exec(`DELETE FROM sessions`); err != nil {
		return fmt.Errorf("clearing sessions: %w", err)
	}
	return nil
}
