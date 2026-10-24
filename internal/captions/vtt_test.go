package captions

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseVTTAndAt(t *testing.T) {
	data := []byte("WEBVTT\n\n" +
		"1\n" +
		"00:00:01.000 --> 00:00:04.000\n" +
		"Hello world\n\n" +
		"00:00:05.000 --> 00:00:08.500\n" +
		"Second\n" +
		"line\n")

	cues := ParseVTT(data)
	if len(cues) != 2 {
		t.Fatalf("len(cues) = %d, want 2", len(cues))
	}
	if cues[0].Start != 1 || cues[0].End != 4 || cues[0].Text != "Hello world" {
		t.Fatalf("cue[0] = %+v", cues[0])
	}
	if cues[1].Start != 5 || cues[1].End != 8.5 || cues[1].Text != "Second line" {
		t.Fatalf("cue[1] = %+v", cues[1])
	}

	if got := At(cues, 0.5); got != "" {
		t.Errorf("At(0.5) = %q, want empty", got)
	}
	if got := At(cues, 1); got != "Hello world" {
		t.Errorf("At(1) = %q, want Hello world", got)
	}
	if got := At(cues, 3.9); got != "Hello world" {
		t.Errorf("At(3.9) = %q, want Hello world", got)
	}
	if got := At(cues, 4); got != "" {
		t.Errorf("At(4) = %q, want empty (gap)", got)
	}
	if got := At(cues, 8.5); got != "" {
		t.Errorf("At(8.5) = %q, want empty (past last cue)", got)
	}
	if got := At(cues, 9); got != "" {
		t.Errorf("At(9) = %q, want empty (past last cue)", got)
	}
}

func TestParseVTTShortTimestampsTagsAndNotes(t *testing.T) {
	data := []byte("NOTE ignore me\n\n" +
		"STYLE\n::cue { color: red; }\n\n" +
		"00:01.000 --> 00:02.000 align:start\n" +
		"<v Narrator>Hello <c>world</c>\n")

	cues := ParseVTT(data)
	if len(cues) != 1 {
		t.Fatalf("len(cues) = %d, want 1", len(cues))
	}
	if cues[0].Start != 1 || cues[0].End != 2 {
		t.Fatalf("times = %v-%v, want 1-2", cues[0].Start, cues[0].End)
	}
	if cues[0].Text != "Hello world" {
		t.Fatalf("text = %q, want Hello world", cues[0].Text)
	}
}

func TestParseVTTDecodesEntitiesAndStripsControls(t *testing.T) {
	data := []byte("WEBVTT\n\n" +
		"00:00:01.000 --> 00:00:02.000\n" +
		"A&amp;B &lt;C&gt; &#39;quote&#39;\n\n" +
		"00:00:02.000 --> 00:00:03.000\n" +
		"safe\x1b]52;c;Zm9v\x07 text\n")

	cues := ParseVTT(data)
	if len(cues) != 2 {
		t.Fatalf("len(cues) = %d, want 2: %+v", len(cues), cues)
	}
	if cues[0].Text != "A&B <C> 'quote'" {
		t.Fatalf("entities = %q", cues[0].Text)
	}
	if cues[1].Text != "safe]52;c;Zm9v text" {
		t.Fatalf("controls = %q", cues[1].Text)
	}
	if containsRune(cues[1].Text, 0x1b) || containsRune(cues[1].Text, 0x07) {
		t.Fatalf("control bytes survived: %q", cues[1].Text)
	}
}

func TestParseVTTHeaderForms(t *testing.T) {
	cues := "00:00:01.000 --> 00:00:02.000\nFirst\n\n00:00:03.000 --> 00:00:04.000\nSecond\n"
	for _, test := range []struct {
		name string
		data string
	}{
		{"no blank line after the header", "WEBVTT\n" + cues},
		{"header lines", "WEBVTT\nKind: captions\nLanguage: en\n\n" + cues},
		{"byte order mark and CRLF", "\xEF\xBB\xBFWEBVTT\r\n\r\n" + strings.ReplaceAll(cues, "\n", "\r\n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ParseVTT([]byte(test.data))
			want := []Cue{{Start: 1, End: 2, Text: "First"}, {Start: 3, End: 4, Text: "Second"}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ParseVTT() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseVTTKeepsAGreaterThanSignOutsideATag(t *testing.T) {
	data := []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n5 > 3 and <i>x</i> -> y\n")

	cues := ParseVTT(data)
	if len(cues) != 1 || cues[0].Text != "5 > 3 and x -> y" {
		t.Fatalf("ParseVTT() = %+v, want one cue with the text %q", cues, "5 > 3 and x -> y")
	}
}

func TestParseVTTSkipsCuesWithTimesThatAreNotFiniteOrNegative(t *testing.T) {
	data := []byte("WEBVTT\n\n" +
		"00:00:00.000 --> 00:00:inf\nInfinite\n\n" +
		"00:00:00.000 --> 00:00:NaN\nNot a number\n\n" +
		"-1:00.000 --> 00:01.000\nNegative\n\n" +
		"01:-5:00.000 --> 01:00:00.000\nNegative minutes\n\n" +
		"00:00:05.000 --> 00:00:06.000\nValid\n")

	cues := ParseVTT(data)
	want := []Cue{{Start: 5, End: 6, Text: "Valid"}}
	if !reflect.DeepEqual(cues, want) {
		t.Fatalf("ParseVTT() = %+v, want %+v", cues, want)
	}
	if got := At(cues, 5.5); got != "Valid" {
		t.Errorf("At(5.5) = %q, want Valid", got)
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
