package arabic

import (
	"testing"
	"unicode/utf8"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"voweled", "اَلْعَرَبِيَّةُ", "العربيه"},
		{"unvoweled", "العربية", "العربيه"},
		{"mixed", "Go و Python مُمْتَاز", "Go و Python ممتاز"},
		{"tatweel", "العــربية", "العربيه"},
		{"alef variants", "آأإٱ", "اااا"},
		{"seated hamza", "مسؤول فئة", "مسوول فيه"},
		{"alef maksura", "على", "علي"},
		{"empty", "", ""},
		{"marks only", "َُِّْ", ""},
		// U+00E9 and e + U+0301 are canonically equivalent; both become e.
		{"canonical decomposition", "café", "cafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.input); got != tt.want {
				t.Fatalf("Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeRejectsEmptyKeys(t *testing.T) {
	for _, s := range []string{"", "َّ", "ــ"} {
		if got := Normalize(s); got != "" {
			t.Fatalf("Normalize(%q) = %q; want empty key (callers compare against ErrEmptyKey)", s, got)
		}
	}
}

func addNormalizeSeeds(f *testing.F) {
	seeds := []string{
		"اَلْعَرَبِيَّةُ",
		"العربية",
		"Go و Python مُمْتَاز",
		"العــربية",
		"آأإٱ",
		"مسؤول فئة",
		"على",
		"",
		"َُِّْ",
		"café",
		"ﻻ", // Arabic ligature lam-alef (compatibility form).
		"ﷲ", // Allah ligature.
	}
	for _, s := range seeds {
		f.Add(s)
	}
}

// FuzzNormalizeIdempotent checks that applying Normalize twice is the same
// as applying it once: the output of normalization is already a fixed point
// of normalization.
func FuzzNormalizeIdempotent(f *testing.F) {
	addNormalizeSeeds(f)

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		once := Normalize(s)
		twice := Normalize(once)
		if once != twice {
			t.Fatalf("not idempotent: Normalize(%q) = %q, but Normalize(that) = %q", s, once, twice)
		}
	})
}

// FuzzNormalizeValidUTF8 checks that valid UTF-8 input always normalizes to
// valid UTF-8 output.
func FuzzNormalizeValidUTF8(f *testing.F) {
	addNormalizeSeeds(f)

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		got := Normalize(s)
		if !utf8.ValidString(got) {
			t.Fatalf("Normalize(%q) produced invalid UTF-8: %q", s, got)
		}
	})
}

// FuzzNormalizeNoPanic checks that Normalize never panics, including on byte
// sequences that are not valid UTF-8 (the fuzzer's string inputs are
// arbitrary bytes, not guaranteed-valid text).
func FuzzNormalizeNoPanic(f *testing.F) {
	addNormalizeSeeds(f)
	f.Add(string([]byte{0xff, 0xfe, 0x00}))
	f.Add(string([]byte{'a', 0x80, 'b'}))

	f.Fuzz(func(t *testing.T, s string) {
		_ = Normalize(s)
	})
}
