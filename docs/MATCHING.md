# Rasm and fuzzy matching

Phase 9 adds two independent high-recall search tools. `--rasm` changes the
normalization key, while `--fuzzy[=N]` changes how normalized keys are compared.
They can be used separately or together.

## Dotless rasm

`--rasm` removes i'jam distinctions from the Arabic consonants listed below.

| Letters | Skeleton |
| --- | --- |
| ب ت ث ن ي | ٮ |
| ج ح خ | ح |
| د ذ | د |
| ر ز | ر |
| س ش | س |
| ص ض | ص |
| ط ظ | ط |
| ع غ | ع |
| ف ق | ٯ |

```sh
agrep --rasm "بنت" ocr.txt
agrep --rasm -C 2 "مستشرق" manuscript.txt
```

Rasm is deliberately broad and can create many false positives. Use context,
restrict the input corpus, or combine it with a longer query so results can be
reviewed. `ProfileLoose` enables rasm because that preset promises every general
fold. The default `search` profile does not; `--rasm` enables it explicitly on
any profile.

The transform runs after language canonicalization. Shared Yeh forms selected
by a mixed language set can therefore reach the Arabic Yeh skeleton. Distinct
Urdu retroflexes, Pashto letters, Kurdish letters, and Uyghur vowels are not in
the rasm table and remain distinct, following the Phase 8 language contract.
Yeh with Hamza loses its hamza under rasm because its Yeh base becomes the
shared dotless skeleton; this also keeps normalization idempotent.

With `--regex`, the regular expression is still evaluated directly over the
normalized key and is not rewritten. A rasm regular expression must therefore
contain the skeleton characters it intends to match.

## Fuzzy matching

`--fuzzy` permits one edit. Use `--fuzzy=N` for an explicit non-negative
distance, including `--fuzzy=0` for exact normalized matching.

```sh
agrep --fuzzy "كتاب" OCR.txt
agrep --fuzzy=2 -w "المستشرقون" corpus.txt
agrep --rasm --fuzzy=1 "مخطوط" scans.txt
```

Distance is unit-cost Levenshtein distance over normalized Unicode codepoints,
not UTF-8 bytes. An insertion, deletion, or substitution costs one. Profiles,
language selection, rasm, transliterated input, and Unicode case folding all run
before or as part of fuzzy comparison. `--regex` and `--fuzzy` are mutually
exclusive because one consumes regular-expression syntax and the other consumes
literal query keys.

The detector implements the bit-vector algorithm from Gene Myers,
[A Fast Bit-Vector Algorithm for Approximate String Matching Based on Dynamic
Programming](https://doi.org/10.1145/316542.316550). Patterns up to 64
codepoints use one machine word. Longer patterns use the same recurrence with
arbitrary-length bit vectors rather than falling back to a per-line dynamic
programming matrix. Tests compare detected endpoints with the defining
Levenshtein recurrence for both paths.

## Span policy

Fuzzy output modes need one original-text interval even when the two sides have
different lengths. agrep uses this policy:

- Every alignment must cover at least one text codepoint. A query deleted in
  its entirety never creates a zero-length output match.
- Inserted text codepoints are included in the span.
- A deleted query codepoint contributes no source bytes; the span covers the
  surviving text side of the alignment.
- At one endpoint, the lowest-distance alignment wins. A tie chooses the
  longest text interval, which is the earliest possible start and includes all
  equally valid inserted context.
- Across overlapping candidates, lower edit distance wins first, followed by
  earlier text position and then the longer interval. Returned matches are
  non-overlapping and sorted in text order.

These are normalized-key intervals. The existing `NormalizeMapped` index then
maps them into decoded UTF-8 `Match.Text`, and `--translit-out` applies its
second mapping afterward. JSON spans, `-o`, color, and `-w` therefore retain
the same coordinate contract as literal and regular-expression matching.

The default distance is fixed at one rather than silently scaling with query
length. An explicit threshold is reproducible across corpora, and callers can
choose a larger `N` when the expected OCR error rate justifies it.
