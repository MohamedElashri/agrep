package arabic

import (
	"fmt"
	"strings"
)

// LanguageSet selects the Arabic-script orthographies whose equivalences are
// active. A single language preserves its distinct alphabet. Selecting Arabic
// with another language additionally folds codepoint variants shared across
// those writing systems to an Arabic canonical form.
type LanguageSet uint8

const (
	LanguageArabic LanguageSet = 1 << iota
	LanguagePersian
	LanguageUrdu
	LanguagePashto
	LanguageKurdish
	LanguageUyghur

	// LanguageAll contains every language currently recognized by ParseLanguages.
	LanguageAll = LanguageArabic | LanguagePersian | LanguageUrdu |
		LanguagePashto | LanguageKurdish | LanguageUyghur
)

// ParseLanguages parses the comma-separated CLI spelling ar,fa,ur,ps,ku,ug.
func ParseLanguages(value string) (LanguageSet, error) {
	if value == "" {
		return 0, fmt.Errorf("arabic: language list is empty")
	}
	var languages LanguageSet
	for _, item := range strings.Split(value, ",") {
		switch strings.TrimSpace(item) {
		case "ar":
			languages |= LanguageArabic
		case "fa":
			languages |= LanguagePersian
		case "ur":
			languages |= LanguageUrdu
		case "ps":
			languages |= LanguagePashto
		case "ku":
			languages |= LanguageKurdish
		case "ug":
			languages |= LanguageUyghur
		case "":
			return 0, fmt.Errorf("arabic: empty language in %q", value)
		default:
			return 0, fmt.Errorf("arabic: unknown language %q (want ar, fa, ur, ps, ku, or ug)", strings.TrimSpace(item))
		}
	}
	return languages, nil
}

// String returns languages in stable CLI order.
func (languages LanguageSet) String() string {
	var names []string
	for _, item := range []struct {
		language LanguageSet
		name     string
	}{
		{LanguageArabic, "ar"},
		{LanguagePersian, "fa"},
		{LanguageUrdu, "ur"},
		{LanguagePashto, "ps"},
		{LanguageKurdish, "ku"},
		{LanguageUyghur, "ug"},
	} {
		if languages.Has(item.language) {
			names = append(names, item.name)
		}
	}
	return strings.Join(names, ",")
}

// Has reports whether every bit in language is selected.
func (languages LanguageSet) Has(language LanguageSet) bool {
	return languages&language == language
}

// PreservesZWNJ reports whether ZWNJ carries orthographic meaning for at
// least one selected language. Persian, Sorani Kurdish, and Uyghur use it to
// distinguish otherwise joining sequences, so it must also remain word-internal.
func (languages LanguageSet) PreservesZWNJ() bool {
	return languages&(LanguagePersian|LanguageKurdish|LanguageUyghur) != 0
}

func (languages LanguageSet) crossesArabic() bool {
	return languages.Has(LanguageArabic) && languages&(LanguageAll&^LanguageArabic) != 0
}

func foldLanguagePrecomposed(s string, languages LanguageSet) string {
	if languages.Has(LanguagePersian) && strings.ContainsRune(s, 'ۀ') {
		return strings.ReplaceAll(s, "ۀ", "ه")
	}
	return s
}
