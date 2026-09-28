package arabic

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseLanguages(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  LanguageSet
		text  string
	}{
		{"ar", LanguageArabic, "ar"},
		{"fa", LanguagePersian, "fa"},
		{"ar,fa", LanguageArabic | LanguagePersian, "ar,fa"},
		{"ug, ar,fa,ar", LanguageArabic | LanguagePersian | LanguageUyghur, "ar,fa,ug"},
		{"ur,ps,ku", LanguageUrdu | LanguagePashto | LanguageKurdish, "ur,ps,ku"},
	} {
		got, err := ParseLanguages(tt.input)
		if err != nil || got != tt.want || got.String() != tt.text {
			t.Fatalf("ParseLanguages(%q) = %v, %v, String=%q; want %v and %q", tt.input, got, err, got.String(), tt.want, tt.text)
		}
	}
	for _, input := range []string{"", "ar,", "xx"} {
		if _, err := ParseLanguages(input); err == nil {
			t.Fatalf("ParseLanguages(%q) returned no error", input)
		}
	}
}

func TestLanguageNormalization(t *testing.T) {
	tests := []struct {
		name      string
		languages LanguageSet
		input     string
		want      string
	}{
		{"Arabic preserves Persian forms", LanguageArabic, "كک يی", "كک يی"},
		{"Persian alone preserves Arabic distinction", LanguagePersian, "كک يی", "كک يی"},
		{"Arabic Persian cross-fold", LanguageArabic | LanguagePersian, "كک يی", "كك يي"},
		{"Arabic Urdu shared forms", LanguageArabic | LanguageUrdu, "كک يی هہ ھ ے ں ٹ ڈ ڑ", "كك يي هه ھ ے ں ٹ ڈ ڑ"},
		{"Arabic Pashto shared forms", LanguageArabic | LanguagePashto, "كک يی ۍ ښ ږ ځ څ ډ ړ ټ ڼ", "كك يي ۍ ښ ږ ځ څ ډ ړ ټ ڼ"},
		{"Arabic Kurdish shared forms", LanguageArabic | LanguageKurdish, "كک يی ڵ ڕ ۆ ێ", "كك يي ڵ ڕ ۆ ێ"},
		{"Arabic Uyghur shared forms", LanguageArabic | LanguageUyghur, "كک يی ې ۈ ۆ ۇ", "كك يي ې ۈ ۆ ۇ"},
		{"Persian deprecated ezafe form", LanguagePersian, "ۀ هٔ", "ه ه"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ProfileSearch
			p.Languages = tt.languages
			if got := p.Normalize(tt.input); got != tt.want {
				t.Fatalf("Normalize(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLanguageSpecificLettersRemainDistinct(t *testing.T) {
	tests := []struct {
		name      string
		languages LanguageSet
		letters   string
	}{
		{"Urdu", LanguageUrdu, "هہحھيےنںتٹدڈرڑ"},
		{"Pashto", LanguagePashto, "شښژږجځچڅدډرړتټنڼيیۍئ"},
		{"Kurdish", LanguageKurdish, "لڵرڕوۆيێ"},
		{"Uyghur", LanguageUyghur, "يېوۈۆۇ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ProfileStrict
			p.Languages = tt.languages
			seen := make(map[string]rune)
			for _, letter := range tt.letters {
				key := p.Normalize(string(letter))
				if prior, exists := seen[key]; exists {
					t.Fatalf("%U and %U both normalized to %q", prior, letter, key)
				}
				seen[key] = letter
			}
		})
	}
}

func TestLanguageAwareZWNJ(t *testing.T) {
	const input = "می\u200cروم\u200d"
	for _, languages := range []LanguageSet{LanguagePersian, LanguageKurdish, LanguageUyghur} {
		p := ProfileSearch
		p.Languages = languages
		if got := p.Normalize(input); got != "می\u200cروم" {
			t.Fatalf("languages=%s Normalize(%q) = %q", languages, input, got)
		}
	}
	p := ProfileSearch
	p.Languages = LanguageArabic
	if got := p.Normalize(input); got != "میروم" {
		t.Fatalf("Arabic Normalize(%q) = %q", input, got)
	}
}

func FuzzLanguageNormalizeProperties(f *testing.F) {
	for _, seed := range []string{
		"كتاب کتاب",
		"ۀ هٔ می\u200cروم",
		"ہ ھ ے ں ٹ ڈ ڑ",
		"ښ ږ ځ څ ډ ړ ټ ڼ ی ۍ ئ",
		"ڵ ڕ ۆ ێ",
		"ې ۈ ۆ ۇ",
	} {
		f.Add(seed, allProfileBits, 0, uint8(LanguageAll))
		f.Add(seed, uint16(0), 0, uint8(LanguageArabic))
	}
	f.Fuzz(func(t *testing.T, input string, bits uint16, scope int, languageBits uint8) {
		if !utf8.ValidString(input) {
			t.Skip()
		}
		p := profileFromBits(bits, scope)
		p.Languages = LanguageSet(languageBits) & LanguageAll
		once := p.Normalize(input)
		if twice := p.Normalize(once); once != twice {
			t.Fatalf("not idempotent under %+v: once=%q twice=%q", p, once, twice)
		}
		mapped, idx := p.NormalizeMapped(input)
		if mapped != once {
			t.Fatalf("mapped=%q Normalize=%q under %+v", mapped, once, p)
		}
		assertValidMapping(t, input, mapped, idx)
	})
}

func TestLanguageNamesContainNoWhitespace(t *testing.T) {
	if strings.ContainsAny(LanguageAll.String(), " \t\n") {
		t.Fatalf("LanguageAll.String() = %q", LanguageAll.String())
	}
}
