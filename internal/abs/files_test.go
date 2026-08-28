package abs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscriptIno(t *testing.T) {
	files := []LibraryFile{
		{Ino: "audio-1", Metadata: FileMetadata{Filename: "book.m4b", Ext: ".m4b"}},
		{Ino: "vtt-1", Metadata: FileMetadata{Filename: "book.vtt", Ext: ".vtt"}},
		{Ino: "vtt-2", Metadata: FileMetadata{Filename: "other.VTT"}},
	}

	if got := TranscriptIno(files, "book.m4b"); got != "vtt-1" {
		t.Errorf("match by stem = %q, want vtt-1", got)
	}
	if got := TranscriptIno(files, "BOOK.M4B"); got != "vtt-1" {
		t.Errorf("case-insensitive match = %q, want vtt-1", got)
	}
	if got := TranscriptIno(files, "missing.m4b"); got != "" {
		t.Errorf("unmatched stem = %q, want empty", got)
	}
	if got := TranscriptIno(files, ""); got != "" {
		t.Errorf("empty filename with multiple vtts = %q, want empty", got)
	}

	single := []LibraryFile{
		{Ino: "only-vtt", Metadata: FileMetadata{Filename: "captions.vtt"}},
	}
	if got := TranscriptIno(single, ""); got != "only-vtt" {
		t.Errorf("single vtt fallback = %q, want only-vtt", got)
	}
}

func TestGetLibraryFileHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/items/li-001/file/ino-9" {
			t.Errorf("path = %q, want /api/items/li-001/file/ino-9", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi\n"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	data, err := c.GetLibraryFile(context.Background(), "li-001", "ino-9")
	if err != nil {
		t.Fatalf("GetLibraryFile() error: %v", err)
	}
	if want := "WEBVTT"; len(data) < len(want) || string(data[:len(want)]) != want {
		t.Errorf("body = %q, want prefix %q", data, want)
	}
}

func TestPlaySessionLibraryFilesDeserialization(t *testing.T) {
	raw := []byte(`{
		"id": "session-1",
		"audioTracks": [{"index": 0, "contentUrl": "/s/a", "metadata": {"filename": "book.m4b"}}],
		"libraryItem": {
			"libraryFiles": [
				{"ino": "1", "fileType": "audio", "metadata": {"filename": "book.m4b", "ext": ".m4b"}},
				{"ino": "2", "metadata": {"filename": "book.vtt", "ext": ".vtt"}}
			]
		}
	}`)
	var session PlaySession
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if session.TranscriptIno("book.m4b") != "2" {
		t.Fatalf("TranscriptIno = %q, want 2", session.TranscriptIno("book.m4b"))
	}
}

func TestAudioTrackMetadataDeserialization(t *testing.T) {
	raw := []byte(`{"index": 0, "startOffset": 0, "duration": 10, "contentUrl": "/s/a", "metadata": {"filename": "book.m4b"}}`)
	var track AudioTrack
	if err := json.Unmarshal(raw, &track); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if track.Metadata.Filename != "book.m4b" {
		t.Errorf("filename = %q, want book.m4b", track.Metadata.Filename)
	}
}
