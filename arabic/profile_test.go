package arabic

import (
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestTashkilFastPathContainsNoNonspacingMarks(t *testing.T) {
	for _, bounds := range [][2]rune{{0, 0x0300}, {0x0620, 0x064B}} {
		for r := bounds[0]; r < bounds[1]; r++ {
			if unicode.Is(unicode.Mn, r) {
				t.Fatalf("U+%04X is a nonspacing mark inside the fast path", r)
			}
		}
	}
}

func TestProfileValidate(t *testing.T) {
	for name, p := range map[string]Profile{
		"search": ProfileSearch, "strict": ProfileStrict, "loose": ProfileLoose,
		"lucene": ProfileLucene, "camel": ProfileCAMeL, "zero value": {},
	} {
		t.Run(name, func(t *testing.T) {
			if err := p.Validate(); err != nil {
				t.Fatalf("Validate() = %v; want nil", err)
			}
		})
	}

	if err := (Profile{TashkilScope: 99}).Validate(); err == nil {
		t.Fatal("Validate() with an out-of-range TashkilScope returned nil")
	}
	if err := (Profile{Languages: LanguageSet(0x80)}).Validate(); err == nil {
		t.Fatal("Validate() with unknown language bits returned nil")
	}
}

func TestNormalizeReordersMarksAfterDroppedStarter(t *testing.T) {
	p := Profile{StripTatweel: true}
	input := "a\u0315ـ\u0300"
	want := "a\u0300\u0315"
	if got := p.Normalize(input); got != want {
		t.Fatalf("Normalize(%q) = %q; want %q", input, got, want)
	}
}

// TestNormalizePackageFuncIsProfileSearch verifies that the package-level
// Normalize uses the default search profile.
func TestNormalizePackageFuncIsProfileSearch(t *testing.T) {
	for _, s := range []string{"", "أحمد", "café", "plain"} {
		if got, want := Normalize(s), ProfileSearch.Normalize(s); got != want {
			t.Fatalf("Normalize(%q) = %q; ProfileSearch.Normalize(%q) = %q", s, got, s, want)
		}
	}
}

// TestKeepHamzaPreservesDistinction checks that with FoldAlefHamza off, NFD
// decomposition must not let a blanket StripTashkil silently re-merge أ and
// ا by stripping the leftover combining hamza-above mark. See the Profile
// doc comment.
func TestKeepHamzaPreservesDistinction(t *testing.T) {
	p := ProfileSearch
	p.FoldAlefHamza = false
	p.FoldAlefWasla = false
	p.FoldHamzaSeat = false

	withHamza := p.Normalize("أحمد")    // أحمد
	withoutHamza := p.Normalize("احمد") // احمد
	if withHamza == withoutHamza {
		t.Fatalf("FoldAlefHamza=false did not preserve the distinction: both normalized to %q", withHamza)
	}

	waw := p.Normalize("مسؤول")   // مسؤول (waw with hamza)
	plain := p.Normalize("مسوول") // مسوول (plain waw)
	if waw == plain {
		t.Fatalf("FoldHamzaSeat=false did not preserve the distinction: both normalized to %q", waw)
	}
}

func TestProfileStrict(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"voweled, ta-marbuta kept",
			"اَلْعَرَبِيَّةُ",
			"العربية"},
		{"tatweel stripped, ta-marbuta kept",
			"العــربية",
			"العربية"},
		{"alef variants kept apart (madda/hamza marks survive, decomposed)",
			"آأإٱ",
			"آأإٱ"},
		{"seated hamza and ta-marbuta kept apart",
			"مسؤول فئة",
			"مسؤول فئة"},
		{"alef maksura kept apart from yeh",
			"على",
			"على"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProfileStrict.Normalize(tt.input); got != tt.want {
				t.Fatalf("ProfileStrict.Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestProfileLoose includes rasm in addition to the ordinary search folds.
func TestProfileLoose(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"voweled", "اَلْعَرَبِيَّةُ", "العرٮٮه"},
		{"tatweel", "العــربية", "العرٮٮه"},
		{"alef variants", "آأإٱ", "اااا"},
		{"seated hamza", "مسؤول فئة", "مسوول ٯٮه"},
		{"alef maksura", "على", "علٮ"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProfileLoose.Normalize(tt.input); got != tt.want {
				t.Fatalf("ProfileLoose.Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestProfileLucene is verified against ArabicNormalizer.java from the
// Apache Lucene repository (lucene/analysis/common/src/java/org/apache/
// lucene/analysis/ar/ArabicNormalizer.java); see docs/NORMALIZATION.md.
func TestProfileLucene(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"voweled (tatweel/harakat stripped like Search)",
			"اَلْعَرَبِيَّةُ",
			"العربيه"},
		{"tatweel stripped",
			"العــربية",
			"العربيه"},
		{"madda/hamza folded, wasla left alone (unlike ProfileSearch/CAMeL)",
			"آأإٱ",
			"اااٱ"},
		{"hamza-seated waw/yeh NOT folded (unlike ProfileSearch/CAMeL); ta-marbuta folded",
			"مسؤول فئة",
			"مسؤول فئه"},
		{"alef maksura folded",
			"على",
			"علي"},
		{"non-Arabic combining marks outside Lucene's 8-codepoint harakat set survive (documented divergence from real, non-NFD Lucene)",
			"café",
			"café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProfileLucene.Normalize(tt.input); got != tt.want {
				t.Fatalf("ProfileLucene.Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestProfileCAMeL is verified against normalize.py, dediac.py, and
// charsets.py from the CAMeL Tools repository (camel_tools/utils/); see
// docs/NORMALIZATION.md.
func TestProfileCAMeL(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"voweled (superscript-alef-aware dediac, ta-marbuta folded)",
			"اَلْعَرَبِيَّةُ",
			"العربيه"},
		{"tatweel NOT stripped (unlike every other preset; no CAMeL helper for it)",
			"العــربية",
			"العــربيه"},
		{"full alef group folded, including wasla (matches normalize_alef_ar's regex exactly)",
			"آأإٱ",
			"اااا"},
		{"hamza-seated waw/yeh NOT folded; ta-marbuta folded",
			"مسؤول فئة",
			"مسؤول فئه"},
		{"alef maksura folded",
			"على",
			"علي"},
		{"non-Arabic combining marks survive (documented divergence: real CAMeL composes/NFKC instead of decomposing)",
			"café",
			"café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProfileCAMeL.Normalize(tt.input); got != tt.want {
				t.Fatalf("ProfileCAMeL.Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// profileBit indexes a Profile bool field for profileFromBits/bitsFromProfile.
// Adding another boolean field means adding one more constant
// here and one more line in each of those two functions. Multi-valued
// dimensions such as Languages get a separate fuzz
// input, as FuzzLanguageNormalizeProperties demonstrates. An explicit,
// growing positional-bool parameter list on every fuzz function does not
// scale the same way.
type profileBit uint

const (
	bitStripTashkil profileBit = iota
	bitStripTatweel
	bitFoldAlefHamza
	bitFoldAlefWasla
	bitFoldHamzaSeat
	bitFoldTaMarbuta
	bitFoldAlefMaksura
	bitFoldPresentation
	bitStripJoiners
	bitStripBidi
	bitFoldDigits
	bitFoldPunctuation
	bitStripQuranic
	bitRasm
)

func profileFromBits(bits uint16, scope int) Profile {
	has := func(b profileBit) bool { return bits&(1<<b) != 0 }
	return Profile{
		StripTashkil:     has(bitStripTashkil),
		TashkilScope:     TashkilScope(scope),
		StripTatweel:     has(bitStripTatweel),
		FoldAlefHamza:    has(bitFoldAlefHamza),
		FoldAlefWasla:    has(bitFoldAlefWasla),
		FoldHamzaSeat:    has(bitFoldHamzaSeat),
		FoldTaMarbuta:    has(bitFoldTaMarbuta),
		FoldAlefMaksura:  has(bitFoldAlefMaksura),
		FoldPresentation: has(bitFoldPresentation),
		StripJoiners:     has(bitStripJoiners),
		StripBidi:        has(bitStripBidi),
		FoldDigits:       has(bitFoldDigits),
		FoldPunctuation:  has(bitFoldPunctuation),
		StripQuranic:     has(bitStripQuranic),
		Rasm:             has(bitRasm),
	}
}

const allProfileBits uint16 = 1<<bitStripTashkil | 1<<bitStripTatweel | 1<<bitFoldAlefHamza |
	1<<bitFoldAlefWasla | 1<<bitFoldHamzaSeat | 1<<bitFoldTaMarbuta | 1<<bitFoldAlefMaksura |
	1<<bitFoldPresentation | 1<<bitStripJoiners | 1<<bitStripBidi | 1<<bitFoldDigits |
	1<<bitFoldPunctuation | 1<<bitStripQuranic | 1<<bitRasm

func addProfileFuzzSeeds(f *testing.F) {
	seeds := []string{
		"اَلْعَرَبِيَّةُ",
		"آأإٱ",
		"مسؤول فئة",
		"العــربية",
		"café",
		"ﻻ ﷲ ﻛﺘﺎﺏ", // presentation forms and a ligature
		"a" + string(rune(0x200C)) + "b" + string(rune(0x200D)) + "c", // ZWNJ, ZWJ
		string(rune(0x061C)) + "حمد" + string(rune(0x200E)),           // ALM, LRM
		"١٢٣ ۱۲۳",                      // Arabic-Indic and Extended Arabic-Indic digits
		"قال، فقال؛",                   // Arabic comma, semicolon
		"اللهۖ" + string(rune(0x06DD)), // Quranic small-high-ligature mark, end of ayah
		"",
	}
	for _, s := range seeds {
		for scope := 0; scope <= 2; scope++ {
			f.Add(s, allProfileBits, scope)
			f.Add(s, uint16(0), scope)
		}
	}
}

// FuzzProfileNormalizeIdempotent fuzzes over the *entire* flag space, not
// just the five named presets: every bool combination plus every int value
// of TashkilScope, in or out of its defined range: Normalize must degrade
// safely and stay idempotent even for a scope Validate() would reject; that
// contract belongs to Normalize, not just to the valid-scope subset. This
// is what actually exercises the hamza lookback logic (and, as it turned
// out, the mark-reordering-after-a-merge case; see the final
// re-normalization step in Normalize) across combinations no named preset
// happens to cover.
func FuzzProfileNormalizeIdempotent(f *testing.F) {
	addProfileFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, s string, bits uint16, scope int) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		p := profileFromBits(bits, scope)
		once := p.Normalize(s)
		twice := p.Normalize(once)
		if once != twice {
			t.Fatalf("not idempotent for %+v: Normalize(%q) = %q, but Normalize(that) = %q", p, s, once, twice)
		}
	})
}

// FuzzProfileNormalizeValidUTF8 checks that valid UTF-8 input always
// normalizes to valid UTF-8 output, across the full flag space (not just
// ProfileSearch, which FuzzNormalizeValidUTF8 in normalize_test.go already
// covers).
func FuzzProfileNormalizeValidUTF8(f *testing.F) {
	addProfileFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, s string, bits uint16, scope int) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		p := profileFromBits(bits, scope)
		got := p.Normalize(s)
		if !utf8.ValidString(got) {
			t.Fatalf("Normalize(%q) under %+v produced invalid UTF-8: %q", s, p, got)
		}
	})
}

// FuzzProfileNormalizeNoPanic covers invalid UTF-8 and out-of-range
// TashkilScope values (Validate() exists precisely to let callers reject
// the latter; Normalize itself must still degrade safely, not panic).
func FuzzProfileNormalizeNoPanic(f *testing.F) {
	addProfileFuzzSeeds(f)
	f.Add(string([]byte{0xff, 0xfe}), allProfileBits, 7)

	f.Fuzz(func(t *testing.T, s string, bits uint16, scope int) {
		p := profileFromBits(bits, scope)
		_ = p.Normalize(s)
		_ = p.Validate()
	})
}
