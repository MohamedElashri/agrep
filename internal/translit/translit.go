// Package translit converts between Arabic text and supported Latin schemes.
package translit

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Scheme identifies a transliteration scheme.
type Scheme uint8

const (
	Buckwalter Scheme = iota
	ArabTeX
	ISO233
)

// Parse resolves a command-line scheme name.
func Parse(name string) (Scheme, error) {
	switch name {
	case "buckwalter":
		return Buckwalter, nil
	case "arabtex":
		return ArabTeX, nil
	case "iso233", "iso-233":
		return ISO233, nil
	default:
		return Buckwalter, fmt.Errorf("unknown transliteration %q (want buckwalter, arabtex, or iso233)", name)
	}
}

// String returns the canonical command-line spelling.
func (s Scheme) String() string {
	switch s {
	case ArabTeX:
		return "arabtex"
	case ISO233:
		return "iso233"
	default:
		return "buckwalter"
	}
}

type pair struct {
	arabic rune
	latin  string
}

type table struct {
	toArabic []pair
	toLatin  map[rune]string
}

// FromLatin converts a transliterated query to Arabic. Unrecognized text is
// copied unchanged. Longest-token matching handles ArabTeX and ISO 233 tokens.
func FromLatin(s string, scheme Scheme) string {
	if !utf8.ValidString(s) {
		return s
	}
	t := tableFor(scheme)
	if scheme == ISO233 {
		s = norm.NFC.String(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		matched := false
		for _, entry := range t.toArabic {
			if strings.HasPrefix(s, entry.latin) {
				b.WriteRune(entry.arabic)
				s = s[len(entry.latin):]
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		b.WriteRune(r)
		s = s[size:]
	}
	return b.String()
}

// ToLatin converts Arabic text and returns a byte-boundary map from the input
// to the rendered output. The map has len(s)+1 entries, allowing Match.Spans to
// be remapped so JSON offsets continue to index the adjacent rendered text.
func ToLatin(s string, scheme Scheme) (rendered string, boundaries []int) {
	t := tableFor(scheme)
	boundaries = make([]int, len(s)+1)
	var b strings.Builder
	b.Grow(len(s))
	for start, r := range s {
		end := start + utf8.RuneLen(r)
		outputStart := b.Len()
		if token, ok := t.toLatin[r]; ok {
			b.WriteString(token)
		} else {
			b.WriteRune(r)
		}
		for offset := start; offset < end; offset++ {
			boundaries[offset] = outputStart
		}
		boundaries[end] = b.Len()
	}
	return b.String(), boundaries
}

// MapSpan remaps a half-open input byte range through ToLatin's boundary map.
func MapSpan(boundaries []int, start, end int) (int, int, bool) {
	if start < 0 || end < start || end >= len(boundaries) {
		return 0, 0, false
	}
	return boundaries[start], boundaries[end], true
}

func tableFor(scheme Scheme) table {
	switch scheme {
	case ArabTeX:
		return arabTeXTable
	case ISO233:
		return iso233Table
	default:
		return buckwalterTable
	}
}

func newTable(pairs []pair) table {
	t := table{
		toArabic: append([]pair(nil), pairs...),
		toLatin:  make(map[rune]string, len(pairs)),
	}
	for _, entry := range pairs {
		if _, exists := t.toLatin[entry.arabic]; !exists {
			t.toLatin[entry.arabic] = entry.latin
		}
	}
	sort.SliceStable(t.toArabic, func(i, j int) bool {
		return len(t.toArabic[i].latin) > len(t.toArabic[j].latin)
	})
	return t
}

// Buckwalter follows the canonical character table used by CAMeL Tools.
var buckwalterPairs = []pair{
	{'ء', "'"}, {'آ', "|"}, {'أ', ">"}, {'ؤ', "&"}, {'إ', "<"}, {'ئ', "}"},
	{'ا', "A"}, {'ب', "b"}, {'ة', "p"}, {'ت', "t"}, {'ث', "v"}, {'ج', "j"},
	{'ح', "H"}, {'خ', "x"}, {'د', "d"}, {'ذ', "*"}, {'ر', "r"}, {'ز', "z"},
	{'س', "s"}, {'ش', "$"}, {'ص', "S"}, {'ض', "D"}, {'ط', "T"}, {'ظ', "Z"},
	{'ع', "E"}, {'غ', "g"}, {'ـ', "_"}, {'ف', "f"}, {'ق', "q"}, {'ك', "k"},
	{'ل', "l"}, {'م', "m"}, {'ن', "n"}, {'ه', "h"}, {'و', "w"}, {'ى', "Y"},
	{'ي', "y"}, {'ً', "F"}, {'ٌ', "N"}, {'ٍ', "K"}, {'َ', "a"}, {'ُ', "u"},
	{'ِ', "i"}, {'ّ', "~"}, {'ْ', "o"}, {'ٰ', "`"}, {'ٱ', "{"}, {'پ', "P"},
	{'چ', "J"}, {'ڤ', "V"}, {'گ', "G"},
}

// ArabTeX uses its documented ASCII letter codes. Contextual typesetting
// rules such as automatic hamza carriers and consonant doubling are outside
// this character-level conversion; the forms below provide stable tokens for
// literal search and output.
var arabTeXPairs = []pair{
	{'آ', "'A"}, {'أ', "'a"}, {'إ', "'i"}, {'ؤ', "'w"}, {'ئ', "'y"}, {'ء', "'"},
	{'ا', "A"}, {'ب', "b"}, {'ة', "T"}, {'ت', "t"}, {'ث', "_t"}, {'ج', "^g"},
	{'ح', ".h"}, {'خ', "_h"}, {'د', "d"}, {'ذ', "_d"}, {'ر', "r"}, {'ز', "z"},
	{'س', "s"}, {'ش', "^s"}, {'ص', ".s"}, {'ض', ".d"}, {'ط', ".t"}, {'ظ', ".z"},
	{'ع', "`"}, {'غ', ".g"}, {'ـ', "--"}, {'ف', "f"}, {'ق', "q"}, {'ك', "k"},
	{'ل', "l"}, {'م', "m"}, {'ن', "n"}, {'ه', "h"}, {'و', "w"}, {'ى', "Y"},
	{'ي', "y"}, {'ً', "aN"}, {'ٌ', "uN"}, {'ٍ', "iN"}, {'َ', "a"}, {'ُ', "u"},
	{'ِ', "i"}, {'ّ', "~"}, {'ْ', "o"}, {'ٰ', "_a"}, {'ٱ', "{A"}, {'پ', "p"},
	{'چ', "^c"}, {'ڤ', "v"}, {'گ', "g"},
}

// ISO 233 uses Unicode diacritics for the standard's consonant values. The
// hamza-carrier spellings use practical multi-character search tokens; this
// table does not implement the standard's contextual vowel rules.
var iso233Pairs = []pair{
	{'آ', "ʾā"}, {'أ', "ʾa"}, {'إ', "ʾi"}, {'ؤ', "ʾw"}, {'ئ', "ʾy"}, {'ء', "ʾ"},
	{'ا', "ā"}, {'ب', "b"}, {'ة', "ẗ"}, {'ت', "t"}, {'ث', "ṯ"}, {'ج', "ǧ"},
	{'ح', "ḥ"}, {'خ', "ẖ"}, {'د', "d"}, {'ذ', "ḏ"}, {'ر', "r"}, {'ز', "z"},
	{'س', "s"}, {'ش', "š"}, {'ص', "ṣ"}, {'ض', "ḍ"}, {'ط', "ṭ"}, {'ظ', "ẓ"},
	{'ع', "ʿ"}, {'غ', "ġ"}, {'ـ', "_"}, {'ف', "f"}, {'ق', "q"}, {'ك', "k"},
	{'ل', "l"}, {'م', "m"}, {'ن', "n"}, {'ه', "h"}, {'و', "w"}, {'ى', "ỳ"},
	{'ي', "y"}, {'ً', "an"}, {'ٌ', "un"}, {'ٍ', "in"}, {'َ', "a"}, {'ُ', "u"},
	{'ِ', "i"}, {'ّ', "ː"}, {'ْ', "°"}, {'ٰ', "á"}, {'ٱ', "ă"},
	{'،', ","}, {'؛', ";"}, {'؟', "?"},
}

var (
	buckwalterTable = newTable(buckwalterPairs)
	arabTeXTable    = newTable(arabTeXPairs)
	iso233Table     = newTable(iso233Pairs)
)
