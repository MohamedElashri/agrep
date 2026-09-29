package match

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
)

func TestRawRejectionAnchorSources(t *testing.T) {
	// NFD and presentation expansion are the only stages that could create
	// these letters without an identical raw rune. All presentation forms
	// have an EF leading byte; a new Unicode decomposition would fail here.
	p := arabic.Profile{FoldPresentation: true}
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		source := string(r)
		if strings.ContainsRune(rawRejectionAnchorRunes, r) {
			continue
		}
		if got := p.Normalize(source); strings.ContainsAny(got, rawRejectionAnchorRunes) {
			if source[0] != 0xef {
				t.Fatalf("U+%04X normalizes to %q, introducing an anchor", r, got)
			}
			for _, anchor := range got {
				if strings.ContainsRune(rawRejectionAnchorRunes, anchor) &&
					!arabic.PresentationFormContains(r, anchor) {
					t.Fatalf("U+%04X introduces %q without a presentation-form hint", r, anchor)
				}
			}
		}
	}
}

func TestRawRejection(t *testing.T) {
	tests := []struct {
		name, query, line string
		profile           arabic.Profile
		ignoreCase        bool
		want              bool
	}{
		{"miss", "غيرموجود", "أعلنت المدينة افتتاح مكتبة", arabic.ProfileSearch, false, true},
		{"raw hit", "غ", "غابة", arabic.ProfileSearch, false, false},
		{"presentation hit", "غ", "ﻍ", arabic.ProfileSearch, false, false},
		{"irrelevant presentation", "غيرموجود", "ﻻ يوجد كتاب", arabic.ProfileSearch, false, true},
		{"presentation fold disabled", "غ", "ﻍ", arabic.Profile{FoldPresentation: false}, false, true},
		{"rasm fallback", "غيرموجود", "كتاب", arabic.ProfileLoose, false, false},
		{"case fold fallback", "غيرموجود", "كتاب", arabic.ProfileSearch, true, false},
		{"no anchor fallback", "ا", "كتاب", arabic.ProfileSearch, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewLiterals([]string{tt.query}, tt.profile, tt.ignoreCase)
			if err != nil {
				t.Fatal(err)
			}
			if got := m.(*literalMatcher).CannotMatchRaw(tt.line); got != tt.want {
				t.Fatalf("CannotMatchRaw(%q) = %v; want %v", tt.line, got, tt.want)
			}
		})
	}

	m, err := NewLiterals([]string{"غيرموجود", "ثوب"}, arabic.ProfileSearch, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.(*literalMatcher).CannotMatchRaw("أعلنت المكتبة") {
		t.Fatal("two impossible patterns were not rejected")
	}
	m, err = NewLiterals([]string{"غيرموجود", "ا"}, arabic.ProfileSearch, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.(*literalMatcher).CannotMatchRaw("أعلنت المكتبة") {
		t.Fatal("pattern without a safe anchor was rejected")
	}
}

func FuzzRawRejectionImpliesNoNormalizedMatch(f *testing.F) {
	f.Add("غ", "ﻍ", "", uint16(1<<7), uint8(1), uint8(0), false)
	f.Add("غيرموجود", "أعلنت المدينة", "ثوب", uint16(0x1fff), uint8(0x3f), uint8(1), false)
	f.Add("كتاب", "کتاب", "غ", uint16(0x3fff), uint8(3), uint8(2), true)
	f.Add("غيرموجود", "ﻻ يوجد كتاب", "", uint16(1<<7), uint8(1), uint8(0), false)
	f.Fuzz(func(t *testing.T, first, line, second string, bits uint16, languages, scope uint8, ignoreCase bool) {
		if !utf8.ValidString(line) {
			return
		}
		has := func(bit uint) bool { return bits&(1<<bit) != 0 }
		p := arabic.Profile{
			Languages:        arabic.LanguageSet(languages) & arabic.LanguageAll,
			StripTashkil:     has(0),
			TashkilScope:     arabic.TashkilScope(scope % 3),
			StripTatweel:     has(1),
			FoldAlefHamza:    has(2),
			FoldAlefWasla:    has(3),
			FoldHamzaSeat:    has(4),
			FoldTaMarbuta:    has(5),
			FoldAlefMaksura:  has(6),
			FoldPresentation: has(7),
			StripJoiners:     has(8),
			StripBidi:        has(9),
			FoldDigits:       has(10),
			FoldPunctuation:  has(11),
			StripQuranic:     has(12),
			Rasm:             has(13),
		}
		queries := []string{first}
		if second != "" {
			queries = append(queries, second)
		}
		m, err := NewLiterals(queries, p, ignoreCase)
		if err != nil {
			return
		}
		literal := m.(*literalMatcher)
		if got, want := literal.CannotMatchRawBytes([]byte(line)), literal.CannotMatchRaw(line); got != want {
			t.Fatalf("byte rejection differs from string rejection: got %v, want %v, line %q", got, want, line)
		}
		if literal.CannotMatchRawBytes([]byte(line)) && literal.Matches(p.Normalize(line)) {
			t.Fatalf("rejected a match: queries=%q line=%q profile=%+v ignoreCase=%v", queries, line, p, ignoreCase)
		}
		if literal.FirstPossibleRawByteIndex([]byte(line)) < 0 && literal.Matches(p.Normalize(line)) {
			t.Fatalf("candidate scan missed a match: queries=%q line=%q profile=%+v ignoreCase=%v", queries, line, p, ignoreCase)
		}
	})
}

func TestStableRawMatchImpliesNormalizedMatch(t *testing.T) {
	profiles := []arabic.Profile{arabic.ProfileSearch, arabic.ProfileStrict, arabic.ProfileLoose}
	queries := []string{"مكتبة", "المدينه", "ا", "مَد", "كتاب", "فارسی", "abc", "١٢٣"}
	lines := []string{
		"مكتبة", "مَكْتَبَة", "المدينة", "المدينه", "مَد",
		"كتاب", "ﻛﺘﺎﺏ", "فارسی", "abc", "أ", "١٢٣", "بِالكتاب",
	}
	for _, p := range profiles {
		for _, query := range queries {
			m, err := NewLiteral(query, p)
			if err != nil {
				continue
			}
			literal := m.(*literalMatcher)
			for _, line := range lines {
				if literal.MatchesStableRaw(line) && !strings.Contains(p.Normalize(line), literal.keys[0]) {
					t.Fatalf("raw positive was false after normalization: query %q, line %q, profile %+v", query, line, p)
				}
			}
		}
	}
	caseFolded, err := NewLiterals([]string{"ABC"}, arabic.ProfileSearch, true)
	if err != nil {
		t.Fatal(err)
	}
	if caseFolded.(*literalMatcher).MatchesStableRaw("ABC") {
		t.Fatal("case-folded literal used the raw positive shortcut")
	}
	folded, err := NewLiteral("المدينه", arabic.ProfileSearch)
	if err != nil {
		t.Fatal(err)
	}
	if !folded.(*literalMatcher).MatchesStableRaw("المدينة") {
		t.Fatal("common ta-marbuta spelling did not use the raw positive shortcut")
	}
	persian, err := NewLiteral("فارسی", arabic.ProfileSearch)
	if err != nil {
		t.Fatal(err)
	}
	if !persian.(*literalMatcher).MatchesStableRaw("متن فارسی") {
		t.Fatal("Persian yeh did not use the raw positive shortcut")
	}
}

func FuzzStableRawMatchImpliesNormalizedMatch(f *testing.F) {
	f.Add("مكتبة", "افتتاح مكتبة جديدة", uint8(0))
	f.Add("المدينه", "المدينة", uint8(0))
	f.Add("كتاب", "كِتاب", uint8(0))
	f.Add("ا", "أِ", uint8(1))
	f.Add("فارسی", "متن فارسی", uint8(0))
	profiles := [...]arabic.Profile{arabic.ProfileSearch, arabic.ProfileStrict, arabic.ProfileLoose}
	f.Fuzz(func(t *testing.T, query, line string, profileIndex uint8) {
		if !utf8.ValidString(line) {
			return
		}
		profile := profiles[int(profileIndex)%len(profiles)]
		m, err := NewLiteral(query, profile)
		if err != nil {
			return
		}
		literal := m.(*literalMatcher)
		if got, want := literal.MatchesStableRawBytes([]byte(line)), literal.MatchesStableRaw(line); got != want {
			t.Fatalf("byte positive differs from string positive: got %v, want %v, line %q", got, want, line)
		}
		if literal.MatchesStableRawBytes([]byte(line)) && !literal.Matches(profile.Normalize(line)) {
			t.Fatalf("raw positive was false after normalization: query %q, line %q, profile %+v", query, line, profile)
		}
	})
}

func TestNewLiteralRejectsEmptyKey(t *testing.T) {
	for _, query := range []string{"", "َّ", "ــ"} {
		if _, err := NewLiteral(query, arabic.ProfileSearch); !errors.Is(err, arabic.ErrEmptyKey) {
			t.Fatalf("NewLiteral(%q) error = %v; want errors.Is(err, arabic.ErrEmptyKey)", query, err)
		}
	}
}

func TestNewLiteralRejectsInvalidUTF8(t *testing.T) {
	if _, err := NewLiteral(string([]byte{'x', 0xff}), arabic.ProfileSearch); err == nil {
		t.Fatal("NewLiteral(invalid UTF-8) returned no error")
	}
}

func TestNewLiteralRejectsInvalidProfile(t *testing.T) {
	if _, err := NewLiteral("query", arabic.Profile{TashkilScope: 99}); err == nil {
		t.Fatal("NewLiteral(invalid profile) returned no error")
	}
}

func TestNewLiteralString(t *testing.T) {
	m, err := NewLiteral("مَدْرَسَةٌ", arabic.ProfileSearch)
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	if got := m.String(); got != "مَدْرَسَةٌ" {
		t.Fatalf("String() = %q; want the original, unnormalized query", got)
	}
}

func TestNewLiteralProfile(t *testing.T) {
	m, err := NewLiteral("query", arabic.ProfileStrict)
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	if got := m.Profile(); got != arabic.ProfileStrict {
		t.Fatalf("Profile() = %+v; want %+v", got, arabic.ProfileStrict)
	}
}

// TestFindAllUsesItsOwnProfileNotSearch is the direct regression test for
// why Matcher exposes Profile() at all: with a non-default profile, the
// query's own normalized key must reflect THAT profile, not
// arabic.Normalize's fixed ProfileSearch default: otherwise a caller that
// (wrongly) normalized the haystack with the wrong profile would get a
// silently mismatched comparison.
func TestFindAllUsesItsOwnProfileNotSearch(t *testing.T) {
	// Under ProfileStrict, ta-marbuta is NOT folded to heh, so a query
	// spelled with heh must not match haystack text spelled with
	// ta-marbuta once both sides are normalized under ProfileStrict.
	m, err := NewLiteral("مدرسه", arabic.ProfileStrict) // heh
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	haystack := arabic.ProfileStrict.Normalize("مدرسة") // ta-marbuta, normalized under the SAME profile as the matcher
	if spans := m.FindAll(haystack); spans != nil {
		t.Fatalf("FindAll(%q) = %v; want nil (heh query must not match ta-marbuta haystack under ProfileStrict)", haystack, spans)
	}

	// The same two spellings DO match once both are normalized under
	// ProfileSearch, which folds ta-marbuta to heh.
	mSearch, err := NewLiteral("مدرسه", arabic.ProfileSearch)
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	haystackSearch := arabic.ProfileSearch.Normalize("مدرسة")
	if spans := mSearch.FindAll(haystackSearch); len(spans) != 1 {
		t.Fatalf("FindAll(%q) = %v; want one span (heh query must match ta-marbuta haystack under ProfileSearch)", haystackSearch, spans)
	}
}

func TestFindAllNormalizesQueryAndHaystack(t *testing.T) {
	// Query is voweled; haystack (already normalized by the caller, as scan
	// would do) is not. They must still collapse to the same key.
	m, err := NewLiteral("مَدْرَسَةٌ", arabic.ProfileSearch)
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	normalized := arabic.Normalize("مدرسه")
	spans := m.FindAll(normalized)
	if len(spans) != 1 || spans[0] != (Span{Start: 0, End: len(normalized)}) {
		t.Fatalf("FindAll(%q) = %v; want one span covering the whole string", normalized, spans)
	}
}

func TestFindAllMultipleNonOverlapping(t *testing.T) {
	m, err := NewLiteral("aa", arabic.ProfileSearch)
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	// "aaaa" contains "aa" at offsets 0 and 2 under non-overlapping search.
	spans := m.FindAll("aaaa")
	want := []Span{{0, 2}, {2, 4}}
	if len(spans) != len(want) || spans[0] != want[0] || spans[1] != want[1] {
		t.Fatalf("FindAll(\"aaaa\") = %v; want %v", spans, want)
	}
}

func TestFindAllNoMatch(t *testing.T) {
	m, err := NewLiteral("missing", arabic.ProfileSearch)
	if err != nil {
		t.Fatalf("NewLiteral: %v", err)
	}
	if spans := m.FindAll("present"); spans != nil {
		t.Fatalf("FindAll(no match) = %v; want nil", spans)
	}
}

func TestNewLiteralsMatchesAnyQueryInTextOrder(t *testing.T) {
	m, err := NewLiterals([]string{"beta", "alpha"}, arabic.ProfileSearch, false)
	if err != nil {
		t.Fatalf("NewLiterals: %v", err)
	}
	spans := m.FindAll("alpha beta")
	want := []Span{{0, 5}, {6, 10}}
	if len(spans) != len(want) || spans[0] != want[0] || spans[1] != want[1] {
		t.Fatalf("FindAll = %v; want %v", spans, want)
	}
}

func TestNewLiteralsUnicodeCaseFold(t *testing.T) {
	m, err := NewLiterals([]string{"STRASSE"}, arabic.ProfileSearch, true)
	if err != nil {
		t.Fatalf("NewLiterals: %v", err)
	}
	normalized := arabic.ProfileSearch.Normalize("Straße")
	spans := m.FindAll(normalized)
	if len(spans) != 1 || spans[0] != (Span{Start: 0, End: len(normalized)}) {
		t.Fatalf("FindAll Unicode-folded text = %v; want original normalized range", spans)
	}
}

func TestUnicodeCaseFoldExpansionMapsPartialMatchToWholeRune(t *testing.T) {
	m, err := NewLiterals([]string{"s"}, arabic.ProfileSearch, true)
	if err != nil {
		t.Fatalf("NewLiterals: %v", err)
	}
	spans := m.FindAll("ß")
	if len(spans) != 2 || spans[0] != (Span{0, len("ß")}) || spans[1] != (Span{0, len("ß")}) {
		t.Fatalf("FindAll(ß) = %v; want both folded matches mapped to the whole rune", spans)
	}
}

func TestNewLiteralsRejectsNoQueries(t *testing.T) {
	if _, err := NewLiterals(nil, arabic.ProfileSearch, false); err == nil {
		t.Fatal("NewLiterals(nil) returned no error")
	}
}

func FuzzNewLiteralNoPanic(f *testing.F) {
	f.Add("query", 0)
	f.Add("", 1)
	f.Add("مدرسه", 2)
	f.Add(string([]byte{0xff, 0xfe}), 3)

	profiles := [...]arabic.Profile{
		arabic.ProfileSearch, arabic.ProfileStrict, arabic.ProfileLoose,
		arabic.ProfileLucene, arabic.ProfileCAMeL,
	}

	f.Fuzz(func(t *testing.T, query string, profileIdx int) {
		i := profileIdx % len(profiles)
		if i < 0 {
			i += len(profiles)
		}
		p := profiles[i]

		m, err := NewLiteral(query, p)
		if err != nil {
			return
		}
		_ = m.FindAll(p.Normalize(query))
		_ = m.FindAll("")
		_ = m.String()
		_ = m.Profile()
	})
}
