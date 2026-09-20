package main

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var errEmptyQuery = errors.New("query is empty after normalization")

// normalizeArabic returns a comparison key. NFD first makes canonically
// equivalent Unicode spellings identical; removing Mn then strips tashkil and
// other combining marks. Non-Arabic base characters retain their spelling.
func normalizeArabic(s string) string {
	if s == "" {
		return ""
	}

	decomposed := s
	if !norm.NFD.IsNormalString(s) {
		decomposed = norm.NFD.String(s)
	}

	var b strings.Builder
	changed := false

	for offset, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			if !changed {
				b.Grow(len(decomposed))
				b.WriteString(decomposed[:offset])
				changed = true
			}
			continue
		}

		original := r
		switch r {
		case '\u0640': // Tatweel.
			if !changed {
				b.Grow(len(decomposed))
				b.WriteString(decomposed[:offset])
				changed = true
			}
			continue
		case '\u0622', '\u0623', '\u0625', '\u0671': // Alef variants and wasla.
			r = '\u0627'
		case '\u0624': // Waw with hamza.
			r = '\u0648'
		case '\u0626': // Yeh with hamza.
			r = '\u064A'
		case '\u0629': // Ta-marbuta.
			r = '\u0647'
		case '\u0649': // Alef maksura.
			r = '\u064A'
		}

		if r != original && !changed {
			b.Grow(len(decomposed))
			b.WriteString(decomposed[:offset])
			changed = true
		}
		if changed {
			b.WriteRune(r)
		}
	}

	if !changed {
		return decomposed
	}
	return b.String()
}

func normalizeQuery(query string) (string, error) {
	if !utf8.ValidString(query) {
		return "", errors.New("query is not valid UTF-8")
	}

	normalized := normalizeArabic(query)
	if normalized == "" {
		return "", errEmptyQuery
	}

	return normalized, nil
}
