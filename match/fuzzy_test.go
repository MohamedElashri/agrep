package match

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
)

func TestFuzzyLevenshteinEditsAndSpans(t *testing.T) {
	tests := []struct {
		name  string
		query string
		text  string
		want  string
	}{
		{"exact", "كتاب", "هذا كتاب جيد", "كتاب"},
		{"substitution", "كتاب", "هذا كتلب جيد", "كتلب"},
		{"insertion", "كتاب", "هذا كتااب جيد", "كتااب"},
		{"deletion", "كتاب", "هذا كتب جيد", "كتب"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewFuzzy([]string{tt.query}, arabic.ProfileStrict, false, 1)
			if err != nil {
				t.Fatal(err)
			}
			key := arabic.ProfileStrict.Normalize(tt.text)
			spans := m.FindAll(key)
			if len(spans) != 1 || key[spans[0].Start:spans[0].End] != tt.want {
				t.Fatalf("FindAll(%q) = %v; want one span covering %q", key, spans, tt.want)
			}
		})
	}
}

func TestFuzzyDistanceCountsRunesNotUTF8Bytes(t *testing.T) {
	m, err := NewFuzzy([]string{"كتاب"}, arabic.ProfileStrict, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	if spans := m.FindAll("كتلب"); len(spans) != 1 || spans[0] != (Span{0, len("كتلب")}) {
		t.Fatalf("one Arabic-rune substitution spans = %v", spans)
	}
}

func TestFuzzyPrefersLowerDistanceForOverlaps(t *testing.T) {
	m, err := NewFuzzy([]string{"abc"}, arabic.ProfileStrict, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	spans := m.FindAll("abc abx")
	want := []Span{{0, 3}, {4, 7}}
	if len(spans) != len(want) || spans[0] != want[0] || spans[1] != want[1] {
		t.Fatalf("FindAll = %v; want %v", spans, want)
	}
}

func TestFuzzyLongPatternUsesMultiwordEngine(t *testing.T) {
	query := strings.Repeat("a", 70)
	text := strings.Repeat("a", 35) + "b" + strings.Repeat("a", 34)
	m, err := NewFuzzy([]string{query}, arabic.ProfileStrict, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	if spans := m.FindAll(text); len(spans) != 1 || spans[0] != (Span{0, len(text)}) {
		t.Fatalf("FindAll long pattern = %v", spans)
	}
}

func TestFuzzyIgnoreCaseMapsExpansion(t *testing.T) {
	m, err := NewFuzzy([]string{"STRASSE"}, arabic.ProfileStrict, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if spans := m.FindAll("Straße"); len(spans) != 1 || spans[0] != (Span{0, len("Straße")}) {
		t.Fatalf("FindAll case-fold expansion = %v", spans)
	}
}

func TestNewFuzzyValidation(t *testing.T) {
	if _, err := NewFuzzy(nil, arabic.ProfileSearch, false, 1); err == nil {
		t.Fatal("NewFuzzy accepted no queries")
	}
	if _, err := NewFuzzy([]string{"x"}, arabic.ProfileSearch, false, -1); err == nil {
		t.Fatal("NewFuzzy accepted a negative distance")
	}
	if _, err := NewFuzzy([]string{"َّ"}, arabic.ProfileSearch, false, 1); !errors.Is(err, arabic.ErrEmptyKey) {
		t.Fatalf("empty normalized query error = %v", err)
	}
	if _, err := NewFuzzy([]string{string([]byte{0xff})}, arabic.ProfileSearch, false, 1); err == nil {
		t.Fatal("NewFuzzy accepted invalid UTF-8")
	}
}

func TestMyersEndpointsMatchSubstringDP(t *testing.T) {
	patterns := []string{"a", "ab", "aba", "abc", strings.Repeat("a", 65)}
	texts := []string{"", "a", "xabc", "ababa", strings.Repeat("a", 32) + "b" + strings.Repeat("a", 32)}
	for _, pattern := range patterns {
		for _, text := range texts {
			p := newBitPattern([]rune(pattern))
			for distance := 0; distance <= 2; distance++ {
				got := p.findEnds([]rune(text), distance)
				want := oracleFuzzyEnds([]rune(pattern), []rune(text), distance)
				if !equalInts(got, want) {
					t.Fatalf("pattern=%q text=%q distance=%d ends=%v; want %v", pattern, text, distance, got, want)
				}
			}
		}
	}
}

func oracleFuzzyEnds(pattern, text []rune, maxDistance int) []int {
	previous := make([]int, len(pattern)+1)
	for i := range previous {
		previous[i] = i
	}
	var ends []int
	for j, textRune := range text {
		current := make([]int, len(pattern)+1)
		current[0] = 0
		for i, patternRune := range pattern {
			cost := 1
			if patternRune == textRune {
				cost = 0
			}
			current[i+1] = min(previous[i]+cost, previous[i+1]+1, current[i]+1)
		}
		if current[len(pattern)] <= maxDistance {
			ends = append(ends, j+1)
		}
		previous = current
	}
	return ends
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func FuzzNewFuzzyNoPanic(f *testing.F) {
	f.Add("كتاب", "هذا كتلب", 1, false)
	f.Add("STRASSE", "Straße", 0, true)
	f.Add("", "text", 1, false)
	f.Fuzz(func(t *testing.T, query, text string, distance int, ignoreCase bool) {
		if distance < 0 {
			distance = -distance
		}
		distance %= 4
		m, err := NewFuzzy([]string{query}, arabic.ProfileSearch, ignoreCase, distance)
		if err != nil {
			return
		}
		if !utf8.ValidString(text) {
			return
		}
		key := m.Profile().Normalize(text)
		spans := m.FindAll(key)
		if fast, ok := m.(interface{ Matches(string) bool }); ok {
			if got := fast.Matches(key); got != (len(spans) > 0) {
				t.Fatalf("Matches=%v but FindAll=%v for query=%q text=%q distance=%d", got, spans, query, text, distance)
			}
		}
	})
}

func FuzzMyersEndpointsMatchDP(f *testing.F) {
	f.Add("كتاب", "هذا كتلب", uint8(1))
	f.Add(strings.Repeat("a", 65), strings.Repeat("a", 64)+"b", uint8(1))
	f.Add("abc", "xabcab", uint8(0))
	f.Fuzz(func(t *testing.T, pattern, text string, rawDistance uint8) {
		if !utf8.ValidString(pattern) || !utf8.ValidString(text) {
			t.Skip()
		}
		patternRunes := []rune(pattern)
		textRunes := []rune(text)
		if len(patternRunes) == 0 {
			t.Skip()
		}
		if len(patternRunes) > 96 {
			patternRunes = patternRunes[:96]
		}
		if len(textRunes) > 96 {
			textRunes = textRunes[:96]
		}
		distance := int(rawDistance % 4)
		bitPattern := newBitPattern(patternRunes)
		got := bitPattern.findEnds(textRunes, distance)
		want := oracleFuzzyEnds(patternRunes, textRunes, distance)
		if !equalInts(got, want) {
			t.Fatalf("pattern=%q text=%q distance=%d ends=%v; want %v", string(patternRunes), string(textRunes), distance, got, want)
		}
	})
}

func BenchmarkFuzzyMatcherPositional(b *testing.B) {
	m, err := NewFuzzy([]string{"المستشرقون"}, arabic.ProfileSearch, false, 1)
	if err != nil {
		b.Fatal(err)
	}
	text := strings.Repeat("هذا نص عربي للبحث في مجموعة كبيرة من الكلمات. ", 200) + "المستشرقين"
	key := m.Profile().Normalize(text)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = m.FindAll(key)
	}
}

func BenchmarkFuzzyMatcherBoolean(b *testing.B) {
	m, err := NewFuzzy([]string{"المستشرقون"}, arabic.ProfileSearch, false, 1)
	if err != nil {
		b.Fatal(err)
	}
	text := strings.Repeat("هذا نص عربي للبحث في مجموعة كبيرة من الكلمات. ", 200) + "المستشرقين"
	key := m.Profile().Normalize(text)
	boolean := m.(*fuzzyMatcher)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = boolean.Matches(key)
	}
}
