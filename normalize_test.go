package main

import (
	"testing"
	"unicode/utf8"
)

func TestNormalizeArabic(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"voweled", "\u0627\u064E\u0644\u0652\u0639\u064E\u0631\u064E\u0628\u0650\u064A\u064E\u0651\u0629\u064F", "\u0627\u0644\u0639\u0631\u0628\u064A\u0647"},
		{"unvoweled", "\u0627\u0644\u0639\u0631\u0628\u064A\u0629", "\u0627\u0644\u0639\u0631\u0628\u064A\u0647"},
		{"mixed", "Go \u0648 Python \u0645\u064F\u0645\u0652\u062A\u064E\u0627\u0632", "Go \u0648 Python \u0645\u0645\u062A\u0627\u0632"},
		{"tatweel", "\u0627\u0644\u0639\u0640\u0640\u0631\u0628\u064A\u0629", "\u0627\u0644\u0639\u0631\u0628\u064A\u0647"},
		{"alef variants", "\u0622\u0623\u0625\u0671", "\u0627\u0627\u0627\u0627"},
		{"seated hamza", "\u0645\u0633\u0624\u0648\u0644 \u0641\u0626\u0629", "\u0645\u0633\u0648\u0648\u0644 \u0641\u064A\u0647"},
		{"alef maksura", "\u0639\u0644\u0649", "\u0639\u0644\u064A"},
		{"empty", "", ""},
		{"marks only", "\u064E\u064F\u0650\u0652\u0651", ""},
		// U+00E9 and e + U+0301 are canonically equivalent; both become e.
		{"canonical decomposition", "caf\u00e9", "cafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeArabic(tt.input); got != tt.want {
				t.Fatalf("normalizeArabic(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeQueryRejectsEmptyKeys(t *testing.T) {
	for _, query := range []string{"", "\u064E\u0651", "\u0640\u0640"} {
		if _, err := normalizeQuery(query); err == nil {
			t.Fatalf("normalizeQuery(%q) returned no error", query)
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

// FuzzNormalizeIdempotent checks that applying normalizeArabic twice is the
// same as applying it once: the output of normalization is already a fixed
// point of normalization.
func FuzzNormalizeIdempotent(f *testing.F) {
	addNormalizeSeeds(f)

	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		once := normalizeArabic(s)
		twice := normalizeArabic(once)
		if once != twice {
			t.Fatalf("not idempotent: normalizeArabic(%q) = %q, but normalizeArabic(that) = %q", s, once, twice)
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
		got := normalizeArabic(s)
		if !utf8.ValidString(got) {
			t.Fatalf("normalizeArabic(%q) produced invalid UTF-8: %q", s, got)
		}
	})
}

// FuzzNormalizeNoPanic checks that normalizeArabic and normalizeQuery never
// panic, including on byte sequences that are not valid UTF-8 (the fuzzer's
// string inputs are arbitrary bytes, not guaranteed-valid text).
func FuzzNormalizeNoPanic(f *testing.F) {
	addNormalizeSeeds(f)
	f.Add(string([]byte{0xff, 0xfe, 0x00}))
	f.Add(string([]byte{'a', 0x80, 'b'}))

	f.Fuzz(func(t *testing.T, s string) {
		_ = normalizeArabic(s)
		_, _ = normalizeQuery(s)
	})
}
