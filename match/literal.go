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
}

// NewLiteral builds a Matcher that finds query as a literal, normalized
// substring. query must be valid UTF-8 and must not normalize to the empty
// string (ErrEmptyKey wraps arabic.ErrEmptyKey in that case).
func NewLiteral(query string) (Matcher, error) {
	if !utf8.ValidString(query) {
		return nil, fmt.Errorf("match: query is not valid UTF-8")
	}

	key := arabic.Normalize(query)
	if key == "" {
		return nil, fmt.Errorf("match: %w", arabic.ErrEmptyKey)
	}

	return &literalMatcher{query: query, key: key}, nil
}

type literalMatcher struct {
	query string // original, unnormalized pattern
	key   string // normalized comparison key; guaranteed non-empty
}

func (m *literalMatcher) String() string { return m.query }

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
