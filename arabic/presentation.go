package arabic

import "strings"

// expandPresentationForms replaces every rune found in the generated
// presentationForms table with its letter-sequence expansion, leaving
// everything else untouched. It runs before NFD decomposition in
// Profile.Normalize: NFD does not touch presentation forms at all (they
// carry only *compatibility* decompositions, which NFD ignores), so
// without this pass they survive normalization completely unfolded: the
// defect this rule exists to fix.
//
// The result of an expansion is always ordinary, non-presentation-form
// Arabic text (plain letters, and occasionally a plain space; see
// gen/main.go's package comment on the FE70-FE7F standalone-diacritic
// forms), so it flows into the rest of Normalize exactly as if it had been
// typed that way to begin with. That is also why this pass is idempotent
// by construction: a fully expanded string contains no more table keys
// left to expand, so applying it twice does nothing the first pass didn't
// already do.
func expandPresentationForms(s string) string {
	var b strings.Builder
	changed := false

	for offset, r := range s {
		expansion, ok := presentationForms[r]
		if !ok {
			if changed {
				b.WriteRune(r)
			}
			continue
		}
		if !changed {
			b.Grow(len(s))
			b.WriteString(s[:offset])
			changed = true
		}
		b.WriteString(expansion)
	}

	if !changed {
		return s
	}
	return b.String()
}
