package match

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Span is a half-open byte range [Start, End) in a normalized string.
type Span struct {
	Start, End int
}

// Matcher finds occurrences in an already-normalized haystack. A zero-length
// result means no match on that haystack.
type Matcher interface {
	// FindAll returns every non-overlapping occurrence in normalized,
	// in order, or nil if there is none.
	FindAll(normalized string) []Span

	// String returns the original, unnormalized pattern the Matcher was
	// built from, for diagnostics and human-readable output.
	String() string

	// Profile returns the arabic.Profile the Matcher was built with.
	// Callers (scan.Search in particular) must normalize a haystack with
	// this same profile before calling FindAll: a Matcher and its caller
	// disagreeing about which profile normalized the haystack would make
	// FindAll compare keys that were never meant to be compared.
	Profile() arabic.Profile
}

// NewLiteral builds a Matcher that finds query as a literal substring of
// text normalized under p. query must be valid UTF-8, p must be a valid
// Profile (see Profile.Validate), and query must not normalize to the empty
// string (ErrEmptyKey wraps arabic.ErrEmptyKey in that case).
func NewLiteral(query string, p arabic.Profile) (Matcher, error) {
	return NewLiterals([]string{query}, p, false)
}

// NewLiterals builds a Matcher that finds any of queries. When ignoreCase is
// true, it applies Unicode default case folding to both queries and haystacks;
// returned spans still index the normalized, pre-fold haystack.
func NewLiterals(queries []string, p arabic.Profile, ignoreCase bool) (Matcher, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("match: no queries")
	}

	keys := make([]string, 0, len(queries))
	for _, query := range queries {
		if !utf8.ValidString(query) {
			return nil, fmt.Errorf("match: query is not valid UTF-8")
		}
		key := p.Normalize(query)
		if ignoreCase {
			key = cases.Fold().String(key)
		}
		if key == "" {
			return nil, fmt.Errorf("match: %w", arabic.ErrEmptyKey)
		}
		keys = append(keys, key)
	}

	return &literalMatcher{
		queries:    append([]string(nil), queries...),
		keys:       keys,
		stableKeys: stableRawKeys(keys, queries, p, ignoreCase),
		rawAnchors: rawRejectionAnchors(keys, p, ignoreCase),
		profile:    p,
		ignoreCase: ignoreCase,
	}, nil
}

type literalMatcher struct {
	queries    []string // original, unnormalized patterns
	keys       []string // normalized comparison keys; guaranteed non-empty
	stableKeys []string // keys whose raw occurrence survives normalization
	rawAnchors string   // necessary raw characters, one per key; empty disables rejection
	profile    arabic.Profile
	ignoreCase bool
}

func (m *literalMatcher) String() string          { return strings.Join(m.queries, " | ") }
func (m *literalMatcher) Profile() arabic.Profile { return m.profile }

// MatchesStableRaw can prove a positive literal match without normalizing the
// whole line. A false result is inconclusive: folding may still create a match.
func (m *literalMatcher) MatchesStableRaw(line string) bool {
	for _, key := range m.stableKeys {
		if strings.Contains(line, key) {
			return true
		}
	}
	return false
}

// These letters are unchanged by every non-rasm profile and are not targets
// of language or spelling folds. NFD cannot introduce one from a different
// non-presentation rune; TestRawRejectionAnchorSources checks that Unicode
// property against the current normalization tables. Presentation forms can
// introduce them, but every such form has an EF leading UTF-8 byte.
const rawRejectionAnchorRunes = "بتثجحخدذرزسشصضطظعغفقلمن"

func rawRejectionAnchors(keys []string, p arabic.Profile, ignoreCase bool) string {
	if ignoreCase || p.Rasm || len(keys) > 4 {
		return ""
	}
	var anchors strings.Builder
	for _, key := range keys {
		found := false
		for _, r := range key {
			if !strings.ContainsRune(rawRejectionAnchorRunes, r) {
				continue
			}
			if !strings.ContainsRune(anchors.String(), r) {
				anchors.WriteRune(r)
			}
			found = true
			break
		}
		if !found {
			return ""
		}
	}
	return anchors.String()
}

