package match

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
	"golang.org/x/text/cases"
)

// NewFuzzy builds a Matcher using unit-cost Levenshtein distance over
// normalized Unicode codepoints. Insertions, deletions, and substitutions each
// cost one. Returned spans cover the complete non-empty text side of the chosen
// alignment.
func NewFuzzy(queries []string, p arabic.Profile, ignoreCase bool, maxDistance int) (Matcher, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("match: %w", err)
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("match: no queries")
	}
	if maxDistance < 0 {
		return nil, fmt.Errorf("match: fuzzy distance must be non-negative")
	}

	patterns := make([]fuzzyPattern, 0, len(queries))
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
		patterns = append(patterns, newFuzzyPattern([]rune(key)))
	}

	return &fuzzyMatcher{
		queries:     append([]string(nil), queries...),
		patterns:    patterns,
		profile:     p,
		ignoreCase:  ignoreCase,
		maxDistance: maxDistance,
	}, nil
}

type fuzzyMatcher struct {
	queries     []string
	patterns    []fuzzyPattern
	profile     arabic.Profile
	ignoreCase  bool
	maxDistance int
}

func (m *fuzzyMatcher) String() string          { return strings.Join(m.queries, " | ") }
func (m *fuzzyMatcher) Profile() arabic.Profile { return m.profile }

func (m *fuzzyMatcher) Matches(normalized string) bool {
	if m.ignoreCase {
		normalized = cases.Fold().String(normalized)
	}
	if normalized == "" {
		return false
	}
	for i := range m.patterns {
		if m.patterns[i].hasMatch(normalized, m.maxDistance) {
			return true
		}
	}
	return false
}

func (m *fuzzyMatcher) FindAll(normalized string) []Span {
	comparison := normalized
	var foldStarts, foldEnds []int
	if m.ignoreCase {
		comparison, foldStarts, foldEnds = foldMapped(normalized)
	}
	runes, offsets := splitRunes(comparison)
	if len(runes) == 0 {
		return nil
	}

	var candidates []fuzzyCandidate
	for i := range m.patterns {
		candidates = append(candidates, m.patterns[i].candidates(runes, m.maxDistance)...)
	}
	selected := selectFuzzyCandidates(candidates, len(runes))
	spans := make([]Span, 0, len(selected))
	for _, candidate := range selected {
		start := offsets[candidate.start]
		end := offsets[candidate.end]
		if m.ignoreCase {
			start = foldStarts[start]
			end = foldEnds[end-1]
		}
		spans = append(spans, Span{Start: start, End: end})
	}

	// A full case-fold expansion can map two disjoint comparison intervals to
	// the same source rune. Keep the final normalized-text spans non-overlapping.
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start == spans[j].Start {
			return spans[i].End < spans[j].End
		}
		return spans[i].Start < spans[j].Start
	})
	out := spans[:0]
	for _, span := range spans {
		if len(out) > 0 && span.Start < out[len(out)-1].End {
			continue
		}
		out = append(out, span)
	}
	return out
}

type fuzzyPattern struct {
	forward bitPattern
	reverse bitPattern
}

func newFuzzyPattern(pattern []rune) fuzzyPattern {
	reversed := make([]rune, len(pattern))
	for i := range pattern {
		reversed[len(pattern)-1-i] = pattern[i]
	}
	return fuzzyPattern{
		forward: newBitPattern(pattern),
		reverse: newBitPattern(reversed),
	}
}

func (p *fuzzyPattern) hasMatch(text string, maxDistance int) bool {
	if p.forward.short {
		pv, mv := ^uint64(0), uint64(0)
		score := p.forward.length
		for _, r := range text {
			score = p.forward.advance64(pv, mv, score, r, false, &pv, &mv)
			if score <= maxDistance {
				return true
			}
		}
		return false
	}

	pv := new(big.Int).Set(&p.forward.mask)
	mv := new(big.Int)
	score := p.forward.length
	for _, r := range text {
		score = p.forward.advanceBig(pv, mv, score, r, false)
		if score <= maxDistance {
			return true
		}
	}
	return false
}

