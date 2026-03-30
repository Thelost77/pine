package abs

import "testing"

func TestTotalDuration_MediaDuration(t *testing.T) {
	dur := 55908.2
	m := Media{Duration: &dur}
	if got := m.TotalDuration(); got != 55908.2 {
		t.Errorf("TotalDuration() = %f, want 55908.2", got)
	}
}

func TestTotalDuration_MetadataFallback(t *testing.T) {
	dur := 3600.0
	m := Media{Metadata: MediaMetadata{Duration: &dur}}
	if got := m.TotalDuration(); got != 3600.0 {
		t.Errorf("TotalDuration() = %f, want 3600.0", got)
	}
}

func TestTotalDuration_MediaTakesPrecedence(t *testing.T) {
	mediaDur := 100.0
	metaDur := 200.0
	m := Media{Duration: &mediaDur, Metadata: MediaMetadata{Duration: &metaDur}}
	if got := m.TotalDuration(); got != 100.0 {
		t.Errorf("TotalDuration() = %f, want 100.0 (media.duration should take precedence)", got)
	}
}

func TestTotalDuration_NeitherSet(t *testing.T) {
	m := Media{}
	if got := m.TotalDuration(); got != 0 {
		t.Errorf("TotalDuration() = %f, want 0", got)
	}
}

func TestHasDuration(t *testing.T) {
	dur := 100.0
	zero := 0.0

	tests := []struct {
		name string
		m    Media
		want bool
	}{
		{"media duration set", Media{Duration: &dur}, true},
		{"metadata duration set", Media{Metadata: MediaMetadata{Duration: &dur}}, true},
		{"zero media duration", Media{Duration: &zero}, false},
		{"no duration", Media{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.HasDuration(); got != tt.want {
				t.Errorf("HasDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
