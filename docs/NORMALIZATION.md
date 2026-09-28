# Normalization reference

`agrep` expands enabled Arabic presentation forms, applies selected language
substitutions, decomposes with Unicode NFD, and applies profile rules. It
compares the resulting keys while displaying the original text. Use
`--profile` for a preset and `--keep-*` or `--fold-*` options for overrides;
[CLI.md](CLI.md) lists every option.

## Profiles

| Profile | Use | Main differences |
| --- | --- | --- |
| `search` (default) | General Arabic text | Strips tashkil and tatweel; folds alef/hamza, ta-marbuta, alef-maksura, presentation forms, joiners, bidi controls, and Quranic marks. |
| `strict` | Keep letter distinctions | Strips cosmetic marks and expands presentation forms; keeps hamza, ta-marbuta, and alef-maksura distinct. |
| `loose` | Maximum recall | `search` plus digit, punctuation, and dotless rasm folds. |
| `lucene` | ArabicNormalizer-compatible search | Eight harakat marks; no wasla, hamza-seat, presentation, joiner, bidi, or Quranic fold. |
| `camel` | CAMeL Arabic helper-compatible search | Eight harakat plus superscript alef; keeps tatweel and hamza-seat. |

All presets default to `--lang=ar`. `--lang` and `--rasm` can extend any preset;
Lucene/CAMeL compatibility claims apply only to their unextended Arabic
settings. The exact fields are below (`✓` enabled, `—` disabled).

| Profile | Tashkil scope | Tatweel | Alef hamza | Wasla | Hamza seat | ة→ه | ى→ي |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `search` | all Mn | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `strict` | all Mn | ✓ | — | — | — | — | — |
| `loose` | all Mn | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `lucene` | eight harakat | ✓ | ✓ | — | — | ✓ | ✓ |
| `camel` | harakat + U+0670 | — | ✓ | ✓ | — | ✓ | ✓ |

| Profile | Presentation | Joiners | Bidi | Quranic | Digits | Punctuation | Rasm |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `search` | ✓ | ✓ | ✓ | ✓ | — | — | — |
| `strict` | ✓ | ✓ | ✓ | ✓ | — | — | — |
| `loose` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `lucene` | — | — | — | — | — | — | — |
| `camel` | — | — | — | — | — | — | — |

## Rules and codepoints

| Rule / Go field | Input → key or removal |
| --- | --- |
| `StripTashkil`, `TashkilScope` | All non-spacing marks (`Mn`) by default; Lucene scope removes U+064B–U+0652; CAMeL scope adds U+0670. Recognized hamza/madda marks follow the hamza flags instead. |
| `StripTatweel` | Remove U+0640. |
| `FoldAlefHamza` | آ, أ, إ → ا (U+0622/0623/0625 → U+0627). |
| `FoldAlefWasla` | ٱ → ا (U+0671 → U+0627). |
| `FoldHamzaSeat` | ؤ → و; ئ → ي (U+0624/0626 → U+0648/064A). |
| `FoldTaMarbuta` | ة → ه (U+0629 → U+0647). |
| `FoldAlefMaksura` | ى → ي (U+0649 → U+064A). |
| `FoldPresentation` | Contextual forms and ligatures in U+FB50–U+FDFF and U+FE70–U+FEFF expand to ordinary Arabic sequences. |
| `StripJoiners` | Remove ZWNJ U+200C and ZWJ U+200D; ZWNJ stays under `fa`, `ku`, or `ug`. |
| `StripBidi` | Remove U+061C, U+200E–U+200F, U+202A–U+202E, U+2066–U+2069. |
| `StripQuranic` | Remove U+06D6–U+06ED, including non-combining symbols that Mn stripping misses. |
| `FoldDigits` | U+0660–U+0669 and U+06F0–U+06F9 → ASCII `0`–`9`. |
| `FoldPunctuation` | See table below. |
| `Languages`, `Rasm` | See [languages](LANGUAGES.md) and [matching](MATCHING.md). |

| Arabic punctuation | ASCII |
| --- | --- |
| ، U+060C; ٬ U+066C | `,` |
| ؛ U+061B | `;` |
| ؟ U+061F | `?` |
| ٪ U+066A | `%` |
| ٫ U+066B; ۔ U+06D4 | `.` |

**Hamza and NFD.** NFD decomposes آ/أ/إ/ؤ/ئ into a base letter plus a
combining mark. The hamza flags decide whether that mark survives in its
recognized base-letter context, even when tashkil stripping is enabled.
Canonical ordering can place vowel marks between the base and hamza mark, so
the implementation carries the base context across combining marks. A strict
key may therefore differ byte-for-byte from its input while preserving the
spelling distinction. Normalization is idempotent across profiles.

**Presentation forms.** NFD alone does not expand compatibility forms. The
Unicode 18.0 generated table covers 731 presentation-form codepoints with
positional decompositions; examples include ﻻ → لا and ﷲ → الله. It leaves
U+FDFD ﷽ unchanged because Unicode gives it no decomposition. This is an
Arabic-only expansion, not general NFKC. It also runs under `strict` because
contextual glyph shapes are alternate encodings of the same letters. See the
[generator notes](../arabic/gen/README.md).

**Quranic marks.** U+06D6–U+06ED includes combining marks, format characters,
symbols, and modifier letters; stripping Mn alone cannot remove the whole
range.

## Compatibility scope

`lucene` follows [Lucene ArabicNormalizer][lucene-src]: alef hamza, ta-marbuta,
alef-maksura, tatweel, and exactly eight harakat. It leaves wasla, hamza-seat,
and superscript alef alone. `camel` follows the combination of CAMeL Tools'
[`normalize_alef_ar`, `normalize_alef_maksura_ar`, and
`normalize_teh_marbuta_ar`][camel-normalize] with [`dediac_ar`][camel-dediac]
and its [character set][camel-charsets]. It also folds wasla and removes
superscript alef, while leaving tatweel untouched.

These presets describe **Arabic-script** behavior. agrep always uses NFD;
Lucene does not, and CAMeL's separate `normalize_unicode` uses NFKC. They can
therefore differ on non-Arabic precomposed text such as `café`.

## Original-text spans

`Profile.NormalizeMapped` returns the same key as `Normalize` plus source byte
boundaries. `arabic.MapSpan` maps a match back to decoded original text,
including stripped marks and complete ligatures. For example, a match for `ل`
inside `ﻻ` covers the whole original ligature. The CLI builds this mapping
for `-o`, JSON spans, color, and `-w`.

[lucene-src]: https://github.com/apache/lucene/blob/main/lucene/analysis/common/src/java/org/apache/lucene/analysis/ar/ArabicNormalizer.java
[camel-normalize]: https://github.com/CAMeL-Lab/camel_tools/blob/master/camel_tools/utils/normalize.py
[camel-dediac]: https://github.com/CAMeL-Lab/camel_tools/blob/master/camel_tools/utils/dediac.py
[camel-charsets]: https://github.com/CAMeL-Lab/camel_tools/blob/master/camel_tools/utils/charsets.py
