# Normalization

`arabic.Profile` controls which rules `Normalize` applies. Every profile
first expands Arabic presentation forms if `FoldPresentation` is on, then
decomposes the input with Unicode NFD, then applies the remaining rules
below in a single pass. This document is the source of truth for what each
rule does, which codepoints it touches, and which of the five built-in
presets enable it. It exists so a claim like "matches Apache Lucene's
ArabicNormalizer" is checkable against something more precise than the
code.

## Rules

| Rule | Field | Codepoints | What it does |
| --- | --- | --- | --- |
| Strip tashkil | `StripTashkil` + `TashkilScope` | see [Tashkil scopes](#tashkil-scopes) below | Removes combining diacritics, scoped by `TashkilScope`. |
| Strip tatweel | `StripTatweel` | U+0640 ARABIC TATWEEL | Removes the elongation character. |
| Fold alef-hamza | `FoldAlefHamza` | U+0622 (madda), U+0623 (hamza above), U+0625 (hamza below) → U+0627 (bare alef) | Folds three of the four hamza-seated/madda alef letters to bare alef. Does **not** include wasla; see next row. |
| Fold alef-wasla | `FoldAlefWasla` | U+0671 (wasla) → U+0627 (bare alef) | Kept separate from the rule above because Lucene folds the hamza group but not wasla, while CAMeL Tools folds all four together. |
| Fold hamza-seat | `FoldHamzaSeat` | U+0624 (waw+hamza) → U+0648 (waw); U+0626 (yeh+hamza) → U+064A (yeh) | Folds the two hamza-seated letters that aren't alef-based. |
| Fold ta-marbuta | `FoldTaMarbuta` | U+0629 → U+0647 | Ta-marbuta to heh. |
| Fold alef-maksura | `FoldAlefMaksura` | U+0649 → U+064A | Alef-maksura to yeh. |
| Fold presentation forms | `FoldPresentation` | U+FB50–U+FDFF, U+FE70–U+FEFF (731 codepoints) | See [Presentation forms](#presentation-forms) below. |
| Strip joiners | `StripJoiners` | U+200C (ZWNJ), U+200D (ZWJ) | Removes the two zero-width joining-control characters. |
| Strip bidi marks | `StripBidi` | U+061C, U+200E–U+200F, U+202A–U+202E, U+2066–U+2069 (12 codepoints) | Removes the Unicode `Bidi_Control` codepoints; see [Bidi and joiner marks](#bidi-and-joiner-marks). |
| Fold digits | `FoldDigits` | U+0660–U+0669 (Arabic-Indic), U+06F0–U+06F9 (Extended Arabic-Indic) | Folds both digit ranges to ASCII `0`–`9`. |
| Fold punctuation | `FoldPunctuation` | U+060C, U+061B, U+061F, U+066A, U+066B, U+066C, U+06D4 | Folds seven Arabic punctuation marks to their ASCII equivalents; see [Punctuation](#punctuation). |
| Strip Quranic marks | `StripQuranic` | U+06D6–U+06ED (24 codepoints) | Removes the Quranic annotation/recitation-mark block; see [Quranic annotation marks](#quranic-annotation-marks). |

### Why hamza folding needs NFD-aware handling, not a simple table

U+0622/0623/0624/0625/0626 all have a canonical NFD decomposition into a
base letter (ا, و, or ي) followed by a standalone combining mark: U+0653
(MADDA ABOVE), U+0654 (HAMZA ABOVE), or U+0655 (HAMZA BELOW). Since
`Normalize` decomposes with NFD first, it never actually sees those five
precomposed codepoints directly; it sees the base letter and a trailing
mark. `FoldAlefHamza`/`FoldHamzaSeat` decide whether to drop that mark
(folding the letter) or keep it (preserving the distinction) based on which
base letter it follows; `StripTashkil`/`TashkilScope` never overrides that
decision, in either direction.

This is more than a bookkeeping detail. Unicode's canonical ordering
algorithm sorts every harakat mark (combining class 27–35) *before* a
hamza/madda mark (230, or 220 for hamza-below) that shares the same base
letter, so an ordinary vocalized spelling like أُمّ ("mother") or إِلَى
("to") decomposes as *base, vowel mark, hamza mark*, not *base, hamza
mark, vowel mark*. A profile that folds hamza has to still recognize the
hamza mark once it finally arrives, and a profile that keeps hamza distinct
has to not let `StripTashkil` strip the intervening vowel mark's neighbor by
mistake. `arabic.Normalize`'s implementation keeps the base-letter context
alive across a run of combining marks (and across a dropped tatweel, which
can merge two marks that used to be in separate canonical-ordering runs)
specifically so this resolves correctly and stays idempotent:
`Normalize(Normalize(x)) == Normalize(x)` for any `Profile`, fuzz-tested
across the full flag space in `arabic/profile_test.go`.

One consequence: a profile that keeps a hamza distinction (e.g.
`ProfileStrict`, or `--keep-hamza`) does not guarantee its normalized key is
byte-identical to the original spelling: a kept hamza mark stays in its
NFD-decomposed form (base letter + standalone combining mark) rather than
being recomposed to the original precomposed codepoint. It only guarantees
the key stays *distinguishable* from a differently-spelled key.

### Tashkil scopes

`TashkilScope` selects which marks `StripTashkil` removes (it has no effect
on the hamza/madda marks in their recognized context, per above):

| Scope | Codepoints removed |
| --- | --- |
| `TashkilAllMn` (default) | Every Unicode non-spacing mark (`Mn`): agrep's original behavior. Includes Quranic annotation marks, superscript alef, and anything else category `Mn`. |
| `TashkilLuceneHarakat` | Exactly U+064B–U+0652 (fathatan, dammatan, kasratan, fatha, damma, kasra, shadda, sukun): the eight codepoints Lucene's `ArabicNormalizer` treats as harakat. Nothing else. |
| `TashkilCAMeLDiac` | The same eight, plus U+0670 (superscript alef): CAMeL Tools' `AR_DIAC_CHARSET`. |

### Presentation forms

Arabic Presentation Forms-A (U+FB50–U+FDFF) and Forms-B (U+FE70–U+FEFF)
encode the same letters as the main Arabic block, but as glyph *shapes*:
contextual letter forms (initial/medial/final/isolated), ligatures, and a
few standalone-diacritic display forms. Unicode gives each of these a
*compatibility* decomposition to the plain letter sequence it stands for,
but NFD, which every other rule in this document runs on top of, only
follows *canonical* decompositions and ignores compatibility ones
entirely. Left alone, presentation forms (which is how most PDF text
layers and some legacy word-processor exports encode Arabic) survive
normalization completely unfolded, so a query typed with ordinary letters
never matches them. This was the defect flagged from this project's very
first plan.

`FoldPresentation` fixes this with a table generated from the Unicode
Character Database: `arabic/gen/main.go` reads `arabic/gen/UnicodeData.txt`
(vendored, Unicode 18.0.0; see `arabic/gen/README.md`) and keeps every
entry in the two presentation-form blocks whose decomposition field carries
one of the four positional tags `<isolated>`, `<initial>`, `<medial>`,
`<final>`. That is 731 of the 797 codepoints in the two blocks; the other
66 (dot/ring diacritic symbols, and the "Jalla wa-Alaa"-style short
religious ligatures) have no decomposition in the UCD at all, so there is
nothing to expand them to. The result is `arabic/tables.go`.

This is deliberately narrower than a full NFKC/compatibility
decomposition: it picks up every contextual letter form, ligature, and
presentation form of a standalone diacritic in those two blocks, including
ﻻ LAM WITH ALEF (U+FEFB → لا), ﷲ ALLAH (U+FDF2 → الله), ﷺ SALLALLAHOU
ALAYHE WASALLAM (U+FDFA → صلى الله عليه وسلم), ﷻ JALLAJALALOUHOU (U+FDFB →
جل جلاله), and the FE70–FE7F isolated/medial harakat display forms (e.g.
U+FE76 ARABIC FATHA ISOLATED FORM → a space followed by fatha, exactly as
the UCD defines it), while leaving out everything else NFKC would also
fold that has nothing to do with Arabic presentation forms: superscripts,
full-width Latin, roman numerals, and so on.

One deliberate, documented omission: **U+FDFD ARABIC LIGATURE BISMILLAH
AR-RAHMAN AR-RAHEEM has no decomposition in the UCD** (it stands for a
whole phrase, not a letter sequence), so `FoldPresentation` leaves it
untouched. CAMeL Tools special-cases this one codepoint with a hardcoded
phrase substitution (`'﷽': 'بسم الله الرحمن الرحيم'` in its
`normalize.py`); this implementation deliberately doesn't replicate that,
to keep the table purely UCD-generated and auditable: a design choice, not
an oversight.

Unlike every other fold in this document, `FoldPresentation` runs even
under `ProfileStrict`. It resolves an alternate *encoding* of the same
text, not a linguistic merge of distinct spellings: a final-form beh
(U+FE92) is the same letter as an isolated-form beh (U+FE8F), just a
different glyph selection for rendering context, so expanding it loses no
orthographic distinction a strict reading would want to keep, the same
reasoning that already makes the unconditional NFD step apply regardless
of profile.

### Bidi and joiner marks

`StripBidi` removes the twelve codepoints Unicode's `PropList.txt` marks
`Bidi_Control`: U+061C (ARABIC LETTER MARK), U+200E–U+200F (LRM, RLM),
U+202A–U+202E (LRE, RLE, PDF, LRO, RLO), and U+2066–U+2069 (LRI, RLI, FSI,
PDI). `StripJoiners` removes U+200C (ZWNJ) and U+200D (ZWJ). Both are
invisible formatting artifacts rather than textual content, common in text
copied from right-to-left web pages. Like presentation forms, both are on
by default even under `ProfileStrict`.

### Punctuation

`FoldPunctuation` maps seven Arabic punctuation marks to their ASCII
equivalents:

| Codepoint | Name | ASCII |
| --- | --- | --- |
| U+060C | ARABIC COMMA | `,` |
| U+061B | ARABIC SEMICOLON | `;` |
| U+061F | ARABIC QUESTION MARK | `?` |
| U+066A | ARABIC PERCENT SIGN | `%` |
| U+066B | ARABIC DECIMAL SEPARATOR | `.` |
| U+066C | ARABIC THOUSANDS SEPARATOR | `,` |
| U+06D4 | ARABIC FULL STOP | `.` |

Unlike the rules above, this one is a visible, opinionated transformation
of what the text displays as, so, matching the "Opt-in" annotation this
project's plan gave it from the start, it is off by default even under
`ProfileSearch`; only `ProfileLoose` enables it.

### Quranic annotation marks

`StripQuranic` removes U+06D6–U+06ED, a block of 24 Quranic recitation and
annotation marks. The range is not uniform in Unicode general category: 19
of the 24 are combining marks (`Mn`), but U+06DD (ARABIC END OF AYAH) is a
format character (`Cf`), U+06DE (ARABIC START OF RUB EL HIZB) and U+06E9
(ARABIC PLACE OF SAJDAH) are symbols (`So`), and U+06E5 (ARABIC SMALL WAW)
and U+06E6 (ARABIC SMALL YEH) are modifier letters (`Lm`). A generic
`TashkilScope`/Mn-based strip would never catch those five, which is why
`StripQuranic` is its own category-agnostic range check rather than a
fourth `TashkilScope` value.

## Presets

| Preset | Strip tashkil | Tashkil scope | Strip tatweel | Fold alef-hamza | Fold wasla | Fold hamza-seat | Fold ta-marbuta | Fold alef-maksura |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `ProfileSearch` | ✓ | AllMn | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ProfileStrict` | ✓ | AllMn | ✓ | - | - | - | - | - |
| `ProfileLoose` | ✓ | AllMn | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ProfileLucene` | ✓ | LuceneHarakat | ✓ | ✓ | - | - | ✓ | ✓ |
| `ProfileCAMeL` | ✓ | CAMeLDiac | - | ✓ | ✓ | - | ✓ | ✓ |

| Preset | Fold presentation | Strip joiners | Strip bidi | Strip Quranic | Fold digits | Fold punctuation |
| --- | --- | --- | --- | --- | --- | --- |
| `ProfileSearch` | ✓ | ✓ | ✓ | ✓ | - | - |
| `ProfileStrict` | ✓ | ✓ | ✓ | ✓ | - | - |
| `ProfileLoose` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ProfileLucene` | - | - | - | - | - | - |
| `ProfileCAMeL` | - | - | - | - | - | - |

- **`ProfileSearch`** is agrep's original, default, fixed behavior. As of
  Phase 4 it also expands presentation forms and strips joiners, bidi
  marks, and Quranic annotation marks. That is a deliberate, documented,
  tested exception to "never silently change": this is the
  presentation-form defect flagged from this project's very first plan,
  fixed explicitly rather than left in place to preserve exact
  byte-compatibility of a pre-1.0, unreleased tool. See plan.md's Phase 4
  completion notes.
- **`ProfileStrict`** strips only cosmetic/encoding-level marks (tashkil,
  tatweel, presentation forms, joiners, bidi marks, Quranic marks) and
  preserves every letter-level orthographic distinction. See the "not
  byte-identical" note above for what "preserves" actually guarantees, and
  the [Presentation forms](#presentation-forms) section above for why
  presentation-form expansion counts as encoding-level rather than
  orthographic.
- **`ProfileLoose`** enables every fold this build defines. As of Phase 4,
  digit and punctuation folding are what actually distinguish it from
  `ProfileSearch` (the two rules plan.md's Phase 4 table still marks
  "Opt-in"), since everything else the two profiles now share. Phase
  5/7/9's Rasm and language folding will add further divergence.
- **`ProfileLucene`** and **`ProfileCAMeL`** leave every Phase 4 field off:
  neither real library does presentation-form expansion, joiner/bidi
  stripping, digit/punctuation folding, or Quranic-mark stripping, so
  turning any of them on would break the fidelity claim these two profiles
  exist to make. Described in detail below.

## `ProfileLucene`

Reproduces [Apache Lucene's `ArabicNormalizer`][lucene-src] (Apache
License 2.0), read directly from source:

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

- It does **not** fold wasla (U+0671), only the madda/hamza-above/hamza-below
  group. `ProfileLucene` sets `FoldAlefWasla: false` to match.
- It does **not** fold hamza-seated waw/yeh (ؤ/ئ) at all: there's no case
  for either. `ProfileLucene` sets `FoldHamzaSeat: false`.
- It strips exactly the eight listed harakat codepoints, not every Unicode
  `Mn` mark: no Quranic annotation marks, no superscript alef.
  `ProfileLucene` uses `TashkilLuceneHarakat`.

**Known, documented divergence:** real Lucene never decomposes anything;
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

Reproduces the normalization [CAMeL Tools][camel-tools] (MIT License) users
compose from several of its functions, read directly from source:

- [`normalize_alef_ar`][camel-normalize]: `re.sub(r'[إأٱآ]', 'ا', s)`.
  Unlike Lucene, this **does** fold wasla (U+0671) along with the other
  three. `ProfileCAMeL` sets `FoldAlefWasla: true`.
- [`normalize_alef_maksura_ar`][camel-normalize]: `s.replace('ى', 'ي')`.
- [`normalize_teh_marbuta_ar`][camel-normalize]: `s.replace('ة', 'ه')`.
- [`dediac_ar`][camel-dediac], via `AR_DIAC_CHARSET` in
  [`charsets.py`][camel-charsets], `frozenset('ًٌٍَُِّْٰ')`:
  Lucene's eight harakat **plus** U+0670 (superscript alef). `ProfileCAMeL`
  uses `TashkilCAMeLDiac`.

Notably:

- CAMeL Tools' normalize module has **no public helper for tatweel**, so
  `ProfileCAMeL` sets `StripTatweel: false`. It is the only preset that
  leaves tatweel untouched.
- It has no helper for hamza-seated waw/yeh either: `FoldHamzaSeat: false`,
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
