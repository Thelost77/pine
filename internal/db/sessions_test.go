package db

import (
	"testing"
)

func TestSaveListeningSession(t *testing.T) {
	s := openTestStore(t)

	session := ListeningSession{
		ItemID:      "item-123",
		EpisodeID:   "",
		SessionID:   "sess-456",
		CurrentTime: 42.5,
		Duration:    3600.0,
	}

	if err := s.SaveListeningSession(session); err != nil {
		t.Fatalf("SaveListeningSession() error: %v", err)
	}

	got, err := s.GetLastSession()
	if err != nil {
		t.Fatalf("GetLastSession() error: %v", err)
	}
	if got.ItemID != session.ItemID {
		t.Errorf("ItemID = %q, want %q", got.ItemID, session.ItemID)
	}
	if got.EpisodeID != session.EpisodeID {
		t.Errorf("EpisodeID = %q, want %q", got.EpisodeID, session.EpisodeID)
	}
	if got.SessionID != session.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, session.SessionID)
	}
	if got.CurrentTime != session.CurrentTime {
		t.Errorf("CurrentTime = %f, want %f", got.CurrentTime, session.CurrentTime)
	}
	if got.Duration != session.Duration {
		t.Errorf("Duration = %f, want %f", got.Duration, session.Duration)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestSaveListeningSession_WithEpisodeID(t *testing.T) {
	s := openTestStore(t)

	session := ListeningSession{
		ItemID:      "item-123",
		EpisodeID:   "ep-789",
		SessionID:   "sess-456",
		CurrentTime: 42.5,
		Duration:    3600.0,
	}

	if err := s.SaveListeningSession(session); err != nil {
		t.Fatalf("SaveListeningSession() error: %v", err)
	}

	got, err := s.GetLastSession()
	if err != nil {
		t.Fatalf("GetLastSession() error: %v", err)
	}
	if got.EpisodeID != "ep-789" {
		t.Errorf("EpisodeID = %q, want %q", got.EpisodeID, "ep-789")
	}
}

func TestSaveListeningSession_Overwrites(t *testing.T) {
	s := openTestStore(t)

	first := ListeningSession{
		ItemID:      "item-1",
		SessionID:   "sess-1",
		CurrentTime: 10.0,
		Duration:    100.0,
	}
	second := ListeningSession{
		ItemID:      "item-2",
		SessionID:   "sess-2",
		CurrentTime: 20.0,
		Duration:    200.0,
	}

	if err := s.SaveListeningSession(first); err != nil {
		t.Fatalf("first SaveListeningSession() error: %v", err)
	}
	if err := s.SaveListeningSession(second); err != nil {
		t.Fatalf("second SaveListeningSession() error: %v", err)
	}

	got, err := s.GetLastSession()
	if err != nil {
		t.Fatalf("GetLastSession() error: %v", err)
	}
	if got.ItemID != "item-2" {
		t.Errorf("ItemID = %q, want %q (should be latest)", got.ItemID, "item-2")
	}
	if got.SessionID != "sess-2" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-2")
	}
}

func TestGetLastSession_Empty(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetLastSession()
	if err == nil {
		t.Fatal("expected error when no session exists, got nil")
	}
}

func TestClearSession(t *testing.T) {
	s := openTestStore(t)

	session := ListeningSession{
		ItemID:      "item-123",
		SessionID:   "sess-456",
		CurrentTime: 42.5,
		Duration:    3600.0,
	}
	if err := s.SaveListeningSession(session); err != nil {
		t.Fatal(err)
	}

	if err := s.ClearSession(); err != nil {
		t.Fatalf("ClearSession() error: %v", err)
	}

	_, err := s.GetLastSession()
	if err == nil {
		t.Fatal("expected error after ClearSession(), got nil")
	}
}

func TestClearSession_Empty(t *testing.T) {
	s := openTestStore(t)

	// Should not error when no sessions exist
	if err := s.ClearSession(); err != nil {
		t.Fatalf("ClearSession() on empty table error: %v", err)
	}
}

func TestSaveListeningSession_UpdateCurrentTime(t *testing.T) {
	s := openTestStore(t)

	session := ListeningSession{
		ItemID:      "item-123",
		SessionID:   "sess-456",
		CurrentTime: 10.0,
		Duration:    3600.0,
	}
	if err := s.SaveListeningSession(session); err != nil {
		t.Fatal(err)
	}

	// Save again with updated time (same session)
	session.CurrentTime = 500.0
	if err := s.SaveListeningSession(session); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetLastSession()
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentTime != 500.0 {
		t.Errorf("CurrentTime = %f, want 500.0", got.CurrentTime)
	}
}
