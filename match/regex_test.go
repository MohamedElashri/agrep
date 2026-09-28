package match

import (
	"testing"

	"github.com/MohamedElashri/agrep/arabic"
)

func TestRegexMatchesNormalizedText(t *testing.T) {
	m, err := NewRegex([]string{`مدرس[هة]`}, arabic.ProfileSearch, false)
	if err != nil {
		t.Fatalf("NewRegex: %v", err)
	}
	key := arabic.ProfileSearch.Normalize("مَدْرَسَة")
	if spans := m.FindAll(key); len(spans) != 1 || spans[0] != (Span{0, len(key)}) {
		t.Fatalf("FindAll(%q) = %v", key, spans)
	}
}

func TestRegexPatternIsNotNormalized(t *testing.T) {
	m, err := NewRegex([]string{`ة`}, arabic.ProfileSearch, false)
	if err != nil {
		t.Fatalf("NewRegex: %v", err)
	}
	if spans := m.FindAll(arabic.ProfileSearch.Normalize("مدرسة")); len(spans) != 0 {
		t.Fatal("ta-marbuta regex unexpectedly matched text normalized to heh")
	}
}

func TestRegexIgnoreCase(t *testing.T) {
	m, err := NewRegex([]string{`hello\s+world`}, arabic.ProfileSearch, true)
	if err != nil {
		t.Fatalf("NewRegex: %v", err)
	}
	if spans := m.FindAll("Hello WORLD"); len(spans) != 1 {
		t.Fatal("case-insensitive regex did not match")
	}
}

func TestRegexRejectsInvalidPattern(t *testing.T) {
	if _, err := NewRegex([]string{"["}, arabic.ProfileSearch, false); err == nil {
		t.Fatal("NewRegex accepted invalid syntax")
	}
}

func FuzzNewRegexNoPanic(f *testing.F) {
	f.Add("مدرس.*", "مَدْرَسَة", false)
	f.Add("[", "anything", true)
	f.Add("^$", "", false)
	f.Fuzz(func(t *testing.T, pattern, text string, ignoreCase bool) {
		m, err := NewRegex([]string{pattern}, arabic.ProfileSearch, ignoreCase)
		if err != nil {
			return
		}
		key := arabic.ProfileSearch.Normalize(text)
		_ = m.FindAll(key)
	})
}