// CannotMatchRaw proves a negative only when every key has a stable anchor
// and the line contains neither any anchor nor a possible presentation form.
// A true result means normalization and matching can be skipped for this line.
func (m *literalMatcher) CannotMatchRaw(line string) bool {
	if m.rawAnchors == "" {
		return false
	}
	if len(m.rawAnchors) == 2 {
		if strings.Contains(line, m.rawAnchors) {
			return false
		}
	} else if strings.ContainsAny(line, m.rawAnchors) {
		return false
	}
	return strings.IndexByte(line, 0xef) < 0
}

func stableRawKeys(keys, queries []string, p arabic.Profile, ignoreCase bool) []string {
	if ignoreCase {
		return nil
	}
	var stable []string
	for i, key := range keys {
		safe := true
		for _, r := range key {
			// These are canonical starters. Normalization cannot move a mark
			// across one or insert text between two of them. Check that this
			// profile leaves each character unchanged as well.
			if !rawStarter(r) || p.Normalize(string(r)) != string(r) {
				safe = false
				break
			}
		}
		if safe {
			stable = append(stable, key)
			// Common single-letter folds can still be recognized in raw text.
			// Bound the combinations so a long query cannot make every line
			// probe an exponential number of variants.
			variants := []string{key}
			for pos, r := range []rune(key) {
				var alternate rune
				switch {
				case r == 'ه' && p.FoldTaMarbuta:
					alternate = 'ة'
				case r == 'ي' && p.FoldAlefMaksura:
					alternate = 'ى'
				}
				if alternate == 0 || len(variants) >= 32 {
					continue
				}
				for _, variant := range append([]string(nil), variants...) {
					runes := []rune(variant)
					runes[pos] = alternate
					candidate := string(runes)
					if p.Normalize(candidate) == key {
						variants = append(variants, candidate)
						stable = append(stable, candidate)
					}
				}
			}
		}
		// An exact copy of the original query also survives as its normalized
		// key when it consists only of canonical starters. This covers common
		// spellings such as مكتبة, whose key contains a folded final letter.
		query := queries[i]
		if query == key || !norm.NFD.IsNormalString(query) {
			continue
		}
		safe = true
		for _, r := range query {
			if !rawStarter(r) {
				safe = false
				break
			}
		}
		if safe {
			seen := false
			for _, candidate := range stable {
				if candidate == query {
					seen = true
					break
				}
			}
			if !seen {
				stable = append(stable, query)
			}
		}
	}
	return stable
}

func rawStarter(r rune) bool {
	return (r >= 'ا' && r <= 'ي') ||
		(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9')
}

func (m *literalMatcher) FindAll(normalized string) []Span {
	var starts, ends []int
	if m.ignoreCase {
		normalized, starts, ends = foldMapped(normalized)
	}
	var spans []Span
	for _, key := range m.keys {
		start := 0
		for {
			idx := strings.Index(normalized[start:], key)
			if idx < 0 {
				break
			}
			s := start + idx
			e := s + len(key)
			span := Span{Start: s, End: e}
			if m.ignoreCase {
				span.Start = starts[s]
				span.End = ends[e-1]
			}
			spans = append(spans, span)
			start = e // keys are never empty, so this always advances.
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start == spans[j].Start {
			return spans[i].End < spans[j].End
		}
		return spans[i].Start < spans[j].Start
	})
	return spans
}

// foldMapped folds s and records which byte range in s produced every folded
// byte. Separate start/end maps are necessary for expanding folds: the byte
// boundary between the two "s" bytes produced by "ß" is simultaneously the
// end of one possible match and the start of another, but either match must
// cover the complete original rune.
func foldMapped(s string) (folded string, starts, ends []int) {
	var b strings.Builder
	b.Grow(len(s))
	starts = make([]int, 0, len(s))
	ends = make([]int, 0, len(s))
	for start, r := range s {
		end := start + utf8.RuneLen(r)
		part := cases.Fold().String(s[start:end])
		b.WriteString(part)
		for range len(part) {
			starts = append(starts, start)
			ends = append(ends, end)
		}
	}
	return b.String(), starts, ends
}

// Matches reports whether any configured literal occurs. scan.Search uses this
// allocation-free fast path when it only needs line selection, while FindAll
// remains available for callers that need spans.
func (m *literalMatcher) Matches(normalized string) bool {
	if m.ignoreCase {
		normalized = cases.Fold().String(normalized)
	}
	for _, key := range m.keys {
		if strings.Contains(normalized, key) {
			return true
		}
	}
	return false
}
