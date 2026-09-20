package arabic

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// TashkilScope selects which combining marks StripTashkil removes.
type TashkilScope int

const (
	// TashkilAllMn removes every Unicode non-spacing mark (Mn) — agrep's
	// original, default behavior. It also catches the hamza/madda marks
	// below when they appear outside their recognized alef/waw/yeh
	// context (see the note on FoldAlefHamza).
	TashkilAllMn TashkilScope = iota

	// TashkilLuceneHarakat removes exactly the eight codepoints Apache
	// Lucene's ArabicNormalizer treats as harakat: fathatan, dammatan,
	// kasratan, fatha, damma, kasra, shadda, and sukun. It leaves other
	// combining marks — superscript alef, Quranic annotation marks —
	// untouched, matching Lucene's narrower, closed set.
	TashkilLuceneHarakat

	// TashkilCAMeLDiac removes CAMeL Tools' dediac_ar set: Lucene's eight
	// harakat plus superscript alef (U+0670).
	TashkilCAMeLDiac
)

// Validate reports whether p's fields hold a recognized value. Only
// TashkilScope can currently be out of range (constructed directly rather
// than through a preset).
func (p Profile) Validate() error {
	switch p.TashkilScope {
	case TashkilAllMn, TashkilLuceneHarakat, TashkilCAMeLDiac:
		return nil
	default:
		return errors.New("arabic: invalid TashkilScope")
	}
}

func tashkilScopeMatches(scope TashkilScope, r rune) bool {
	switch scope {
	case TashkilLuceneHarakat:
		switch r {
		case 'ً', 'ٌ', 'ٍ', 'َ', 'ُ', 'ِ', 'ّ', 'ْ':
			return true
		}
		return false
	case TashkilCAMeLDiac:
		switch r {
		case 'ً', 'ٌ', 'ٍ', 'َ', 'ُ', 'ِ', 'ّ', 'ْ', 'ٰ':
			return true
		}
		return false
	default: // TashkilAllMn
		return unicode.Is(unicode.Mn, r)
	}
}

// Profile controls which normalization rules Normalize applies. The zero
// Profile applies no rules at all (Normalize becomes NFD decomposition with
// nothing folded or stripped).
//
// Hamza handling needs a short explanation. Unicode NFD canonically
// decomposes أ, آ, إ, ؤ, and ئ into a base letter (ا, و, or ي) followed by a
// standalone combining mark: COMBINING MADDA ABOVE (U+0653), COMBINING
// HAMZA ABOVE (U+0654), or COMBINING HAMZA BELOW (U+0655). Normalize always
// decomposes with NFD first, so those marks — not the precomposed letters —
// are what it actually sees. FoldAlefHamza and FoldHamzaSeat decide whether
// each mark is dropped (folding the letter to its bare base) or kept
// (preserving the distinction) when it immediately follows its usual base
// letter; StripTashkil/TashkilScope never overrides that decision. This
// matters because a mark left in place is what makes "preserve every
// orthographic distinction" (see ProfileStrict) actually true even when
// StripTashkil is also on — without this split, NFD decomposition plus a
// blanket Mn-strip would silently re-merge hamza spellings regardless of
// FoldAlefHamza/FoldHamzaSeat.
//
// A madda/hamza mark that does *not* immediately follow ا, و, or ي (an
// unusual, non-standard placement) is not specially recognized: it falls
// back to ordinary StripTashkil/TashkilScope handling, like any other
// combining mark.
type Profile struct {
	// StripTashkil removes combining marks selected by TashkilScope. It
	// never removes the hamza/madda marks covered by FoldAlefHamza or
	// FoldHamzaSeat when they're in their recognized context — see above.
	StripTashkil bool
	// TashkilScope selects which marks StripTashkil removes. The zero
	// value, TashkilAllMn, has no effect when StripTashkil is false.
	TashkilScope TashkilScope

	// StripTatweel removes tatweel (U+0640).
	StripTatweel bool

	// FoldAlefHamza folds آ (madda), أ (hamza above), and إ (hamza below)
	// to bare ا. It does not affect ٱ (wasla, U+0671); see FoldAlefWasla.
	// Lucene's ArabicNormalizer folds this group but leaves wasla alone;
	// CAMeL Tools' normalize_alef_ar folds all four together.
	FoldAlefHamza bool
	// FoldAlefWasla folds ٱ (wasla, U+0671) to bare ا. Kept separate from
	// FoldAlefHamza so a profile can match either convention above.
	FoldAlefWasla bool

	// FoldHamzaSeat folds ؤ (waw with hamza) to و and ئ (yeh with hamza)
	// to ي.
	FoldHamzaSeat bool

	// FoldTaMarbuta folds ة to ه.
	FoldTaMarbuta bool
	// FoldAlefMaksura folds ى to ي.
	FoldAlefMaksura bool
}

