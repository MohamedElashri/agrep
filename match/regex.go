package match

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/MohamedElashri/agrep/arabic"
)

// NewRegex builds a Matcher from regular expressions evaluated directly over
// normalized text. Patterns are intentionally not normalized: regex syntax and
// character classes must retain their meaning. When ignoreCase is true, Go's
// Unicode-aware simple case folding is enabled for each expression.
func NewRegex(patterns []string, p arabic.Profile, ignoreCase bool) (Matcher, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	if len(patterns) == 0 {
		return nil, fmt.Errorf("match: no regex patterns")
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		expression := pattern
		if ignoreCase {
			expression = "(?i:" + pattern + ")"
		}
		re, err := regexp.Compile(expression)
		if err != nil {
			return nil, fmt.Errorf("match: invalid regex %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}
	return &regexMatcher{patterns: append([]string(nil), patterns...), expressions: compiled, profile: p}, nil
}

type regexMatcher struct {
	patterns    []string
	expressions []*regexp.Regexp
	profile     arabic.Profile
}

func (m *regexMatcher) String() string          { return strings.Join(m.patterns, " | ") }
func (m *regexMatcher) Profile() arabic.Profile { return m.profile }

func (m *regexMatcher) Matches(normalized string) bool {
	for _, expression := range m.expressions {
		if expression.MatchString(normalized) {
			return true
		}
	}
	return false
}

func (m *regexMatcher) FindAll(normalized string) []Span {
	var spans []Span
	for _, expression := range m.expressions {
		for _, pair := range expression.FindAllStringIndex(normalized, -1) {
			spans = append(spans, Span{Start: pair[0], End: pair[1]})
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
