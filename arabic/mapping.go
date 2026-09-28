package arabic

import (
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var mappedTokenPool sync.Pool

const maxPooledMappedTokens = 1 << 17 // about 3 MiB of mappedRune storage

// NormalizeMapped returns the same comparison key as Normalize and an index
// mapping normalized byte offsets back to byte offsets in s. idx has
// len(key)+1 entries and idx[len(key)] is len(s). Bytes produced by one input
// rune, including presentation-form and canonical decompositions, map to the
// start of that rune's original combining cluster.
//
// Use MapSpan to map a half-open normalized range. Looking up idx[end]
// directly is insufficient when one original rune expands to multiple
// normalized runes because an internal expansion boundary has the same origin
// as the bytes on both sides.
func (p Profile) NormalizeMapped(s string) (key string, idx []int32) {
	return p.NormalizeMappedInto(s, nil)
}

// NormalizeMappedInto is NormalizeMapped with caller-provided index storage.
// dst is reset to length zero and reused when its capacity is sufficient.
func (p Profile) NormalizeMappedInto(s string, dst []int32) (key string, idx []int32) {
	if s == "" {
		dst = dst[:0]
		return "", append(dst, 0)
	}

	var tokenBuffer *[]mappedRune
	if pooled := mappedTokenPool.Get(); pooled != nil {
		tokenBuffer = pooled.(*[]mappedRune)
	} else {
		tokenBuffer = new([]mappedRune)
	}
	tokens := mappedDecomposeInto(s, p.FoldPresentation, p.Languages, (*tokenBuffer)[:0])
	kept := tokens[:0]
	lastBase := rune(0)
	for _, token := range tokens {
		out, drop, nextBase := p.transformRune(token.r, lastBase)
		lastBase = nextBase
		if drop {
			continue
		}
		if out != token.r {
			token.r = out
			token.ccc = combiningClass(out)
		}
		kept = append(kept, token)
	}
	canonicalOrderMapped(kept)

	var b strings.Builder
	b.Grow(len(s))
	dst = dst[:0]
	for _, token := range kept {
		before := b.Len()
		b.WriteRune(token.r)
		for range b.Len() - before {
			dst = append(dst, int32(token.start))
		}
	}
	dst = append(dst, int32(len(s)))
	if cap(tokens) <= maxPooledMappedTokens {
		*tokenBuffer = tokens[:0]
		mappedTokenPool.Put(tokenBuffer)
	}
	return b.String(), dst
}

// MapSpan maps a half-open normalized byte range through idx. It advances the
// end across equal-origin bytes so a match covering only one component of an
// expanded presentation form still covers the complete original rune.
func MapSpan(idx []int32, start, end int) (originalStart, originalEnd int) {
	if len(idx) == 0 || start < 0 || end < start || end >= len(idx) {
		return 0, 0
	}
	originalStart = int(idx[start])
	if start == end {
		return originalStart, originalStart
	}
	originalEnd = int(idx[end])
	for i := end + 1; originalEnd <= originalStart && i < len(idx); i++ {
		originalEnd = int(idx[i])
	}
	if originalEnd < originalStart {
		originalEnd = originalStart
	}
	return originalStart, originalEnd
}

type mappedRune struct {
	r          rune
	ccc        uint8
	start, end int
}

func mappedDecomposeInto(s string, foldPresentation bool, languages LanguageSet, tokens []mappedRune) []mappedRune {
	if count := utf8.RuneCountInString(s); cap(tokens) < count {
		tokens = make([]mappedRune, 0, count)
	}
	for offset := 0; offset < len(s); {
		r, size := utf8.DecodeRuneInString(s[offset:])
		end := offset + size
		expansion := s[offset:end]
		if foldPresentation {
			if mapped, ok := presentationForms[r]; ok {
				expansion = mapped
			}
		}
		expansion = foldLanguagePrecomposed(expansion, languages)
		tokens = appendMappedDecomposition(tokens, expansion, offset, end)
		offset = end
	}
	canonicalOrderMapped(tokens)
	return tokens
}

func appendMappedDecomposition(tokens []mappedRune, source string, start, end int) []mappedRune {
	for offset := 0; offset < len(source); {
		properties := norm.NFD.PropertiesString(source[offset:])
		size := properties.Size()
		r, _ := utf8.DecodeRuneInString(source[offset:])
		decomposition := properties.Decomposition()
		if len(decomposition) == 0 {
			if r < 0xAC00 || r > 0xD7A3 {
				tokens = append(tokens, mappedRune{r: r, ccc: properties.CCC(), start: start, end: end})
			} else {
				// Hangul decomposition is algorithmic and is not exposed by
				// Properties.Decomposition. It is rare in Arabic corpora, so
				// use the allocating fallback only for that case.
				decomposed := norm.NFD.String(source[offset : offset+size])
				tokens = appendMappedNormalizedString(tokens, decomposed, start, end)
			}
		} else {
			tokens = appendMappedNormalizedBytes(tokens, decomposition, start, end)
		}
		offset += size
	}
	return tokens
}

func appendMappedNormalizedString(tokens []mappedRune, normalized string, start, end int) []mappedRune {
	for offset := 0; offset < len(normalized); {
		properties := norm.NFD.PropertiesString(normalized[offset:])
		r, size := utf8.DecodeRuneInString(normalized[offset:])
		tokens = append(tokens, mappedRune{r: r, ccc: properties.CCC(), start: start, end: end})
		offset += size
	}
	return tokens
}

func appendMappedNormalizedBytes(tokens []mappedRune, normalized []byte, start, end int) []mappedRune {
	for offset := 0; offset < len(normalized); {
		properties := norm.NFD.Properties(normalized[offset:])
		r, size := utf8.DecodeRune(normalized[offset:])
		tokens = append(tokens, mappedRune{r: r, ccc: properties.CCC(), start: start, end: end})
		offset += size
	}
	return tokens
}

func combiningClass(r rune) uint8 {
	return norm.NFD.PropertiesString(string(r)).CCC()
}

// canonicalOrderMapped performs NFD's stable canonical ordering while keeping
// provenance attached. Every rune in a combining cluster receives the union
// of that cluster's original interval. This keeps idx monotonic even when NFD
// moves a later input mark ahead of an earlier decomposed mark.
func canonicalOrderMapped(tokens []mappedRune) {
	for segmentStart := 0; segmentStart < len(tokens); {
		segmentEnd := segmentStart + 1
		for segmentEnd < len(tokens) && tokens[segmentEnd].ccc != 0 {
			segmentEnd++
		}

		sortStart := segmentStart
		if tokens[segmentStart].ccc == 0 {
			sortStart++
		}
		if segmentEnd-sortStart > 1 {
			for i := sortStart + 1; i < segmentEnd; i++ {
				current := tokens[i]
				j := i
				for j > sortStart && tokens[j-1].ccc > current.ccc {
					tokens[j] = tokens[j-1]
					j--
				}
				tokens[j] = current
			}
		}

		clusterStart, clusterEnd := tokens[segmentStart].start, tokens[segmentStart].end
		for i := segmentStart + 1; i < segmentEnd; i++ {
			if tokens[i].start < clusterStart {
				clusterStart = tokens[i].start
			}
			if tokens[i].end > clusterEnd {
				clusterEnd = tokens[i].end
			}
		}
		for i := segmentStart; i < segmentEnd; i++ {
			tokens[i].start = clusterStart
			tokens[i].end = clusterEnd
		}
		segmentStart = segmentEnd
	}
}