var (
	// ProfileSearch is agrep's original, default behavior: every fold
	// below is enabled and every Unicode non-spacing mark is stripped.
	// This is the default profile; its output must never silently change
	// between releases.
	ProfileSearch = Profile{
		StripTashkil:    true,
		TashkilScope:    TashkilAllMn,
		StripTatweel:    true,
		FoldAlefHamza:   true,
		FoldAlefWasla:   true,
		FoldHamzaSeat:   true,
		FoldTaMarbuta:   true,
		FoldAlefMaksura: true,
	}

	// ProfileStrict strips only cosmetic marks (tashkil, tatweel) and
	// preserves every letter-level orthographic distinction: hamza seat,
	// ta-marbuta vs. heh, and alef-maksura vs. yeh are all kept apart.
	// "Preserves" means the normalized key stays distinguishable from a
	// differently-spelled key, not that it is byte-identical to the
	// original spelling — a kept hamza mark stays in its NFD-decomposed
	// form (base letter + standalone combining mark) rather than being
	// recomposed to its precomposed codepoint.
	ProfileStrict = Profile{
		StripTashkil: true,
		TashkilScope: TashkilAllMn,
		StripTatweel: true,
	}

	// ProfileLoose enables every fold this phase defines. It is currently
	// identical to ProfileSearch: there is nothing left to turn on until
	// Phase 4 adds digit/punctuation folding and Phase 5/7/9 add language
	// and Rasm folding, at which point ProfileLoose enables those too and
	// ProfileSearch does not.
	ProfileLoose = Profile{
		StripTashkil:    true,
		TashkilScope:    TashkilAllMn,
		StripTatweel:    true,
		FoldAlefHamza:   true,
		FoldAlefWasla:   true,
		FoldHamzaSeat:   true,
		FoldTaMarbuta:   true,
		FoldAlefMaksura: true,
	}

	// ProfileLucene reproduces Apache Lucene's ArabicNormalizer: folds the
	// madda/hamza-above/hamza-below alef group (but not wasla), ta-marbuta,
	// and alef-maksura; strips tatweel and exactly the eight codepoints
	// Lucene treats as harakat. It does not fold hamza-seated waw/yeh —
	// Lucene's normalizer has no case for either. Verified against
	// ArabicNormalizer.java from the Apache Lucene repository; see
	// docs/NORMALIZATION.md for the exact source reference. This
	// implementation's NFD-based engine may decompose non-Arabic
	// precomposed characters (e.g. Latin é) that real Lucene, which never
	// decomposes anything, would leave untouched — the fidelity claim
	// covers Arabic-script normalization only.
	ProfileLucene = Profile{
		StripTashkil:    true,
		TashkilScope:    TashkilLuceneHarakat,
		StripTatweel:    true,
		FoldAlefHamza:   true,
		FoldAlefWasla:   false,
		FoldHamzaSeat:   false,
		FoldTaMarbuta:   true,
		FoldAlefMaksura: true,
	}

	// ProfileCAMeL reproduces the normalization CAMeL Tools users compose
	// from normalize_alef_ar (which, unlike Lucene, does fold wasla),
	// normalize_alef_maksura_ar, normalize_teh_marbuta_ar, and dediac_ar's
	// AR_DIAC_CHARSET (Lucene's eight harakat plus superscript alef). It
	// does not strip tatweel or fold hamza-seated waw/yeh — CAMeL Tools'
	// normalize module has no public helper for either. Verified against
	// normalize.py, dediac.py, and charsets.py from the CAMeL Tools
	// repository; see docs/NORMALIZATION.md. As with ProfileLucene, the
	// fidelity claim covers Arabic-script normalization only: CAMeL Tools'
	// own normalize_unicode composes (NFKC) rather than decomposes, so it
	// leaves precomposed non-Arabic characters like Latin é untouched,
	// while this engine's NFD-first pass may decompose them.
	ProfileCAMeL = Profile{
		StripTashkil:    true,
		TashkilScope:    TashkilCAMeLDiac,
		StripTatweel:    false,
		FoldAlefHamza:   true,
		FoldAlefWasla:   true,
		FoldHamzaSeat:   false,
		FoldTaMarbuta:   true,
		FoldAlefMaksura: true,
	}
)

