package arabic

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ErrEmptyKey is returned by callers that reject a value which normalizes to
// the empty string, such as a search query consisting only of tashkil.
var ErrEmptyKey = errors.New("arabic: value is empty after normalization")

// Normalize returns a comparison key for s. NFD first makes canonically
// equivalent Unicode spellings identical; removing Mn then strips tashkil and
// other combining marks. Non-Arabic base characters retain their spelling.
func Normalize(s string) string {
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
		case 'ـ': // Tatweel.
			if !changed {
				b.Grow(len(decomposed))
				b.WriteString(decomposed[:offset])
				changed = true
			}
			continue
		case 'آ', 'أ', 'إ', 'ٱ': // Alef variants and wasla.
			r = 'ا'
		case 'ؤ': // Waw with hamza.
			r = 'و'
		case 'ئ': // Yeh with hamza.
			r = 'ي'
		case 'ة': // Ta-marbuta.
			r = 'ه'
		case 'ى': // Alef maksura.
			r = 'ي'
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
