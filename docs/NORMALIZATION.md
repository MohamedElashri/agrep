# Normalization

`arabic.Profile` controls which rules `Normalize` applies. Every profile
first decomposes the input with Unicode NFD, then applies the rules below in
a single pass. This document is the source of truth for what each rule does,
which codepoints it touches, and which of the five built-in presets enable
it. It exists so a claim like "matches Apache Lucene's ArabicNormalizer" is
checkable against something more precise than the code.

## Rules

| Rule | Field | Codepoints | What it does |
| --- | --- | --- | --- |
| Strip tashkil | `StripTashkil` + `TashkilScope` | see [Tashkil scopes](#tashkil-scopes) below | Removes combining diacritics, scoped by `TashkilScope`. |
| Strip tatweel | `StripTatweel` | U+0640 ARABIC TATWEEL | Removes the elongation character. |
| Fold alef-hamza | `FoldAlefHamza` | U+0622 (madda), U+0623 (hamza above), U+0625 (hamza below) → U+0627 (bare alef) | Folds three of the four hamza-seated/madda alef letters to bare alef. Does **not** include wasla — see next row. |
| Fold alef-wasla | `FoldAlefWasla` | U+0671 (wasla) → U+0627 (bare alef) | Kept separate from the rule above because Lucene folds the hamza group but not wasla, while CAMeL Tools folds all four together. |
| Fold hamza-seat | `FoldHamzaSeat` | U+0624 (waw+hamza) → U+0648 (waw); U+0626 (yeh+hamza) → U+064A (yeh) | Folds the two hamza-seated letters that aren't alef-based. |
| Fold ta-marbuta | `FoldTaMarbuta` | U+0629 → U+0647 | Ta-marbuta to heh. |
| Fold alef-maksura | `FoldAlefMaksura` | U+0649 → U+064A | Alef-maksura to yeh. |

### Why hamza folding needs NFD-aware handling, not a simple table

U+0622/0623/0624/0625/0626 all have a canonical NFD decomposition into a
base letter (ا, و, or ي) followed by a standalone combining mark — U+0653
(MADDA ABOVE), U+0654 (HAMZA ABOVE), or U+0655 (HAMZA BELOW). Since
`Normalize` decomposes with NFD first, it never actually sees those five
precomposed codepoints directly; it sees the base letter and a trailing
mark. `FoldAlefHamza`/`FoldHamzaSeat` decide whether to drop that mark
(folding the letter) or keep it (preserving the distinction) based on which
base letter it follows — `StripTashkil`/`TashkilScope` never overrides that
decision, in either direction.

This is more than a bookkeeping detail. Unicode's canonical ordering
algorithm sorts every harakat mark (combining class 27–35) *before* a
hamza/madda mark (230, or 220 for hamza-below) that shares the same base
letter, so an ordinary vocalized spelling like أُمّ ("mother") or إِلَى
("to") decomposes as *base, vowel mark, hamza mark* — not *base, hamza
mark, vowel mark*. A profile that folds hamza has to still recognize the
hamza mark once it finally arrives, and a profile that keeps hamza distinct
has to not let `StripTashkil` strip the intervening vowel mark's neighbor by
mistake. `arabic.Normalize`'s implementation keeps the base-letter context
alive across a run of combining marks (and across a dropped tatweel, which
can merge two marks that used to be in separate canonical-ordering runs)
specifically so this resolves correctly and stays idempotent —
`Normalize(Normalize(x)) == Normalize(x)` for any `Profile`, fuzz-tested
across the full flag space in `arabic/profile_test.go`.

One consequence: a profile that keeps a hamza distinction (e.g.
`ProfileStrict`, or `--keep-hamza`) does not guarantee its normalized key is
byte-identical to the original spelling — a kept hamza mark stays in its
NFD-decomposed form (base letter + standalone combining mark) rather than
being recomposed to the original precomposed codepoint. It only guarantees
the key stays *distinguishable* from a differently-spelled key.

### Tashkil scopes

`TashkilScope` selects which marks `StripTashkil` removes (it has no effect
on the hamza/madda marks in their recognized context, per above):

| Scope | Codepoints removed |
| --- | --- |
| `TashkilAllMn` (default) | Every Unicode non-spacing mark (`Mn`) — agrep's original behavior. Includes Quranic annotation marks, superscript alef, and anything else category `Mn`. |
| `TashkilLuceneHarakat` | Exactly U+064B–U+0652 (fathatan, dammatan, kasratan, fatha, damma, kasra, shadda, sukun) — the eight codepoints Lucene's `ArabicNormalizer` treats as harakat. Nothing else. |
| `TashkilCAMeLDiac` | The same eight, plus U+0670 (superscript alef) — CAMeL Tools' `AR_DIAC_CHARSET`. |

## Presets

| Preset | Strip tashkil | Tashkil scope | Strip tatweel | Fold alef-hamza | Fold wasla | Fold hamza-seat | Fold ta-marbuta | Fold alef-maksura |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `ProfileSearch` | ✓ | AllMn | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ProfileStrict` | ✓ | AllMn | ✓ | — | — | — | — | — |
| `ProfileLoose` | ✓ | AllMn | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ProfileLucene` | ✓ | LuceneHarakat | ✓ | ✓ | — | — | ✓ | ✓ |
| `ProfileCAMeL` | ✓ | CAMeLDiac | — | ✓ | ✓ | — | ✓ | ✓ |

- **`ProfileSearch`** is agrep's original, default, fixed behavior. Its
  output must never silently change between releases.
- **`ProfileStrict`** strips only cosmetic marks (tashkil, tatweel) and
  preserves every letter-level orthographic distinction. See the "not
  byte-identical" note above for what "preserves" actually guarantees.
- **`ProfileLoose`** enables every fold this phase defines — which is
  currently identical to `ProfileSearch`, since there's nothing left to turn
  on. Phase 4 (digit/punctuation folding) and Phase 5/7/9 (Rasm, language
  folding) are what will make it diverge.
- **`ProfileLucene`** and **`ProfileCAMeL`** are described in detail below.

## `ProfileLucene`

Reproduces [Apache Lucene's `ArabicNormalizer`][lucene-src] 

```java
switch (s[i]) {
  case ALEF_MADDA:
  case ALEF_HAMZA_ABOVE:
  case ALEF_HAMZA_BELOW:
    s[i] = ALEF;
    break;
  case DOTLESS_YEH: // alef maksura
    s[i] = YEH;
    break;
  case TEH_MARBUTA:
    s[i] = HEH;
    break;
  case TATWEEL:
  case KASRATAN: case DAMMATAN: case FATHATAN:
  case FATHA: case DAMMA: case KASRA:
  case SHADDA: case SUKUN:
    len = delete(s, i, len);
    i--;
    break;
}
```

Notably:

- It does **not** fold wasla (U+0671) — only the madda/hamza-above/hamza-below
  group. `ProfileLucene` sets `FoldAlefWasla: false` to match.
- It does **not** fold hamza-seated waw/yeh (ؤ/ئ) at all — there's no case
  for either. `ProfileLucene` sets `FoldHamzaSeat: false`.
- It strips exactly the eight listed harakat codepoints, not every Unicode
  `Mn` mark — no Quranic annotation marks, no superscript alef.
  `ProfileLucene` uses `TashkilLuceneHarakat`.

**Known, documented divergence:** real Lucene never decomposes anything —
it switches on the raw input characters directly, with no NFD step. This
implementation always decomposes first (that's what makes the hamza logic
above work at all). On Arabic-script text this makes no difference, since
none of the codepoints above survive precomposed through NFD anyway. On
non-Arabic text it can: `café` under `ProfileLucene` normalizes to a
5-rune decomposed form (`cafe` + a standalone combining acute accent),
because U+0301 isn't in Lucene's 8-codepoint harakat set, while real
Lucene leaves the precomposed `café` (4 runes) untouched. The fidelity
claim here covers Arabic-script normalization only.

## `ProfileCAMeL`

Reproduces the normalization [CAMeL Tools][camel-tools] users compose from
several of its functions

- [`normalize_alef_ar`][camel-normalize] — `re.sub(r'[إأٱآ]', 'ا', s)`.
  Unlike Lucene, this **does** fold wasla (U+0671) along with the other
  three. `ProfileCAMeL` sets `FoldAlefWasla: true`.
- [`normalize_alef_maksura_ar`][camel-normalize] — `s.replace('ى', 'ي')`.
- [`normalize_teh_marbuta_ar`][camel-normalize] — `s.replace('ة', 'ه')`.
- [`dediac_ar`][camel-dediac], via `AR_DIAC_CHARSET` in
  [`charsets.py`][camel-charsets] —
  `frozenset('ًٌٍَُِّْٰ')`:
  Lucene's eight harakat **plus** U+0670 (superscript alef). `ProfileCAMeL`
  uses `TashkilCAMeLDiac`.

Notably:

- CAMeL Tools' normalize module has **no public helper for tatweel**, so
  `ProfileCAMeL` sets `StripTatweel: false` — it is the only preset that
  leaves tatweel untouched.
- It has no helper for hamza-seated waw/yeh either — `FoldHamzaSeat: false`,
  same as Lucene.

**Known, documented divergence:** CAMeL Tools' own
[`normalize_unicode`][camel-normalize] composes (NFKC by default) rather
than decomposes, so it leaves precomposed non-Arabic characters like `café`
untouched (and would even re-compose an already-decomposed one). This
engine's NFD-first pass can decompose them instead, the same divergence
noted for `ProfileLucene` above and for the same reason. Again, the
fidelity claim covers Arabic-script normalization only.

[lucene-src]: https://github.com/apache/lucene/blob/main/lucene/analysis/common/src/java/org/apache/lucene/analysis/ar/ArabicNormalizer.java
[camel-tools]: https://github.com/CAMeL-Lab/camel_tools
[camel-normalize]: https://github.com/CAMeL-Lab/camel_tools/blob/master/camel_tools/utils/normalize.py
[camel-dediac]: https://github.com/CAMeL-Lab/camel_tools/blob/master/camel_tools/utils/dediac.py
[camel-charsets]: https://github.com/CAMeL-Lab/camel_tools/blob/master/camel_tools/utils/charsets.py