// Normalize returns a comparison key for s under p. NFD decomposition runs
// first so canonically equivalent Unicode spellings become identical; see
// the Profile doc comment for how that interacts with hamza folding.
func (p Profile) Normalize(s string) string {
	if s == "" {
		return ""
	}

	decomposed := s
	if !norm.NFD.IsNormalString(s) {
		decomposed = norm.NFD.String(s)
	}

	var b strings.Builder
	changed := false
	var lastBase rune // 0627, 0648, or 064A if that was just written; else 0

	for offset, r := range decomposed {
		drop := false
		out := r

		switch r {
		case 'ٓ', 'ٔ', 'ٕ': // combining madda/hamza-above/hamza-below
			switch {
			case lastBase == 'ا': // follows alef: آ, أ, or إ
				drop = p.FoldAlefHamza
			case r == 'ٔ' && (lastBase == 'و' || lastBase == 'ي'): // follows waw/yeh: ؤ or ئ
				drop = p.FoldHamzaSeat
			default: // unrecognized context: treat as an ordinary mark
				drop = p.StripTashkil && tashkilScopeMatches(p.TashkilScope, r)
			}
		case 'ـ': // Tatweel
			drop = p.StripTatweel
		case 'ٱ': // Alef wasla
			if p.FoldAlefWasla {
				out = 'ا'
			}
		case 'ة': // Ta marbuta
			if p.FoldTaMarbuta {
				out = 'ه'
			}
		case 'ى': // Alef maksura
			if p.FoldAlefMaksura {
				out = 'ي'
			}
		default: // includes bare ا, و, ي themselves (out == r, never dropped)
			drop = p.StripTashkil && tashkilScopeMatches(p.TashkilScope, r)
		}

		// Decide what context the *next* iteration sees, from how this one
		// actually turned out:
		var nextBase rune
		switch {
		case !drop && (out == 'ا' || out == 'و' || out == 'ي'):
			// A real alef/waw/yeh reached the output — whether it started
			// that way or a fold above (wasla→alef, alef-maksura→yeh) just
			// produced it. It can carry a following hamza/madda mark. This
			// has to key off the OUTPUT, not the input rune r, or folding
			// wasla to ا would make a trailing hamza mark resolve as
			// "unrecognized context" on this pass while a second
			// Normalize call — seeing the already-folded ا directly —
			// would recognize it, breaking idempotence.
			nextBase = out
		case drop || unicode.Is(unicode.Mn, r):
			// Either this rune vanished from the output — tatweel, a
			// stripped mark, a folded-away hamza mark — so whatever came
			// before and after it become adjacent in the result and any
			// in-progress context must survive across it; or it's a
			// combining mark that was kept but isn't itself a base letter.
			// Either way, preserve whatever context was already in
			// effect. (Canonical ordering sorts every harakat mark —
			// combining class 27-35 — before a hamza/madda mark, 230, or
			// 220 for hamza-below, on the same base, so a vowel mark
			// commonly sits between a hamza-carrying letter and its hamza
			// mark: NFD(أُ) is alef, damma, hamza-above, not alef,
			// hamza-above, damma. Without preserving context across a
			// dropped separator too, removing e.g. a tatweel that used to
			// sit between a base and its later-arriving hamza mark would
			// make the two ends of a fresh Normalize call disagree with
			// what a second call — now seeing them truly adjacent — would
			// resolve, which is the same idempotence break in a different
			// guise.)
			nextBase = lastBase
		default:
			// An ordinary, kept, non-mark, non-alef/waw/yeh character (a
			// different letter, space, digit, punctuation, ...) breaks any
			// in-progress cluster.
			nextBase = 0
		}
		lastBase = nextBase

		if drop {
			if !changed {
				b.Grow(len(decomposed))
				b.WriteString(decomposed[:offset])
				changed = true
			}
			continue
		}
		if out != r && !changed {
			b.Grow(len(decomposed))
			b.WriteString(decomposed[:offset])
			changed = true
		}
		if changed {
			b.WriteRune(out)
		}
	}

	if !changed {
		return decomposed
	}

	// Dropping a rune with combining class 0 (a "starter" — tatweel, or a
	// letter this profile folded away) can merge two previously-separate
	// runs of combining marks that canonical ordering never needed to sort
	// against each other while the starter still kept them apart. The loop
	// above preserves each surviving mark's original relative position, so
	// a merge like that can leave the result out of true canonical order.
	// Re-normalizing here is what makes Normalize(Normalize(x)) == Normalize(x):
	// without it, a second call's own leading NFD check would silently
	// re-sort what this call already returned.
	out := b.String()
	if !norm.NFD.IsNormalString(out) {
		out = norm.NFD.String(out)
	}
	return out
}
