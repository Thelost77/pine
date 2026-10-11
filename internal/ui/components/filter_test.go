package components

import (
	"slices"
	"testing"
)

func TestUnicodeFilterReturnsRuneIndexes(t *testing.T) {
	const title = "Sympatyczny trochę ciemny rock"

	ranks := UnicodeFilter("rock", []string{title})
	if len(ranks) != 1 {
		t.Fatalf("got %d matches, want 1", len(ranks))
	}

	matched := make([]rune, len(ranks[0].MatchedIndexes))
	titleRunes := []rune(title)
	for i, index := range ranks[0].MatchedIndexes {
		matched[i] = titleRunes[index]
	}
	if !slices.Equal(matched, []rune("rock")) {
		t.Fatalf("highlighted runes = %q, want %q; indexes = %v", string(matched), "rock", ranks[0].MatchedIndexes)
	}
}