func (p *fuzzyPattern) candidates(text []rune, maxDistance int) []fuzzyCandidate {
	ends := p.forward.findEnds(text, maxDistance)
	candidates := make([]fuzzyCandidate, 0, len(ends))
	for _, end := range ends {
		if candidate, ok := p.bestAtEnd(text, end, maxDistance); ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func (p *fuzzyPattern) bestAtEnd(text []rune, end, maxDistance int) (fuzzyCandidate, bool) {
	minLength := p.forward.length - maxDistance
	if minLength < 1 {
		minLength = 1
	}
	maxLength := end
	if maxDistance <= int(^uint(0)>>1)-p.forward.length {
		if limit := p.forward.length + maxDistance; limit < maxLength {
			maxLength = limit
		}
	}
	if minLength > maxLength {
		return fuzzyCandidate{}, false
	}

	var best fuzzyCandidate
	found := false
	if p.reverse.short {
		pv, mv := ^uint64(0), uint64(0)
		score := p.reverse.length
		for length := 1; length <= maxLength; length++ {
			score = p.reverse.advance64(pv, mv, score, text[end-length], true, &pv, &mv)
			if length >= minLength && score <= maxDistance &&
				(!found || score < best.distance || score == best.distance && length > best.end-best.start) {
				best = fuzzyCandidate{start: end - length, end: end, distance: score}
				found = true
			}
		}
	} else {
		pv := new(big.Int).Set(&p.reverse.mask)
		mv := new(big.Int)
		score := p.reverse.length
		for length := 1; length <= maxLength; length++ {
			score = p.reverse.advanceBig(pv, mv, score, text[end-length], true)
			if length >= minLength && score <= maxDistance &&
				(!found || score < best.distance || score == best.distance && length > best.end-best.start) {
				best = fuzzyCandidate{start: end - length, end: end, distance: score}
				found = true
			}
		}
	}
	return best, found
}

type bitPattern struct {
	length int
	short  bool
	masks  map[rune]uint64
	bigEq  map[rune]*big.Int
	mask   big.Int
}

func newBitPattern(pattern []rune) bitPattern {
	p := bitPattern{length: len(pattern), short: len(pattern) <= 64}
	if p.short {
		p.masks = make(map[rune]uint64)
		for i, r := range pattern {
			p.masks[r] |= uint64(1) << uint(i)
		}
		return p
	}
	p.bigEq = make(map[rune]*big.Int)
	for i, r := range pattern {
		bits := p.bigEq[r]
		if bits == nil {
			bits = new(big.Int)
			p.bigEq[r] = bits
		}
		bits.SetBit(bits, i, 1)
	}
	p.mask.Lsh(big.NewInt(1), uint(len(pattern)))
	p.mask.Sub(&p.mask, big.NewInt(1))
	return p
}

func (p *bitPattern) advance64(pv, mv uint64, score int, r rune, global bool, nextPV, nextMV *uint64) int {
	eq := p.masks[r]
	xv := eq | mv
	xh := (((eq & pv) + pv) ^ pv) | eq
	ph := mv | ^(xh | pv)
	mh := pv & xh
	high := uint64(1) << uint(p.length-1)
	if ph&high != 0 {
		score++
	}
	if mh&high != 0 {
		score--
	}
	ph <<= 1
	if global {
		ph |= 1
	}
	mh <<= 1
	*nextPV = mh | ^(xv | ph)
	*nextMV = ph & xv
	return score
}

func (p *bitPattern) advanceBig(pv, mv *big.Int, score int, r rune, global bool) int {
	eq := p.bigEq[r]
	var zero big.Int
	if eq == nil {
		eq = &zero
	}
	var xv, xh, ph, mh, tmp, inverted big.Int
	xv.Or(eq, mv)
	tmp.And(eq, pv)
	tmp.Add(&tmp, pv)
	xh.Xor(&tmp, pv)
	xh.Or(&xh, eq)
	xh.And(&xh, &p.mask)
	ph.Or(&xh, pv)
	ph.Not(&ph)
	ph.And(&ph, &p.mask)
	ph.Or(&ph, mv)
	mh.And(pv, &xh)
	if ph.Bit(p.length-1) != 0 {
		score++
	}
	if mh.Bit(p.length-1) != 0 {
		score--
	}
	ph.Lsh(&ph, 1)
	if global {
		ph.SetBit(&ph, 0, 1)
	}
	ph.And(&ph, &p.mask)
	mh.Lsh(&mh, 1)
	mh.And(&mh, &p.mask)
	inverted.Or(&xv, &ph)
	inverted.Not(&inverted)
	inverted.And(&inverted, &p.mask)
	pv.Or(&mh, &inverted)
	pv.And(pv, &p.mask)
	mv.And(&ph, &xv)
	return score
}

func (p *bitPattern) findEnds(text []rune, maxDistance int) []int {
	var ends []int
	if p.short {
		pv, mv := ^uint64(0), uint64(0)
		score := p.length
		for i, r := range text {
			score = p.advance64(pv, mv, score, r, false, &pv, &mv)
			if score <= maxDistance {
				ends = append(ends, i+1)
			}
		}
		return ends
	}

	pv := new(big.Int).Set(&p.mask)
	mv := new(big.Int)
	score := p.length
	for i, r := range text {
		score = p.advanceBig(pv, mv, score, r, false)
		if score <= maxDistance {
			ends = append(ends, i+1)
		}
	}
	return ends
}

type fuzzyCandidate struct {
	start, end int
	distance   int
}

func selectFuzzyCandidates(candidates []fuzzyCandidate, textLength int) []fuzzyCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].start != candidates[j].start {
			return candidates[i].start < candidates[j].start
		}
		return candidates[i].end > candidates[j].end
	})
	occupied := newFenwick(textLength)
	selected := make([]fuzzyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.start < 0 || candidate.end <= candidate.start || candidate.end > textLength {
			continue
		}
		if occupied.rangeSum(candidate.start, candidate.end) != 0 {
			continue
		}
		selected = append(selected, candidate)
		for i := candidate.start; i < candidate.end; i++ {
			occupied.add(i, 1)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].start == selected[j].start {
			return selected[i].end < selected[j].end
		}
		return selected[i].start < selected[j].start
	})
	return selected
}

type fenwick []int

func newFenwick(length int) fenwick { return make(fenwick, length+1) }

func (f fenwick) add(index, delta int) {
	for i := index + 1; i < len(f); i += i & -i {
		f[i] += delta
	}
}

func (f fenwick) prefix(end int) int {
	total := 0
	for i := end; i > 0; i -= i & -i {
		total += f[i]
	}
	return total
}

func (f fenwick) rangeSum(start, end int) int { return f.prefix(end) - f.prefix(start) }

func splitRunes(s string) ([]rune, []int) {
	runes := make([]rune, 0, utf8.RuneCountInString(s))
	offsets := make([]int, 0, cap(runes)+1)
	for offset, r := range s {
		runes = append(runes, r)
		offsets = append(offsets, offset)
	}
	offsets = append(offsets, len(s))
	return runes, offsets
}
