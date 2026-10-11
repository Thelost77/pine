package components

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/sahilm/fuzzy"
)

// UnicodeFilter matches list items while returning rune indexes for highlighting.
func UnicodeFilter(term string, targets []string) []list.Rank {
	matches := fuzzy.Find(term, targets)
	ranks := make([]list.Rank, len(matches))
	for i, match := range matches {
		ranks[i] = list.Rank{
			Index:          match.Index,
			MatchedIndexes: byteToRuneIndexes(targets[match.Index], match.MatchedIndexes),
		}
	}
	return ranks
}

func byteToRuneIndexes(value string, byteIndexes []int) []int {
	if len(byteIndexes) == 0 {
		return nil
	}

	runeByByte := make(map[int]int, len([]rune(value)))
	runeIndex := 0
	for byteIndex := range value {
		runeByByte[byteIndex] = runeIndex
		runeIndex++
	}

	indexes := make([]int, len(byteIndexes))
	for i, byteIndex := range byteIndexes {
		indexes[i] = runeByByte[byteIndex]
	}
	return indexes
}
