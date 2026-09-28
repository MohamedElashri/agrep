package arabic

import (
	"strings"
	"testing"
)

func TestExpandPresentationForms(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"lam-alef ligature", "ﻻ", "لا"}, // ﻻ -> لا
		{"allah ligature", "ﷲ", "الله"},  // ﷲ -> الله
		// كتاب (kaf-teh-alef-beh) in the contextual forms real shaping
		// would actually choose: kaf initial, teh medial, alef final
		// (alef never connects forward), beh isolated.
		{"contextual letter forms", "ﻛﺘﺎﺏ", "كتاب"},
		{"no presentation forms: unchanged", "plain text مرحبا", "plain text مرحبا"},
		{"empty", "", ""},
		{"presentation form at start", "ﻻمرحبا", "لامرحبا"},
		{"presentation form at end", "مرحباﻻ", "مرحبالا"},
		{"two ligatures back to back", "ﻻﻻ", "لالا"},
		// FDFD (Bismillah ligature) has no UCD decomposition at all (see
		// gen/main.go's package comment), so it is deliberately left
		// unexpanded, unlike CAMeL Tools' hardcoded phrase substitution.
		{"Bismillah ligature has no decomposition: unchanged", "﷽", "﷽"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandPresentationForms(tt.input); got != tt.want {
				t.Fatalf("expandPresentationForms(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPresentationFormsIdempotent(t *testing.T) {
	inputs := []string{"ﻻ", "ﷲ", "ﷺ", "ﷻ", "", "plain", "ﻻﻻ مرحبا"}
	for _, s := range inputs {
		once := expandPresentationForms(s)
		twice := expandPresentationForms(once)
		if once != twice {
			t.Errorf("not idempotent: expandPresentationForms(%q) = %q, then %q", s, once, twice)
		}
	}
}

// TestPresentationFormDefectFixed is the literal regression test for the
// defect flagged from this project's very first plan: presentation forms
// carry only compatibility decompositions, which NFD alone never touches.
func TestPresentationFormDefectFixed(t *testing.T) {
	// "لا يوجد كتاب" ("there is no book"), typed with the lam-alef ligature
	// and each letter of "كتاب" in the contextual form real shaping would
	// choose (kaf initial, teh medial, alef final, beh isolated), the way
	// a PDF text layer commonly encodes Arabic.
	line := "ﻻ يوجد ﻛﺘﺎﺏ"
	query := "لا يوجد كتاب" // لا يوجد كتاب, plain letters

	normalizedLine := ProfileSearch.Normalize(line)
	normalizedQuery := ProfileSearch.Normalize(query)
	if normalizedQuery == "" {
		t.Fatal("query normalized to empty")
	}
	if !strings.Contains(normalizedLine, normalizedQuery) {
		t.Fatalf("presentation-form line %q (normalized: %q) does not contain plain-letter query %q (normalized: %q)",
			line, normalizedLine, query, normalizedQuery)
	}
}
