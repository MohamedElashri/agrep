package match

import (
	"errors"
	"testing"

	"github.com/MohamedElashri/agrep/arabic"
)

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
