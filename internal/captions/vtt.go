// Package captions parses timed transcript sidecars and selects the active cue.
package captions

import (
	"bytes"
	"html"
	"strconv"
	"strings"
	"unicode"
)

// Cue is a timed caption interval. Start is inclusive and End is exclusive.
type Cue struct {
	Start float64
	End   float64
	Text  string
}

// ParseVTT parses WebVTT bytes into cues. A missing WEBVTT header is tolerated.
// Invalid cue blocks are skipped. The result is always valid UTF-8 display text.
func ParseVTT(data []byte) []Cue {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "WEBVTT") {
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			i++
		}
	}

	var cues []Cue
	for i < len(lines) {
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= len(lines) {
			break
		}

		line := strings.TrimSpace(lines[i])
		if isVTTBlock(line, "NOTE") || isVTTBlock(line, "STYLE") || isVTTBlock(line, "REGION") {
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
				i++
			}
			continue
		}

		if !strings.Contains(line, "-->") {
			i++
			if i >= len(lines) {
				break
			}
			line = strings.TrimSpace(lines[i])
		}
		if !strings.Contains(line, "-->") {
			i++
			continue
		}

		start, end, ok := parseTimes(line)
		i++
		if !ok {
			continue
		}

		var textLines []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			textLines = append(textLines, strings.TrimSpace(lines[i]))
			i++
		}
		cueText := sanitizeCueText(strings.Join(textLines, " "))
		if cueText == "" {
			continue
		}
		cues = append(cues, Cue{Start: start, End: end, Text: cueText})
	}
	return cues
}

// At returns the text of the cue that contains t, or empty if none does.
func At(cues []Cue, t float64) string {
	for _, cue := range cues {
		if t >= cue.Start && t < cue.End {
			return cue.Text
		}
	}
	return ""
}

func isVTTBlock(line, kind string) bool {
	if line == kind {
		return true
	}
	return strings.HasPrefix(line, kind+" ") || strings.HasPrefix(line, kind+"\t")
}

func parseTimes(line string) (float64, float64, bool) {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err := parseTimestamp(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(parts[1]))
	if len(fields) == 0 {
		return 0, 0, false
	}
	end, err := parseTimestamp(fields[0])
	if err != nil {
		return 0, 0, false
	}
	return start, end, true
}

func parseTimestamp(s string) (float64, error) {
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.Split(s, ":")
	var hours, minutes int
	var seconds float64
	var err error
	switch len(parts) {
	case 3:
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, err
		}
		seconds, err = strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, err
		}
	case 2:
		minutes, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		seconds, err = strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
	default:
		return 0, strconv.ErrSyntax
	}
	return float64(hours)*3600 + float64(minutes)*60 + seconds, nil
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeCueText(s string) string {
	s = html.UnescapeString(stripTags(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n':
			b.WriteByte(' ')
		case r < 32, r == 127, unicode.In(r, unicode.Cc):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
