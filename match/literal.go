package match

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
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
	// this same profile before calling FindAll — a Matcher and its caller
	// disagreeing about which profile normalized the haystack would make
	// FindAll compare keys that were never meant to be compared.
	Profile() arabic.Profile
}

// NewLiteral builds a Matcher that finds query as a literal substring of
// text normalized under p. query must be valid UTF-8, p must be a valid
// Profile (see Profile.Validate), and query must not normalize to the empty
// string (ErrEmptyKey wraps arabic.ErrEmptyKey in that case).
func NewLiteral(query string, p arabic.Profile) (Matcher, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	if !utf8.ValidString(query) {
		return nil, fmt.Errorf("match: query is not valid UTF-8")
	}

	key := p.Normalize(query)
	if key == "" {
		return nil, fmt.Errorf("match: %w", arabic.ErrEmptyKey)
	}

	return &literalMatcher{query: query, key: key, profile: p}, nil
}

type literalMatcher struct {
	query   string // original, unnormalized pattern
	key     string // normalized comparison key; guaranteed non-empty
	profile arabic.Profile
}

func (m *literalMatcher) String() string          { return m.query }
func (m *literalMatcher) Profile() arabic.Profile { return m.profile }

func (m *literalMatcher) FindAll(normalized string) []Span {
	var spans []Span
	start := 0
	for {
		idx := strings.Index(normalized[start:], m.key)
		if idx < 0 {
			return spans
		}
		s := start + idx
		e := s + len(m.key)
		spans = append(spans, Span{Start: s, End: e})
		start = e // key is never empty, so this always advances.
	}
}
